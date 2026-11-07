package main

import "testing"

func TestGoRunDefaultAndSwitches(t *testing.T) {
	for _, c := range []struct {
		shell, goRun string
		want         bool
	}{
		{"", "", true},    // varsayılan: Go
		{"1", "", false},  // kabuğa dönüş
		{"0", "", true},   // yalnızca "1" kabuğa döndürür
		{"", "0", false},  // eski anahtarla kapatma
		{"", "1", true},   // eski anahtar hâlâ zararsız
		{"1", "1", false}, // kabuk isteği kazanır
	} {
		t.Setenv("SSHPOCKET_SHELL_RUN", c.shell)
		t.Setenv("SSHPOCKET_GO_RUN", c.goRun)
		if got := goRunEnabled(); got != c.want {
			t.Errorf("SHELL_RUN=%q GO_RUN=%q: %v != %v", c.shell, c.goRun, got, c.want)
		}
	}
}
