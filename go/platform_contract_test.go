package main

// Platform sözleşmesi (karakterizasyon): macOS'a özgü dış araç çağrılarının TAM biçimini sabitler. İleride bu çağrılar `internal/platform`
// arayüzlerinin macOS uygulamasına taşındığında davranışın birebir aynı kaldığını bu testler gösterir. Yalnız macOS'ta anlamlı olanlar
// diğer sistemlerde atlanır. (Keychain `security` sözleşmesi: internal/servers, internal/cli, internal/runner altındaki *_contract_test.go.)

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func needDarwin(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("macOS sözleşmesi")
	}
}

// fakeTool: PATH'in başına, argümanlarını FAKE_LOG'a yazan ve verilen betiği çalıştıran sahte bir araç koyar; günlük yolunu döner.
func fakeTool(t *testing.T, name, body string) string {
	t.Helper()
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "calls.log")
	script := "#!/bin/bash\necho \"$*\" >> \"" + log + "\"\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return log
}

func readLog(t *testing.T, p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

func TestContractSSHAddUsesAppleKeychainOnMac(t *testing.T) {
	needDarwin(t)
	needScript(t)
	srvs := t.TempDir()
	a := &App{serversDir: srvs, compDir: filepath.Join(t.TempDir(), "components"), stateDir: t.TempDir(), jobs: map[string]*Job{}, up: map[string]bool{}}
	if err := os.MkdirAll(filepath.Join(srvs, "kvm"), 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(srvs, "kvm", "server.env"), []byte("IP=192.0.2.10\nUSER=admin\nPORT=22\n"), 0o600)
	key := filepath.Join(srvs, "kvm", "id_ed25519_kvm")
	if err := os.WriteFile(key, []byte("fake-private-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	log := fakeTool(t, "ssh-add", "exit 0")
	job, code, err := a.startServerTask("sshadd", "kvm")
	if err != nil {
		t.Fatalf("başlamadı (%d): %v", code, err)
	}
	waitFor(t, a, "bitiş", 10*time.Second, func() bool { return job.Done })
	if got := strings.TrimSpace(readLog(t, log)); got != "--apple-use-keychain "+key {
		t.Fatalf("ssh-add argümanları: %q (beklenen --apple-use-keychain <anahtar>)", got)
	}
}

func TestContractPtyArgvMac(t *testing.T) {
	needDarwin(t)
	got := ptyArgv([]string{"ssh-add", "--apple-use-keychain", "/k ey"})
	want := []string{"script", "-q", "/dev/null", "ssh-add", "--apple-use-keychain", "/k ey"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("ptyArgv macOS: %q", got)
	}
}

func TestContractNativePasswordDialogMac(t *testing.T) {
	needDarwin(t)
	// osascript yok → pencere kullanılamaz
	t.Setenv("PATH", t.TempDir())
	if askpassAvailable() {
		t.Fatal("osascript yokken parola penceresi 'var' sayıldı")
	}
	t.Setenv("PATH", os.Getenv("PATH")+":/bin:/usr/bin")

	// osascript var: tek sahte araç hem kullanılabilirlik hem çağrı sözleşmesi
	log := fakeTool(t, "osascript", `printf 'gizli-parola\n'`)
	if !askpassAvailable() {
		t.Fatal("osascript varken parola penceresi yok sayıldı")
	}
	got, err := askPassNative("Sudo parolası \"web1\": ")
	if err != nil || string(got) != "gizli-parola" {
		t.Fatalf("parola: %q %v", got, err)
	}
	line := readLog(t, log)
	// parola KOMUT SATIRINDA değil, yalnız osascript çıktısında gelir; betik istemi (kaçışlı) ve gizli girişi içerir
	if !strings.HasPrefix(line, "-e ") || !strings.Contains(line, `with title "sshpocket"`) || !strings.Contains(line, "with hidden answer") ||
		!strings.Contains(line, `buttons {"İptal", "Tamam"}`) || !strings.Contains(line, `\"web1\"`) || strings.Contains(line, "gizli-parola") {
		t.Fatalf("osascript çağrısı: %q", line)
	}
	// iptal (osascript hata kodu) → anlamlı hata, parola yok
	fakeTool(t, "osascript", "exit 1")
	if b, err := askPassNative("x"); err == nil || b != nil || !strings.Contains(err.Error(), "iptal") {
		t.Fatalf("iptal: %q %v", b, err)
	}
}
