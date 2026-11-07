package settings

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func shellRun(t *testing.T, conf string, env []string, fn string, args ...string) (int, string, string) {
	t.Helper()
	root, _ := filepath.Abs("../../..")
	if _, err := os.Stat(filepath.Join(root, "referans/kabuk/sistem/Helpers/Settings.sh")); err != nil {
		t.Skip("kabuk motoru yok")
	}
	cmd := exec.Command("bash", append([]string{"-c", `. "$1/Helpers/Settings.sh"; shift; "$@"`, "_", filepath.Join(root, "referans/kabuk/sistem"), fn}, args...)...)
	cmd.Env = append(append(os.Environ(), "SSHPOCKET_CONF="+conf, "SSHPOCKET_STATUS=", "SSHPOCKET_STATUS_TIMEOUT=", "SSHPOCKET_UI="), env...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return code, so.String(), se.String()
}

var confs = []string{
	"",
	"STATUS=off\n",
	"STATUS=garip\nSTATUS_TIMEOUT=3\n",
	"# yorum\nSTATUS_TIMEOUT = 4\nUI=fzf\n",
	"STATUS_TIMEOUT=2\nSTATUS_TIMEOUT=5\n",
	"STATUS_TIMEOUT=9\n",
	"STATUS=OFF\nUI=arrows\r\n",
	"STATUS=  yes  \n",
	"XSTATUS=off\n",
}

func TestGetSourceMatchShell(t *testing.T) {
	dir := t.TempDir()
	envs := [][2]string{{}, {"SSHPOCKET_STATUS", "off"}, {"SSHPOCKET_STATUS_TIMEOUT", "7"}, {"SSHPOCKET_STATUS_TIMEOUT", "2"}, {"SSHPOCKET_UI", "numbers"}}
	for i, c := range confs {
		f := filepath.Join(dir, "c"+string(rune('a'+i)))
		os.WriteFile(f, []byte(c), 0o644)
		for _, e := range envs {
			var shEnv []string
			if e[0] != "" {
				shEnv = []string{e[0] + "=" + e[1]}
			}
			for _, k := range []string{"STATUS", "STATUS_TIMEOUT", "UI", "BILINMEYEN"} {
				// ortamı Go tarafında da kur
				for _, n := range []string{"SSHPOCKET_STATUS", "SSHPOCKET_STATUS_TIMEOUT", "SSHPOCKET_UI"} {
					os.Unsetenv(n)
				}
				if e[0] != "" {
					t.Setenv(e[0], e[1])
				}
				_, so, _ := shellRun(t, f, shEnv, "settings_get", k)
				code, _, _ := shellRun(t, f, shEnv, "settings_get", k)
				v, ok := (Store{f}).Get(k)
				if (code == 0) != ok || (ok && v != so) {
					t.Errorf("conf %d env %v %s: kabuk=%d %q go=%v %q", i, e, k, code, so, ok, v)
				}
				if k != "BILINMEYEN" {
					_, ss, _ := shellRun(t, f, shEnv, "settings_source", k)
					if got := (Store{f}).Source(k); got+"\n" != ss {
						t.Errorf("conf %d env %v %s kaynak: kabuk=%q go=%q", i, e, k, ss, got)
					}
				}
			}
		}
	}
	for _, n := range []string{"SSHPOCKET_STATUS", "SSHPOCKET_STATUS_TIMEOUT", "SSHPOCKET_UI"} {
		os.Unsetenv(n)
	}
}

func TestSetMatchesShell(t *testing.T) {
	for i, c := range confs {
		for _, kv := range [][2]string{{"STATUS", "off"}, {"STATUS", "1"}, {"STATUS_TIMEOUT", "5"}, {"STATUS_TIMEOUT", "6"}, {"UI", "numbers"}, {"UI", "x"}, {"BILINMEYEN", "1"}, {"STATUS", ""}} {
			d1, d2 := t.TempDir(), t.TempDir()
			f1, f2 := filepath.Join(d1, "c.conf"), filepath.Join(d2, "c.conf")
			if c != "" || i%2 == 0 {
				if c != "" {
					os.WriteFile(f1, []byte(c), 0o600)
					os.WriteFile(f2, []byte(c), 0o600)
				}
			}
			code, _, se := shellRun(t, f1, nil, "settings_set", kv[0], kv[1])
			err := (Store{f2}).Set(kv[0], kv[1])
			if (code == 0) != (err == nil) || (err != nil && strings.TrimSpace(se) != strings.TrimSpace(err.Error())) {
				t.Errorf("conf %d %v: kabuk=%d %q go=%v", i, kv, code, se, err)
				continue
			}
			b1, e1 := os.ReadFile(f1)
			b2, e2 := os.ReadFile(f2)
			s1, _ := os.Stat(f1)
			s2, _ := os.Stat(f2)
			if (e1 == nil) != (e2 == nil) || string(b1) != string(b2) || (s1 != nil && s2 != nil && s1.Mode().Perm() != s2.Mode().Perm()) {
				t.Errorf("conf %d %v: dosya farklı\nkabuk=%q %v\ngo=   %q %v", i, kv, b1, s1, b2, s2)
			}
		}
	}
}
