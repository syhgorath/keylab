package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var reLogTime = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z`)

type cliEnv struct {
	template string // sunucu şablonu (anahtar dahil)
	tools    string
}

func setupCLI(t *testing.T) *cliEnv {
	e := &cliEnv{template: t.TempDir(), tools: t.TempDir()}
	os.WriteFile(filepath.Join(e.tools, "ssh"), []byte(fakeSSH2), 0o755)
	os.WriteFile(filepath.Join(e.tools, "ssh-add"), []byte(fakeSSHAdd), 0o755)
	os.WriteFile(filepath.Join(e.tools, "python3.12"), []byte(fmt.Sprintf(fakePyTmpl, "3.12.4")), 0o755)
	os.WriteFile(filepath.Join(e.tools, "ansible-src"), []byte(fakeAnsible), 0o755)
	t.Setenv("PATH", e.tools+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_LOG", filepath.Join(e.tools, "calls.log"))
	t.Setenv("FAKE_ANSIBLE_SRC", filepath.Join(e.tools, "ansible-src"))
	t.Setenv("SSHPOCKET_PYTHON_CANDIDATES", "python3.12")
	sdir := filepath.Join(e.template, "srv")
	os.MkdirAll(sdir, 0o755)
	priv := filepath.Join(sdir, "id_ed25519_srv")
	if out, err := exec.Command("/usr/bin/ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	pubB, _ := os.ReadFile(priv + ".pub")
	f := strings.Fields(string(pubB))
	known := filepath.Join(e.template, "known_hosts")
	os.WriteFile(known, []byte("10.0.0.5 "+f[0]+" "+f[1]+"\n"), 0o600)
	t.Setenv("SSHPOCKET_KNOWN_HOSTS", known)
	fpOut := mustRun(t, "/usr/bin/ssh-keygen", "-lf", priv)
	t.Setenv("FAKE_AGENT", "256 "+strings.Fields(fpOut)[1]+" x\n")
	t.Setenv("FAKE_OSREL", "ID=ubuntu\nVERSION_ID=\"26.04\"\n")
	return e
}

func copyDir(t *testing.T, src, dst string) {
	if out, err := exec.Command("cp", "-R", src, dst).CombinedOutput(); err != nil {
		t.Fatalf("cp: %v %s", err, out)
	}
}

func normCLI(s string, dirs map[string]string) string {
	for k, v := range dirs {
		s = strings.ReplaceAll(s, k, v)
	}
	s = reWork.ReplaceAllString(s, "sshpocket-run.X")
	s = reStamp.ReplaceAllString(s, "STAMP")
	s = reLogTime.ReplaceAllString(s, "TIME")
	s = reBashJob.ReplaceAllString(s, "")
	return strings.ReplaceAll(s, "//", "/")
}

func treeOf(root string, dirs map[string]string) map[string]string {
	out := map[string]string{}
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = normCLI(rel, dirs)
		st, _ := os.Stat(p)
		if d.IsDir() {
			out[rel+"/"] = fmt.Sprintf("%o", st.Mode().Perm())
			return nil
		}
		b, _ := os.ReadFile(p)
		if strings.HasSuffix(rel, "bin/python") || strings.HasSuffix(rel, "bin/ansible-playbook") || strings.HasPrefix(filepath.Base(rel), "id_ed25519") {
			b = []byte("BIN")
		}
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			switch {
			case strings.HasSuffix(rel, "results") && strings.Count(l, "|") == 3:
				lines[i] = "T|" + l[strings.Index(l, "|")+1:]
			case strings.HasPrefix(filepath.Base(rel), "result.") && i == 0:
				lines[i] = "T"
			case strings.HasPrefix(l, "# zaman:") || strings.HasPrefix(l, "# süre:"):
				lines[i] = l[:strings.Index(l, ":")] + ": X"
			}
		}
		out[rel] = fmt.Sprintf("%o %s", st.Mode().Perm(), normCLI(strings.Join(lines, "\n"), dirs))
		return nil
	})
	return out
}

type cliCase struct {
	name       string
	args       [3]string
	envFile    string // server.env içeriği ("" = varsayılan)
	stdin      string
	env        map[string]string
	preVenv    bool
	preResults []string
	vars       string
	noServer   bool
	noKey      bool
}

func TestCLIRunMatchesShell(t *testing.T) {
	root := repoRoot(t)
	e := setupCLI(t)
	defEnv := "IP=10.0.0.5\nUSER=deploy\nPORT=22\nKEY=\n"
	okOut := "SSHPOCKET_PROGRESS alan=base sira=1/1 durum=bitti degisen=2\nSSHPOCKET_RESULT utility.durum timezone=UTC locale=en_US ntp_servis=yes ntp_senkron=yes offset_ms=3 kaynak_sayisi=2"
	freshPreview := []string{"%d|0.1.0|preview|ok"}
	cases := []cliCase{
		{name: "durum başarı (venv hazır)", args: [3]string{"utility", "durum", "srv"}, preVenv: true, env: map[string]string{"FAKE_ANS_OUT": okOut}},
		{name: "venv yok, e", args: [3]string{"utility", "durum", "srv"}, stdin: "e\n", env: map[string]string{"FAKE_ANS_OUT": okOut}},
		{name: "venv yok, h", args: [3]string{"utility", "durum", "srv"}, stdin: "h\n"},
		{name: "venv yok, EOF", args: [3]string{"utility", "durum", "srv"}},
		{name: "venv yok, pip hatası", args: [3]string{"utility", "durum", "srv"}, stdin: "e\n", env: map[string]string{"FAKE_PIP_RC": "1"}},
		{name: "apply onayla", args: [3]string{"utility", "apply", "srv"}, preVenv: true, stdin: "e\n", preResults: freshPreview, env: map[string]string{"FAKE_ANS_OUT": "SSHPOCKET_RESULT utility.apply degisen=1 plan=ok yedek=yok"}},
		{name: "apply ön koşul yok", args: [3]string{"utility", "apply", "srv"}, preVenv: true, stdin: "e\n"},
		{name: "ansible başarısız", args: [3]string{"utility", "durum", "srv"}, preVenv: true, env: map[string]string{"FAKE_ANS_RC": "2", "FAKE_ANS_OUT": "fatal"}},
		{name: "geçersiz paket", args: [3]string{"Utility", "durum", "srv"}},
		{name: "geçersiz eylem", args: [3]string{"utility", "Durum", "srv"}},
		{name: "geçersiz sunucu adı", args: [3]string{"utility", "durum", "Srv"}},
		{name: "eylem yok", args: [3]string{"utility", "yokeylem", "srv"}, preVenv: true},
		{name: "paket kilitte yok", args: [3]string{"yokpaket", "durum", "srv"}},
		{name: "sunucu yok", args: [3]string{"utility", "durum", "srv"}, noServer: true},
		{name: "anahtar dosyası yok", args: [3]string{"utility", "durum", "srv"}, preVenv: true, noKey: true},
		{name: "geçersiz ip", args: [3]string{"utility", "durum", "srv"}, preVenv: true, envFile: "IP=bad ip\nUSER=deploy\nPORT=22\n"},
		{name: "geçersiz port", args: [3]string{"utility", "durum", "srv"}, preVenv: true, envFile: "IP=10.0.0.5\nUSER=deploy\nPORT=70000\n"},
		{name: "kullanıcı eksik", args: [3]string{"utility", "durum", "srv"}, preVenv: true, envFile: "IP=10.0.0.5\nPORT=22\n"},
		{name: "port varsayılan + tırnaklı + yorum", args: [3]string{"utility", "durum", "srv"}, preVenv: true, envFile: "IP=\"10.0.0.5\"  # yorum\nUSER='deploy'\n# PORT=99\n", env: map[string]string{"FAKE_ANS_OUT": okOut}},
		{name: "eski biçim .env", args: [3]string{"utility", "durum", "srv"}, preVenv: true, envFile: "LEGACY", env: map[string]string{"FAKE_ANS_OUT": okOut}},
		{name: "ssh profili algılanamadı", args: [3]string{"utility", "durum", "srv"}, preVenv: true, env: map[string]string{"FAKE_OSREL": "", "FAKE_RC": "255", "FAKE_ERR": "ssh: connect to host 10.0.0.5 port 22: Connection refused"}},
		{name: "profil desteklenmiyor", args: [3]string{"utility", "durum", "srv"}, preVenv: true, env: map[string]string{"FAKE_OSREL": "ID=arch\nVERSION_ID=1\n"}},
		{name: "ubuntu sudo.ws", args: [3]string{"utility", "durum", "srv"}, preVenv: true, env: map[string]string{"FAKE_SUDOWS": "SSHPOCKET_SUDO_WS=yes\n", "FAKE_ANS_OUT": okOut}},
		{name: "debian (sudo.ws sorulmaz)", args: [3]string{"utility", "durum", "srv"}, preVenv: true, env: map[string]string{"FAKE_OSREL": "ID=debian\nVERSION_ID=\"13\"\n", "FAKE_ANS_OUT": okOut}},
		{name: "değişken dosyası", args: [3]string{"utility", "durum", "srv"}, preVenv: true, vars: "utility_timezone=Europe/Istanbul\n", env: map[string]string{"FAKE_ANS_OUT": okOut}},
		{name: "geçersiz değişken", args: [3]string{"utility", "durum", "srv"}, preVenv: true, vars: "utility_backup=belki\n"},
	}
	parent := t.TempDir() // ASCII yol: alt test adlarındaki Türkçe harfler geçici klasör adına girmesin (printf %q yerel ayara göre kaçırır)
	seq := 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, k := range []string{"FAKE_ANS_OUT", "FAKE_ANS_RC", "FAKE_PIP_RC", "FAKE_ASK", "FAKE_ERR", "FAKE_RC", "FAKE_SUDOWS", "FAKE_ANS_SIGNAL"} {
				t.Setenv(k, "")
			}
			t.Setenv("FAKE_OSREL", "ID=ubuntu\nVERSION_ID=\"26.04\"\n")
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			type res struct {
				rc                    int
				stdout, stderr        string
				servers, state, cache map[string]string
			}
			var out [2]res
			for impl := 0; impl < 2; impl++ {
				seq++
				base := filepath.Join(parent, fmt.Sprintf("c%d", seq))
				os.MkdirAll(base, 0o755)
				servers := filepath.Join(base, "servers")
				state := filepath.Join(base, "state")
				cache := filepath.Join(base, "cache")
				os.MkdirAll(servers, 0o755)
				os.MkdirAll(state, 0o755)
				os.MkdirAll(cache, 0o755)
				if !c.noServer {
					copyDir(t, filepath.Join(e.template, "srv"), servers)
					envf := c.envFile
					if envf == "" {
						envf = defEnv
					}
					name := "server.env"
					if envf == "LEGACY" {
						name, envf = ".env", "SRV_IP=10.0.0.5\nSRV_USER=deploy\nSRV_PORT=22\n"
					}
					os.WriteFile(filepath.Join(servers, "srv", name), []byte(envf), 0o600)
					if c.noKey {
						os.Remove(filepath.Join(servers, "srv", "id_ed25519_srv"))
					}
					if c.vars != "" {
						os.WriteFile(filepath.Join(servers, "srv", "utility.vars"), []byte(c.vars), 0o600)
					}
				}
				t.Setenv("SSHPOCKET_CACHE_DIR", cache)
				if c.preVenv {
					pkgDir := filepath.Join(root, "components/utility/0.1.0")
					if _, err := VenvEnsure("utility", pkgDir, ">=3.12", ""); err != nil {
						t.Fatalf("venv hazırlanamadı: %v", err)
					}
				}
				if len(c.preResults) > 0 {
					d := filepath.Join(state, "utility", "srv")
					os.MkdirAll(d, 0o700)
					var sb strings.Builder
					for _, r := range c.preResults {
						sb.WriteString(fmt.Sprintf(r, nowUnix()-10) + "\n")
					}
					os.WriteFile(filepath.Join(d, "results"), []byte(sb.String()), 0o600)
				}
				dirs := map[string]string{servers: "SERVERS", state: "STATE", cache: "CACHE", e.template: "TEMPLATE"}
				lockPath := filepath.Join(root, "components.lock")
				compDir := filepath.Join(root, "components")
				if impl == 0 {
					env := []string{"SSHPOCKET_SERVERS_DIR=" + servers, "SSHPOCKET_STATE_DIR=" + state, "SSHPOCKET_COMPONENTS_DIR=" + compDir,
						"SSHPOCKET_COMPONENTS_LOCK=" + lockPath, "SSHPOCKET_CACHE_DIR=" + cache}
					o := runShell(t, root, env, c.stdin, "", "comp_cli_run", c.args[0], c.args[1], c.args[2])
					out[0] = res{o.code, normCLI(o.stdout, dirs), normCLI(o.stderr, dirs), treeOf(servers, dirs), treeOf(state, dirs), treeOf(cache, dirs)}
				} else {
					var so, se bytes.Buffer
					cli := &CLI{ComponentsDir: compDir, LockPath: lockPath, ServersDir: servers, StateDir: state, KnownHosts: os.Getenv("SSHPOCKET_KNOWN_HOSTS"),
						Ctx: context.Background(), Stdin: strings.NewReader(c.stdin), Stdout: &so, Stderr: &se}
					rc := cli.Run(c.args[0], c.args[1], c.args[2])
					out[1] = res{rc, normCLI(so.String(), dirs), normCLI(se.String(), dirs), treeOf(servers, dirs), treeOf(state, dirs), treeOf(cache, dirs)}
				}
			}
			a, b := out[0], out[1]
			if a.rc != b.rc {
				t.Fatalf("çıkış kabuk=%d go=%d\n kabuk stderr=%q\n go stderr   =%q", a.rc, b.rc, a.stderr, b.stderr)
			}
			if a.stdout != b.stdout {
				t.Errorf("stdout\n kabuk=%q\n go   =%q", a.stdout, b.stdout)
			}
			if a.stderr != b.stderr {
				t.Errorf("stderr\n kabuk=%q\n go   =%q", a.stderr, b.stderr)
			}
			for name, pair := range map[string][2]map[string]string{"servers": {a.servers, b.servers}, "state": {a.state, b.state}, "cache": {a.cache, b.cache}} {
				if !reflect.DeepEqual(pair[0], pair[1]) {
					for k := range pair[0] {
						if pair[0][k] != pair[1][k] {
							t.Errorf("%s farkı %s\n kabuk=%q\n go   =%q", name, k, pair[0][k], pair[1][k])
						}
					}
					for k := range pair[1] {
						if _, ok := pair[0][k]; !ok {
							t.Errorf("%s: yalnız Go'da: %s", name, k)
						}
					}
				}
			}
		})
	}
}

func nowUnix() int64 {
	var n int64
	fmt.Sscan(strings.TrimSpace(func() string { o, _ := exec.Command("date", "+%s").Output(); return string(o) }()), &n)
	return n
}
