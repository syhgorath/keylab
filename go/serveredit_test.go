package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const editEnv = "# bu yorum korunmalı\nIP=192.0.2.9\nUSER=eski\nPORT=22\nOS=ubuntu\nKNOCK=keychain\nKNOCK_PROTO=udp\nKNOCK_DELAY_MS=450\n"

func editApp(t *testing.T) (*App, string) {
	t.Helper()
	srv := filepath.Join(t.TempDir(), "servers")
	d := filepath.Join(srv, "web1")
	os.MkdirAll(d, 0o700)
	os.WriteFile(filepath.Join(d, "server.env"), []byte(editEnv), 0o600)
	os.WriteFile(filepath.Join(d, "id_ed25519_web1"), []byte("anahtar"), 0o600)
	a := &App{serversDir: srv, host: "127.0.0.1:9999", token: "jeton", jobs: map[string]*Job{}, up: map[string]bool{"web1": true}}
	return a, filepath.Join(d, "server.env")
}

func TestUpdateServerAddr(t *testing.T) {
	a, env := editApp(t)
	if code, err := a.updateServerAddr("web1", "198.51.100.7", "yeni", "2222"); err != nil {
		t.Fatalf("kod %d: %v", code, err)
	}
	b, _ := os.ReadFile(env)
	got := string(b)
	for _, want := range []string{"IP=198.51.100.7", "USER=yeni", "PORT=2222", "# bu yorum korunmalı", "OS=ubuntu", "KNOCK=keychain", "KNOCK_PROTO=udp", "KNOCK_DELAY_MS=450"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q yok:\n%s", want, got)
		}
	}
	for _, old := range []string{"IP=192.0.2.9", "USER=eski"} {
		if strings.Contains(got, old) {
			t.Errorf("eski değer kalmış %q:\n%s", old, got)
		}
	}
	if _, ok := a.up["web1"]; ok {
		t.Error("eski adrese göre ölçülmüş durum silinmeliydi")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(env), "id_ed25519_web1")); err != nil {
		t.Error("anahtar dosyasına dokunulmamalı")
	}
	// boş port 22 olur
	if _, err := a.updateServerAddr("web1", "198.51.100.7", "yeni", ""); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(env); !strings.Contains(string(b), "PORT=22") {
		t.Errorf("boş port 22 olmalıydı:\n%s", b)
	}
}

func TestUpdateServerAddrRejects(t *testing.T) {
	cases := []struct {
		name, host, user, port string
		code                   int
	}{
		{"web1", "", "u", "22", 400},
		{"web1", "bozuk host!", "u", "22", 400},
		{"web1", "192.0.2.1", "-kotu", "22", 400},
		{"web1", "192.0.2.1", "u", "0", 400},
		{"web1", "192.0.2.1", "u", "70000", 400},
		{"web1", "192.0.2.1", "u", "99999999999999999999", 400},
		{"web1", "192.0.2.1", "u; rm -rf /", "22", 400},
		{"../web1", "192.0.2.1", "u", "22", 400},
		{"yok", "192.0.2.1", "u", "22", 404},
	}
	for _, c := range cases {
		a, env := editApp(t)
		code, err := a.updateServerAddr(c.name, c.host, c.user, c.port)
		if err == nil || code != c.code {
			t.Errorf("%+v: kod %d hata %v, beklenen %d", c, code, err, c.code)
		}
		if b, _ := os.ReadFile(env); string(b) != editEnv {
			t.Errorf("%+v: reddedilen istek server.env'i değiştirmiş:\n%s", c, b)
		}
		if _, ok := a.up["web1"]; !ok {
			t.Errorf("%+v: reddedilen istek durum önbelleğini silmemeli", c)
		}
	}
}

func TestServerEditHandlerGuard(t *testing.T) {
	a, env := editApp(t)
	do := func(body string, hdr map[string]string, cookie bool) int {
		req := httptest.NewRequest("POST", "/api/server-edit", strings.NewReader(body))
		req.Host = "127.0.0.1:9999"
		if cookie {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: "jeton"})
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		a.guard(a.serverEditHandler)(w, req)
		return w.Code
	}
	ok := `{"name":"web1","host":"198.51.100.7","user":"yeni","port":"22"}`
	if c := do(ok, nil, true); c != 403 {
		t.Errorf("CSRF başlığı olmadan 403 beklenirdi: %d", c)
	}
	if c := do(ok, xhr, false); c != 401 {
		t.Errorf("çerezsiz 401 beklenirdi: %d", c)
	}
	if b, _ := os.ReadFile(env); string(b) != editEnv {
		t.Error("reddedilen istekler server.env'i değiştirmemeli")
	}
	if c := do(ok, xhr, true); c != 200 {
		t.Errorf("geçerli istek 200 olmalı: %d", c)
	}
	if c := do(`{`, xhr, true); c != 400 {
		t.Errorf("bozuk gövde 400 olmalı: %d", c)
	}
	big := `{"name":"web1","host":"198.51.100.8","user":"u","port":"22","dolgu":"` + strings.Repeat("a", 5000) + `"}`
	if c := do(big, xhr, true); c != 400 {
		t.Errorf("aşırı büyük gövde 400 olmalı: %d", c)
	}
}
