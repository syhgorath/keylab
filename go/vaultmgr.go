package main

import (
	"errors"
	"fmt"
	"keylab/internal/platform"
	"keylab/internal/servers"
	"keylab/internal/vault"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Şifreli passphrase kasasının uygulama tarafı (çekirdek: internal/vault). Yalnız misafir/taşınabilir modda etkindir (özel ssh-agent vardır).
//   - Açılışta/girişte ana parola kasayı açar; kasadaki passphrase'ler ÖZEL ajana yüklenir (ssh-add); eylemler anahtarı ajanda bulur, parola sorulmaz.
//   - Kilitlenince (elle ya da boşta) kasa belleği sıfırlanır ve özel ajandaki anahtarlar silinir (ssh-add -D). Çalışan görevler öldürülmez.
//   - Yanlış ana parola denemeleri artan bekleme ile yavaşlatılır (1, 2, 4 … en çok 30 sn).
//   - Kasa dosyası var ve kilitliyse, oturum açıkken bile tüm /api uçları 423 verir (kasa uçları ve çıkış hariç).

const (
	vaultFile       = "vault.bin"
	vaultMinPassLen = 8
	vaultMaxBackoff = 30 * time.Second
)

var (
	errVaultThrottled = errors.New("çok fazla yanlış deneme; biraz bekleyin")
	errVaultWeak      = fmt.Errorf("ana parola en az %d karakter olmalı", vaultMinPassLen)
	errVaultOff       = errors.New("kasa yalnızca taşınabilir (misafir) modda kullanılabilir")
	errVaultUnlocked  = errors.New("kasa zaten açık")
	errVaultSecret    = errors.New("passphrase satır sonu içeremez")
)

type vaultMgr struct {
	mu        sync.Mutex
	path      string
	v         *vault.Vault
	fails     int
	retryAt   time.Time
	lastTouch time.Time
	idleAfter time.Duration // 0: otomatik kilit yok

	serversDir string
	tmpDir     string
	enabled    func() bool
	now        func() time.Time
	addKey     func(keyPath, passphrase string) error // ssh-add ile özel ajana ekler (testte değiştirilir)
	clearAgent func()                                 // özel ajandaki anahtarları siler (testte değiştirilir)
}

func (a *App) vaultM() *vaultMgr {
	a.vmOnce.Do(func() {
		if a.vm == nil {
			a.vm = newVaultMgr(a)
		}
	})
	return a.vm
}

func newVaultMgr(a *App) *vaultMgr {
	m := &vaultMgr{
		path: filepath.Join(a.stateDir, vaultFile), serversDir: a.serversDir, tmpDir: os.TempDir(),
		enabled: platform.GuestMode, now: a.now, idleAfter: 15 * time.Minute,
	}
	if s := os.Getenv("KEYLAB_VAULT_IDLE_MIN"); s != "" { // 0: kapalı
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n >= 0 && n <= 24*60 {
			m.idleAfter = time.Duration(n) * time.Minute
		}
	}
	m.addKey = m.addKeyViaSSHAdd
	m.clearAgent = func() { _ = exec.Command("ssh-add", "-D").Run() }
	return m
}

type vaultStatus struct {
	Available  bool     `json:"available"`
	Exists     bool     `json:"exists"`
	Unlocked   bool     `json:"unlocked"`
	RetryAfter int      `json:"retryAfter"`
	Stored     []string `json:"stored,omitempty"` // passphrase'i kasada olan sunucular (yalnız açıkken; değerler asla)
	IdleMin    int      `json:"idleMin"`
}

func (m *vaultMgr) status() vaultStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := vaultStatus{Available: m.enabled(), IdleMin: int(m.idleAfter / time.Minute)}
	if !st.Available {
		return st
	}
	st.Exists = vault.Exists(m.path)
	st.Unlocked = m.v != nil
	if d := m.retryAt.Sub(m.now()); d > 0 {
		st.RetryAfter = int(d.Seconds()) + 1
	}
	if m.v != nil {
		for _, n := range servers.Names(m.serversDir) {
			if _, ok := m.v.Get(servers.PassphraseAccount(servers.Prefix(n))); ok {
				st.Stored = append(st.Stored, n)
			}
		}
	}
	return st
}

// locked: kasa dosyası var ama açık değil (arayüz perdelenir).
func (m *vaultMgr) locked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled() && m.v == nil && vault.Exists(m.path)
}

func (m *vaultMgr) touch() {
	m.mu.Lock()
	m.lastTouch = m.now()
	m.mu.Unlock()
}

// throttleLocked: bekleme sürüyorsa hata. m.mu tutulurken çağrılır.
func (m *vaultMgr) throttleLocked() error {
	if m.now().Before(m.retryAt) {
		return errVaultThrottled
	}
	return nil
}

func (m *vaultMgr) failLocked() {
	m.fails++
	d := time.Second << uint(min(m.fails-1, 5))
	if d > vaultMaxBackoff {
		d = vaultMaxBackoff
	}
	m.retryAt = m.now().Add(d)
}

// create: yeni kasa (açık olarak döner). Var olan kasanın üzerine yazılmaz.
func (m *vaultMgr) create(password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled() {
		return errVaultOff
	}
	if utf8.RuneCountInString(password) < vaultMinPassLen {
		return errVaultWeak
	}
	v, err := vault.Create(m.path, password)
	if err != nil {
		return err
	}
	m.v, m.lastTouch = v, m.now()
	return nil
}

