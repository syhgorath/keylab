package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sahte macOS `security`: -i standart girdiden "add-generic-password -U -a <hesap> -s <servis> -w <değer>" okur.
const fakeSecurity = `#!/bin/bash
S="$FAKE_KC"
case "$1" in
  -i) read -r cmdline; acct=""; val=""; set -- $cmdline
      while [ $# -gt 0 ]; do case "$1" in -a) acct="$2" ;; -w) val="$2" ;; esac; shift; done
      { grep -v "^$acct=" "$S" 2>/dev/null; echo "$acct=$val"; } > "$S.n"; mv "$S.n" "$S"; exit 0 ;;
  find-generic-password)
      grep -q "^$3=" "$S" 2>/dev/null || exit 44
      if [ "$5" = "-w" ] || [ "$6" = "-w" ]; then grep "^$3=" "$S" | tail -1 | cut -d= -f2-; fi; exit 0 ;;
  delete-generic-password) grep -q "^$3=" "$S" 2>/dev/null || exit 44; grep -v "^$3=" "$S" > "$S.n"; mv "$S.n" "$S"; exit 0 ;;
esac
exit 1
`

func knockApp(t *testing.T, env string) (*App, string) {
	t.Helper()
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "security"), []byte(fakeSecurity), 0o755)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	kc := filepath.Join(t.TempDir(), "kc")
	t.Setenv("FAKE_KC", kc)
	srv := filepath.Join(t.TempDir(), "servers")
	os.MkdirAll(filepath.Join(srv, "kn"), 0o700)
	os.WriteFile(filepath.Join(srv, "kn", "server.env"), []byte(env), 0o600)
	return &App{serversDir: srv, host: "127.0.0.1:9999", token: "jeton", jobs: map[string]*Job{}, up: map[string]bool{}}, kc
}

func call(app *App, method, target, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:9999"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: app.newSession()})
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	app.guard(app.knockHandler)(w, req)
	return w
}

var xhr = map[string]string{"X-Requested-With": "sshpocket", "Origin": "http://127.0.0.1:9999"}

func TestKnockAPIFlow(t *testing.T) {
	app, kc := knockApp(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")

	if w := call(app, "GET", "/api/knock?name=kn", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Fatalf("ilk durum: %d %s", w.Code, w.Body)
	}
	w := call(app, "POST", "/api/knock", `{"name":"kn","action":"save","proto":"tcp","delay":"300","seq":"7001,7002,7003"}`, xhr)
	if w.Code != 200 {
		t.Fatalf("kaydet: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "7001") {
		t.Error("yanıt diziyi içeriyor")
	}
	if b, _ := os.ReadFile(kc); !strings.Contains(string(b), "USBMonitor_KN_Knock=7001,7002,7003") {
		t.Errorf("Keychain'de dizi yok: %q", b)
	}
	w = call(app, "GET", "/api/knock?name=kn", "", nil)
	if !strings.Contains(w.Body.String(), `"hint":"7••1, 7••2, 7••3"`) {
		t.Errorf("yanıtta maskeli ipucu yok: %s", w.Body)
	}
	if !strings.Contains(w.Body.String(), `"enabled":true`) || !strings.Contains(w.Body.String(), `"hasSeq":true`) || strings.Contains(w.Body.String(), "7001") || strings.Contains(w.Body.String(), "7002") || strings.Contains(w.Body.String(), "7003") {
		t.Errorf("kayıttan sonra durum: %s", w.Body)
	}
	if w := call(app, "POST", "/api/knock", `{"name":"kn","action":"disable"}`, xhr); w.Code != 200 {
		t.Fatalf("kapat: %d %s", w.Code, w.Body)
	}
	if w := call(app, "GET", "/api/knock?name=kn", "", nil); !strings.Contains(w.Body.String(), `"hasSeq":false`) {
		t.Errorf("kapattıktan sonra: %s", w.Body)
	}
}

func TestKnockAPIRejects(t *testing.T) {
	app, kc := knockApp(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")
	cases := []struct {
		name, method, target, body string
		hdr                        map[string]string
		code                       int
	}{
		{"CSRF başlığı yok", "POST", "/api/knock", `{"name":"kn","action":"save","proto":"tcp","delay":"300","seq":"7001"}`, nil, 403},
		{"yabancı Origin", "POST", "/api/knock", `{"name":"kn","action":"save","proto":"tcp","delay":"300","seq":"7001"}`, map[string]string{"X-Requested-With": "sshpocket", "Origin": "http://kotu.example"}, 403},
		{"geçersiz dizi", "POST", "/api/knock", `{"name":"kn","action":"save","proto":"tcp","delay":"300","seq":"7001,abc"}`, xhr, 400},
		{"geçersiz protokol", "POST", "/api/knock", `{"name":"kn","action":"save","proto":"x","delay":"300","seq":"7001"}`, xhr, 400},
		{"yol gezme", "POST", "/api/knock", `{"name":"../kn","action":"disable"}`, xhr, 400},
		{"bilinmeyen eylem", "POST", "/api/knock", `{"name":"kn","action":"sil"}`, xhr, 400},
		{"bozuk gövde", "POST", "/api/knock", `{`, xhr, 400},
		{"aşırı büyük gövde", "POST", "/api/knock", `{"name":"kn","action":"disable","dolgu":"` + strings.Repeat("a", 5000) + `"}`, xhr, 400},
		{"GET geçersiz ad", "GET", "/api/knock?name=..%2Fx", "", nil, 400},
		{"PUT", "PUT", "/api/knock", "", xhr, 405},
	}
	for _, c := range cases {
		if w := call(app, c.method, c.target, c.body, c.hdr); w.Code != c.code {
			t.Errorf("%s: kod %d, beklenen %d (%s)", c.name, w.Code, c.code, strings.TrimSpace(w.Body.String()))
		}
	}
	if b, _ := os.ReadFile(kc); len(b) != 0 {
		t.Errorf("reddedilen isteklerden Keychain'e yazılmış: %q", b)
	}
	// çerezsiz istek
	req := httptest.NewRequest("GET", "/api/knock?name=kn", nil)
	req.Host = "127.0.0.1:9999"
	w := httptest.NewRecorder()
	app.guard(app.knockHandler)(w, req)
	if w.Code != 401 {
		t.Errorf("çerezsiz istek: %d", w.Code)
	}
}
