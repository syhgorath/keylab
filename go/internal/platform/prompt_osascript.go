package platform

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// OSAScriptPrompt: macOS parola penceresi (osascript + System Events; öne gelir, İptal → osascript hata kodu verir).
type OSAScriptPrompt struct{}

// SanitizePrompt: pencerede gösterilecek istem metni (kontrol karakterleri atılır, en fazla 160 karakter; boşsa "Parola").
func SanitizePrompt(p string) string {
	var b strings.Builder
	for _, r := range p {
		if r >= 32 && r != 127 {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if rs := []rune(out); len(rs) > 160 {
		out = string(rs[:160])
	}
	if out == "" {
		out = "Parola"
	}
	return out
}

// AppleScriptQuote: AppleScript metin değişmezi için kaçışlar (\ ve ").
func AppleScriptQuote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

// askpassScript: gizli yanıtlı parola penceresi betiği.
func askpassScript(prompt string) string {
	return "tell application \"System Events\"\n" +
		"activate\n" +
		"set r to text returned of (display dialog " + AppleScriptQuote(SanitizePrompt(prompt)) + " with title \"sshpocket\" default answer \"\" with hidden answer buttons {\"İptal\", \"Tamam\"} default button \"Tamam\" cancel button \"İptal\" with icon caution)\n" +
		"end tell\n" +
		"return r"
}

func (OSAScriptPrompt) Available() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("osascript")
	return err == nil
}

// Secret: parolayı sistem penceresinden alır; argümanla değil osascript'in çıktısıyla gelir.
func (OSAScriptPrompt) Secret(prompt string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e", askpassScript(prompt)).Output()
	if err != nil {
		return nil, fmt.Errorf("parola penceresi iptal edildi")
	}
	return []byte(strings.TrimSuffix(string(out), "\n")), nil
}
