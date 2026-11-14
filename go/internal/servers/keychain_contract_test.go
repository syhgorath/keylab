package servers

// Keychain sözleşmesi (karakterizasyon): macOS `security` çağrılarının TAM argümanları. `internal/platform` Secrets arayüzüne taşınırken
// macOS uygulamasının aynı çağrıları yapması gerekir; bu testler bunu sabitler. Parola/değer komut satırında GEÇMEZ (yalnız -w ile çıktıda).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contractSecurity(t *testing.T, body string) string {
	t.Helper()
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "calls.log")
	script := "#!/bin/bash\necho \"$*\" >> \"" + log + "\"\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, "security"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return log
}

func calls(t *testing.T, log string) []string {
	b, _ := os.ReadFile(log)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestKeychainContractArgv(t *testing.T) {
	if KeychainService != "USBMonitor" {
		t.Fatalf("servis adı değişti (eski kayıtlar okunamaz olur): %q", KeychainService)
	}
	const acct = "USBMonitor_WEB1_Passphrase"
	log := contractSecurity(t, `case "$1" in find-generic-password) case " $* " in *" -w "*) echo "sir-deger";; esac; exit 0;; delete-generic-password) exit 0;; esac; exit 1`)

	// Find: değeri -w ile okur ve sondaki satır sonunu atar
	if v, ok := (Keychain{}).Find(acct); !ok || v != "sir-deger" {
		t.Fatalf("Find: %q %v", v, ok)
	}
	// Has: -w YOK (değer okunmaz)
	if !(Keychain{}).Has(acct) {
		t.Fatal("Has false")
	}
	// Delete
	if !(Keychain{}).Delete(acct) {
		t.Fatal("Delete false")
	}
	got := calls(t, log)
	want := []string{
		"find-generic-password -a " + acct + " -s USBMonitor -w",
		"find-generic-password -a " + acct + " -s USBMonitor",
		"delete-generic-password -a " + acct + " -s USBMonitor",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("security çağrıları:\n got  %q\n want %q", got, want)
	}
}

func TestKeychainContractFailuresAndMissingTool(t *testing.T) {
	// araç hata verirse: yok sayılır (Find boş+false, Has false, Delete false)
	contractSecurity(t, "exit 44")
	if v, ok := (Keychain{}).Find("A"); ok || v != "" {
		t.Errorf("Find hata: %q %v", v, ok)
	}
	if (Keychain{}).Has("A") || (Keychain{}).Delete("A") {
		t.Error("hata kodunda Has/Delete true döndü")
	}
	// araç hiç yoksa (macOS dışı): sessizce yok
	t.Setenv("PATH", t.TempDir())
	if _, ok := (Keychain{}).Find("A"); ok || (Keychain{}).Has("A") || (Keychain{}).Delete("A") {
		t.Error("araç yokken true döndü")
	}
}
