package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSecurity: PATH'in başına argümanlarını (ve stdin'i) günlüğe yazan sahte `security` koyar.
func fakeSecurity(t *testing.T, body string) string {
	t.Helper()
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "calls.log")
	script := "#!/bin/bash\necho \"argv: $*\" >> \"" + log + "\"\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, "security"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return log
}

func logOf(t *testing.T, p string) string { b, _ := os.ReadFile(p); return string(b) }

func TestKeychainServiceNameIsStable(t *testing.T) {
	if KeychainService != "USBMonitor" {
		t.Fatalf("servis adı değişti (eski Keychain kayıtları okunamaz olur): %q", KeychainService)
	}
}

func TestDefaultAndForUseServiceName(t *testing.T) {
	log := fakeSecurity(t, `exit 0`)
	Default().Has("A")
	For("").Has("B")
	For("Ozel").Has("C")
	got := strings.Split(strings.TrimSpace(logOf(t, log)), "\n")
	want := []string{
		"argv: find-generic-password -a A -s USBMonitor",
		"argv: find-generic-password -a B -s USBMonitor",
		"argv: find-generic-password -a C -s Ozel",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSetWritesValueOnStdinNotArgv(t *testing.T) {
	stdin := filepath.Join(t.TempDir(), "stdin.txt")
	log := fakeSecurity(t, `cat >> "`+stdin+`"`)
	if err := Default().Set("USBMonitor_WEB1_Passphrase", "p4ss w0rd"); err != nil {
		t.Fatal(err)
	}
	in, _ := os.ReadFile(stdin)
	if string(in) != "add-generic-password -U -a USBMonitor_WEB1_Passphrase -s USBMonitor -w p4ss w0rd\n" {
		t.Fatalf("stdin: %q", in)
	}
	if got := strings.TrimSpace(logOf(t, log)); got != "argv: -i" || strings.Contains(got, "p4ss") {
		t.Fatalf("argv (değer komut satırında olmamalı): %q", got)
	}
	// araç hata verirse hata döner
	fakeSecurity(t, "cat >/dev/null; exit 1")
	if err := Default().Set("A", "b"); err == nil {
		t.Fatal("hata yutuldu")
	}
}

func TestSetInteractiveArgvAndStdio(t *testing.T) {
	log := fakeSecurity(t, `read -r v; echo "istem" >&2; echo "tamam"; test "$v" = "typed"`)
	var out, errb bytes.Buffer
	if err := Default().SetInteractive("A", strings.NewReader("typed\n"), &out, &errb); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(logOf(t, log)); got != "argv: add-generic-password -U -a A -s USBMonitor -w" {
		t.Fatalf("argv: %q", got)
	}
	if !strings.Contains(out.String(), "tamam") || !strings.Contains(errb.String(), "istem") {
		t.Fatalf("stdio bağlanmadı: out=%q err=%q", out.String(), errb.String())
	}
	// araç hata verirse hata döner
	fakeSecurity(t, "exit 1")
	if err := Default().SetInteractive("A", strings.NewReader(""), &out, &errb); err == nil {
		t.Fatal("hata yutuldu")
	}
}

func TestAvailable(t *testing.T) {
	fakeSecurity(t, "exit 0")
	if !Default().Available() {
		t.Fatal("araç varken Available false")
	}
	t.Setenv("PATH", t.TempDir())
	if Default().Available() {
		t.Fatal("araç yokken Available true")
	}
}
