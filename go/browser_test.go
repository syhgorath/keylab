package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestChromiumCandidatesOrder(t *testing.T) {
	c := chromiumCandidates("/Users/x")
	want := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Users/x/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Users/x/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Users/x/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Users/x/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	if len(c) != len(want) {
		t.Fatalf("aday sayısı %d, beklenen %d: %v", len(c), len(want), c)
	}
	for i := range want {
		if c[i] != want[i] {
			t.Errorf("aday %d: %q, beklenen %q", i, c[i], want[i])
		}
	}
}

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
	for in, want := range map[string]string{"": "auto", "auto": "auto", "default": "default"} {
		if got, ok := normalizeBrowserMode(in); !ok || got != want {
			t.Errorf("%q → %q,%v (beklenen %q)", in, got, ok, want)
		}
	}
	if _, ok := normalizeBrowserMode("safari"); ok {
		t.Error("bilinmeyen kip reddedilmeli")
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
