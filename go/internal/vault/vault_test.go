package vault

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	iterations, minIter = 2000, 1000 // testler hızlı olsun; sınır mantığı aynı
	os.Exit(m.Run())
}

func newVault(t *testing.T) (string, *Vault) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "state", "vault.bin")
	v, err := Create(p, "ana-parola")
	if err != nil {
		t.Fatal(err)
	}
	return p, v
}

func TestRoundTripAndPersistence(t *testing.T) {
	p, v := newVault(t)
	if err := v.Set("USBMonitor_WEB1_Passphrase", "gizli-passphrase"); err != nil {
		t.Fatal(err)
	}
	if err := v.Set("USBMonitor_DB_Passphrase", "başka"); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	w, err := Open(p, "ana-parola")
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := w.Get("USBMonitor_WEB1_Passphrase"); !ok || s != "gizli-passphrase" {
		t.Fatalf("Get: %q %v", s, ok)
	}
	if got := strings.Join(w.Names(), ","); got != "USBMonitor_DB_Passphrase,USBMonitor_WEB1_Passphrase" {
		t.Fatalf("Names: %s", got)
	}
	if ok, _ := w.Delete("USBMonitor_DB_Passphrase"); !ok {
		t.Fatal("Delete")
	}
	if ok, _ := w.Delete("yok"); ok {
		t.Fatal("olmayan silindi")
	}
	x, _ := Open(p, "ana-parola")
	if _, ok := x.Get("USBMonitor_DB_Passphrase"); ok {
		t.Fatal("silinen kayıt geri geldi")
	}
}

func TestWrongPasswordAndTamper(t *testing.T) {
	p, v := newVault(t)
	v.Set("a", "değer")
	v.Lock()
	if _, err := Open(p, "yanlış"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("yanlış parola: %v", err)
	}
	if _, err := Open(p, ""); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("boş parola: %v", err)
	}
	orig, _ := os.ReadFile(p)
	flip := func(i int) []byte { b := append([]byte{}, orig...); b[i] ^= 0x01; return b }
	hdr := len(magic) + 4 + saltLen
	for name, data := range map[string][]byte{
		"şifreli metin": flip(len(orig) - 20),
		"GCM etiketi":   flip(len(orig) - 1),
		"nonce":         flip(hdr + 1),
		"tuz":           flip(len(magic) + 4 + 3),
		"magic":         flip(0),
		"kesilmiş":      orig[:len(orig)-5],
		"boş":           {},
	} {
		os.WriteFile(p, data, 0o600)
		if _, err := Open(p, "ana-parola"); err == nil {
			t.Errorf("%s: kurcalanmış dosya açıldı", name)
		}
	}
	// başlıktaki tur sayısını kurcalamak (AAD ya da sınır) dosyayı açtırmaz
	b := append([]byte{}, orig...)
	binary.BigEndian.PutUint32(b[len(magic):], uint32(iterations+1))
	os.WriteFile(p, b, 0o600)
	if _, err := Open(p, "ana-parola"); err == nil {
		t.Error("tur sayısı kurcalanmış dosya açıldı (AAD çalışmıyor)")
	}
	// zayıflatılmış tur sayısı reddedilir
	b = append([]byte{}, orig...)
	binary.BigEndian.PutUint32(b[len(magic):], 1)
	os.WriteFile(p, b, 0o600)
	if _, err := Open(p, "ana-parola"); err == nil {
		t.Error("düşük tur sayısı kabul edildi")
	}
	os.WriteFile(p, orig, 0o600)
	if _, err := Open(p, "ana-parola"); err != nil {
		t.Fatalf("özgün dosya açılmıyor: %v", err)
	}
}

func TestFileHasNoPlaintextAndPaddedAndFreshNonce(t *testing.T) {
	p, v := newVault(t)
	secret := "çok-gizli-passphrase-123"
	v.Set("USBMonitor_WEB1_Passphrase", secret)
	b1, _ := os.ReadFile(p)
	for _, leak := range []string{secret, "USBMonitor_WEB1", "ana-parola", "entries"} {
		if bytes.Contains(b1, []byte(leak)) {
			t.Fatalf("dosyada düz metin var: %q", leak)
		}
	}
	hdr := len(magic) + 4 + saltLen
	if (len(b1)-hdr-nonceLen-16)%padBlock != 0 {
		t.Fatalf("düz metin %d katına doldurulmadı: %d", padBlock, len(b1))
	}
	v.Set("USBMonitor_WEB1_Passphrase", secret) // aynı içerik, yeni kayıt: nonce değişmeli
	b2, _ := os.ReadFile(p)
	if bytes.Equal(b1[hdr:hdr+nonceLen], b2[hdr:hdr+nonceLen]) {
		t.Fatal("nonce yeniden kullanıldı")
	}
	if bytes.Equal(b1, b2) {
		t.Fatal("aynı içerik iki kayıtta aynı çıktı")
	}
}

