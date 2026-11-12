package settings

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/shellgold"
)

// shRes: kabuk çağrısının kaydedilen sonucu (altın dosya testdata/shell.json; yeniden kayıt: KEYLAB_KAYIT_KABUK=1).
type shRes struct {
	Code     int
	Out, Err string
	Content  string // set testinde: çalıştırmadan sonra conf içeriği
	Exists   bool
	Perm     uint32
}

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
	gold := shellgold.Open(t, "shell")
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
				g := shellgold.Do(t, gold, fmt.Sprintf("get|%q|%q|%s", c, shEnv, k), func() shRes {
					code, so, _ := shellRun(t, f, shEnv, "settings_get", k)
					return shRes{Code: code, Out: so}
				})
				code, so := g.Code, g.Out
				v, ok := (Store{f}).Get(k)
				if (code == 0) != ok || (ok && v != so) {
					t.Errorf("conf %d env %v %s: kabuk=%d %q go=%v %q", i, e, k, code, so, ok, v)
				}
				if k != "BILINMEYEN" {
					ss := shellgold.Do(t, gold, fmt.Sprintf("source|%q|%q|%s", c, shEnv, k), func() shRes {
						_, o, _ := shellRun(t, f, shEnv, "settings_source", k)
						return shRes{Out: o}
					}).Out
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
	gold := shellgold.Open(t, "shell")
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
			g := shellgold.Do(t, gold, fmt.Sprintf("set|%q|%v|%d", c, kv, i%2), func() shRes {
				code, _, se := shellRun(t, f1, nil, "settings_set", kv[0], kv[1])
				r := shRes{Code: code, Err: se}
				if b, e := os.ReadFile(f1); e == nil {
					r.Content, r.Exists = string(b), true
					if st, e := os.Stat(f1); e == nil {
						r.Perm = uint32(st.Mode().Perm())
					}
				}
				return r
			})
			code, se := g.Code, g.Err
			err := (Store{f2}).Set(kv[0], kv[1])
			if (code == 0) != (err == nil) || (err != nil && strings.TrimSpace(se) != strings.TrimSpace(err.Error())) {
				t.Errorf("conf %d %v: kabuk=%d %q go=%v", i, kv, code, se, err)
				continue
			}
			b2, e2 := os.ReadFile(f2)
			s2, _ := os.Stat(f2)
			if g.Exists != (e2 == nil) || g.Content != string(b2) || (g.Exists && e2 == nil && g.Perm != uint32(s2.Mode().Perm())) {
				t.Errorf("conf %d %v: dosya farklı\nkabuk=%q %o %v\ngo=   %q %v", i, kv, g.Content, g.Perm, g.Exists, b2, s2)
			}
		}
	}
}
