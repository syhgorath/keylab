//go:build !windows

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Yeniden başlatma yardımcısı: uygulama (PID) YAŞARKEN yeni örneği AÇMAZ; süreç bitince `open -n <app>` çalıştırır.
func TestRelaunchWaitsForTheProcessThenOpensTheApp(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "open.log")
	os.WriteFile(filepath.Join(bin, "open"), []byte("#!/bin/sh\necho \"open $*\" >> \""+log+"\"\n"), 0o755)
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	app := exec.Command("/bin/sleep", "60")
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Process.Kill(); app.Wait() })
	if err := relaunchAfter("/Volumes/My USB/KeyLab.app", app.Process.Pid); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2500 * time.Millisecond)
	if b, _ := os.ReadFile(log); len(b) != 0 {
		t.Fatalf("uygulama hâlâ yaşarken yeniden açıldı: %q", b)
	}
	app.Process.Kill()
	app.Wait()
	var got string
	for i := 0; i < 80 && got == ""; i++ {
		time.Sleep(100 * time.Millisecond)
		b, _ := os.ReadFile(log)
		got = string(b)
	}
	if strings.TrimSpace(got) != "open -n /Volumes/My USB/KeyLab.app" {
		t.Fatalf("süreç bitince beklenen çağrı yapılmadı (boşluklu yol dahil): %q", got)
	}
}