func TestPermissionsAtomicAndCreateRefusesOverwrite(t *testing.T) {
	p, v := newVault(t)
	v.Set("a", "b")
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("izin %v", st.Mode().Perm())
	}
	ents, _ := os.ReadDir(filepath.Dir(p))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".vault-") {
			t.Fatalf("geçici dosya kaldı: %s", e.Name())
		}
	}
	if _, err := Create(p, "başka"); !errors.Is(err, ErrExists) {
		t.Fatalf("üzerine yazıldı: %v", err)
	}
	if _, err := Create(filepath.Join(t.TempDir(), "x"), ""); !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("boş parola: %v", err)
	}
	if _, err := Open(filepath.Join(t.TempDir(), "yok"), "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("olmayan dosya: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	p, v := newVault(t)
	v.Set("a", "değer")
	before, _ := os.ReadFile(p)
	if err := v.ChangePassword("yeni-parola"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if bytes.Equal(before[len(magic)+4:len(magic)+4+saltLen], after[len(magic)+4:len(magic)+4+saltLen]) {
		t.Fatal("tuz yenilenmedi")
	}
	v.Lock()
	if _, err := Open(p, "ana-parola"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("eski parola hâlâ açıyor: %v", err)
	}
	w, err := Open(p, "yeni-parola")
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := w.Get("a"); s != "değer" {
		t.Fatalf("veri kayboldu: %q", s)
	}
	if err := w.ChangePassword(""); !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("boş yeni parola: %v", err)
	}
}

func TestLockZeroesAndBlocks(t *testing.T) {
	_, v := newVault(t)
	v.Set("a", "b")
	key := v.key
	v.Lock()
	for _, x := range key {
		if x != 0 {
			t.Fatal("anahtar sıfırlanmadı")
		}
	}
	if _, ok := v.Get("a"); ok {
		t.Fatal("kilitliyken okundu")
	}
	if err := v.Set("a", "c"); !errors.Is(err, ErrLocked) {
		t.Fatalf("kilitliyken yazıldı: %v", err)
	}
	if err := v.ChangePassword("x"); !errors.Is(err, ErrLocked) {
		t.Fatalf("kilitliyken parola değişti: %v", err)
	}
}

func TestFailedSaveRollsBack(t *testing.T) {
	p, v := newVault(t)
	v.Set("a", "1")
	v.path = filepath.Join(p, "altında-dosya", "vault.bin") // dosyanın altı klasör olamaz: kayıt başarısız
	if err := v.Set("b", "2"); err == nil {
		t.Fatal("yazma hatası beklenirdi")
	}
	if _, ok := v.Get("b"); ok {
		t.Fatal("başarısız kayıt bellekte kaldı")
	}
}

// Alt sınır: zayıf tur sayısıyla (doğru parolayla bile) yazılmış kasa reddedilir (sessizce zayıflatılmış kasa kullanılmasın).
func TestWeakIterationCountIsRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "weak.bin")
	old := iterations
	iterations = minIter / 2
	v, err := Create(p, "ana-parola")
	iterations = old
	if err != nil {
		t.Fatal(err)
	}
	v.Lock()
	if _, err := Open(p, "ana-parola"); err == nil || errors.Is(err, ErrWrongPassword) {
		t.Fatalf("zayıf tur sayılı kasa açıldı ya da yanlış nedenle reddedildi: %v", err)
	}
}

// Üst sınır: aşırı büyük tur sayısı türetmeye girmeden reddedilir (hizmet dışı bırakma).
func TestHugeIterationCountRejectedFast(t *testing.T) {
	p := filepath.Join(t.TempDir(), "huge.bin")
	b := append([]byte(magic), 0xFF, 0xFF, 0xFF, 0xFF)
	b = append(b, make([]byte, saltLen+nonceLen+64)...)
	os.WriteFile(p, b, 0o600)
	if _, err := Open(p, "x"); err == nil || errors.Is(err, ErrWrongPassword) {
		t.Fatalf("aşırı tur sayısı reddedilmedi: %v", err)
	}
}
