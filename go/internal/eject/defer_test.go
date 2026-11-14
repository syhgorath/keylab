package eject

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// KeyLab çıkarılacak USB'nin kendisinden çalışıyorsa çıkarma SENKRON yapılmaz (süreç bitince yardımcı süreç yapar).
func TestEjectDefersWhenRunningFromTheVolume(t *testing.T) {
	usb := mkUSB(t)
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls.log")
	os.WriteFile(filepath.Join(bin, "diskutil"), []byte("#!/bin/bash\necho \"diskutil $*\" >> \""+log+"\"\nexit 0\n"), 0o755)
	os.WriteFile(filepath.Join(bin, "ssh-add"), []byte("#!/bin/bash\nexit 1\n"), 0o755)
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	old := executablePath
	t.Cleanup(func() { executablePath = old })
	executablePath = func() (string, error) { return filepath.Join(usb, "KeyLab.app", "Contents", "MacOS", "KeyLab"), nil }

	var out bytes.Buffer
	if code := Run(&out, usb); code != 0 {
		t.Fatalf("çıkış %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "uygulama kapandıktan hemen sonra") {
		t.Fatalf("ileti: %q", out.String())
	}
	// Run dönerken diskutil HENÜZ çağrılmamış olmalı (yardımcı 2 sn bekler)
	if b, _ := os.ReadFile(log); len(b) != 0 {
		t.Fatalf("senkron çıkarma yapıldı: %q", b)
	}
	// yardımcı süreç sonra çıkarır (normal eject; kuvvet gerekmez)
	var got string
	for i := 0; i < 80 && !strings.Contains(got, "eject"); i++ {
		time.Sleep(100 * time.Millisecond)
		b, _ := os.ReadFile(log)
		got = string(b)
	}
	if !strings.Contains(got, "diskutil eject "+usb) || strings.Contains(got, "unmount force") {
		t.Fatalf("yardımcı beklenen çağrıyı yapmadı: %q", got)
	}
}

func TestSelfOnVolume(t *testing.T) {
	old := executablePath
	t.Cleanup(func() { executablePath = old })
	usb := t.TempDir()
	cases := map[string]bool{
		filepath.Join(usb, "KeyLab.app", "Contents", "MacOS", "KeyLab"): true,
		filepath.Join(usb, "keylab"):                                    true,
		usb + "x/keylab":                                                false, // önek tuzağı
		"/Applications/KeyLab.app/Contents/MacOS/KeyLab":                false,
	}
	for exe, want := range cases {
		executablePath = func() (string, error) { return exe, nil }
		if got := selfOnVolume(usb); got != want {
			t.Errorf("%q: %v (beklenen %v)", exe, got, want)
		}
	}
}
