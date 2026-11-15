//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/platform"
	"keylab/internal/servers"
)

// Uçtan uca (gerçek araçlarla): parolalı gerçek anahtar + gerçek özel ssh-agent + gerçek ssh-add/askpass. Kasa → ajana yükleme → kilit → ajan boş.
func TestVaultRealAgentEndToEnd(t *testing.T) {
	for _, tool := range []string{"ssh-keygen", "ssh-add", "ssh-agent"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s yok", tool)
		}
	}
	dir, err := os.MkdirTemp("/tmp", "kl-e2e-") // kısa yol: unix soket sınırı
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock, stop, err := platform.StartPrivateAgent(dir)
	if err != nil {
		t.Skipf("özel ajan başlamadı: %v", err)
	}
	t.Cleanup(stop)
	t.Setenv("SSH_AUTH_SOCK", sock)

	f := newVaultFx(t)
	key := servers.KeyPath(f.a.serversDir, "web1")
	os.Remove(key)
	const pass = "gerçek-passphrase-π"
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", pass, "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v %s", err, out)
	}
	f.m.tmpDir = dir
	f.m.addKey = f.m.addKeyViaSSHAdd // sahte değil: gerçek ssh-add
	f.m.clearAgent = func() { _ = exec.Command("ssh-add", "-D").Run() }
	listed := func() string { o, _ := exec.Command("ssh-add", "-l").CombinedOutput(); return string(o) }
	fp := fingerprintOfKey(t, key)

	if err := f.m.create("ana-parola-1"); err != nil {
		t.Fatal(err)
	}
	// yanlış passphrase gerçek ssh-add tarafından reddedilir, kaydedilmez
	if err := f.m.setPassphrase("web1", "yanlış-passphrase"); err == nil {
		t.Fatal("yanlış passphrase kabul edildi")
	}
	if strings.Contains(listed(), fp) {
		t.Fatal("yanlış passphrase ile anahtar ajana girdi")
	}
	if err := f.m.setPassphrase("web1", pass); err != nil {
		t.Fatalf("doğru passphrase: %v", err)
	}
	if !strings.Contains(listed(), fp) {
		t.Fatalf("anahtar ajanda yok: %s", listed())
	}
	// geçici passphrase dosyaları kalmadı
	if ents, _ := filepath.Glob(filepath.Join(dir, "kl-ap-*")); len(ents) != 0 {
		t.Fatalf("passphrase dosyası kaldı: %v", ents)
	}
	f.m.lock()
	if strings.Contains(listed(), fp) {
		t.Fatal("kilitten sonra anahtar ajanda kaldı")
	}
	if _, failed, err := f.m.unlock("ana-parola-1"); err != nil || len(failed) != 0 {
		t.Fatalf("unlock: %v %v", failed, err)
	}
	if !strings.Contains(listed(), fp) {
		t.Fatal("unlock anahtarı ajana yüklemedi")
	}
	// kasa dosyasında düz passphrase yok
	b, _ := os.ReadFile(f.m.path)
	if strings.Contains(string(b), pass) {
		t.Fatal("kasa dosyasında düz passphrase var")
	}
}

func fingerprintOfKey(t *testing.T, key string) string {
	t.Helper()
	o, err := exec.Command("ssh-keygen", "-lf", key).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(o))[1]
}
