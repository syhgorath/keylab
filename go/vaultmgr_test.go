package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"keylab/internal/servers"
	"keylab/internal/vault"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type vaultFx struct {
	a       *App
	m       *vaultMgr
	clock   time.Time
	added   map[string]string // anahtar yolu → passphrase (ajana eklenenler)
	cleared int
	okPass  map[string]bool // addKey hangi passphrase'i kabul eder
}

func newVaultFx(t *testing.T) *vaultFx {
	t.Helper()
	dir := t.TempDir()
	srv := filepath.Join(dir, "servers")
	for _, n := range []string{"web1", "db1"} {
		d := filepath.Join(srv, n)
		os.MkdirAll(d, 0o700)
		os.WriteFile(filepath.Join(d, "server.env"), []byte("IP=10.0.0.1\nUSER=root\nPORT=22\n"), 0o600)
		os.WriteFile(filepath.Join(d, "id_ed25519_"+n), []byte("anahtar"), 0o600)
	}
	f := &vaultFx{clock: time.Unix(1_700_000_000, 0), added: map[string]string{}, okPass: map[string]bool{"p-web1": true, "p-db1": true}}
	a := &App{serversDir: srv, stateDir: filepath.Join(dir, "state"), jobs: map[string]*Job{}, up: map[string]bool{}}
	m := newVaultMgr(a)
	m.enabled = func() bool { return true }
	m.now = func() time.Time { return f.clock }
	m.idleAfter = 10 * time.Minute
	m.addKey = func(key, pass string) error {
		if !f.okPass[pass] {
			return errors.New("yanlış")
		}
		f.added[key] = pass
		return nil
	}
	m.clearAgent = func() { f.cleared++; f.added = map[string]string{} }
	a.vm = m
	f.a, f.m = a, m
	return f
}

func TestVaultCreateUnlockLoadsKeysIntoAgent(t *testing.T) {
	f := newVaultFx(t)
	if err := f.m.create("kısa"); !errors.Is(err, errVaultWeak) {
		t.Fatalf("zayıf parola: %v", err)
	}
	if err := f.m.create("ana-parola-1"); err != nil {
		t.Fatal(err)
	}
	if err := f.m.create("başka-parola"); !errors.Is(err, vault.ErrExists) {
		t.Fatalf("üzerine yazıldı: %v", err)
	}
	if err := f.m.setPassphrase("web1", "p-web1"); err != nil {
		t.Fatal(err)
	}
	if err := f.m.setPassphrase("db1", "p-db1"); err != nil {
		t.Fatal(err)
	}
	f.m.lock()
	if len(f.added) != 0 || f.cleared == 0 || !f.m.locked() {
		t.Fatalf("kilit ajanı temizlemedi: %v %d %v", f.added, f.cleared, f.m.locked())
	}
	loaded, failed, err := f.m.unlock("ana-parola-1")
	if err != nil || len(loaded) != 2 || len(failed) != 0 {
		t.Fatalf("unlock: %v %v %v", loaded, failed, err)
	}
	k := servers.KeyPath(f.a.serversDir, "web1")
	if f.added[k] != "p-web1" {
		t.Fatalf("anahtar ajana yüklenmedi: %v", f.added)
	}
	if st := f.m.status(); !st.Unlocked || len(st.Stored) != 2 {
		t.Fatalf("durum: %+v", st)
	}
}

