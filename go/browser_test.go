package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindChromium(t *testing.T) {
	edge := "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"
	brave := "/Users/x/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
	if got := findChromium("/Users/x", func(p string) bool { return p == edge || p == brave }); got != edge {
		t.Errorf("öncelik Edge olmalı: %q", got)
	}
	if got := findChromium("/Users/x", func(p string) bool { return p == brave }); got != brave {
		t.Errorf("yalnız Brave varsa Brave: %q", got)
	}
	if got := findChromium("/Users/x", func(string) bool { return false }); got != "" {
		t.Errorf("hiç yoksa boş olmalı: %q", got)
	}
}

func TestIsolatedBrowserArgs(t *testing.T) {
	prof := filepath.Join("/state", "browser-profile")
	args := isolatedBrowserArgs("http://127.0.0.1:5555/?b=abc", prof)
	for _, want := range []string{"--user-data-dir=" + prof, "--app=http://127.0.0.1:5555/?b=abc", "--disable-extensions", "--no-first-run", "--no-default-browser-check"} {
		found := false
		for _, a := range args {
			if a == want {
				found = true
			}
		}
		if !found {
			t.Errorf("argüman eksik: %q (var: %v)", want, args)
		}
	}
}

func TestNormalizeBrowserMode(t *testing.T) {
	for in, want := range map[string]string{"": "auto", "auto": "auto", "default": "default", "window": "window"} {
		if got, ok := normalizeBrowserMode(in); !ok || got != want {
			t.Errorf("%q → %q,%v (beklenen %q)", in, got, ok, want)
		}
	}
	if _, ok := normalizeBrowserMode("safari"); ok {
		t.Error("bilinmeyen kip reddedilmeli")
	}
}

// auto: gömülü pencere varsa pencere; yoksa eski davranış. Açık seçimler (default/window) olduğu gibi kalır.
func TestEffectiveBrowserMode(t *testing.T) {
	for _, c := range []struct {
		mode     string
		windowOK bool
		want     string
	}{
		{"auto", true, "window"}, {"auto", false, "auto"},
		{"default", true, "default"}, {"default", false, "default"},
		{"window", true, "window"}, {"window", false, "window"},
	} {
		if got := effectiveBrowserMode(c.mode, c.windowOK); got != c.want {
			t.Errorf("effectiveBrowserMode(%q,%v)=%q, beklenen %q", c.mode, c.windowOK, got, c.want)
		}
	}
}

// Açılış jetonu TEK kullanımlıktır; gerçek jeton adrese hiç girmez.
func TestBootTokenOneTime(t *testing.T) {
	app := &App{host: "127.0.0.1:1234", token: "gercek", bootTok: "acilis"}
	h := app.root(http.NotFoundHandler())
	get := func(q string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://127.0.0.1:1234/"+q, nil)
		r.Host = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	hasCookie := func(w *httptest.ResponseRecorder) bool {
		for _, c := range w.Result().Cookies() {
			// çerez değeri jeton DEĞİL, geçerli bir oturum biletidir
			if c.Name == cookieName && c.Value != "gercek" && c.Value != "acilis" && app.sessionValid(c.Value) && c.HttpOnly && c.SameSite == http.SameSiteStrictMode {
				return true
			}
		}
		return false
	}
	if w := get("?b=yanlis"); hasCookie(w) || w.Code != http.StatusSeeOther {
		t.Errorf("yanlış jeton çerez vermemeli (kod %d)", w.Code)
	}
	if w := get("?b=acilis"); !hasCookie(w) || w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Errorf("doğru jeton ilk kullanımda çerez vermeli (kod %d, konum %q)", w.Code, w.Header().Get("Location"))
	}
	if w := get("?b=acilis"); hasCookie(w) {
		t.Error("açılış jetonu ikinci kez kullanılamaz")
	}
	// elle açılan adres (?t=gerçek jeton) eskisi gibi çalışır
	if w := get("?t=gercek"); !hasCookie(w) {
		t.Error("?t= ile açılış çalışmalı")
	}
	// boş açılış jetonu hiçbir zaman eşleşmez
	app2 := &App{host: "127.0.0.1:1234", token: "gercek"}
	if app2.useBootToken("") || app2.useBootToken("x") {
		t.Error("açılış jetonu tanımsızsa kullanılamaz")
	}
	if strings.Contains(isolatedBrowserArgs("http://h/?b=acilis", "/p")[1], "gercek") {
		t.Error("gerçek jeton komut satırında olmamalı")
	}
}

// sistem tarayıcısı yolu: -browser default (ya da Chromium yok) → `open <adres>`; ayrı profil kullanılmadı (false)
func TestLaunchBrowserDefaultUsesSystemOpen(t *testing.T) {
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "calls.log")
	if err := os.WriteFile(filepath.Join(bin, "open"), []byte("#!/bin/bash\necho \"$*\" >> \""+log+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if launchBrowser("http://127.0.0.1:9/?b=x", t.TempDir(), "default") {
		t.Fatal("default kipte ayrı profil kullanıldı denmemeli")
	}
	for i := 0; i < 50; i++ {
		if b, _ := os.ReadFile(log); strings.TrimSpace(string(b)) != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if b, _ := os.ReadFile(log); strings.TrimSpace(string(b)) != "http://127.0.0.1:9/?b=x" {
		t.Fatalf("open çağrısı: %q", b)
	}
}
