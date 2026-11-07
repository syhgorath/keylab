package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fakeSecurityStdin = `#!/bin/bash
S="$FAKE_KC"
[ -n "$FAKE_ARGV_LOG" ] && echo "$*" >> "$FAKE_ARGV_LOG"
case "$1" in
  -i)
    read -r cmdline
    acct=""; val=""; set -- $cmdline
    while [ $# -gt 0 ]; do case "$1" in -a) acct="$2" ;; -w) val="$2" ;; esac; shift; done
    { grep -v "^$acct=" "$S" 2>/dev/null; echo "$acct=$val"; } > "$S.n"; mv "$S.n" "$S"; exit 0 ;;
  find-generic-password)
    grep -q "^$3=" "$S" 2>/dev/null || exit 44
    if [ "$5" = "-w" ] || [ "$6" = "-w" ]; then grep "^$3=" "$S" | tail -1 | cut -d= -f2-; fi; exit 0 ;;
  delete-generic-password) grep -q "^$3=" "$S" 2>/dev/null || exit 44; grep -v "^$3=" "$S" > "$S.n"; mv "$S.n" "$S"; exit 0 ;;
  add-generic-password)
    [ "${FAKE_ADD_FAIL_KC:-0}" = 1 ] && exit 1
    acct=""; while [ $# -gt 0 ]; do [ "$1" = -a ] && acct="$2"; shift; done
    read -r v; { grep -v "^$acct=" "$S" 2>/dev/null; echo "$acct=$v"; } > "$S.n"; mv "$S.n" "$S"; exit 0 ;;
esac
exit 1
`

const fakeSSHAddMgr = `#!/bin/bash
A="$FAKE_AGENT"
case "$1" in
  -l) [ -s "$A" ] && { cat "$A"; exit 0; }; echo "The agent has no identities."; exit 1 ;;
  -d) : > "$A"; exit 0 ;;
  *) [ "${FAKE_ADD_FAIL:-0}" = 1 ] && exit 1; ssh-keygen -lf "$1" > "$A"; exit 0 ;;
esac
`

func mgrEnv(t *testing.T) (bin string) {
	bin = t.TempDir()
	os.WriteFile(filepath.Join(bin, "security"), []byte(fakeSecurityStdin), 0o755)
	os.WriteFile(filepath.Join(bin, "ssh-add"), []byte(fakeSSHAddMgr), 0o755)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return
}

func mkServer(t *testing.T, root, name string, env string, withKey bool) {
	d := filepath.Join(root, name)
	os.MkdirAll(d, 0o700)
	os.WriteFile(filepath.Join(d, "server.env"), []byte(env), 0o600)
	if withKey {
		if o, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", filepath.Join(d, "id_ed25519_"+name)).CombinedOutput(); err != nil {
			t.Fatalf("keygen %v %s", err, o)
		}
	}
}

func dirTree(t *testing.T, dir string) string {
	var lines []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || p == dir || strings.HasSuffix(p, ".pub") || strings.Contains(filepath.Base(p), "id_ed25519") {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		if info.IsDir() {
			lines = append(lines, rel+"/")
		} else {
			b, _ := os.ReadFile(p)
			lines = append(lines, fmt.Sprintf("%s %v %q", rel, info.Mode().Perm(), b))
		}
		return nil
	})
	return strings.Join(lines, "\n")
}

func runShellScript(t *testing.T, r, script string, env []string, stdin string, args ...string) res {
	cmd := exec.Command("bash", append([]string{"-c", script, "_", filepath.Join(r, "referans/kabuk/sistem")}, args...)...)
	cmd.Env = append(os.Environ(), append(env, "LC_ALL=en_US.UTF-8", "SSHPOCKET_UI=numbers")...)
	cmd.Stdin = strings.NewReader(stdin)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return res{code, so.String(), se.String()}
}

func goServers(root, answers, secret string) (*Servers, *bytes.Buffer, *bytes.Buffer, func()) {
	var so, se bytes.Buffer
	pr, pw, _ := os.Pipe()
	pw.WriteString(secret)
	pw.Close()
	s := &Servers{Dir: root, In: bufio.NewReader(strings.NewReader(answers)), Out: &so, Err: &se, ChildIn: pr, DefaultUser: os.Getenv("USER")}
	return s, &so, &se, func() { pr.Close() }
}

