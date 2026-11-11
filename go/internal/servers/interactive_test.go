package servers

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fakeSecurity = `#!/bin/bash
S="$FAKE_KC"
case "$1" in
  find-generic-password) grep -qxF -- "$3" "$S" 2>/dev/null; exit $? ;;
  delete-generic-password) grep -qxF -- "$3" "$S" 2>/dev/null || exit 44; grep -vxF -- "$3" "$S" > "$S.n"; mv "$S.n" "$S"; exit 0 ;;
esac
exit 1
`

func fakeTools(t *testing.T) string {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "security"), []byte(fakeSecurity), 0o755)
	os.WriteFile(filepath.Join(d, "ssh"), []byte("#!/bin/bash\nexit ${FAKE_SSH_RC:-255}\n"), 0o755)
	os.WriteFile(filepath.Join(d, "ssh-copy-id"), []byte("#!/bin/bash\necho \"copy-id $*\"\nexit ${FAKE_COPY_RC:-1}\n"), 0o755)
	t.Setenv("PATH", d+":"+os.Getenv("PATH"))
	return d
}

var promptRe = []string{
	"Bu public key bu sunucuya eklensin mi? (y/n) ",
	"Girişte başka bir key dosyası kullanılsın mı? (yol girin, Enter = agent/parola) ",
}

func stripPrompts(s string, extra ...string) string {
	for _, p := range append(promptRe, extra...) {
		s = strings.ReplaceAll(s, p, "")
	}
	return s
}

func goKeyCopy(stdin string, user, ip, port, key string) (int, string) {
	var out, errb bytes.Buffer
	c := Console{In: bufio.NewReader(strings.NewReader(stdin)), Out: &out, Err: &errb}
	code := KeyCopy(c, user, ip, port, key)
	return code, out.String() + "|ERR|" + strings.TrimRight(stripPrompts(errb.String()), "\n")
}

func TestKeyCopyMatchesShell(t *testing.T) {
	r := root(t)
	fakeTools(t)
	dir := t.TempDir()
	key := filepath.Join(dir, "id")
	if o, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, o)
	}
	other := filepath.Join(dir, "baska")
	os.WriteFile(other, []byte("x"), 0o600)
	type sc struct {
		name, stdin, user string
		keyPath           string
		dropPub           bool
		sshRC, copyRC     string
	}
	for _, s := range []sc{
		{"anahtar yok", "y\n", "u", filepath.Join(dir, "yok"), false, "255", "0"},
		{"iptal n", "n\n", "u", key, false, "255", "0"},
		{"girdi yok", "", "u", key, false, "255", "0"},
		{"zaten yetkili", "y\n", "u", key, false, "0", "0"},
		{"basari", "y\n\n", "u", key, false, "255", "0"},
		{"root basari", "y\n\n", "root", key, false, "255", "0"},
		{"basarisiz + elle ekleme", "y\n\n", "u", key, false, "255", "1"},
		{"boot yok dosya", "y\n/yok/dosya\n", "u", key, false, "255", "0"},
		{"boot var", "y\n" + other + "\n", "u", key, false, "255", "0"},
		{"pub yok türet", "y\n\n", "u", key, true, "255", "0"},
		{"Y büyük", "Y\n\n", "u", key, false, "255", "0"},
	} {
		t.Setenv("FAKE_SSH_RC", s.sshRC)
		t.Setenv("FAKE_COPY_RC", s.copyRC)
		if s.dropPub {
			os.Remove(key + ".pub")
		}
		sh1 := shell(t, r, []string{"FAKE_SSH_RC=" + s.sshRC, "FAKE_COPY_RC=" + s.copyRC}, "ssh_copy_key", s.user, "10.0.0.5", "22", s.keyPath)
		if s.dropPub {
			os.Remove(key + ".pub")
		}
		_ = sh1
		// kabuk stdin'i: shell() yardımcısı stdin vermiyor; doğrudan çalıştır
		shOut := runShellStdin(t, r, s.stdin, []string{"FAKE_SSH_RC=" + s.sshRC, "FAKE_COPY_RC=" + s.copyRC}, "ssh_copy_key", s.user, "10.0.0.5", "22", s.keyPath)
		if s.dropPub {
			os.Remove(key + ".pub")
		}
		code, got := goKeyCopy(s.stdin, s.user, "10.0.0.5", "22", s.keyPath)
		want := fmt.Sprintf("%s|ERR|%s", shOut.out, strings.TrimRight(stripPrompts(shOut.err), "\n"))
		if code != shOut.code || got != want {
			t.Errorf("%s: kabuk=%d go=%d\nkabuk: %q\ngo:    %q", s.name, shOut.code, code, want, got)
		}
		if _, err := os.Stat(key + ".pub"); err != nil {
			t.Errorf("%s: pub dosyası kalmadı", s.name)
		}
	}
}

