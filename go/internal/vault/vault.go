// Package vault: USB'de (ya da herhangi bir klasörde) duran, ana parolayla şifrelenmiş küçük sır kasası (ör. SSH anahtar passphrase'leri).
//
// Güvenlik modeli:
//   - Anahtar YALNIZ ana paroladan türetilir (PBKDF2-HMAC-SHA256, 600k tur, rastgele 16 bayt tuz); anahtar ya da parola diske yazılmaz.
//   - Şifreleme AES-256-GCM; başlık (sürüm, tur sayısı, tuz) kimlik doğrulamasına dahildir (AAD): başlığı kurcalayan dosya açılmaz.
//   - Her kayıtta yeni rastgele nonce. Düz metin 1 KiB katlarına boşlukla doldurulur (kayıt sayısı/uzunluğu dosya boyutundan sızmasın).
//   - Yanlış parola ile bozuk/kurcalanmış dosya birbirinden ayırt EDİLEMEZ (ikisi de ErrWrongPassword): GCM doğrulaması başarısız olur.
//   - Dosya 0600, yazma atomik (aynı klasörde geçici dosya + rename). Kilitlenince bellekteki anahtar ve kayıtlar sıfırlanır (Go'da en iyi çaba).
//
// Bu paket yalnız saklama/şifrelemedir: parola sorma, yanlış-parola gecikmesi ve ajana yükleme çağıranın işidir.
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	magic     = "KLVAULT1"
	saltLen   = 16
	nonceLen  = 12
	keyLen    = 32
	padBlock  = 1024
	maxFile   = 4 << 20 // 4 MiB: kasa küçük bir dosyadır; daha büyüğü bozuk/kötü niyetli sayılır
	maxIterOK = 20_000_000
)

// Varsayılan ve alt sınır tur sayısı (testte düşürülebilir). minIter altındaki dosya reddedilir (zayıflatılmış kasa).
var (
	iterations = 600_000
	minIter    = 100_000
)

var (
	// ErrWrongPassword: parola yanlış ya da dosya bozuk/kurcalanmış (ayırt edilemez).
	ErrWrongPassword = errors.New("ana parola yanlış ya da kasa dosyası bozuk")
	ErrExists        = errors.New("kasa zaten var")
	ErrNotFound      = errors.New("kasa dosyası yok")
	ErrEmptyPassword = errors.New("ana parola boş olamaz")
	ErrLocked        = errors.New("kasa kilitli")
)

// Vault: açılmış (kilidi çözülmüş) kasa. Eşzamanlı kullanıma güvenlidir.
type Vault struct {
	mu      sync.Mutex
	path    string
	key     []byte
	salt    []byte
	iter    int
	entries map[string]string
}

type payload struct {
	Entries map[string]string `json:"entries"`
}

// Exists: kasa dosyası var mı.
func Exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// Create: yeni kasa oluşturur (varsa ErrExists; üzerine yazmaz).
func Create(path, password string) (*Vault, error) {
	if password == "" {
		return nil, ErrEmptyPassword
	}
	if Exists(path) {
		return nil, ErrExists
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, err := derive(password, salt, iterations)
	if err != nil {
		return nil, err
	}
	v := &Vault{path: path, key: key, salt: salt, iter: iterations, entries: map[string]string{}}
	if err := v.saveLocked(); err != nil {
		v.zero()
		return nil, err
	}
	return v, nil
}

// Open: kasayı açar. Yanlış parola ya da bozuk dosya: ErrWrongPassword.
func Open(path, password string) (*Vault, error) {
	b, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	iter, salt, nonce, ct, aad, err := parse(b)
	if err != nil {
		return nil, err
	}
	key, err := derive(password, salt, iter)
	if err != nil {
		return nil, err
	}
	pt, err := open(key, nonce, ct, aad)
	if err != nil {
		wipe(key)
		return nil, ErrWrongPassword
	}
	var p payload
	if err := json.Unmarshal(pt, &p); err != nil {
		wipe(key)
		return nil, ErrWrongPassword
	}
	wipe(pt)
	if p.Entries == nil {
		p.Entries = map[string]string{}
	}
	return &Vault{path: path, key: key, salt: salt, iter: iter, entries: p.Entries}, nil
}

// Get: kaydı verir.
func (v *Vault) Get(name string) (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return "", false
	}
	s, ok := v.entries[name]
	return s, ok
}

