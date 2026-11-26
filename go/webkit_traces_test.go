package main

import (
	"keylab/internal/platform"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testBundleID = platform.WebKitBundleID

func mkWebKitID(t *testing.T, home, id string) []string {
	t.Helper()
	dirs := platform.WebKitTraceDirs(home, id)
	if len(dirs) != 2 {
		t.Fatalf("%q için iki klasör beklenir: %v", id, dirs)
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(d, "WebsiteData"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "WebsiteData", "x"), []byte("iz"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dirs
}

func mkWebKit(t *testing.T) (home string, dirs []string) {
	t.Helper()
	home = t.TempDir()
	return home, mkWebKitID(t, home, testBundleID)
}

func assertDirs(t *testing.T, dirs []string, want bool, msg string) {
	t.Helper()
	for _, d := range dirs {
		_, err := os.Stat(filepath.Join(d, "WebsiteData", "x"))
		if (err == nil) != want {
			t.Fatalf("%s: %s", msg, d)
		}
	}
}

// Beklenen yollar elle yazılır: WebKit'in kullandığı yerler.
func TestWebKitTraceDirsPaths(t *testing.T) {
	got := platform.WebKitTraceDirs("/h", "keylab")
	want := []string{"/h/Library/WebKit/keylab", "/h/Library/Caches/keylab"}
	if len(got) != 2 || got[0] != filepath.FromSlash(want[0]) || got[1] != filepath.FromSlash(want[1]) {
		t.Fatalf("yollar: %v", got)
	}
	for _, id := range []string{"", ".", "..", "a/b", "../x", "keylab/"} {
		if d := platform.WebKitTraceDirs("/h", id); d != nil {
			t.Errorf("geçersiz ad %q kabul edildi: %v", id, d)
		}
	}
	for _, home := range []string{"", "goreli"} {
		if d := platform.WebKitTraceDirs(home, "keylab"); d != nil {
			t.Errorf("geçersiz ev klasörü %q kabul edildi: %v", home, d)
		}
	}
}

func TestWebKitOwnID(t *testing.T) {
	cases := []struct {
		name   string
		window bool
		exe    string
		want   string
	}{
		{"kurulu .app", true, "/Applications/KeyLab.app/Contents/MacOS/KeyLab", "io.github.syhgorath.keylab"},
		{"USB .app", true, "/Volumes/USB/KeyLab.app/Contents/MacOS/KeyLab", "io.github.syhgorath.keylab"},
		{"adı değiştirilmiş .app", true, "/Volumes/USB/Anahtar 2.app/Contents/MacOS/KeyLab", "io.github.syhgorath.keylab"},
		{"deneme ikilisi (keylabtest)", true, "/h/Developer/keylabtest/keylab", "keylab"},
		{"paketsiz yayın ikilisi", true, "/Volumes/USB/keylab-4.0.16-darwin-arm64", "keylab-4.0.16-darwin-arm64"},
		{"büyük harfli ad", true, "/x/KeyLab", "KeyLab"},
		{"MacOS klasörü ama .app değil", true, "/x/Contents/MacOS/keylab", "keylab"},
		{"başka ada çevrilmiş ikili", true, "/x/Safari", ""},
		{"keylab ile başlamayan", true, "/x/benim-keylab", ""},
		{"pencere yok: .app", false, "/Applications/KeyLab.app/Contents/MacOS/KeyLab", ""},
		{"pencere yok: paketsiz (--chrome)", false, "/h/Developer/keylabtest/keylab", ""},
	}
	for _, c := range cases {
		if got := webKitOwnID(c.window, filepath.FromSlash(c.exe)); got != c.want {
			t.Errorf("%s: %q, beklenen %q", c.name, got, c.want)
		}
	}
}

func TestSweepWebKitTracesRemovesBothDirsInGuestMode(t *testing.T) {
	home, dirs := mkWebKit(t)
	other := mkWebKitID(t, home, "baska.uygulama")
	if n := sweepWebKitTraces(true, home, testBundleID, os.RemoveAll); n != 2 {
		t.Fatalf("silinen: %d", n)
	}
	for _, d := range dirs {
		if _, err := os.Lstat(d); err == nil {
			t.Fatalf("kaldı: %s", d)
		}
	}
	assertDirs(t, other, true, "başka uygulamanın verisine dokunuldu")
}

// Paketsiz deneme ikilisi yalnız kendi klasörünü siler: aynı Mac'teki kurulu KeyLab.app'in verisi kalır.
func TestSweepWebKitTracesBareBinaryKeepsInstalledAppData(t *testing.T) {
	home, installed := mkWebKit(t)
	own := mkWebKitID(t, home, "keylab")
	if n := sweepWebKitTraces(true, home, webKitOwnID(true, "/x/keylab"), os.RemoveAll); n != 2 {
		t.Fatalf("silinen: %d", n)
	}
	assertDirs(t, own, false, "kendi izi kaldı")
	assertDirs(t, installed, true, "kurulu uygulamanın verisi silindi")
}

func TestSweepWebKitTracesLeavesEverythingOutsideGuestMode(t *testing.T) {
	home, dirs := mkWebKit(t)
	if n := sweepWebKitTraces(false, home, testBundleID, os.RemoveAll); n != 0 {
		t.Fatalf("misafir değilken silindi: %d", n)
	}
	assertDirs(t, dirs, true, "kurulu modda veri silindi")
}

func TestSweepWebKitTracesNoopWhenMissingOrUnknown(t *testing.T) {
	if n := sweepWebKitTraces(true, t.TempDir(), testBundleID, os.RemoveAll); n != 0 {
		t.Fatalf("yokken: %d", n)
	}
	called := false
	rm := func(string) error { called = true; return nil }
	home, _ := mkWebKit(t)
	if n := sweepWebKitTraces(true, "", testBundleID, rm); n != 0 || called {
		t.Fatal("ev klasörü bilinmezken silme denendi")
	}
	if n := sweepWebKitTraces(true, home, "", rm); n != 0 || called {
		t.Fatal("pencere kimliği yokken silme denendi")
	}
}

// Sembolik bağ: bağın kendisi silinir, gösterdiği klasöre dokunulmaz.
func TestSweepWebKitTracesDoesNotFollowSymlink(t *testing.T) {
	home := t.TempDir()
	victim := t.TempDir()
	os.WriteFile(filepath.Join(victim, "onemli"), []byte("x"), 0o600)
	d := platform.WebKitTraceDirs(home, testBundleID)[0]
	os.MkdirAll(filepath.Dir(d), 0o755)
	if err := os.Symlink(victim, d); err != nil {
		t.Skip(err)
	}
	sweepWebKitTraces(true, home, testBundleID, os.RemoveAll)
	if _, err := os.Stat(filepath.Join(victim, "onemli")); err != nil {
		t.Fatal("bağın hedefi silindi")
	}
}

// Bağlantı: sweepWebKitNow gerçek derlemeyi ve gerçek ikili adını kullanır (sahte ev klasöründe). Test ikilisi paketsizdir ve adı "keylab" ile
// başlar: pencereli derlemede (-tags webview) yalnız kendi klasörü silinir, pencere yoksa hiçbir şey; kurulu uygulamanınki her iki durumda kalır.
func TestSweepWebKitNowTouchesOnlyOwnTraces(t *testing.T) {
	name := filepath.Base(selfExe())
	if !strings.HasPrefix(name, "keylab") {
		t.Skipf("test ikilisinin adı beklenmedik: %s", name)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KEYLAB_GUEST", "1")
	installed := mkWebKitID(t, home, testBundleID)
	own := mkWebKitID(t, home, name)
	sweepWebKitNow()
	assertDirs(t, own, !windowSupported(), "kendi klasörü (pencereliyse silinmeli, değilse kalmalı)")
	assertDirs(t, installed, true, "kurulu uygulamanın verisi silindi")
}
