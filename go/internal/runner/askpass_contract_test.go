package runner

// SSH_ASKPASS sözleşmesi (karakterizasyon): ssh-add bu ikiliyi askpass olarak çağırır; parola Keychain'den `security find-generic-password -w` ile
// okunup stdout'a yazılır. Yanlış parolada ssh-add sonsuz yeniden deneyeceği için askpass YALNIZ BİR KEZ yanıt verir (durum dosyası).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeSecurityFor(t *testing.T, body string) string {
	t.Helper()
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "calls.log")
	script := "#!/bin/bash\necho \"$*\" >> \"" + log + "\"\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, "security"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return log
}

func TestAskpassContract(t *testing.T) {
	log := fakeSecurityFor(t, `echo "keychain-parola"`)
	state := filepath.Join(t.TempDir(), "durum")
	t.Setenv("SSHPOCKET_ASKPASS", "1")
	t.Setenv("ASKPASS_ACCOUNT", "USBMonitor_WEB1_Passphrase")
	t.Setenv("ASKPASS_SERVICE", "")
	t.Setenv(askpassEnv, state)

	// 1. çağrı: parola yazılır
	var out bytes.Buffer
	if h, code := AskpassMain(&out); !h || code != 0 || out.String() != "keychain-parola\n" {
		t.Fatalf("1. çağrı: h=%v code=%d out=%q", h, code, out.String())
	}
	if got := strings.TrimSpace(readFile(t, log)); got != "find-generic-password -a USBMonitor_WEB1_Passphrase -s USBMonitor -w" {
		t.Fatalf("security argümanları: %q", got)
	}
	// 2. çağrı (durum dosyası var): yalnız boş satır; Keychain'e tekrar gidilmez
	out.Reset()
	if h, code := AskpassMain(&out); !h || code != 0 || out.String() != "\n" {
		t.Fatalf("2. çağrı: h=%v code=%d out=%q", h, code, out.String())
	}
	if n := strings.Count(readFile(t, log), "find-generic-password"); n != 1 {
		t.Fatalf("Keychain %d kez soruldu (1 olmalı)", n)
	}
}

func TestAskpassContractEdges(t *testing.T) {
	// etkin değilse hiçbir şey yapmaz
	t.Setenv("SSHPOCKET_ASKPASS", "")
	if h, _ := AskpassMain(&bytes.Buffer{}); h {
		t.Fatal("etkin değilken işledi")
	}
	t.Setenv("SSHPOCKET_ASKPASS", "1")
	t.Setenv(askpassEnv, "")
	// hesap yok → hata kodu 1
	t.Setenv("ASKPASS_ACCOUNT", "")
	if h, code := AskpassMain(&bytes.Buffer{}); !h || code != 1 {
		t.Fatalf("hesapsız: h=%v code=%d", h, code)
	}
	// özel servis adı verilebilir; Keychain'de yoksa (araç hata verir) kod 1 ve parola yazılmaz
	log := fakeSecurityFor(t, "exit 44")
	t.Setenv("ASKPASS_ACCOUNT", "A")
	t.Setenv("ASKPASS_SERVICE", "OzelServis")
	var out bytes.Buffer
	if h, code := AskpassMain(&out); !h || code != 1 || out.Len() != 0 {
		t.Fatalf("kayıt yok: h=%v code=%d out=%q", h, code, out.String())
	}
	if got := strings.TrimSpace(readFile(t, log)); got != "find-generic-password -a A -s OzelServis -w" {
		t.Fatalf("özel servis argümanları: %q", got)
	}
}

func readFile(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
