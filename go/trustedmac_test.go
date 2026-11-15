package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWantGuestMode(t *testing.T) {
	for _, c := range []struct {
		env     string
		trusted bool
		want    bool
	}{
		{"", false, true},   // taşınabilir, işaret yok: misafir (varsayılan)
		{"", true, false},   // bu bilgisayar güvenilir: Keychain
		{"0", false, false}, // ortam değişkeni açıkça kapatır
		{"0", true, false},
		{"1", false, true}, // ortam değişkeni açıkça açar: işaret bile ezilir
		{"1", true, true},
		{"evet", true, true}, // "0" dışındaki her açık değer misafir (eski davranış)
	} {
		if got := wantGuestMode(c.env, c.trusted); got != c.want {
			t.Errorf("env=%q trusted=%v: %v, beklenen %v", c.env, c.trusted, got, c.want)
		}
	}
}

func trustedFx(t *testing.T, portable bool) (*App, string) {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "home", ".keylab", "keychain-ok")
	old := trustedMacFile
	trustedMacFile = func() string { return marker }
	t.Cleanup(func() { trustedMacFile = old })
	a := testApp()
	a.portableFn = func() bool { return portable }
	return a, marker
}

func callTrusted(a *App, method, body string) (*httptest.ResponseRecorder, trustedMacStatus) {
	w := httptest.NewRecorder()
	a.trustedMacHandler(w, httptest.NewRequest(method, "/api/trusted-mac", strings.NewReader(body)))
	var st trustedMacStatus
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	return w, st
}

func TestTrustedMacHandlerTogglesMarkerOnThisMac(t *testing.T) {
	a, marker := trustedFx(t, true)
	if w, st := callTrusted(a, "GET", ""); w.Code != 200 || !st.Portable || st.Trusted {
		t.Fatalf("başlangıç: %d %+v", w.Code, st)
	}
	if w, st := callTrusted(a, "POST", `{"enabled":true}`); w.Code != 200 || !st.Trusted {
		t.Fatalf("açma: %d %+v %s", w.Code, st, w.Body)
	}
	fi, err := os.Stat(marker)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("işaret dosyası yok ya da izin yanlış: %v %v", err, fi)
	}
	if !trustedMac() {
		t.Fatal("trustedMac() işareti görmüyor")
	}
	if w, st := callTrusted(a, "POST", `{"enabled":false}`); w.Code != 200 || st.Trusted {
		t.Fatalf("kapatma: %d %+v", w.Code, st)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("işaret silinmedi: %v", err)
	}
	if w, _ := callTrusted(a, "POST", `{"enabled":false}`); w.Code != 200 {
		t.Fatalf("olmayan işareti kapatmak hata verdi: %d", w.Code)
	}
}

func TestTrustedMacRefusesOutsidePortableAndBadRequests(t *testing.T) {
	a, marker := trustedFx(t, false)
	if w, _ := callTrusted(a, "POST", `{"enabled":true}`); w.Code != http.StatusConflict {
		t.Fatalf("taşınabilir değilken kabul edildi: %d", w.Code)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("taşınabilir değilken işaret yazıldı")
	}
	a2, _ := trustedFx(t, true)
	if w, _ := callTrusted(a2, "POST", `{`); w.Code != http.StatusBadRequest {
		t.Fatalf("bozuk gövde: %d", w.Code)
	}
	if w, _ := callTrusted(a2, "DELETE", ``); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE: %d", w.Code)
	}
}
