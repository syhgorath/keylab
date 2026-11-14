package main

import (
	"os"
	"path/filepath"
	"testing"
)

func mkdirs(t *testing.T, ps ...string) {
	t.Helper()
	for _, p := range ps {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAppBase(t *testing.T) {
	for in, want := range map[string]string{
		"/Volumes/USB/KeyLab.app/Contents/MacOS/keylab-bin": "/Volumes/USB",
		"/Applications/KeyLab.app/Contents/MacOS/KeyLab":    "/Applications",
		"/Volumes/USB/sshpocket":                            "/Volumes/USB",
		"/x/Contents/MacOS/keylab":                          "/x/Contents/MacOS", // .app değil: ikilinin klasörü
		"/x/Foo.app/Contents/Other/keylab":                  "/x/Foo.app/Contents/Other",
	} {
		if got := appBase(in); got != want {
			t.Errorf("appBase(%q)=%q, beklenen %q", in, got, want)
		}
	}
}

func TestResolveServersDirOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	base := t.TempDir()
	exe := filepath.Join(base, "KeyLab.app", "Contents", "MacOS", "keylab-bin")
	work := t.TempDir()
	cwd := filepath.Join(work, "go")
	mkdirs(t, cwd)

	// hiçbiri yok → ~/.keylab/servers oluşturulacak
	if d, c := resolveServersDir("", "", exe, cwd); d != filepath.Join(home, ".keylab", "servers") || !c {
		t.Fatalf("boş durum: %q create=%v", d, c)
	}
	// eski geliştirici davranışı: ../servers
	mkdirs(t, filepath.Join(work, "servers"))
	if d, c := resolveServersDir("", "", exe, cwd); d != filepath.Join(work, "servers") || c {
		t.Fatalf("../servers: %q create=%v", d, c)
	}
	// ~/.keylab/servers varsa ../servers'ı geçer
	mkdirs(t, filepath.Join(home, ".keylab", "servers"))
	if d, _ := resolveServersDir("", "", exe, cwd); d != filepath.Join(home, ".keylab", "servers") {
		t.Fatalf("~/.keylab: %q", d)
	}
	// taşınabilir: .app'in yanındaki servers/ hepsini geçer
	mkdirs(t, filepath.Join(base, "servers"))
	if d, c := resolveServersDir("", "", exe, cwd); d != filepath.Join(base, "servers") || c {
		t.Fatalf("taşınabilir: %q create=%v", d, c)
	}
	// ortam değişkeni taşınabiliri geçer (varsa); olmayan yol yok sayılır
	envDir := t.TempDir()
	if d, _ := resolveServersDir("", envDir, exe, cwd); d != envDir {
		t.Fatalf("env: %q", d)
	}
	if d, _ := resolveServersDir("", filepath.Join(envDir, "yok"), exe, cwd); d != filepath.Join(base, "servers") {
		t.Fatalf("olmayan env yok sayılmalı: %q", d)
	}
	// bayrak her şeyi geçer (var olması gerekmez)
	if d, c := resolveServersDir("/baska/yol", envDir, exe, cwd); d != "/baska/yol" || c {
		t.Fatalf("bayrak: %q create=%v", d, c)
	}
}

func TestResolveServersDirSurvivesNoHome(t *testing.T) {
	t.Setenv("HOME", "")
	exe := filepath.Join(t.TempDir(), "keylab")
	if d, c := resolveServersDir("", "", exe, ""); d != "" || c {
		t.Fatalf("ev klasörü yokken: %q create=%v", d, c)
	}
}

func TestEnsureHomeDirsPerms(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".keylab")
	if err := ensureHomeDirs(filepath.Join(root, "servers"), filepath.Join(root, "components")); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(root, "servers"))
	if err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("servers izni: %v %v", st, err)
	}
	if !isDir(filepath.Join(root, "components")) {
		t.Fatal("components yok")
	}
	if err := ensureHomeDirs(filepath.Join(root, "servers"), filepath.Join(root, "components")); err != nil {
		t.Fatal("ikinci çağrı hata verdi:", err)
	}
}

func TestFindRootInstalledMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KEYLAB_ROOT", "")
	t.Setenv("SSHPOCKET_ROOT", "")
	// components.lock yok, ~/.keylab da yok → eski varsayılan (ikilinin üstü; test ikilisi için anlamsız ama ~/.keylab olmamalı)
	if r := findRoot(""); r == filepath.Join(home, ".keylab") {
		t.Fatal("~/.keylab yokken seçilmemeli")
	}
	mkdirs(t, filepath.Join(home, ".keylab"))
	if r := findRoot(""); r != filepath.Join(home, ".keylab") {
		t.Fatalf("klasör varken kök ~/.keylab olmalı: %q", r)
	}
	if r := findRoot("/x"); r != "/x" {
		t.Fatalf("bayrak: %q", r)
	}
}
