package main

import (
	"keylab/internal/platform"
	"os"
	"path/filepath"
	"testing"
)

func mkWebKit(t *testing.T) (home string, dirs []string) {
	t.Helper()
	home = t.TempDir()
	dirs = platform.WebKitTraceDirs(home)
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(d, "WebsiteData"), 0o755)
		os.WriteFile(filepath.Join(d, "WebsiteData", "x"), []byte("iz"), 0o600)
	}
	return
}

func TestSweepWebKitTracesRemovesBothDirsInGuestMode(t *testing.T) {
	home, dirs := mkWebKit(t)
	other := filepath.Join(home, "Library", "WebKit", "baska.uygulama")
	os.MkdirAll(other, 0o755)
	if n := sweepWebKitTraces(true, home, os.RemoveAll); n != 2 {
		t.Fatalf("silinen: %d", n)
	}
	for _, d := range dirs {
		if _, err := os.Lstat(d); err == nil {
			t.Fatalf("kaldı: %s", d)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("başka uygulamanın verisine dokunuldu")
	}
}

func TestSweepWebKitTracesLeavesEverythingOutsideGuestMode(t *testing.T) {
	home, dirs := mkWebKit(t)
	if n := sweepWebKitTraces(false, home, os.RemoveAll); n != 0 {
		t.Fatalf("misafir değilken silindi: %d", n)
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			t.Fatalf("kurulu modda veri silindi: %s", d)
		}
	}
}

func TestSweepWebKitTracesNoopWhenMissingOrHomeUnknown(t *testing.T) {
	if n := sweepWebKitTraces(true, t.TempDir(), os.RemoveAll); n != 0 {
		t.Fatalf("yokken: %d", n)
	}
	called := false
	if n := sweepWebKitTraces(true, "", func(string) error { called = true; return nil }); n != 0 || called {
		t.Fatal("ev klasörü bilinmezken silme denendi")
	}
}

// Sembolik bağ: bağın kendisi silinir, gösterdiği klasöre dokunulmaz.
func TestSweepWebKitTracesDoesNotFollowSymlink(t *testing.T) {
	home := t.TempDir()
	victim := t.TempDir()
	os.WriteFile(filepath.Join(victim, "onemli"), []byte("x"), 0o600)
	d := platform.WebKitTraceDirs(home)[0]
	os.MkdirAll(filepath.Dir(d), 0o755)
	if err := os.Symlink(victim, d); err != nil {
		t.Skip(err)
	}
	sweepWebKitTraces(true, home, os.RemoveAll)
	if _, err := os.Stat(filepath.Join(victim, "onemli")); err != nil {
		t.Fatal("bağın hedefi silindi")
	}
}
