package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestProbeHonoursSettings(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer l.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	conf := filepath.Join(t.TempDir(), "sshpocket.conf")
	t.Setenv("SSHPOCKET_CONF", conf)
	srv := []Server{{Name: "x", Host: "127.0.0.1", Port: port, Valid: true}}

	os.WriteFile(conf, []byte("STATUS=off\n"), 0o644)
	a := testApp()
	a.compDir = t.TempDir() + "/components"
	a.probe(srv)
	if _, ok := a.up["x"]; ok {
		t.Error("STATUS=off iken yoklama yapıldı")
	}
	os.WriteFile(conf, []byte("STATUS=on\nSTATUS_TIMEOUT=2\n"), 0o644)
	a.probe(srv)
	if !a.up["x"] {
		t.Error("açık port açık görünmedi")
	}
	if got := confFile("/kok"); got != conf {
		t.Errorf("SSHPOCKET_CONF yok sayıldı: %s", got)
	}
	t.Setenv("SSHPOCKET_CONF", "")
	if got := confFile("/kok"); got != "/kok/sshpocket.conf" {
		t.Errorf("varsayılan yol: %s", got)
	}
}

func TestSettingsCommand(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "c.conf")
	t.Setenv("SSHPOCKET_CONF", conf)
	if c := settingsCommand([]string{"set", "STATUS_TIMEOUT", "4"}); c != 0 {
		t.Fatalf("set: %d", c)
	}
	if c := settingsCommand([]string{"set", "STATUS_TIMEOUT", "9"}); c == 0 {
		t.Error("geçersiz değer kabul edildi")
	}
	if c := settingsCommand([]string{"get", "NOPE"}); c == 0 {
		t.Error("bilinmeyen anahtar kabul edildi")
	}
	if b, _ := os.ReadFile(conf); !strings.Contains(string(b), "STATUS_TIMEOUT=4") {
		t.Errorf("dosya: %q", b)
	}
}
