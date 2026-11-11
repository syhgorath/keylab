package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func prepFixture(t *testing.T, withKey bool) (*App, Server, string) {
	t.Helper()
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "security"), []byte(fakeSecurity), 0o755)
	os.WriteFile(filepath.Join(bin, "ssh-add"), []byte("#!/bin/sh\n[ \"$1\" = -l ] && { [ -s \"$FAKE_AGENT\" ] && cat \"$FAKE_AGENT\" || { echo 'The agent has no identities.'; exit 1; }; }\n"), 0o755)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_KC", filepath.Join(t.TempDir(), "kc"))
	agent := filepath.Join(t.TempDir(), "agent")
	t.Setenv("FAKE_AGENT", agent)
	srv := filepath.Join(t.TempDir(), "servers")
	d := filepath.Join(srv, "web1")
	os.MkdirAll(d, 0o700)
	os.WriteFile(filepath.Join(d, "server.env"), []byte("IP=192.0.2.9\nUSER=u\nPORT=22\n"), 0o600)
	if withKey {
		if o, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", filepath.Join(d, "id_ed25519_web1")).CombinedOutput(); err != nil {
			t.Fatalf("keygen: %v %s", err, o)
		}
	}
	a := &App{serversDir: srv, jobs: map[string]*Job{}, up: map[string]bool{}}
	s, _ := a.findServer("web1")
	return a, s, agent
}

func TestPrepState(t *testing.T) {
	// anahtar yok
	a, s, _ := prepFixture(t, false)
	if st := a.prepState(s); st.Key || st.Agent || st.Passphrase {
		t.Errorf("anahtar yokken hepsi false olmalı: %+v", st)
	}
	// anahtar yokken Keychain'de passphrase kaydı olsa bile adım "hazır" sayılmaz
	os.WriteFile(os.Getenv("FAKE_KC"), []byte("USBMonitor_WEB1_Passphrase=x\n"), 0o600)
	if st := a.prepState(s); st.Passphrase {
		t.Errorf("anahtarsız sunucuda Passphrase=true olmamalı: %+v", st)
	}

	// anahtar var, ajanda ve Keychain'de yok
	a, s, agent := prepFixture(t, true)
	if st := a.prepState(s); !st.Key || st.Agent || st.Passphrase {
		t.Errorf("yalnız Key=true beklenirdi: %+v", st)
	}
	// ajana eklenmiş görünsün: ssh-add -l, anahtarın parmak izini içersin
	fp, _ := exec.Command("ssh-keygen", "-lf", s.keyPath+".pub").Output()
	os.WriteFile(agent, fp, 0o600)
	if st := a.prepState(s); !st.Agent {
		t.Errorf("ajandaki anahtar Agent=true olmalı: %+v", st)
	}
	// başka bir anahtarın parmak izi: Agent=false kalmalı
	os.WriteFile(agent, []byte("256 SHA256:baskaBirParmakIzi comment (ED25519)\n"), 0o600)
	if st := a.prepState(s); st.Agent {
		t.Errorf("başka anahtar Agent=true yapmamalı: %+v", st)
	}
	// Keychain'de passphrase kaydı
	kc := os.Getenv("FAKE_KC")
	os.WriteFile(kc, []byte("USBMonitor_WEB1_Passphrase=gizli\n"), 0o600)
	if st := a.prepState(s); !st.Passphrase {
		t.Errorf("kayıtlı passphrase Passphrase=true olmalı: %+v", st)
	}
	// başka sunucunun kaydı bu sunucuyu etkilemez
	os.WriteFile(kc, []byte("USBMonitor_DIGER_Passphrase=x\n"), 0o600)
	if st := a.prepState(s); st.Passphrase {
		t.Errorf("başka sunucunun kaydı sayılmamalı: %+v", st)
	}
	// passphrase değeri hiçbir alana sızmaz
	if b, _ := os.ReadFile(kc); !strings.Contains(string(b), "DIGER") {
		t.Fatal("fixture bozuldu")
	}
}
