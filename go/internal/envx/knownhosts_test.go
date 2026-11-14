package envx

import (
	"strings"
	"testing"
)

func TestKnownHostsOption(t *testing.T) {
	t.Setenv("KEYLAB_KNOWN_HOSTS", "")
	t.Setenv("SSHPOCKET_KNOWN_HOSTS", "")
	if got := KnownHostsOption(); got != nil {
		t.Fatalf("ayarsız: %v", got)
	}
	t.Setenv("SSHPOCKET_KNOWN_HOSTS", "/eski/kh") // eski ad geriye uyumlu
	if got := strings.Join(KnownHostsOption(), " "); got != "-o UserKnownHostsFile=/eski/kh" {
		t.Fatalf("eski ad: %q", got)
	}
	t.Setenv("KEYLAB_KNOWN_HOSTS", "/yeni/kh") // yeni ad kazanır
	if got := strings.Join(KnownHostsOption(), " "); got != "-o UserKnownHostsFile=/yeni/kh" {
		t.Fatalf("yeni ad: %q", got)
	}
}
