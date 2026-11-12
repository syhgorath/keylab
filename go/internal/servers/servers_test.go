package servers

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"keylab/internal/shellgold"
)

// Kabuk motoru = çalışan şartname: aynı girdiyle yan yana koşturulur, çıkış kodu + stdout + stderr + dosya ağacı karşılaştırılır.

func root(t *testing.T) string {
	t.Helper()
	r, _ := filepath.Abs("../../..")
	if os.Getenv("KEYLAB_KAYIT_KABUK") == "1" { // kayıt: kabuk motoru gerekir; oynatmada gerekmez (altın dosya)
		if _, err := os.Stat(filepath.Join(r, "referans/kabuk/sistem/Helpers/Servers.sh")); err != nil {
			t.Skip("kabuk motoru yok")
		}
	}
	return r
}

// ---- altın dosya (testdata/shell.json): kabuk sonuçları bir kez kaydedilir, testler Go'yu kayıtla karşılaştırır ----
var (
	goldOnce sync.Once
	goldS    *shellgold.Store
	reTmp    = regexp.MustCompile(`(?:/private)?(?:/var/folders/[^/\s]+/[^/\s]+/T|/tmp)/[^\s"']+`) // geçici yollar altın dosyaya girmesin
)

func goldDo[T any](t *testing.T, f func() T) T {
	t.Helper()
	goldOnce.Do(func() { goldS = shellgold.Open(t, "shell") })
	return shellgold.Do(t, goldS, goldS.Next(t), f)
}

func nt(s string) string { return reTmp.ReplaceAllString(s, "<T>") }

type shT struct {
	Code           int
	Out, Err, Tree string
}

// shellG: shell()'in altın dosyalı hali (anahtar: test adı + sıra).
func shellG(t *testing.T, r string, env []string, fn string, args ...string) sh {
	t.Helper()
	g := goldDo(t, func() shT {
		x := shell(t, r, env, fn, args...)
		return shT{x.code, nt(x.out), nt(x.err), ""}
	})
	return sh{g.Code, g.Out, g.Err}
}

// shellGT: shell() + çalışmadan sonra dir'in dosya ağacı (yan etkili işlevler için).
func shellGT(t *testing.T, r, dir string, env []string, fn string, args ...string) (sh, string) {
	t.Helper()
	g := goldDo(t, func() shT {
		x := shell(t, r, env, fn, args...)
		return shT{x.code, nt(x.out), nt(x.err), tree(t, dir)}
	})
	return sh{g.Code, g.Out, g.Err}, g.Tree
}

type sh struct {
	code     int
	out, err string
}

