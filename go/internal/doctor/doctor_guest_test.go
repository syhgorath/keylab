package doctor

import (
	"bytes"
	"strings"
	"testing"

	"keylab/internal/platform"
)

// Misafir (taşınabilir) modda doctor Keychain aracı aramaz; özel ajan satırı gösterir.
func TestDoctorGuestModeLine(t *testing.T) {
	t.Setenv("SSHPOCKET_DOCTOR_OFFLINE", "1")
	t.Setenv("PATH", t.TempDir()) // security yok: misafirde bu bir HATA olmamalı
	platform.SetGuestMode(true)
	t.Cleanup(func() { platform.SetGuestMode(false) })
	var b bytes.Buffer
	Run(&b, Options{})
	out := b.String()
	if !strings.Contains(out, "taşınabilir mod: Keychain kullanılmıyor") || strings.Contains(out, "security (Keychain) bulunamadı") {
		t.Fatalf("misafir doctor çıktısı:\n%s", out)
	}
}
