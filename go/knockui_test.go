package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func knockServer(t *testing.T, knock bool) (*App, string) {
	t.Helper()
	srv := filepath.Join(t.TempDir(), "servers")
	d := filepath.Join(srv, "kn")
	if err := os.MkdirAll(d, 0o700); err != nil {
		t.Fatal(err)
	}
	env := "IP=192.0.2.9\nUSER=u\nPORT=22\n"
	if knock {
		env += "KNOCK=keychain\n"
	}
	if err := os.WriteFile(filepath.Join(d, "server.env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	return &App{serversDir: srv, jobs: map[string]*Job{}, up: map[string]bool{}}, d
}

func TestServerKnockFlag(t *testing.T) {
	for _, knock := range []bool{false, true} {
		a, _ := knockServer(t, knock)
		s, ok := a.findServer("kn")
		if !ok {
			t.Fatal("sunucu bulunamadı")
		}
		if s.Knock != knock {
			t.Errorf("Knock = %v, istenen %v", s.Knock, knock)
		}
	}
}

func TestCopyCmd(t *testing.T) {
	a, _ := knockServer(t, false)
	s, _ := a.findServer("kn")
	if got := a.copyCmd(s); got != s.sshCmd() || strings.Contains(got, "knock-send") {
		t.Errorf("knock yokken yalnız ssh olmalı: %q", got)
	}

	a, _ = knockServer(t, true)
	s, _ = a.findServer("kn")
	got := a.copyCmd(s)
	if !strings.Contains(got, "knock-send 'kn' && ssh ") {
		t.Errorf("knock etkinken önce knock-send, sonra ssh olmalı: %q", got)
	}
	if !strings.Contains(got, "-servers '"+a.serversDir+"'") {
		t.Errorf("sunucu klasörü komutta olmalı: %q", got)
	}
	if !strings.HasSuffix(got, s.sshCmd()) {
		t.Errorf("komut ssh komutuyla bitmeli: %q", got)
	}
}
