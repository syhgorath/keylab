package main

// Misafir (taşınabilir) mod: Keychain kullanılmaz; knock dizisi şifreli kasada tutulur (kasa yokken anlaşılır hata, Keychain'e yazılmaz), /api/about modu bildirir, hiçbir `security` çağrılmaz.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/platform"
)

func TestKnockSaveNeedsVaultInGuestMode(t *testing.T) {
	app, kc := knockApp(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")
	platform.SetGuestMode(true)
	t.Cleanup(func() { platform.SetGuestMode(false) })

	w := call(app, "POST", "/api/knock", `{"name":"kn","action":"save","proto":"tcp","delay":"300","seq":"7001,7002,7003"}`, xhr)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "şifreli kasada tutulur") {
		t.Fatalf("misafirde knock kaydı: %d %s", w.Code, w.Body)
	}
	if b, _ := os.ReadFile(kc); len(b) != 0 {
		t.Fatalf("Keychain'e yazıldı: %q", b)
	}
	env, _ := os.ReadFile(filepath.Join(app.serversDir, "kn", "server.env"))
	if strings.Contains(string(env), "KNOCK") {
		t.Fatalf("server.env değişti: %q", env)
	}
	// misafir değilken aynı istek çalışır (modun ayırıcı olduğunu doğrular)
	platform.SetGuestMode(false)
	if w := call(app, "POST", "/api/knock", `{"name":"kn","action":"save","proto":"tcp","delay":"300","seq":"7001,7002,7003"}`, xhr); w.Code != 200 {
		t.Fatalf("misafir değilken knock kaydı: %d %s", w.Code, w.Body)
	}
}

func TestAboutInfoReportsGuestMode(t *testing.T) {
	app := &App{compDir: filepath.Join(t.TempDir(), "components")}
	platform.SetGuestMode(false)
	if m := app.aboutInfo(); m["guest"] != false {
		t.Fatalf("misafir değilken: %v", m["guest"])
	}
	platform.SetGuestMode(true)
	t.Cleanup(func() { platform.SetGuestMode(false) })
	if m := app.aboutInfo(); m["guest"] != true {
		t.Fatalf("misafirken: %v", m["guest"])
	}
}
