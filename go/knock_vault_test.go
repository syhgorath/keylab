package main

import (
	"errors"
	"keylab/internal/platform"
	"keylab/internal/servers"
	"keylab/internal/vault"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func guestKnockApp(t *testing.T) (*App, *vaultFx) {
	t.Helper()
	platform.SetGuestMode(true)
	t.Cleanup(func() { platform.SetGuestMode(false) })
	f := newVaultFx(t)
	os.WriteFile(filepath.Join(f.a.serversDir, "web1", "server.env"), []byte("IP=127.0.0.1\nUSER=root\nPORT=22\n"), 0o600)
	f.a.host, f.a.token = "127.0.0.1:9999", "jeton"
	return f.a, f
}

// vaultSecrets: yalnız kasa AÇIKKEN çalışır; kilitliyken bulamaz, yazamaz.
func TestVaultSecretsOnlyWorkWhileUnlocked(t *testing.T) {
	f := newVaultFx(t)
	s := vaultSecrets{f.m}
	if s.Available() || s.Has("x") || s.Set("x", "y") == nil {
		t.Fatal("kasa yokken/kilitliyken depo çalışmamalı")
	}
	f.m.create("ana-parola-1")
	if !s.Available() || s.Set("USBMonitor_WEB1_Knock", "7001,7002") != nil {
		t.Fatal("açıkken yazılamadı")
	}
	if v, ok := s.Find("USBMonitor_WEB1_Knock"); !ok || v != "7001,7002" || !s.Has("USBMonitor_WEB1_Knock") {
		t.Fatalf("Find: %q %v", v, ok)
	}
	if s.SetInteractive("a", nil, nil, nil) == nil {
		t.Fatal("etkileşimli giriş desteklenmemeli")
	}
	f.m.lock()
	if _, ok := s.Find("USBMonitor_WEB1_Knock"); ok || s.Delete("USBMonitor_WEB1_Knock") || !errors.Is(s.Set("a", "b"), vault.ErrLocked) {
		t.Fatal("kilitliyken depo erişilebilir")
	}
}

// Misafir modda knock: kasa yokken kayıt 409 (nedenli), kasayla kaydedilir/okunur/gönderilir/kapatılır; dizi kasada (Keychain'de değil) durur.
func TestGuestKnockLivesInTheVault(t *testing.T) {
	a, f := guestKnockApp(t)
	w := call(a, "POST", "/api/knock", `{"name":"web1","action":"save","proto":"tcp","delay":"0","seq":"7001,7002,7003"}`, map[string]string{"X-Requested-With": "sshpocket", "Origin": "http://127.0.0.1:9999"})
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "şifreli kasa") {
		t.Fatalf("kasa yokken: %d %s", w.Code, w.Body)
	}
	if err := f.m.create("ana-parola-1"); err != nil {
		t.Fatal(err)
	}
	xh := map[string]string{"X-Requested-With": "sshpocket", "Origin": "http://127.0.0.1:9999"}
	if w := call(a, "POST", "/api/knock", `{"name":"web1","action":"save","proto":"tcp","delay":"0","seq":"7001,7002,7003"}`, xh); w.Code != 200 {
		t.Fatalf("kayıt: %d %s", w.Code, w.Body)
	}
	if v, ok := f.m.v.Get(servers.KnockAccount("WEB1")); !ok || v != "7001,7002,7003" {
		t.Fatalf("dizi kasada değil: %q %v", v, ok)
	}
	if env, _ := os.ReadFile(filepath.Join(a.serversDir, "web1", "server.env")); !strings.Contains(string(env), "KNOCK=keychain") {
		t.Fatalf("server.env: %s", env)
	}
	if w := call(a, "GET", "/api/knock?name=web1", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"hasSeq":true`) || strings.Contains(w.Body.String(), "7001,7002,7003") {
		t.Fatalf("durum (dizi asla geri verilmez): %d %s", w.Code, w.Body)
	}
	if st := f.m.status(); len(st.Knock) != 1 || st.Knock[0] != "web1" {
		t.Fatalf("kasa durumu knock listesi: %+v", st.Knock)
	}
	if w := call(a, "POST", "/api/knock", `{"name":"web1","action":"send"}`, xh); w.Code != 200 {
		t.Fatalf("gönder: %d %s", w.Code, w.Body)
	}
	if w := call(a, "POST", "/api/knock", `{"name":"web1","action":"disable"}`, xh); w.Code != 200 {
		t.Fatalf("kapat: %d %s", w.Code, w.Body)
	}
	if _, ok := f.m.v.Get(servers.KnockAccount("WEB1")); ok {
		t.Fatal("kapatınca dizi kasadan silinmedi")
	}
}

// Kilitliyken knock uçları 423 (guard); kasada dizi yokken "gönder" gerçek nedeni söyler (Keychain demez).
func TestGuestKnockErrorsNameTheVaultNotTheKeychain(t *testing.T) {
	a, f := guestKnockApp(t)
	xh := map[string]string{"X-Requested-With": "sshpocket", "Origin": "http://127.0.0.1:9999"}
	os.WriteFile(filepath.Join(a.serversDir, "web1", "server.env"), []byte("IP=127.0.0.1\nUSER=root\nPORT=22\nKNOCK=keychain\n"), 0o600)
	if w := call(a, "POST", "/api/knock", `{"name":"web1","action":"send"}`, xh); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "şifreli kasa") || strings.Contains(w.Body.String(), "Keychain'de yok") {
		t.Fatalf("kasa yokken gönder: %d %s", w.Code, w.Body)
	}
	f.m.create("ana-parola-1")
	if w := call(a, "POST", "/api/knock", `{"name":"web1","action":"send"}`, xh); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "kasada yok") {
		t.Fatalf("dizi kasada yokken gönder: %d %s", w.Code, w.Body)
	}
	f.m.lock()
	if w := call(a, "GET", "/api/knock?name=web1", "", nil); w.Code != http.StatusLocked {
		t.Fatalf("kasa kilitliyken 423 beklenirdi: %d", w.Code)
	}
}

// Misafir değilken eski yol aynen: knockSecrets Keychain deposudur (kasa değil).
func TestNonGuestKnockStillUsesKeychainStore(t *testing.T) {
	platform.SetGuestMode(false)
	a := testApp()
	if _, isVault := a.knockSecrets().(vaultSecrets); isVault {
		t.Fatal("misafir değilken kasa kullanılmamalı")
	}
}