func TestImportSSHConfigMatchesShell(t *testing.T) {
	r := repoRoot(t)
	cfg := filepath.Join(t.TempDir(), "config")
	os.WriteFile(cfg, []byte("Host web1\n  HostName 10.0.0.5\n  User root\n  IdentityFile ~/k\nHost mevcut\n  HostName 10.0.0.6\nHost bozuk\n  HostName a b\nHost Yeni-Ad\n  HostName y.example\n  Port 2200\nHost *\n  User x\n"), 0o600)
	empty := filepath.Join(t.TempDir(), "bos")
	os.WriteFile(empty, []byte("Host *\n  User x\n"), 0o600)
	for _, c := range []struct{ name, cfg, in string }{
		{"hepsi", cfg, "a\n"}, {"seçim", cfg, "1,4\n"}, {"boşluklu", cfg, "1 2\n"}, {"geçersiz", cfg, "9, x, 2\n"},
		{"sıfır geçersiz", cfg, "0, 1\n"}, {"iptal", cfg, "\n"}, {"girdi yok", cfg, ""}, {"config yok", filepath.Join(t.TempDir(), "yok"), ""}, {"joker", empty, "a\n"},
	} {
		d1, d2 := t.TempDir(), t.TempDir()
		for _, d := range []string{d1, d2} {
			os.MkdirAll(filepath.Join(d, "mevcut"), 0o700)
		}
		sh := runShellScript(t, r, `export SISTEM_PATH="$1"; bash "$1/ImportSSHConfig.sh" "$2"`, []string{"SSHPOCKET_SERVERS_DIR=" + d1, "SSHPOCKET_MODULES_DIR=" + filepath.Join(d1, "..yok")}, c.in, c.cfg)
		s, so, _, closeFn := goServers(d2, c.in, "")
		code := s.ImportSSHConfig(c.cfg)
		closeFn()
		if code != sh.code || so.String() != sh.out || dirTree(t, d1) != dirTree(t, d2) {
			t.Errorf("%s: kabuk=%d go=%d\nkabuk: %q\ngo:    %q\nağaç kabuk: %s\nağaç go:    %s", c.name, sh.code, code, sh.out, so.String(), dirTree(t, d1), dirTree(t, d2))
		}
	}
}

func TestKnockManageMatchesShell(t *testing.T) {
	r := repoRoot(t)
	mgrEnv(t)
	libs := "Validate.sh ModuleFiles.sh SSHHelper.sh SSHModule.sh SSHModuleEdit.sh Servers.sh Knock.sh"
	script := `export SISTEM_PATH="$1"; shift; for l in ` + libs + `; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; knock_manage "$@"`
	for _, c := range []struct {
		name, answers, secret, kc, env string
	}{
		{"3 geçersiz protokol", "1\nx\ny\nz\n", "7000\n", "", "IP=1.1.1.1\n"},
		{"kapat", "2\n", "", "USBMonitor_WEB1_Knock=7000,8000\n", "IP=1.1.1.1\nKNOCK=keychain\nKNOCK_PROTO=tcp\nKNOCK_DELAY_MS=300\n"},
		{"durum etkin", "0\n", "", "USBMonitor_WEB1_Knock=7000,8000\n", "IP=1.1.1.1\nKNOCK=keychain\nKNOCK_PROTO=udp\nKNOCK_DELAY_MS=100\n"},
		{"geri q", "q\n", "", "", "IP=1.1.1.1\n"},
		{"girdi yok", "", "", "", "IP=1.1.1.1\n"},
	} {
		d1, d2 := t.TempDir(), t.TempDir()
		kc1, kc2 := filepath.Join(t.TempDir(), "kc"), filepath.Join(t.TempDir(), "kc")
		os.WriteFile(kc1, []byte(c.kc), 0o600)
		os.WriteFile(kc2, []byte(c.kc), 0o600)
		mkServer(t, d1, "web1", c.env, false)
		mkServer(t, d2, "web1", c.env, false)
		sh := runShellScript(t, r, script, []string{"FAKE_KC=" + kc1}, c.answers+c.secret, "WEB1", filepath.Join(d1, "web1"))
		t.Setenv("FAKE_KC", kc2)
		s, so, _, closeFn := goServers(d2, c.answers, c.secret)
		code := s.KnockManage("web1")
		closeFn()
		k1, _ := os.ReadFile(kc1)
		k2, _ := os.ReadFile(kc2)
		out := strings.ReplaceAll(sh.out, d1, "<D>")
		got := strings.ReplaceAll(so.String(), d2, "<D>")
		if code != sh.code || out != got || dirTree(t, d1) != dirTree(t, d2) || string(k1) != string(k2) {
			t.Errorf("%s: kabuk=%d go=%d\nkabuk: %q\ngo:    %q\nağaç kabuk: %s\nağaç go:    %s\nkc kabuk=%q go=%q", c.name, sh.code, code, out, got, dirTree(t, d1), dirTree(t, d2), k1, k2)
		}
	}
}

