package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"keylab/internal/selfupdate"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type updFx struct {
	a        *App
	dir, app string
	priv     string
	net      map[string][]byte
	latest   string // "" → çevrimdışı
	relaunch []string
	exited   chan struct{}
}

func fakeKL(ver string) string {
	return "#!/bin/sh\nif [ \"$1\" = version ]; then echo 'sshpocket " + ver + "'; fi\n"
}

func newUpdFx(t *testing.T) *updFx {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen yok")
	}
	f := &updFx{dir: t.TempDir(), net: map[string][]byte{}, latest: "1.2.0", exited: make(chan struct{}, 1)}
	f.priv = filepath.Join(f.dir, "k")
	if o, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", f.priv).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, o)
	}
	pub, _ := os.ReadFile(f.priv + ".pub")
	fl := strings.Fields(string(pub))
	signers := filepath.Join(f.dir, "release_signers")
	os.WriteFile(signers, []byte("sshpocket-release namespaces=\"sshpocket-release\" "+fl[0]+" "+fl[1]+"\n"), 0o644)
	f.app = filepath.Join(f.dir, "Apps", "KeyLab.app")
	os.MkdirAll(filepath.Join(f.app, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(f.app, "Contents", "MacOS", "KeyLab"), []byte(fakeKL("1.0.0")), 0o755)
	a := &App{jobs: map[string]*Job{}, up: map[string]bool{}}
	a.updaterFn = func() *selfupdate.Updater {
		return &selfupdate.Updater{SignersFile: signers, Version: "1.0.0",
			Head: func(url string) string {
				if f.latest == "" || !strings.HasSuffix(url, "/releases/latest") {
					return ""
				}
				return "https://github.com/syhgorath/keylab/releases/tag/v" + f.latest
			},
			Download: func(url, dest string, max int64) error {
				b, ok := f.net[url]
				if !ok {
					return fmt.Errorf("404")
				}
				return os.WriteFile(dest, b, 0o600)
			}}
	}
	a.updateTargetFn = func() (string, string) { return f.app, "" }
	a.verifyAppFn = func(string) error { return nil }
	a.relaunchFn = func(app string) error { f.relaunch = append(f.relaunch, app); return nil }
	a.exitFn = func() { f.exited <- struct{}{} }
	f.a = a
	return f
}

// publish: sürüm notları + imzalı macOS zip'i (sahte ikili `version` ile sürümünü bildirir).
func (f *updFx) publish(t *testing.T, ver, notes string, tamper bool) {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, e := range []struct {
		n, d string
		m    os.FileMode
	}{{"KeyLab.app/", "", os.ModeDir | 0o755}, {"KeyLab.app/Contents/", "", os.ModeDir | 0o755}, {"KeyLab.app/Contents/MacOS/", "", os.ModeDir | 0o755}, {"KeyLab.app/Contents/MacOS/KeyLab", fakeKL(ver), 0o755}} {
		h := &zip.FileHeader{Name: e.n, Method: zip.Deflate}
		h.SetMode(e.m)
		w, _ := zw.CreateHeader(h)
		io.WriteString(w, e.d)
	}
	zw.Close()
	z := b.Bytes()
	zf := filepath.Join(t.TempDir(), "z")
	os.WriteFile(zf, z, 0o600)
	if o, err := exec.Command("ssh-keygen", "-Y", "sign", "-f", f.priv, "-n", "sshpocket-release", zf).CombinedOutput(); err != nil {
		t.Fatalf("sign: %v %s", err, o)
	}
	sig, _ := os.ReadFile(zf + ".sig")
	if tamper {
		z = append(z, []byte("zehir")...)
	}
	u := "https://github.com/syhgorath/keylab/releases/download/v" + ver + "/keylab-" + ver + "-macos.zip"
	f.net[u], f.net[u+".sig"] = z, sig
	f.net["https://api.github.com/repos/syhgorath/keylab/releases/tags/v"+ver] = []byte(fmt.Sprintf(`{"body":%q}`, notes))
}

func (f *updFx) call(h http.HandlerFunc, method, body string) (int, map[string]any) {
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(method, "/api/update", strings.NewReader(body)))
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func (f *updFx) appVersion() string {
	o, _ := exec.Command(filepath.Join(f.app, "Contents", "MacOS", "KeyLab"), "version").Output()
	return strings.TrimSpace(string(o))
}

func TestUpdateCheckReportsNewerVersionNotesAndApplicability(t *testing.T) {
	f := newUpdFx(t)
	f.publish(t, "1.2.0", "### Düzeltildi\n- **Bir şey** düzeldi", false)
	code, m := f.call(f.a.updateCheckHandler, "GET", "")
	if code != 200 || m["current"] != "1.0.0" || m["latest"] != "1.2.0" || m["newer"] != true || m["canApply"] != true || !strings.Contains(m["notes"].(string), "Bir şey") {
		t.Fatalf("denetim: %d %v", code, m)
	}
	f.a.updateTargetFn = func() (string, string) { return "", "translocated" }
	if _, m := f.call(f.a.updateCheckHandler, "GET", ""); m["canApply"] != false || m["reason"] != "translocated" {
		t.Fatalf("uygulanamaz nedeni: %v", m)
	}
	f.latest = "1.0.0" // güncel: notlar yine gösterilir, uygula yok
	f.publish(t, "1.0.0", "mevcut notlar", false)
	if _, m := f.call(f.a.updateCheckHandler, "GET", ""); m["newer"] != false || m["notes"] != "mevcut notlar" || m["canApply"] != false {
		t.Fatalf("güncelken: %v", m)
	}
	f.latest = "" // çevrimdışı
	if code, m := f.call(f.a.updateCheckHandler, "GET", ""); code != http.StatusBadGateway || m["error"] != "offline" {
		t.Fatalf("çevrimdışı: %d %v", code, m)
	}
	if code, _ := f.call(f.a.updateCheckHandler, "POST", ""); code != http.StatusMethodNotAllowed {
		t.Fatal("POST kabul edildi")
	}
}

