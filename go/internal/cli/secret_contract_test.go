package cli

// Keychain yazma sözleşmesi (karakterizasyon): değer `security -i` ile komut satırında GÖRÜNMEDEN, standart girdiden yazılır.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSecretContract(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls.log")
	script := "#!/bin/bash\necho \"argv: $*\" >> \"" + log + "\"\ncat >> \"" + log + "\"\n"
	if err := os.WriteFile(filepath.Join(bin, "security"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	s := &Servers{}
	if err := s.storeSecret("USBMonitor_WEB1_Passphrase", "p4ss w0rd"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	got := string(b)
	// argv yalnızca "-i": değer komut satırında YOK; yazma komutu stdin'den gelir
	if !strings.HasPrefix(got, "argv: -i\n") {
		t.Fatalf("argv: %q", got)
	}
	if want := "add-generic-password -U -a USBMonitor_WEB1_Passphrase -s USBMonitor -w p4ss w0rd\n"; !strings.HasSuffix(got, want) {
		t.Fatalf("stdin: %q", got)
	}
	if strings.Contains(strings.SplitN(got, "\n", 2)[0], "p4ss") {
		t.Fatal("değer komut satırında göründü")
	}
	// araç hata verirse hata döner
	os.WriteFile(filepath.Join(bin, "security"), []byte("#!/bin/bash\ncat >/dev/null; exit 1\n"), 0o755)
	if err := s.storeSecret("A", "b"); err == nil {
		t.Fatal("hata yutuldu")
	}
}