func TestVaultBadPassphraseIsNotStoredAndBadKeyReported(t *testing.T) {
	f := newVaultFx(t)
	f.m.create("ana-parola-1")
	if err := f.m.setPassphrase("web1", "yanlis"); err == nil {
		t.Fatal("yanlış passphrase kaydedildi")
	}
	if _, ok := f.m.v.Get(servers.PassphraseAccount("WEB1")); ok {
		t.Fatal("doğrulanmayan passphrase kasaya yazıldı")
	}
	for _, bad := range []string{"", "a\nb", "a\r"} {
		if err := f.m.setPassphrase("web1", bad); !errors.Is(err, errVaultSecret) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	if err := f.m.setPassphrase("../x", "p-web1"); err == nil {
		t.Fatal("geçersiz sunucu adı kabul edildi")
	}
	f.m.setPassphrase("web1", "p-web1")
	f.okPass["p-web1"] = false // anahtar sonradan geçersiz: açılışta "failed" olarak bildirilir
	f.m.lock()
	loaded, failed, err := f.m.unlock("ana-parola-1")
	if err != nil || len(loaded) != 0 || len(failed) != 1 || failed[0] != "web1" {
		t.Fatalf("başarısız anahtar bildirilmedi: %v %v %v", loaded, failed, err)
	}
}

func TestVaultWrongPasswordBackoff(t *testing.T) {
	f := newVaultFx(t)
	f.m.create("ana-parola-1")
	f.m.lock()
	if _, _, err := f.m.unlock("yanlış-parola"); !errors.Is(err, vault.ErrWrongPassword) {
		t.Fatalf("1. deneme: %v", err)
	}
	// bekleme sürerken DOĞRU parola bile reddedilir (kaba kuvvet yavaşlar)
	if _, _, err := f.m.unlock("ana-parola-1"); !errors.Is(err, errVaultThrottled) {
		t.Fatalf("beklemede doğru parola kabul edildi: %v", err)
	}
	if st := f.m.status(); st.RetryAfter < 1 {
		t.Fatalf("retryAfter yok: %+v", st)
	}
	f.clock = f.clock.Add(2 * time.Second)
	if _, _, err := f.m.unlock("yanlış-2"); !errors.Is(err, vault.ErrWrongPassword) {
		t.Fatalf("2. deneme: %v", err)
	}
	// 2. yanlıştan sonra bekleme 2 sn: 1 sn sonra hâlâ kapalı, 3 sn sonra açılır
	f.clock = f.clock.Add(time.Second)
	if _, _, err := f.m.unlock("ana-parola-1"); !errors.Is(err, errVaultThrottled) {
		t.Fatalf("bekleme artmadı: %v", err)
	}
	f.clock = f.clock.Add(3 * time.Second)
	if _, _, err := f.m.unlock("ana-parola-1"); err != nil {
		t.Fatalf("bekleme sonrası açılmadı: %v", err)
	}
	if f.m.fails != 0 {
		t.Fatal("başarılı girişte sayaç sıfırlanmadı")
	}
}

func TestVaultIdleAutoLock(t *testing.T) {
	f := newVaultFx(t)
	f.m.create("ana-parola-1")
	f.m.setPassphrase("web1", "p-web1")
	f.clock = f.clock.Add(9 * time.Minute)
	if f.m.idleTick() {
		t.Fatal("erken kilitlendi")
	}
	f.m.touch() // kullanıcı etkinliği sayacı sıfırlar
	f.clock = f.clock.Add(9 * time.Minute)
	if f.m.idleTick() {
		t.Fatal("etkinlikten sonra erken kilitlendi")
	}
	f.clock = f.clock.Add(2 * time.Minute)
	if !f.m.idleTick() || !f.m.locked() || len(f.added) != 0 {
		t.Fatalf("boşta kilit çalışmadı: locked=%v added=%v", f.m.locked(), f.added)
	}
	f.m.idleAfter = 0 // 0: otomatik kilit kapalı
	f.m.unlock("ana-parola-1")
	f.clock = f.clock.Add(24 * time.Hour)
	if f.m.idleTick() {
		t.Fatal("idleAfter=0 iken kilitlendi")
	}
}

func TestVaultChangePassword(t *testing.T) {
	f := newVaultFx(t)
	f.m.create("ana-parola-1")
	if err := f.m.changePassword("yanlış", "yeni-parola-2"); !errors.Is(err, vault.ErrWrongPassword) {
		t.Fatalf("yanlış eski parola: %v", err)
	}
	if err := f.m.changePassword("ana-parola-1", "yeni-parola-2"); !errors.Is(err, errVaultThrottled) {
		t.Fatalf("beklemede parola değişti: %v", err)
	}
	f.clock = f.clock.Add(5 * time.Second)
	if err := f.m.changePassword("ana-parola-1", "kısa"); !errors.Is(err, errVaultWeak) {
		t.Fatalf("zayıf yeni parola: %v", err)
	}
	if err := f.m.changePassword("ana-parola-1", "yeni-parola-2"); err != nil {
		t.Fatal(err)
	}
	f.m.lock()
	f.clock = f.clock.Add(time.Minute)
	if _, _, err := f.m.unlock("ana-parola-1"); !errors.Is(err, vault.ErrWrongPassword) {
		t.Fatalf("eski parola hâlâ açıyor: %v", err)
	}
	f.clock = f.clock.Add(time.Minute)
	if _, _, err := f.m.unlock("yeni-parola-2"); err != nil {
		t.Fatalf("yeni parola açmıyor: %v", err)
	}
}

func TestVaultDisabledOutsideGuestMode(t *testing.T) {
	f := newVaultFx(t)
	f.m.enabled = func() bool { return false }
	if err := f.m.create("ana-parola-1"); !errors.Is(err, errVaultOff) {
		t.Fatalf("misafir değilken oluşturuldu: %v", err)
	}
	if f.m.locked() || f.m.status().Available {
		t.Fatal("devre dışıyken kilitli/uygun görünüyor")
	}
}

// guard: kasa dosyası var ve kilitliyse tüm uçlar 423; kasa uçları ve çıkış açık. API yanıtları sır içermez.
func TestVaultGuardAndAPI(t *testing.T) {
	f := newVaultFx(t)
	a := f.a
	a.host = "127.0.0.1:1"
	sess := a.newSession()
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = a.host
		r.AddCookie(&http.Cookie{Name: cookieName, Value: sess})
		if method == http.MethodPost {
			r.Header.Set("X-Requested-With", "sshpocket")
		}
		w := httptest.NewRecorder()
		mux := http.NewServeMux()
		mux.HandleFunc("/api/home", a.guard(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ev")) }))
		mux.HandleFunc("/api/vault", a.guard(a.vaultStatusHandler))
		mux.HandleFunc("/api/vault/", a.guard(a.vaultActionHandler))
		mux.ServeHTTP(w, r)
		return w
	}
	if w := do("GET", "/api/home", ""); w.Code != 200 {
		t.Fatalf("kasa yokken engel: %d", w.Code)
	}
	if w := do("POST", "/api/vault/create", `{"password":"ana-parola-1"}`); w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	if w := do("POST", "/api/vault/passphrase", `{"server":"web1","passphrase":"p-web1"}`); w.Code != 200 {
		t.Fatalf("passphrase: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/api/home", ""); w.Code != 200 {
		t.Fatalf("açıkken engel: %d", w.Code)
	}
	if w := do("POST", "/api/vault/lock", ``); w.Code != 200 {
		t.Fatalf("lock: %d", w.Code)
	}
	if w := do("GET", "/api/home", ""); w.Code != http.StatusLocked {
		t.Fatalf("kilitliyken 423 yok: %d", w.Code)
	}
	if w := do("GET", "/api/vault", ""); w.Code != 200 {
		t.Fatalf("kilitliyken durum okunamıyor: %d", w.Code)
	}
	w := do("POST", "/api/vault/unlock", `{"password":"yanlış-parola"}`)
	if w.Code != http.StatusUnauthorized || bytes.Contains(w.Body.Bytes(), []byte("yanlış-parola")) {
		t.Fatalf("yanlış parola: %d %s", w.Code, w.Body)
	}
	if w := do("POST", "/api/vault/unlock", `{"password":"ana-parola-1"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("beklemede 429 yok: %d", w.Code)
	}
	f.clock = f.clock.Add(5 * time.Second)
	w = do("POST", "/api/vault/unlock", `{"password":"ana-parola-1"}`)
	if w.Code != 200 {
		t.Fatalf("unlock: %d %s", w.Code, w.Body)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if l, _ := resp["loaded"].([]any); len(l) != 1 {
		t.Fatalf("loaded: %v", resp)
	}
	for _, p := range []string{"/api/vault"} {
		b := do("GET", p, "").Body.String()
		for _, secret := range []string{"p-web1", "ana-parola-1"} {
			if strings.Contains(b, secret) {
				t.Fatalf("durum yanıtı sır içeriyor: %s", b)
			}
		}
	}
	// CSRF: başlıksız POST reddedilir
	r := httptest.NewRequest("POST", "/api/vault/lock", nil)
	r.Host = a.host
	r.AddCookie(&http.Cookie{Name: cookieName, Value: sess})
	ww := httptest.NewRecorder()
	a.guard(a.vaultActionHandler)(ww, r)
	if ww.Code != http.StatusForbidden {
		t.Fatalf("CSRF başlıksız POST: %d", ww.Code)
	}
}

// Boşta kilit yalnız POST'larla değil, arayüzün bildirdiği gerçek kullanıcı girdisiyle de sıfırlanır (/api/vault/touch).
func TestVaultTouchEndpointResetsIdle(t *testing.T) {
	f := newVaultFx(t)
	f.m.create("ana-parola-1")
	post := func(path string) int {
		w := httptest.NewRecorder()
		f.a.vaultActionHandler(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
		return w.Code
	}
	f.clock = f.clock.Add(9 * time.Minute)
	if code := post("/api/vault/touch"); code != 200 {
		t.Fatalf("touch: %d", code)
	}
	f.clock = f.clock.Add(9 * time.Minute)
	if f.m.idleTick() {
		t.Fatal("touch sayacı sıfırlamadı: 9+9 dk sonra kilitlendi")
	}
	f.clock = f.clock.Add(2 * time.Minute)
	if !f.m.idleTick() {
		t.Fatal("touch sonrası 11 dk boşta kilitlenmedi")
	}
}