func TestKeychainManageMatchesShell(t *testing.T) {
	r := repoRoot(t)
	mgrEnv(t)
	libs := "Validate.sh ModuleFiles.sh SSHHelper.sh SSHModule.sh SSHModuleEdit.sh Servers.sh"
	script := `export SISTEM_PATH="$1"; shift; for l in ` + libs + `; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; ssh_keychain_manage "$@"`
	for _, c := range []struct {
		name, answers, secret, kc string
		withKey                   bool
		addFail, kcFail           string
		prefill                   bool
	}{
		{"kaydet + doğrula", "1\n", "gizli\n", "", true, "0", "0", false},
		{"kaydet anahtarsız", "1\n", "gizli\n", "", false, "0", "0", false},
		{"kaydet ama doğrulanamaz", "1\n", "gizli\n", "", true, "1", "0", false},
		{"ajanda önceden var, doğrulama başarısız", "1\n", "gizli\n", "", true, "1", "0", true},
		{"kaydedilemedi", "1\n", "gizli\n", "", true, "0", "1", false},
		{"sil", "2\n", "", "USBMonitor_WEB1_Passphrase=x\n", true, "0", "0", false},
		{"sil: kayıt yok", "2\n", "", "", true, "0", "0", false},
		{"durum var", "0\n", "", "USBMonitor_WEB1_Passphrase=x\n", true, "0", "0", false},
		{"girdi yok", "", "", "", true, "0", "0", false},
	} {
		d1, d2 := t.TempDir(), t.TempDir()
		kc1, kc2 := filepath.Join(t.TempDir(), "kc"), filepath.Join(t.TempDir(), "kc")
		a1, a2 := filepath.Join(t.TempDir(), "agent"), filepath.Join(t.TempDir(), "agent")
		os.WriteFile(kc1, []byte(c.kc), 0o600)
		os.WriteFile(kc2, []byte(c.kc), 0o600)
		env := "IP=1.1.1.1\nUSER=u\n"
		mkServer(t, d1, "web1", env, c.withKey)
		mkServer(t, d2, "web1", env, c.withKey)
		if c.prefill {
			o, _ := exec.Command("ssh-keygen", "-lf", filepath.Join(d1, "web1", "id_ed25519_web1")).Output()
			os.WriteFile(a1, o, 0o600)
			o2, _ := exec.Command("ssh-keygen", "-lf", filepath.Join(d2, "web1", "id_ed25519_web1")).Output()
			os.WriteFile(a2, o2, 0o600)
		}
		sh := runShellScript(t, r, script, []string{"FAKE_KC=" + kc1, "FAKE_AGENT=" + a1, "FAKE_ADD_FAIL=" + c.addFail, "FAKE_ADD_FAIL_KC=" + c.kcFail}, c.answers+c.secret, "WEB1", filepath.Join(d1, "web1", "id_ed25519_web1"))
		t.Setenv("FAKE_KC", kc2)
		t.Setenv("FAKE_AGENT", a2)
		t.Setenv("FAKE_ADD_FAIL", c.addFail)
		t.Setenv("FAKE_ADD_FAIL_KC", c.kcFail)
		s, so, _, closeFn := goServers(d2, c.answers, c.secret)
		code := s.KeychainManage("web1")
		closeFn()
		k1, _ := os.ReadFile(kc1)
		k2, _ := os.ReadFile(kc2)
		// kabuk iletileri: ssh_agent_add'in stderr satırları ayrı akışta; yalnızca stdout karşılaştırılır
		if code != sh.code || strings.ReplaceAll(sh.out, d1, "<D>") != strings.ReplaceAll(so.String(), d2, "<D>") || string(k1) != string(k2) {
			t.Errorf("%s: kabuk=%d go=%d\nkabuk: %q\ngo:    %q\nkc kabuk=%q go=%q", c.name, sh.code, code, sh.out, so.String(), k1, k2)
		}
	}
}

func TestSendKnockUsesKeychainSequence(t *testing.T) {
	mgrEnv(t)
	var ports []string
	got := make(chan string, 8)
	for i := 0; i < 3; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skip(err)
		}
		defer l.Close()
		_, p, _ := net.SplitHostPort(l.Addr().String())
		ports = append(ports, p)
		go func(p string) {
			if c, err := l.Accept(); err == nil {
				c.Close()
				got <- p
			}
		}(p)
	}
	root := t.TempDir()
	mkServer(t, root, "web1", "IP=127.0.0.1\nUSER=u\nKNOCK=keychain\nKNOCK_PROTO=tcp\nKNOCK_DELAY_MS=20\n", false)
	kc := filepath.Join(t.TempDir(), "kc")
	os.WriteFile(kc, []byte("USBMonitor_WEB1_Knock="+strings.Join(ports, ",")+"\n"), 0o600)
	t.Setenv("FAKE_KC", kc)
	if err := SendKnock(context.Background(), root, "web1"); err != nil {
		t.Fatal(err)
	}
	for i, p := range ports {
		select {
		case g := <-got:
			if g != p {
				t.Errorf("sıra %d: %s != %s", i, g, p)
			}
		default:
			t.Errorf("vuruş %d gelmedi", i)
		}
	}
	// knock kapalıysa hiçbir şey yapmaz; Keychain'de dizi yoksa hata
	mkServer(t, root, "db", "IP=127.0.0.1\nUSER=u\n", false)
	if err := SendKnock(context.Background(), root, "db"); err != nil {
		t.Errorf("kapalıyken hata: %v", err)
	}
	mkServer(t, root, "app", "IP=127.0.0.1\nUSER=u\nKNOCK=keychain\n", false)
	if err := SendKnock(context.Background(), root, "app"); err == nil {
		t.Error("dizi yokken hata beklenirdi")
	}
	if err := SendKnock(context.Background(), root, "../x"); err == nil {
		t.Error("geçersiz ad kabul edildi")
	}
}

