package platform

import (
	"io"
	"os/exec"
	"strings"
)

// SecurityKeychain: macOS Keychain'i `security` aracıyla (testte PATH'teki sahte araçla değiştirilir). Sıfır değer (Service boş) KeychainService kullanır.
type SecurityKeychain struct{ Service string }

func (k SecurityKeychain) service() string {
	if k.Service == "" {
		return KeychainService
	}
	return k.Service
}

// Find: security find-generic-password -w.
func (k SecurityKeychain) Find(account string) (string, bool) {
	out, err := exec.Command("security", "find-generic-password", "-a", account, "-s", k.service(), "-w").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSuffix(string(out), "\n"), true
}

// Has: kayıt var mı (değeri okumadan).
func (k SecurityKeychain) Has(account string) bool {
	return exec.Command("security", "find-generic-password", "-a", account, "-s", k.service()).Run() == nil
}

// Delete: kaydı siler; yoksa false.
func (k SecurityKeychain) Delete(account string) bool {
	return exec.Command("security", "delete-generic-password", "-a", account, "-s", k.service()).Run() == nil
}

// Set: değeri `security -i` ile komut satırında GÖSTERMEDEN (stdin'den) yazar.
func (k SecurityKeychain) Set(account, value string) error {
	cmd := exec.Command("security", "-i")
	cmd.Stdin = strings.NewReader("add-generic-password -U -a " + account + " -s " + k.service() + " -w " + value + "\n")
	return cmd.Run()
}

// SetInteractive: `-w` değersiz verilir; security parolayı terminalden kendisi sorar ("password data for new item:").
func (k SecurityKeychain) SetInteractive(account string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command("security", "add-generic-password", "-U", "-a", account, "-s", k.service(), "-w")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	return cmd.Run()
}

// Available: `security` aracı var mı?
func (k SecurityKeychain) Available() bool {
	_, err := exec.LookPath("security")
	return err == nil
}