func TestUpdateApplyReplacesAppRelaunchesAndExitsCleanly(t *testing.T) {
	f := newUpdFx(t)
	f.publish(t, "1.2.0", "n", false)
	code, m := f.call(f.a.updateApplyHandler, "POST", `{}`)
	if code != 200 || m["ok"] != true || m["restarting"] != true || m["version"] != "1.2.0" {
		t.Fatalf("uygula: %d %v", code, m)
	}
	if f.appVersion() != "sshpocket 1.2.0" {
		t.Fatalf("uygulama güncellenmedi: %s", f.appVersion())
	}
	select {
	case <-f.exited:
	case <-time.After(5 * time.Second):
		t.Fatal("güncellemeden sonra süreç kapanmadı")
	}
	if len(f.relaunch) != 1 || f.relaunch[0] != f.app {
		t.Fatalf("yeniden başlatma yardımcısı: %v", f.relaunch)
	}
	if _, st := f.call(f.a.updateStatusHandler, "GET", ""); st["state"] != "done" {
		t.Fatalf("durum: %v", st)
	}
}

func TestUpdateApplyRefusals(t *testing.T) {
	// çalışan iş varsa onay ister (force olmadan 409)
	f := newUpdFx(t)
	f.publish(t, "1.2.0", "n", false)
	f.a.jobs["j1"] = &Job{Done: false}
	if code, m := f.call(f.a.updateApplyHandler, "POST", `{}`); code != http.StatusConflict || m["error"] != "running" || f.appVersion() != "sshpocket 1.0.0" {
		t.Fatalf("çalışan iş: %d %v", code, m)
	}
	// uygulanamaz hedef (komut satırı ikilisi / translocation)
	f = newUpdFx(t)
	f.publish(t, "1.2.0", "n", false)
	f.a.updateTargetFn = func() (string, string) { return "", "not_bundle" }
	if code, m := f.call(f.a.updateApplyHandler, "POST", `{}`); code != http.StatusConflict || m["error"] != "not_bundle" {
		t.Fatalf("hedef yok: %d %v", code, m)
	}
	// zaten güncel / çevrimdışı
	f = newUpdFx(t)
	f.latest = "1.0.0"
	if code, m := f.call(f.a.updateApplyHandler, "POST", `{}`); code != http.StatusConflict || m["error"] != "up_to_date" {
		t.Fatalf("güncel: %d %v", code, m)
	}
	f.latest = ""
	if code, m := f.call(f.a.updateApplyHandler, "POST", `{}`); code != http.StatusBadGateway || m["error"] != "offline" {
		t.Fatalf("çevrimdışı: %d %v", code, m)
	}
	// bozuk istek
	if code, _ := f.call(f.a.updateApplyHandler, "POST", `{`); code != http.StatusBadRequest {
		t.Fatalf("bozuk gövde: %d", code)
	}
	// başka bir güncelleme sürerken ikincisi başlamaz (iki indirme/değiştirme birbirine girmesin)
	f = newUpdFx(t)
	f.publish(t, "1.2.0", "n", false)
	f.a.updBusy.Store(true)
	if code, m := f.call(f.a.updateApplyHandler, "POST", `{}`); code != http.StatusConflict || m["error"] != "busy" || f.appVersion() != "sshpocket 1.0.0" {
		t.Fatalf("eşzamanlı güncelleme: %d %v", code, m)
	}
}

// İmzası geçersiz paket: uygulama DEĞİŞMEZ, süreç kapanmaz, yeniden başlatma yardımcısı başlatılmaz, durum 'error'.
func TestUpdateApplyBadSignatureLeavesEverythingAlone(t *testing.T) {
	f := newUpdFx(t)
	f.publish(t, "1.2.0", "n", true) // imzadan sonra değiştirilmiş zip
	code, m := f.call(f.a.updateApplyHandler, "POST", `{}`)
	if code != http.StatusInternalServerError || m["error"] != "failed" || !strings.Contains(m["message"].(string), "İMZA") {
		t.Fatalf("yanıt: %d %v", code, m)
	}
	if f.appVersion() != "sshpocket 1.0.0" || len(f.relaunch) != 0 {
		t.Fatalf("uygulama değişti ya da yardımcı başladı: %s %v", f.appVersion(), f.relaunch)
	}
	select {
	case <-f.exited:
		t.Fatal("başarısız güncellemeden sonra süreç kapandı")
	case <-time.After(1500 * time.Millisecond):
	}
	if _, st := f.call(f.a.updateStatusHandler, "GET", ""); st["state"] != "error" || !strings.Contains(st["error"].(string), "İMZA") {
		t.Fatalf("durum: %v", st)
	}
}

func TestUpdateTargetRejectsNonBundleBinary(t *testing.T) {
	a := &App{}
	if app, reason := a.updateTarget(); app != "" || reason != "not_bundle" {
		t.Fatalf("test ikilisi .app içinde değil: %q %q", app, reason)
	}
}