// Names: kayıt adları (sıralı; değerler değil).
func (v *Vault) Names() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]string, 0, len(v.entries))
	for k := range v.entries {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Set: kaydı yazar/günceller ve kasayı diske kaydeder (yeni nonce).
func (v *Vault) Set(name, value string) error {
	if name == "" {
		return errors.New("kayıt adı boş olamaz")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	old, had := v.entries[name]
	v.entries[name] = value
	if err := v.saveLocked(); err != nil {
		if had {
			v.entries[name] = old
		} else {
			delete(v.entries, name)
		}
		return err
	}
	return nil
}

// Delete: kaydı siler ve kaydeder; yoksa false.
func (v *Vault) Delete(name string) (bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return false, ErrLocked
	}
	old, had := v.entries[name]
	if !had {
		return false, nil
	}
	delete(v.entries, name)
	if err := v.saveLocked(); err != nil {
		v.entries[name] = old
		return false, err
	}
	return true, nil
}

// ChangePassword: yeni parola + yeni tuzla yeniden şifreler (eski parola artık dosyayı açmaz).
func (v *Vault) ChangePassword(newPassword string) error {
	if newPassword == "" {
		return ErrEmptyPassword
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	key, err := derive(newPassword, salt, iterations)
	if err != nil {
		return err
	}
	oldKey, oldSalt, oldIter := v.key, v.salt, v.iter
	v.key, v.salt, v.iter = key, salt, iterations
	if err := v.saveLocked(); err != nil {
		v.key, v.salt, v.iter = oldKey, oldSalt, oldIter
		wipe(key)
		return err
	}
	wipe(oldKey)
	return nil
}

// Lock: bellekteki anahtarı ve kayıtları sıfırlar; sonraki işlemler ErrLocked verir.
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.zero()
}

func (v *Vault) zero() {
	wipe(v.key)
	v.key = nil
	for k := range v.entries {
		v.entries[k] = ""
		delete(v.entries, k)
	}
}

// --- biçim ---

// Dosya: magic(8) | iter(4, BE) | salt(16) | nonce(12) | şifreli metin(+16 GCM etiketi). Başlık (ilk 28 bayt) AAD'dir.
func parse(b []byte) (iter int, salt, nonce, ct, aad []byte, err error) {
	const hdr = len(magic) + 4 + saltLen
	if len(b) < hdr+nonceLen+16 || string(b[:len(magic)]) != magic {
		return 0, nil, nil, nil, nil, ErrWrongPassword
	}
	iter = int(binary.BigEndian.Uint32(b[len(magic) : len(magic)+4]))
	if iter < minIter || iter > maxIterOK {
		return 0, nil, nil, nil, nil, fmt.Errorf("kasa tur sayısı kabul edilemez: %d", iter)
	}
	salt = b[len(magic)+4 : hdr]
	nonce = b[hdr : hdr+nonceLen]
	ct = b[hdr+nonceLen:]
	aad = b[:hdr]
	return
}

func header(iter int, salt []byte) []byte {
	h := make([]byte, 0, len(magic)+4+saltLen)
	h = append(h, magic...)
	h = binary.BigEndian.AppendUint32(h, uint32(iter))
	return append(h, salt...)
}

func (v *Vault) saveLocked() error {
	pt, err := json.Marshal(payload{Entries: v.entries})
	if err != nil {
		return err
	}
	if rem := len(pt) % padBlock; rem != 0 || len(pt) == 0 {
		pt = append(pt, bytes.Repeat([]byte{' '}, padBlock-rem)...)
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	hdr := header(v.iter, v.salt)
	gcm, err := newGCM(v.key)
	if err != nil {
		return err
	}
	out := append(append([]byte{}, hdr...), nonce...)
	out = gcm.Seal(out, nonce, pt, hdr)
	wipe(pt)
	return writeAtomic(v.path, out)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blk)
}

func open(key, nonce, ct, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, aad)
}

func derive(password string, salt []byte, iter int) ([]byte, error) {
	return pbkdf2.Key(sha256.New, password, salt, iter, keyLen)
}

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func readLimited(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > maxFile {
		return nil, ErrWrongPassword
	}
	return os.ReadFile(path)
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".vault-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // rename başarılıysa zaten yok
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
