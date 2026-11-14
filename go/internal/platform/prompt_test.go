package platform

import (
	"os/exec"
	"strings"
	"testing"
)

func TestSanitizePrompt(t *testing.T) {
	if got := SanitizePrompt("BECOME password:\x00\x1b[31m "); strings.ContainsAny(got, "\x00\x1b") {
		t.Errorf("kontrol karakteri kaldı: %q", got)
	}
	if got := SanitizePrompt("a\x7fb"); got != "ab" {
		t.Errorf("DEL karakteri kaldı: %q", got)
	}
	if got := SanitizePrompt(strings.Repeat("a", 500)); len([]rune(got)) != 160 {
		t.Errorf("uzunluk %d", len([]rune(got)))
	}
	if got := SanitizePrompt("  "); got != "Parola" {
		t.Errorf("boş istem: %q", got)
	}
}

func TestAppleScriptQuote(t *testing.T) {
	if got := AppleScriptQuote(`a"b\c`); got != `"a\"b\\c"` {
		t.Errorf("AppleScriptQuote: %q", got)
	}
}

// parola penceresi betiği derlenir mi (çalıştırılmaz: ekranda pencere açmamak için)
func TestAskpassScriptCompiles(t *testing.T) {
	if _, err := exec.LookPath("osacompile"); err != nil {
		t.Skip("osacompile yok")
	}
	out := t.TempDir() + "/a.scpt"
	for _, prompt := range []string{"BECOME password:", `kötü "tırnak" \ ve 'tek'`, "İştah çalışma ğüşöç"} {
		if b, err := exec.Command("osacompile", "-o", out, "-e", askpassScript(prompt)).CombinedOutput(); err != nil {
			t.Fatalf("derlenmedi (%q): %v %s", prompt, err, strings.TrimSpace(string(b)))
		}
	}
}
