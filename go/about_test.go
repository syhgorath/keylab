package main

import (
	"os"
	"path/filepath"
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