func runShellStdin(t *testing.T, r, stdin string, env []string, fn string, args ...string) sh {
	t.Helper()
	script := `export SISTEM_PATH="$1"; shift; for l in Validate.sh SSHHelper.sh SSHKeyCopy.sh; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; "$@"`
	cmd := exec.Command("bash", append([]string{"-c", script, "_", filepath.Join(r, "referans/kabuk/sistem"), fn}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return sh{code, so.String(), se.String()}
}

func TestRemoveInteractiveMatchesShell(t *testing.T) {
	r := root(t)
	tools := fakeTools(t)
	build := func() string {
		d := t.TempDir()
		os.MkdirAll(filepath.Join(d, "web1"), 0o700)
		os.WriteFile(filepath.Join(d, "web1", "server.env"), []byte("IP=1.1.1.1\nUSER=u\n"), 0o600)
		os.WriteFile(filepath.Join(d, "web1", "id_ed25519_web1"), []byte("PRIV"), 0o600)
		os.WriteFile(filepath.Join(d, "web1", "id_ed25519_web1.pub"), []byte("PUB"), 0o644)
		os.MkdirAll(filepath.Join(d, "db"), 0o700)
		return d
	}
	for _, c := range []struct {
		name, stdin, target, kc string
	}{
		{"onay + her ikisi de var, y", "web1\ny\n", "web1", "USBMonitor_WEB1_Knock\nUSBMonitor_WEB1_Passphrase\n"},
		{"onay + n", "web1\nn\n", "web1", "USBMonitor_WEB1_Knock\nUSBMonitor_WEB1_Passphrase\n"},
		{"yalnız knock", "web1\n", "web1", "USBMonitor_WEB1_Knock\n"},
		{"kayıt yok", "web1\n", "web1", ""},
		{"yanlış onay", "yanlis\n", "web1", "USBMonitor_WEB1_Knock\n"},
		{"girdi yok", "", "web1", "USBMonitor_WEB1_Knock\n"},
		{"sunucu yok", "x\n", "yok", ""},
		{"geçersiz ad", "x\n", "Web", ""},
		{"büyük Y", "web1\nY\n", "web1", "USBMonitor_WEB1_Passphrase\n"},
	} {
		d1, d2 := build(), build()
		kc1, kc2 := filepath.Join(t.TempDir(), "kc"), filepath.Join(t.TempDir(), "kc")
		os.WriteFile(kc1, []byte(c.kc), 0o600)
		os.WriteFile(kc2, []byte(c.kc), 0o600)
		cmd := exec.Command("bash", filepath.Join(r, "referans/kabuk/sistem/ModuleManager.sh"), "remove", c.target)
		cmd.Env = append(os.Environ(), "SSHPOCKET_SERVERS_DIR="+d1, "SSHPOCKET_MODULES_DIR="+filepath.Join(d1, "..yok"), "FAKE_KC="+kc1)
		cmd.Stdin = strings.NewReader(c.stdin)
		var so, se bytes.Buffer
		cmd.Stdout, cmd.Stderr = &so, &se
		shErr := cmd.Run()
		shCode := 0
		if ee, ok := shErr.(*exec.ExitError); ok {
			shCode = ee.ExitCode()
		}
		t.Setenv("FAKE_KC", kc2)
		var out, errb bytes.Buffer
		goCode := RemoveInteractive(Console{In: bufio.NewReader(strings.NewReader(c.stdin)), Out: &out, Err: &errb}, Keychain{}, d2, c.target)
		wantOut := strings.ReplaceAll(so.String(), d1, "<D>")
		gotOut := strings.ReplaceAll(out.String(), d2, "<D>")
		if goCode != shCode || wantOut != gotOut {
			t.Errorf("%s: kabuk=%d go=%d\nkabuk: %q\ngo:    %q", c.name, shCode, goCode, wantOut, gotOut)
		}
		if a, b := tree(t, d1), tree(t, d2); a != b {
			t.Errorf("%s: ağaç farklı\nkabuk: %s\ngo:    %s", c.name, a, b)
		}
		k1, _ := os.ReadFile(kc1)
		k2, _ := os.ReadFile(kc2)
		if string(k1) != string(k2) {
			t.Errorf("%s: Keychain kalanı farklı kabuk=%q go=%q", c.name, k1, k2)
		}
		_ = tools
	}
}