// Bilinçli fark: dizi artık macOS'un parola isteminde değil, sshpocket'in kendi (gizli) isteminde sorulur ve `security -i` ile
// stdin'den yazılır. Kabukla karşılaştırılamaz; beklenen davranış doğrudan sınanır.
func TestKnockManageSetsSequenceWithOwnPrompt(t *testing.T) {
	mgrEnv(t)
	for _, c := range []struct {
		name, answers string
		wantCode      int
		wantKC        string
		wantEnv       []string
		wantOut       []string
	}{
		{"geçerli", "1\n\n\n7001,7002,7003\n", 0, "USBMonitor_WEB1_Knock=7001,7002,7003\n", []string{"KNOCK=keychain", "KNOCK_PROTO=tcp", "KNOCK_DELAY_MS=300"}, []string{"✅ Knock etkin (tcp, 300 ms)", "    Dizi (portlar, örn. 7000,8000,9000): "}},
		{"udp 500", "1\nUDP\n500\n1,65535\n", 0, "USBMonitor_WEB1_Knock=1,65535\n", []string{"KNOCK_PROTO=udp", "KNOCK_DELAY_MS=500"}, nil},
		{"boşluklu önce reddedilir, sonra geçerli", "1\n\n\n7001, 7002\n7001,7002\n", 0, "USBMonitor_WEB1_Knock=7001,7002\n", []string{"KNOCK=keychain"}, []string{"Dizi geçerli değil"}},
		{"parola benzeri değer reddedilir", "1\n\n\nAQ563114xx\n", 1, "", nil, []string{"Dizi geçerli değil", "3 geçersiz deneme"}},
		{"girdi yok", "1\n\n\n", 1, "", nil, nil},
	} {
		root := t.TempDir()
		mkServer(t, root, "web1", "IP=1.1.1.1\nUSER=u\n", false)
		kc := filepath.Join(t.TempDir(), "kc")
		os.WriteFile(kc, nil, 0o600)
		argv := filepath.Join(t.TempDir(), "argv")
		t.Setenv("FAKE_KC", kc)
		t.Setenv("FAKE_ARGV_LOG", argv)
		answers := c.answers
		if c.name == "parola benzeri değer reddedilir" {
			answers = "1\n\n\nAQ563114xx\nAQ563114xx\nAQ563114xx\n"
		}
		s, so, se, closeFn := goServers(root, answers, "")
		code := s.KnockManage("web1")
		closeFn()
		got, _ := os.ReadFile(kc)
		env, _ := os.ReadFile(filepath.Join(root, "web1", "server.env"))
		all := so.String() + se.String()
		if code != c.wantCode || string(got) != c.wantKC {
			t.Errorf("%s: kod=%d (beklenen %d) kc=%q (beklenen %q)\n%s", c.name, code, c.wantCode, got, c.wantKC, all)
		}
		for _, w := range c.wantEnv {
			if !strings.Contains(string(env), w) {
				t.Errorf("%s: server.env'de %q yok:\n%s", c.name, w, env)
			}
		}
		for _, w := range c.wantOut {
			if !strings.Contains(all, w) {
				t.Errorf("%s: çıktıda %q yok:\n%s", c.name, w, all)
			}
		}
		if c.wantKC == "" && strings.Contains(string(env), "KNOCK=keychain") {
			t.Errorf("%s: başarısızken KNOCK etkinleştirildi", c.name)
		}
		// dizi komut satırında (argv) ASLA görünmemeli
		if b, _ := os.ReadFile(argv); strings.Contains(string(b), "7001") || strings.Contains(string(b), "AQ5631") {
			t.Errorf("%s: dizi/değer security argümanlarında göründü: %q", c.name, b)
		}
		if strings.Contains(all, "AQ563114xx") && c.name != "parola benzeri değer reddedilir" {
			t.Errorf("%s: değer çıktıya sızdı", c.name)
		}
	}
}