// unlock: kasayı açar ve kayıtlı passphrase'leri özel ajana yükler. loaded/failed: sunucu adları.
func (m *vaultMgr) unlock(password string) (loaded, failed []string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled() {
		return nil, nil, errVaultOff
	}
	if m.v != nil {
		return nil, nil, errVaultUnlocked
	}
	if err := m.throttleLocked(); err != nil {
		return nil, nil, err
	}
	v, err := vault.Open(m.path, password)
	if err != nil {
		if errors.Is(err, vault.ErrWrongPassword) {
			m.failLocked()
		}
		return nil, nil, err
	}
	m.fails, m.retryAt, m.v, m.lastTouch = 0, time.Time{}, v, m.now()
	loaded, failed = m.loadKeysLocked()
	return loaded, failed, nil
}

func (m *vaultMgr) loadKeysLocked() (loaded, failed []string) {
	for _, n := range servers.Names(m.serversDir) {
		pass, ok := m.v.Get(servers.PassphraseAccount(servers.Prefix(n)))
		if !ok {
			continue
		}
		key := servers.KeyPath(m.serversDir, n)
		if st, err := os.Stat(key); err != nil || !st.Mode().IsRegular() {
			failed = append(failed, n)
			continue
		}
		if err := m.addKey(key, pass); err != nil {
			failed = append(failed, n)
		} else {
			loaded = append(loaded, n)
		}
	}
	return
}

// lock: belleği sıfırlar, özel ajandaki anahtarları siler. Açık değilse de ajanı temizler (güvenli taraf).
func (m *vaultMgr) lock() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lockLocked()
}

func (m *vaultMgr) lockLocked() {
	if m.v != nil {
		m.v.Lock()
		m.v = nil
	}
	if m.enabled() && m.clearAgent != nil {
		m.clearAgent()
	}
}

// idleTick: boşta kalma süresi dolduysa kilitler (periyodik çağrılır).
func (m *vaultMgr) idleTick() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.v == nil || m.idleAfter <= 0 || m.now().Sub(m.lastTouch) < m.idleAfter {
		return false
	}
	m.lockLocked()
	return true
}

// setPassphrase: passphrase'i ÖNCE özel ajana ekleyerek doğrular (yanlışsa kaydedilmez), sonra kasaya yazar.
func (m *vaultMgr) setPassphrase(server, pass string) error {
	if strings.ContainsAny(pass, "\r\n") || pass == "" {
		return errVaultSecret
	}
	if !servers.NameValid(server) {
		return errors.New("geçersiz sunucu adı")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.v == nil {
		return vault.ErrLocked
	}
	key := servers.KeyPath(m.serversDir, server)
	if st, err := os.Stat(key); err != nil || !st.Mode().IsRegular() {
		return errors.New("bu sunucunun anahtar dosyası yok")
	}
	if err := m.addKey(key, pass); err != nil {
		return errors.New("passphrase anahtarı açmıyor (yanlış olabilir); kaydedilmedi")
	}
	m.lastTouch = m.now()
	return m.v.Set(servers.PassphraseAccount(servers.Prefix(server)), pass)
}

func (m *vaultMgr) removePassphrase(server string) (bool, error) {
	if !servers.NameValid(server) {
		return false, errors.New("geçersiz sunucu adı")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.v == nil {
		return false, vault.ErrLocked
	}
	return m.v.Delete(servers.PassphraseAccount(servers.Prefix(server)))
}

// changePassword: eski ana parolayı doğrular (yanlışsa gecikme), sonra yeni tuz + yeni parolayla yeniden şifreler.
func (m *vaultMgr) changePassword(oldPw, newPw string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.v == nil {
		return vault.ErrLocked
	}
	if utf8.RuneCountInString(newPw) < vaultMinPassLen {
		return errVaultWeak
	}
	if err := m.throttleLocked(); err != nil {
		return err
	}
	chk, err := vault.Open(m.path, oldPw)
	if err != nil {
		if errors.Is(err, vault.ErrWrongPassword) {
			m.failLocked()
		}
		return err
	}
	chk.Lock()
	m.fails, m.retryAt = 0, time.Time{}
	return m.v.ChangePassword(newPw)
}

// addKeyViaSSHAdd: passphrase'i ssh-add'e SSH_ASKPASS ile verir. Parola komut satırına/ortama YAZILMAZ; 0600 geçici dosyadan okunup hemen silinir
// (askpass tek seferlik cevap verir: yanlış parolada ssh-add yeniden denemez).
func (m *vaultMgr) addKeyViaSSHAdd(keyPath, pass string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(m.tmpDir, "kl-ap-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.WriteString(pass + "\n"); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	cmd := exec.Command("ssh-add", keyPath)
	cmd.Env = append(os.Environ(), "SSH_ASKPASS="+exe, "SSH_ASKPASS_REQUIRE=force", "SSHPOCKET_ASKPASS=1", "SSHPOCKET_ASKPASS_FILE="+name)
	if os.Getenv("DISPLAY") == "" {
		cmd.Env = append(cmd.Env, "DISPLAY=:0")
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	return cmd.Run()
}

// vaultIdleLoop: boşta kilit denetimi (süreç boyunca; ticker 20 sn).
func (a *App) vaultIdleLoop(stop <-chan struct{}) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			a.vaultM().idleTick()
		case <-stop:
			return
		}
	}
}