func shell(t *testing.T, r string, env []string, fn string, args ...string) sh {
	t.Helper()
	script := `export SISTEM_PATH="$1"; shift; for l in Validate.sh ModuleFiles.sh SSHHelper.sh SSHModule.sh SSHModuleEdit.sh SSHKeyCopy.sh Servers.sh Knock.sh SSHConfigImport.sh Settings.sh; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; "$@"`
	cmd := exec.Command("bash", append([]string{"-c", script, "_", filepath.Join(r, "referans/kabuk/sistem"), fn}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return sh{code, so.String(), se.String()}
}

// tree: göreli yol → "izin içerik" (klasörde yalnızca izin).
func tree(t *testing.T, dir string) string {
	var lines []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || p == dir {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		if info.IsDir() {
			lines = append(lines, fmt.Sprintf("%s/ %v", rel, info.Mode().Perm()))
		} else {
			b, _ := os.ReadFile(p)
			lines = append(lines, fmt.Sprintf("%s %v %q", rel, info.Mode().Perm(), b))
		}
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestValidateTargetMatchesShell(t *testing.T) {
	r := root(t)
	ips := []string{"10.0.0.5", "a-b.example.com", "::1", "fe80::1", "", "a b", "a;b", "$(x)", "ü", "a\n", "-x", "host_name"}
	users := []string{"root", "ziya", "_x", "-x", "", "a b", "a.b-c", "a\n", "ü", "9a"}
	ports := []string{"22", "1", "65535", "0", "65536", "-1", "", "abc", "22 ", "0080", "1e3", "٢٢"}
	n := 0
	for _, ip := range ips {
		for _, u := range users {
			for _, p := range ports {
				s := shellG(t, r, nil, "ssh_validate_target", ip, u, p)
				err := ValidateTarget(ip, u, p)
				n++
				if (s.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(s.err) != err.Error()) {
					t.Errorf("%q %q %q: kabuk=%d %q go=%v", ip, u, p, s.code, s.err, err)
				}
			}
		}
	}
	t.Logf("%d birleşim", n)
	// Bilinçli fark: taşan sayı kabukta geçer (hata), Go reddeder
	if ValidateTarget("a", "b", "99999999999999999999") == nil {
		t.Error("taşan port kabul edildi")
	}
}

func TestNameValidMatchesShell(t *testing.T) {
	r := root(t)
	for _, n := range []string{"web1", "a", "_a", "A", "1a", "a-b", "a_b", "", "a b", "ab\n", "ü", "web_01"} {
		s := shellG(t, r, nil, "module_name_valid", n)
		if (s.code == 0) != NameValid(n) {
			t.Errorf("%q: kabuk=%d go=%v", n, s.code, NameValid(n))
		}
	}
}

var envCases = []string{
	"IP=1.2.3.4\nUSER=u\nPORT=22\n",
	"# yorum\nIP = 1.2.3.4  \n  USER=\"u x\"\nPORT='22'\n",
	"IP=1.2.3.4 # sonda yorum\nIP=ikinci\n",
	"IP=1.2.3.4",               // sonda satır sonu yok
	"\n\n#x=1\nKEY=/a b/c\n\n", // boş satırlar, yorumlanmış anahtar
	"IP=a=b=c\nEMPTY=\n",       // değerde =
	"\tIP=tab\n",               // girintili
	"IP=\"\"\nUSER=''\n",       // boş tırnak
	"IP=\"a\nUSER=b'\n",        // eşleşmeyen tırnak
	"IP=ü ğ\n",
	"noequals\nIP=x\n",
	"",
}

func TestEnvGetMatchesShell(t *testing.T) {
	r := root(t)
	dir := t.TempDir()
	for i, c := range envCases {
		f := filepath.Join(dir, fmt.Sprintf("e%d", i))
		os.WriteFile(f, []byte(c), 0o600)
		for _, k := range []string{"IP", "USER", "PORT", "KEY", "EMPTY", "x", "ip", ""} {
			s := shellG(t, r, nil, "_ssh_env_get", f, k)
			v, ok := EnvGet(f, k)
			if (s.code == 0) != ok || (ok && s.out != v) {
				t.Errorf("durum %d anahtar %q: kabuk=%d %q go=%v %q", i, k, s.code, s.out, ok, v)
			}
		}
	}
	if _, ok := EnvGet(filepath.Join(dir, "yok"), "IP"); ok {
		t.Error("olmayan dosya")
	}
}

func TestEnvSetMatchesShell(t *testing.T) {
	r := root(t)
	n := 0
	for i, c := range envCases {
		for _, k := range []string{"IP", "PORT", "NEW", "KEY"} {
			for _, v := range []string{"9.9.9.9", "a b", "x#y", `back\slash\n`, "$HOME", "", "ü", "a=b"} {
				d1, d2 := t.TempDir(), t.TempDir()
				f1, f2 := filepath.Join(d1, "server.env"), filepath.Join(d2, "server.env")
				os.WriteFile(f1, []byte(c), 0o644)
				os.WriteFile(f2, []byte(c), 0o644)
				s, shTree := shellGT(t, r, d1, nil, "_ssh_env_set", f1, k, v)
				err := EnvSet(f2, k, v)
				n++
				if s.code != 0 || err != nil {
					t.Fatalf("durum %d %s=%q: kabuk=%d %q go=%v", i, k, v, s.code, s.err, err)
				}
				if a, b := shTree, tree(t, d2); a != b {
					t.Errorf("durum %d %s=%q: ağaç farklı\nkabuk: %s\ngo:    %s", i, k, v, a, b)
				}
			}
		}
	}
	// dosya hiç yoksa oluşturur (izin 600)
	d1, d2 := t.TempDir(), t.TempDir()
	_, newTree := shellGT(t, r, d1, nil, "_ssh_env_set", filepath.Join(d1, "server.env"), "A", "1")
	EnvSet(filepath.Join(d2, "server.env"), "A", "1")
	if a, b := newTree, tree(t, d2); a != b {
		t.Errorf("yeni dosya: %s | %s", a, b)
	}
	t.Logf("%d birleşim", n)
}

func TestWriteMatchesShell(t *testing.T) {
	r := root(t)
	type c struct{ name, ip, user, port, key, os string }
	for _, x := range []c{
		{"web1", "10.0.0.5", "root", "22", "", ""},
		{"web1", "10.0.0.5", "root", "2222", "/k/id", "ubuntu-26.04"},
		{"web1", "h.example", "u", "22", "~/k", ""},
		{"Web", "10.0.0.5", "root", "22", "", ""},
		{"web1", "a b", "root", "22", "", ""},
		{"web1", "1.1.1.1", "-x", "22", "", ""},
		{"web1", "1.1.1.1", "u", "70000", "", ""},
		{"web1", "1.1.1.1", "u", "22", "a\nb", ""},
		{"web1", "1.1.1.1", "u", "22", "", "o\ns"},
	} {
		d1, d2 := t.TempDir(), t.TempDir()
		s, shTree := shellGT(t, r, d1, nil, "server_write_files", d1, x.name, x.ip, x.user, x.port, x.key, x.os)
		err := Write(d2, x.name, x.ip, x.user, x.port, x.key, x.os)
		if (s.code == 0) != (err == nil) || (err != nil && s.code != 3 && strings.TrimSpace(s.err) != err.Error()) {
			t.Errorf("%+v: kabuk=%d %q go=%v", x, s.code, s.err, err)
		}
		if a, b := shTree, tree(t, d2); a != b {
			t.Errorf("%+v: ağaç farklı\nkabuk: %s\ngo:    %s", x, a, b)
		}
	}
	// hedef zaten var: kabuk 3, Go ErrExists; hiçbir şey değişmez
	d1, d2 := t.TempDir(), t.TempDir()
	for _, d := range []string{d1, d2} {
		os.MkdirAll(filepath.Join(d, "web1"), 0o755)
		os.WriteFile(filepath.Join(d, "web1", "x"), []byte("eski"), 0o644)
	}
	s, existTree := shellGT(t, r, d1, nil, "server_write_files", d1, "web1", "1.1.1.1", "u", "22")
	err := Write(d2, "web1", "1.1.1.1", "u", "22", "", "")
	if s.code != 3 || err != ErrExists || existTree != tree(t, d2) {
		t.Errorf("var olan: kabuk=%d go=%v", s.code, err)
	}
}

func TestNamesSummaryListLineMatchShell(t *testing.T) {
	r := root(t)
	dir := t.TempDir()
	mk := func(name, env string) {
		os.MkdirAll(filepath.Join(dir, name), 0o700)
		if env != "" {
			os.WriteFile(filepath.Join(dir, name, "server.env"), []byte(env), 0o600)
		}
	}
	mk("web1", "IP=10.0.0.5\nUSER=root\nPORT=2222\n")
	mk("db", "IP=db.example\nUSER=u\n")
	mk("eksik", "IP=1.1.1.1\n")
	mk("bozuk", "IP=a b\nUSER=u\nPORT=22\n")
	mk("Buyuk", "IP=1.1.1.1\nUSER=u\n")
	mk("envsiz", "")
	mk(".gizli", "IP=1.1.1.1\nUSER=u\n")
	mk("port22", "IP=1.1.1.1\nUSER=u\nPORT=22\n")
	os.WriteFile(filepath.Join(dir, "dosya"), nil, 0o600)
	env := []string{"SSHPOCKET_SERVERS_DIR=" + dir}
	s := shellG(t, r, env, "srv_names")
	want := strings.Fields(s.out)
	got := Names(dir)
	sort.Strings(want)
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Errorf("Names: kabuk=%v go=%v", want, got)
	}
	for _, n := range []string{"web1", "db", "eksik", "bozuk", "port22", "yok", "Buyuk"} {
		s := shellG(t, r, env, "server_summary", n)
		if s.out != Summary(dir, n) {
			t.Errorf("Summary %s: kabuk=%q go=%q", n, s.out, Summary(dir, n))
		}
	}
	// ModuleManager list satırı
	for _, n := range []string{"web1", "db", "eksik", "bozuk"} {
		o := goldDo(t, func() string {
			c := exec.Command("bash", filepath.Join(r, "referans/kabuk/sistem/ModuleManager.sh"), "list")
			c.Env = append(os.Environ(), env...)
			b, _ := c.Output()
			return strings.ReplaceAll(string(b), dir, "<D>")
		})
		if !strings.Contains(o, strings.ReplaceAll(ListLine(dir, n), dir, "<D>")+"\n") {
			t.Errorf("ListLine %s: %q yok:\n%s", n, ListLine(dir, n), o)
		}
	}
}

func TestRenameMatchesShell(t *testing.T) {
	r := root(t)
	build := func() string {
		d := t.TempDir()
		os.MkdirAll(filepath.Join(d, "eski"), 0o700)
		os.WriteFile(filepath.Join(d, "eski", "server.env"), []byte("IP=1.1.1.1\nUSER=u\n"), 0o600)
		os.WriteFile(filepath.Join(d, "eski", "id_ed25519_eski"), []byte("PRIV"), 0o600)
		os.WriteFile(filepath.Join(d, "eski", "id_ed25519_eski.pub"), []byte("PUB"), 0o644)
		os.WriteFile(filepath.Join(d, "eski", "baska"), []byte("x"), 0o644)
		os.MkdirAll(filepath.Join(d, "var"), 0o700)
		return d
	}
	for _, c := range [][2]string{{"eski", "yeni"}, {"eski", "var"}, {"eski", "Yeni"}, {"yok", "yeni"}, {"Eski", "yeni"}, {"eski", "eski"}} {
		d1, d2 := build(), build()
		type rn struct {
			Failed       bool
			Stderr, Tree string
		}
		g := goldDo(t, func() rn {
			cmd := exec.Command("bash", filepath.Join(r, "referans/kabuk/sistem/ModuleManager.sh"), "rename", c[0], c[1])
			cmd.Env = append(os.Environ(), "SSHPOCKET_SERVERS_DIR="+d1, "SSHPOCKET_MODULES_DIR="+t.TempDir())
			var se bytes.Buffer
			cmd.Stderr = &se
			e := cmd.Run()
			return rn{e != nil, nt(se.String()), tree(t, d1)}
		})
		err2 := Rename(d2, c[0], c[1])
		if g.Failed != (err2 != nil) {
			t.Errorf("%v: kabuk başarısız=%v (%s) go=%v", c, g.Failed, g.Stderr, err2)
		}
		if err2 != nil && !strings.Contains(g.Stderr, strings.TrimPrefix(err2.Error(), "")) {
			t.Errorf("%v: ileti kabuk=%q go=%q", c, g.Stderr, err2)
		}
		if a, b := g.Tree, tree(t, d2); a != b {
			t.Errorf("%v: ağaç farklı\nkabuk: %s\ngo:    %s", c, a, b)
		}
	}
}

func TestKnockPortsValidMatchesShell(t *testing.T) {
	r := root(t)
	for _, s := range []string{"7000", "7000,8000,9000", "1,65535", "0", "65536", "", ",", "7000,", ",7000", "7000,,8000", "a", "70 00", "007000", "1,2,3,4,5,6,7,8,9,10", "1,2,3,4,5,6,7,8,9,10,11", "-1", "7000\n", "0080", "00"} {
		sh := shellG(t, r, nil, "knock_ports_valid", s)
		if (sh.code == 0) != KnockPortsValid(s) {
			t.Errorf("%q: kabuk=%d go=%v", s, sh.code, KnockPortsValid(s))
		}
	}
}

func TestKnockSettingsAndPreConnectMatchShell(t *testing.T) {
	r := root(t)
	for _, c := range []struct{ env, want string }{
		{"IP=1.1.1.1\n", ""},
		{"KNOCK=keychain\n", ""},
		{"KNOCK=keychain\nKNOCK_PROTO=udp\nKNOCK_DELAY_MS=0\n", ""},
		{"KNOCK=keychain\nKNOCK_PROTO=icmp\n", "x"},
		{"KNOCK=keychain\nKNOCK_DELAY_MS=5001\n", "x"},
		{"KNOCK=keychain\nKNOCK_DELAY_MS=abc\n", "x"},
		{"KNOCK=keychain\nKNOCK_DELAY_MS=5000\n", ""},
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "server.env"), []byte(c.env), 0o600)
		// kabuk: dizi Keychain'de "7000,8000" varsayılır; vuruş sahte (kayıt)
		script := `knock_get_sequence() { echo "7000,8000"; }; _knock_send_one() { echo "vurus $1 $2 $3"; }; knock_pre_connect WEB1 "$1" 127.0.0.1`
		type ks struct {
			Failed bool
			Stderr string
		}
		g := goldDo(t, func() ks {
			cmd := exec.Command("bash", "-c", `export SISTEM_PATH="$1"; for l in Validate.sh ModuleFiles.sh SSHHelper.sh SSHModule.sh SSHModuleEdit.sh Servers.sh Knock.sh; do . "$SISTEM_PATH/Helpers/$l"; done; shift; `+script, "_", filepath.Join(r, "referans/kabuk/sistem"), dir)
			var so, se bytes.Buffer
			cmd.Stdout, cmd.Stderr = &so, &se
			e := cmd.Run()
			return ks{e != nil, nt(se.String())}
		})
		_, _, err := KnockSettings(dir)
		if g.Failed != (err != nil) {
			t.Errorf("%q: kabuk başarısız=%v (%s) go=%v", c.env, g.Failed, g.Stderr, err)
			continue
		}
		if err != nil && strings.TrimSpace(strings.TrimPrefix(g.Stderr, "⚠️  ")) != err.Error() {
			t.Errorf("%q: ileti kabuk=%q go=%q", c.env, g.Stderr, err)
		}
	}
}

func TestManualKeyHelpMatchesShell(t *testing.T) {
	r := root(t)
	dir := t.TempDir()
	for i, line := range []string{"ssh-ed25519 AAAA ziya@host", "ssh-ed25519 AAAA it's a 'test'", "ssh-ed25519 AAAA \\ $x `y`", "ssh-ed25519 AAAA ü\nikinci satir"} {
		f := filepath.Join(dir, fmt.Sprintf("k%d.pub", i))
		os.WriteFile(f, []byte(line+"\n"), 0o644)
		s := shellG(t, r, nil, "_ssh_manual_key_help", f)
		got, err := ManualKeyHelp(f)
		if err != nil || got != s.out {
			t.Errorf("%q:\nkabuk=%q\ngo=   %q", line, s.out, got)
		}
	}
}

// ---------- ağ: gerçek yerel dinleyicilerle ----------

func TestPortOpenAndStatus(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	if !PortOpen("127.0.0.1", port, 1) {
		t.Error("açık port kapalı göründü")
	}
	l2, _ := net.Listen("tcp", "127.0.0.1:0")
	_, closedPort, _ := net.SplitHostPort(l2.Addr().String())
	l2.Close()
	if PortOpen("127.0.0.1", closedPort, 1) {
		t.Error("kapalı port açık göründü")
	}
	dir := t.TempDir()
	mk := func(name, env string) {
		os.MkdirAll(filepath.Join(dir, name), 0o700)
		os.WriteFile(filepath.Join(dir, name, "server.env"), []byte(env), 0o600)
	}
	mk("acik", "IP=127.0.0.1\nUSER=u\nPORT="+port+"\n")
	mk("kapali", "IP=127.0.0.1\nUSER=u\nPORT="+closedPort+"\n")
	mk("kilitli", "IP=127.0.0.1\nUSER=u\nPORT="+closedPort+"\nKNOCK=keychain\n")
	mk("ayarsiz", "USER=u\n")
	os.MkdirAll(filepath.Join(dir, "envsiz"), 0o700)
	for n, want := range map[string]string{"acik": "green", "kapali": "red", "kilitli": "lock", "ayarsiz": "unset", "envsiz": "unset"} {
		if got := Status(dir, n, 1); got != want {
			t.Errorf("%s: %s != %s", n, got, want)
		}
	}
}

func TestKnockSendOrderTCP(t *testing.T) {
	var ls []net.Listener
	var ports []string
	got := make(chan string, 10)
	for i := 0; i < 3; i++ {
		l, _ := net.Listen("tcp", "127.0.0.1:0")
		ls = append(ls, l)
		_, p, _ := net.SplitHostPort(l.Addr().String())
		ports = append(ports, p)
		go func(p string) {
			c, err := l.Accept()
			if err == nil {
				c.Close()
				got <- p
			}
		}(p)
		defer l.Close()
	}
	KnockSend(context.Background(), "127.0.0.1", strings.Join(ports, ","), "tcp", 50)
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
}

func execCommand(r, serversDir, cfg string) *exec.Cmd {
	c := exec.Command("bash", filepath.Join(r, "referans/kabuk/sistem/ImportSSHConfig.sh"), cfg)
	c.Env = append(os.Environ(), "SSHPOCKET_SERVERS_DIR="+serversDir, "SSHPOCKET_MODULES_DIR="+filepath.Join(serversDir, "..yok"), "SSHPOCKET_UI=numbers")
	c.Stdin = strings.NewReader("a\n")
	return c
}
