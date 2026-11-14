package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAboutInfoVersionFromBinary(t *testing.T) {
	root := t.TempDir() // kurulu mod: kökte VERSION dosyası YOK
	app := &App{compDir: filepath.Join(root, "components"), serversDir: filepath.Join(root, "servers"), stateDir: filepath.Join(root, "state")}

	old := buildVersion
	t.Cleanup(func() { buildVersion = old })

	buildVersion = "4.0.0"
	m := app.aboutInfo()
	if m["version"] != "4.0.0" || m["root"] != root || m["usb"] != "" {
		t.Fatalf("gömülü sürüm bekleniyordu: %v", m)
	}

	// geliştirme derlemesi (gömülü sürüm yok): kökteki VERSION dosyasına düşer
	buildVersion = ""
	if m := app.aboutInfo(); m["version"] != "" {
		t.Fatalf("sürüm bilinmiyorsa boş olmalı (arayüz — gösterir): %v", m["version"])
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("3.9.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if m := app.aboutInfo(); m["version"] != "3.9.1" {
		t.Fatalf("VERSION dosyası: %v", m["version"])
	}
}

func TestAboutInfoUSBName(t *testing.T) {
	app := &App{compDir: "/Volumes/test/components", serversDir: "/Volumes/test/servers"}
	if m := app.aboutInfo(); m["usb"] != "test" {
		t.Fatalf("usb adı: %v", m["usb"])
	}
}

func TestCheckUpdatesBootstrapsMissingLockAndWarnsWhenEmpty(t *testing.T) {
	root := t.TempDir()
	app := &App{compDir: filepath.Join(root, "components"), serversDir: filepath.Join(root, "servers")}
	app.idxDL = func(url, dest string, max int64) error { return os.ErrNotExist } // ağ yok: yalnız kilit davranışı sınanır
	// kilit hiç yok → gömülü kilit kopyalanır, sonra indeks adımına geçilir (indirme hatası raporlanır)
	rep, err := app.checkUpdates()
	if err != nil {
		t.Fatal(err)
	}
	b, rerr := os.ReadFile(filepath.Join(root, "components.lock"))
	if rerr != nil || !strings.Contains(string(b), "[linux-hardening]") {
		t.Fatalf("gömülü kilit kopyalanmadı: %v", rerr)
	}
	if rep.OK || rep.Exit != 1 {
		t.Fatalf("indirme hatası raporlanmalı: %+v", rep)
	}
	// kilit var ama boş (yalnız yorum) → açık uyarı, indeks denenmez
	os.WriteFile(filepath.Join(root, "components.lock"), []byte("# boş\n"), 0o644)
	called := false
	app.idxDL = func(url, dest string, max int64) error { called = true; return os.ErrNotExist }
	rep, _ = app.checkUpdates()
	if called || rep.Exit != 1 || !strings.Contains(rep.Raw, "paket kilidi") || !strings.Contains(rep.Raw, "boş") {
		t.Fatalf("boş kilit uyarısı: called=%v %+v", called, rep)
	}
}
