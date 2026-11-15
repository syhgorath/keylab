package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIsPortableLayout(t *testing.T) {
	base := t.TempDir()
	exe := filepath.Join(base, "KeyLab.app", "Contents", "MacOS", "KeyLab")
	if !isPortableLayout(filepath.Join(base, "servers"), exe) || !isPortableLayout(filepath.Join(base, "servers")+"/", exe) {
		t.Error("taşınabilir düzen algılanmadı")
	}
	if isPortableLayout("/Users/x/.keylab/servers", exe) || isPortableLayout("/baska/servers", exe) {
		t.Error("taşınabilir olmayan düzen taşınabilir sayıldı")
	}
}

func TestSetupPortableTraces(t *testing.T) {
	for _, k := range []string{"KEYLAB_CACHE_DIR", "SSHPOCKET_CACHE_DIR", "KEYLAB_KNOWN_HOSTS", "SSHPOCKET_KNOWN_HOSTS"} {
		t.Setenv(k, "")
	}
	state := filepath.Join(t.TempDir(), "state")
	prof, cleanup, err := setupPortableTraces(state)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(prof)
	if err != nil || !st.IsDir() || st.Mode().Perm() != 0o700 {
		t.Fatalf("geçici klasör: %v %v", st, err)
	}
	// önbellek geçici klasörde, known_hosts USB'deki durum klasöründe; ikisi de alt süreçler için ortam değişkeni
	if got := os.Getenv("KEYLAB_CACHE_DIR"); got != filepath.Join(prof, "cache") {
		t.Errorf("KEYLAB_CACHE_DIR: %q", got)
	}
	if got := os.Getenv("KEYLAB_KNOWN_HOSTS"); got != filepath.Join(state, "known_hosts") {
		t.Errorf("KEYLAB_KNOWN_HOSTS: %q", got)
	}
	if !isDir(state) {
		t.Error("durum klasörü oluşmadı")
	}
	os.MkdirAll(filepath.Join(prof, "cache", "venv"), 0o700)
	cleanup()
	if _, err := os.Stat(prof); err == nil {
		t.Fatal("geçici klasör silinmedi")
	}
	cleanup() // ikinci çağrı zararsız
}

func TestSetupPortableTracesKeepsUserSettings(t *testing.T) {
	t.Setenv("KEYLAB_CACHE_DIR", "/benim/onbellek")
	t.Setenv("KEYLAB_KNOWN_HOSTS", "/benim/known_hosts")
	_, cleanup, err := setupPortableTraces(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if os.Getenv("KEYLAB_CACHE_DIR") != "/benim/onbellek" || os.Getenv("KEYLAB_KNOWN_HOSTS") != "/benim/known_hosts" {
		t.Fatal("kullanıcının verdiği değişkenler ezildi")
	}
}

func TestCleanupsRunOnce(t *testing.T) {
	var n atomic.Int32
	a := &App{}
	a.addCleanup("x", nil) // güvenli
	a.addCleanup("a", func() { n.Add(1) })
	a.addCleanup("b", func() { n.Add(10) })
	labels := a.runCleanups()
	if strings.Join(labels, ",") != "a,b" {
		t.Fatalf("etiketler: %v", labels)
	}
	if again := a.runCleanups(); len(again) != 0 {
		t.Fatalf("ikinci çalıştırma: %v", again)
	}
	if n.Load() != 11 {
		t.Fatalf("temizlikler: %d (11 beklenir, bir kez)", n.Load())
	}
}

func TestQuitRunsCleanupsBeforeExit(t *testing.T) {
	var order []string
	done := make(chan struct{}, 1)
	a := &App{serversDir: "/Users/x/servers"}
	a.addCleanup("t", func() { order = append(order, "temizlik") })
	a.exitFn = func() { order = append(order, "çıkış"); done <- struct{}{} }
	quitCall(a, `{}`, "POST")
	quitFinish(t, a) // süreç yalnız "Kapat" (ya da zamanaşımı) ile kapanır
	<-done
	if strings.Join(order, ",") != "temizlik,çıkış" {
		t.Fatalf("sıra: %v", order)
	}
}
