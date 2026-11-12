package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// -set: yalnız ad=değer kabul edilir; satır sonu/NUL ve geçersiz adlar reddedilir (çalıştırmadan, kullanım hatası: 2).
func TestRunCommandRejectsBadSet(t *testing.T) {
	for _, bad := range []string{"bozuk", "=x", "A B=1", "ad=de\ner", "ad=de\x00er", "../x=1"} {
		if code := runCommand([]string{"-set", bad, "-servers", t.TempDir(), "-components", t.TempDir(), "p", "a", "s"}); code != 2 {
			t.Errorf("-set %q: çıkış %d (2 bekleniyordu)", bad, code)
		}
	}
}
