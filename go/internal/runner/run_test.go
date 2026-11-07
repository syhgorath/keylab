package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"sshpocket-go/internal/manifest"
)

const fakeAnsible = `#!/bin/bash
echo "ARGV: $*"
echo "ENV: CFG=$ANSIBLE_CONFIG HK=$ANSIBLE_HOST_KEY_CHECKING TO=$ANSIBLE_TIMEOUT PYB=$PYTHONDONTWRITEBYTECODE PWD=$(pwd) PATH0=${PATH%%:*}"
for a in "$@"; do case "$a" in @*) echo "VARS: $(cat "${a#@}")";; esac; done
while [ $# -gt 0 ]; do if [ "$1" = -i ]; then echo "INV:"; cat "$2"; fi; shift; done
[ -n "$FAKE_ANS_OUT" ] && printf '%s\n' "$FAKE_ANS_OUT"
if [ "$FAKE_ASK" = 1 ]; then printf 'BECOME password: '; IFS= read -r pw; echo "got len=${#pw}"; fi
[ -n "$FAKE_ANS_SIGNAL" ] && kill -TERM $$
exit "${FAKE_ANS_RC:-0}"
`

// fakeSSH2: betik runner'ı için ssh davranışları (fakeSSH'ın genişletilmişi).
const fakeSSH2 = `#!/bin/bash
echo "ssh $*" >> "${FAKE_LOG:-/dev/null}"
cmd="${@: -1}"
case "$cmd" in
  "cat /etc/os-release") printf '%s' "$FAKE_OSREL" ;;
  "umask 077; f=\$(mktemp /tmp/sshpocket-XXXXXXXX)"*) h=$(shasum -a 256 | cut -d' ' -f1); echo "REMOTE-WRITE payload=$h" >> "${FAKE_LOG:-/dev/null}"; printf '%s' "$FAKE_RPATH"; [ "$FAKE_WRITE_FAIL" = 1 ] && exit 5 ;;
  "rm -f "*) echo "REMOTE-RM $cmd" >> "${FAKE_LOG:-/dev/null}" ;;
  "bash -s --"*) echo "RUN: $cmd"; echo "STDIN-HASH: $(shasum -a 256 | cut -d' ' -f1)"; [ -n "$FAKE_ERR" ] && echo "$FAKE_ERR" >&2; exit "${FAKE_RC:-0}" ;;
  *"bash -c"*) echo "SUDO-RUN: $cmd"; [ -n "$FAKE_ERR" ] && echo "$FAKE_ERR" >&2; exit "${FAKE_RC:-0}" ;;
  *) printf '%s' "$FAKE_OUT" ;;
esac
exit 0
`

type world struct {
	shared string // anahtar, known_hosts, sahte araçlar (iki uygulama için ortak, salt okunur)
	key    string
	bin    string // venv bin
	known  string
	fp     string
}

func newWorld(t *testing.T) *world {
	dir := t.TempDir()
	w := &world{shared: dir, bin: filepath.Join(dir, "venvbin"), known: filepath.Join(dir, "known_hosts")}
	os.MkdirAll(w.bin, 0o755)
	os.WriteFile(filepath.Join(w.bin, "ansible-playbook"), []byte(fakeAnsible), 0o755)
	tools := filepath.Join(dir, "tools")
	os.MkdirAll(tools, 0o755)
	os.WriteFile(filepath.Join(tools, "ssh"), []byte(fakeSSH2), 0o755)
	os.WriteFile(filepath.Join(tools, "ssh-add"), []byte(fakeSSHAdd), 0o755)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_LOG", filepath.Join(dir, "calls.log"))
	priv, pub := realKey(t, dir)
	w.key = priv
	os.WriteFile(w.known, []byte("10.0.0.5 "+pub+"\n"), 0o600)
	fpOut := mustRun(t, "/usr/bin/ssh-keygen", "-lf", priv)
	w.fp = strings.Fields(fpOut)[1]
	t.Setenv("FAKE_AGENT", "256 "+w.fp+" x (ED25519)\n")
	t.Setenv("FAKE_OSREL", "ID=ubuntu\nVERSION_ID=\"26.04\"\n")
	t.Setenv("SSHPOCKET_KNOWN_HOSTS", w.known)
	return w
}

type scenario struct {
	name       string
	pkgDir     string // gerçek ya da sentetik paket dizini
	action     string
	user       string
	profile    string
	stdin      string
	results    []string // results dosyasına eklenecek satırlar; %d = şimdi - 10
	vars       string   // "\x00" = dosya yok
	env        map[string]string
	runset     string
	becomeExe  string
	noBin      bool
	badHost    bool
	noAgent    bool
	skipVerify bool
}

type runOut struct {
	rc             int
	stdout, stderr string
	tree           map[string]string
}

var (
	reWork  = regexp.MustCompile(`sshpocket-run\.[A-Za-z0-9]+`)
	reStamp = regexp.MustCompile(`[0-9]{8}-[0-9]{6}`)
	// bash, sinyalle ölen alt süreç için kendi iş durumu satırını yazar; Go bunu üretmez (bilinçli fark)
	reBashJob = regexp.MustCompile(`(?m)^[^\n]*line [0-9]+: +[0-9]+ +Terminated: 15[^\n]*\n?`)
)

func normOut(s, state string) string {
	s = strings.ReplaceAll(s, state, "STATE")
	s = reBashJob.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "//", "/") // TMPDIR sonundaki "/" (kabuk "T//x" yazar)
	s = reWork.ReplaceAllString(s, "sshpocket-run.X")
	s = reStamp.ReplaceAllString(s, "STAMP")
	return s
}

func stateTree(state string) map[string]string {
	out := map[string]string{}
	filepath.WalkDir(state, func(p string, d os.DirEntry, err error) error {
		if err != nil || p == state {
			return nil
		}
		rel, _ := filepath.Rel(state, p)
		rel = reStamp.ReplaceAllString(rel, "STAMP")
		st, _ := os.Stat(p)
		if d.IsDir() {
			out[rel+"/"] = fmt.Sprintf("%o", st.Mode().Perm())
			return nil
		}
		b, _ := os.ReadFile(p)
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
		out[rel] = fmt.Sprintf("%o %s", st.Mode().Perm(), normOut(strings.Join(lines, "\n"), state))
		return nil
	})
	return out
}

func prepState(t *testing.T, state string, sc scenario, id string) {
	if len(sc.results) == 0 {
		return
	}
	dir := filepath.Join(state, id, "srv")
	os.MkdirAll(dir, 0o700)
	var sb strings.Builder
	for _, r := range sc.results {
		sb.WriteString(fmt.Sprintf(r, time.Now().Unix()-10) + "\n")
	}
	os.WriteFile(filepath.Join(dir, "results"), []byte(sb.String()), 0o600)
}

func runScenario(t *testing.T, root string, w *world, sc scenario) (sh, gv runOut) {
	t.Helper()
	for k, v := range sc.env {
		t.Setenv(k, v)
	}
	info := infoOf(t, filepath.Join(sc.pkgDir, "component.json"))
	id := infoGet(info, "id")
	ver := infoGet(info, "version")
	bin := w.bin
	if sc.noBin {
		bin = filepath.Join(w.shared, "yok")
	}
	known := w.known
	host := "10.0.0.5"
	if sc.badHost {
		host = "10.9.9.9"
	}
	if sc.noAgent {
		t.Setenv("FAKE_AGENT", "")
	}
	values := filepath.Join(w.shared, "values")
	valArg := values
	if sc.vars == "\x00" {
		valArg = ""
		os.Remove(values)
	} else {
		os.WriteFile(values, []byte(sc.vars), 0o600)
	}

	for impl := 0; impl < 2; impl++ {
		state := t.TempDir()
		prepState(t, state, sc, id)
		if impl == 0 {
			env := []string{
				"SSHPOCKET_STATE_DIR=" + state, "SSHPOCKET_KNOWN_HOSTS=" + known,
				"COMP_CTX_PKGDIR=" + sc.pkgDir, "COMP_CTX_BINDIR=" + bin, "COMP_CTX_SERVER=srv", "COMP_CTX_HOST=" + host, "COMP_CTX_USER=" + sc.user,
				"COMP_CTX_PORT=22", "COMP_CTX_KEY=" + w.key, "COMP_CTX_PREFIX=SRV", "COMP_CTX_PROFILE=" + sc.profile, "COMP_CTX_VALUES=" + valArg,
				"COMP_CTX_BECOME_EXE=" + sc.becomeExe, "COMP_CTX_RUNSET=" + sc.runset, "COMP_REQUIRES_TTL=1800",
			}
			o := runShell(t, root, env, sc.stdin, "", "comp_exec_action", strings.Join(info, "\n"), id, ver, sc.action)
			sh = runOut{o.code, normOut(o.stdout, state), normOut(o.stderr, state), stateTree(state)}
		} else {
			var so, se bytes.Buffer
			ec := &ExecCtx{PkgDir: sc.pkgDir, BinDir: bin, Server: "srv", Host: host, User: sc.user, Port: "22", Key: w.key, Prefix: "SRV", Profile: sc.profile,
				ValuesFile: valArg, BecomeExe: sc.becomeExe, RunSet: sc.runset, Gates: &Gates{StateDir: state}, KnownHosts: known,
				Stdin: strings.NewReader(sc.stdin), Stdout: &so, Stderr: &se, Ctx: context.Background()}
			res := ExecAction(ec, info, id, ver, sc.action)
			gv = runOut{res.RC, normOut(so.String(), state), normOut(se.String(), state), stateTree(state)}
		}
	}
	return
}

func compareRuns(t *testing.T, name string, sh, gv runOut) {
	t.Helper()
	if sh.rc != gv.rc {
		t.Errorf("[%s] çıkış kabuk=%d go=%d\n kabuk stderr=%q\n go stderr   =%q", name, sh.rc, gv.rc, sh.stderr, gv.stderr)
		return
	}
	if sh.stdout != gv.stdout {
		t.Errorf("[%s] stdout\n kabuk=%q\n go   =%q", name, sh.stdout, gv.stdout)
	}
	if sh.stderr != gv.stderr {
		t.Errorf("[%s] stderr\n kabuk=%q\n go   =%q", name, sh.stderr, gv.stderr)
	}
	if !reflect.DeepEqual(sh.tree, gv.tree) {
		var ks []string
		for k := range sh.tree {
			ks = append(ks, k)
		}
		for k := range gv.tree {
			if _, ok := sh.tree[k]; !ok {
				ks = append(ks, k)
			}
		}
		sort.Strings(ks)
		for _, k := range ks {
			if sh.tree[k] != gv.tree[k] {
				t.Errorf("[%s] durum farkı %s\n kabuk=%q\n go   =%q", name, k, sh.tree[k], gv.tree[k])
			}
		}
	}
}

func TestAnsibleExecMatchesShell(t *testing.T) {
	root := repoRoot(t)
	util := filepath.Join(root, "components/utility/0.1.0")
	kur := filepath.Join(root, "components/kurulum/0.1.0")
	freshPreview := []string{"%d|0.1.0|preview|ok"}
	okOut := "SSHPOCKET_PROGRESS alan=base sira=1/1 durum=bitti degisen=2\nSSHPOCKET_RESULT utility.durum timezone=UTC locale=en_US ntp_servis=yes ntp_senkron=yes offset_ms=3 kaynak_sayisi=2"
	scs := []scenario{
		{name: "durum başarı", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "\x00", env: map[string]string{"FAKE_ANS_OUT": okOut}},
		{name: "durum sonuç satırı bozuk", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", env: map[string]string{"FAKE_ANS_OUT": "SSHPOCKET_RESULT utility.durum zzz=1"}},
		{name: "durum sonuç satırı yok", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", env: map[string]string{"FAKE_ANS_OUT": "hiç sonuç yok"}},
		{name: "preview başarı + değişken", pkgDir: util, action: "preview", user: "deploy", profile: "ubuntu-26.04", vars: "utility_timezone=Europe/Istanbul\nutility_ntp_servers=a.example,b.example\nutility_backup=false\n", env: map[string]string{"FAKE_ANS_OUT": "SSHPOCKET_RESULT utility.preview degisen=3 plan=ok", "FAKE_ASK": "1"}, stdin: "gizliparola\n"},
		{name: "apply ön koşul yok", pkgDir: util, action: "apply", user: "deploy", profile: "ubuntu-26.04", vars: "", stdin: "e\n"},
		{name: "apply onay e", pkgDir: util, action: "apply", user: "deploy", profile: "ubuntu-26.04", vars: "", results: freshPreview, stdin: "e\n", env: map[string]string{"FAKE_ASK": "1", "FAKE_ANS_OUT": "SSHPOCKET_RESULT utility.apply degisen=1 plan=ok yedek=yok"}},
		{name: "apply onay hayır", pkgDir: util, action: "apply", user: "deploy", profile: "ubuntu-26.04", vars: "", results: freshPreview, stdin: "h\n"},
		{name: "apply onay yok (EOF)", pkgDir: util, action: "apply", user: "deploy", profile: "ubuntu-26.04", vars: "", results: freshPreview, stdin: ""},
		{name: "apply root reddi", pkgDir: util, action: "apply", user: "root", profile: "ubuntu-26.04", vars: "", results: freshPreview, stdin: "e\n"},
		{name: "durum root serbest", pkgDir: util, action: "durum", user: "root", profile: "ubuntu-26.04", vars: "", env: map[string]string{"FAKE_ANS_OUT": okOut}},
		{name: "bilinmeyen host", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", badHost: true},
		{name: "ajanda anahtar yok", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", noAgent: true},
		{name: "profil desteklenmiyor", pkgDir: util, action: "durum", user: "deploy", profile: "arch-1", vars: ""},
		{name: "profil boş", pkgDir: util, action: "durum", user: "deploy", profile: "", vars: ""},
		{name: "venv yok", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", noBin: true},
		{name: "geçersiz değişken", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "utility_backup=belki\nbilinmeyen_x=1\n"},
		{name: "ansible başarısız", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", env: map[string]string{"FAKE_ANS_RC": "2", "FAKE_ANS_OUT": "fatal: boom"}},
		{name: "sudo reddi ipucu", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", env: map[string]string{"FAKE_ANS_RC": "2", "FAKE_ANS_OUT": "Incorrect sudo password"}},
		{name: "sudo zaman aşımı ipucu", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", env: map[string]string{"FAKE_ANS_RC": "4", "FAKE_ANS_OUT": "Timeout (12s) waiting for privilege escalation prompt:"}},
		{name: "sinyal ile ölüm", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", env: map[string]string{"FAKE_ANS_SIGNAL": "1"}},
		{name: "sudo.ws envanterde", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", becomeExe: "/usr/bin/sudo.ws", env: map[string]string{"FAKE_ANS_OUT": okOut}},
		{name: "parola maskeleme günlükte", pkgDir: util, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "", env: map[string]string{"FAKE_ANS_OUT": "BECOME password: abc123\nghp_abcdefgh12345678"}},
		{name: "kurulum durum + katalog", pkgDir: kur, action: "preview", user: "deploy", profile: "ubuntu-26.04", vars: "", runset: "kurulum_yazilim=docker", env: map[string]string{"FAKE_ANS_OUT": "SSHPOCKET_RESULT kurulum.preview yazilim=docker mod=native degisen=0"}},
		{name: "katalog reddi", pkgDir: kur, action: "preview", user: "deploy", profile: "ubuntu-26.04", vars: "", runset: "kurulum_yazilim=yok_yazilim"},
		{name: "yıkıcı: ad yazılmalı", pkgDir: kur, action: "yukselt", user: "deploy", profile: "ubuntu-26.04", vars: "", results: []string{"%d|0.1.0|preview|ok"}, stdin: "srv\n", env: map[string]string{"FAKE_ANS_OUT": "ok"}},
		{name: "yıkıcı: yanlış ad", pkgDir: kur, action: "yukselt", user: "deploy", profile: "ubuntu-26.04", vars: "", results: []string{"%d|0.1.0|preview|ok"}, stdin: "x\ny\nz\n"},
	}
	w := newWorld(t)
	for _, sc := range scs {
		t.Run(sc.name, func(t *testing.T) {
			for _, k := range []string{"FAKE_ANS_OUT", "FAKE_ANS_RC", "FAKE_ASK", "FAKE_ANS_SIGNAL"} {
				t.Setenv(k, "")
			}
			sh, gv := runScenario(t, root, w, sc)
			compareRuns(t, sc.name, sh, gv)
			if sc.name == "durum başarı" && (!strings.Contains(sh.stdout, "ARGV:") || !strings.Contains(sh.stdout, "VARS:")) {
				t.Errorf("sahte ansible çalışmadı: %q", sh.stdout)
			}
		})
	}
}

// Betik runner'ı: gerçek pakette yok; sentetik paket (script) kurulur.
func synthScriptPkg(t *testing.T, sudo bool) string {
	dir := filepath.Join(t.TempDir(), "scr")
	os.MkdirAll(filepath.Join(dir, "scripts"), 0o755)
	cmd := "scripts/run.sh --flag value"
	manifest := fmt.Sprintf(`{"id":"scr","version":"1.0.0","sshpocket_api":1,"runner":"script","profiles":[{"id":"ubuntu-26.04","status":"supported"}],
	"actions":{"durum":{"cmd":%q,"effects":[],"requires_sudo":%v,"outputs":{"format":"keyvalue-line","keys":["ok"],"prefix":"SSHPOCKET_RESULT scr.durum"}}},
	"vars":[{"name":"scr_x","type":"string","default":"merhaba"}]}`, cmd, sudo)
	os.WriteFile(filepath.Join(dir, "component.json"), []byte(manifest), 0o644)
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "scripts", "run.sh"), []byte("#!/bin/bash\necho SSHPOCKET_RESULT scr.durum ok=1\n"), 0o755)
	return dir
}

func TestScriptRunnerMatchesShell(t *testing.T) {
	root := repoRoot(t)
	w := newWorld(t)
	_ = manifest.InfoGet
	for _, c := range []struct {
		name string
		sudo bool
		env  map[string]string
		mod  func(dir string)
	}{
		{"sudo'suz başarı", false, map[string]string{"FAKE_RC": "0"}, nil},
		{"sudo'suz hata", false, map[string]string{"FAKE_RC": "3", "FAKE_ERR": "boom"}, nil},
		{"sudo yolu", true, map[string]string{"FAKE_RPATH": "/tmp/sshpocket-AbCdEfGh", "FAKE_RC": "0"}, nil},
		{"sudo yolu özet uyuşmuyor", true, map[string]string{"FAKE_RPATH": "/tmp/sshpocket-AbCdEfGh", "FAKE_RC": "97"}, nil},
		{"sudo yolu geçersiz uzak yol", true, map[string]string{"FAKE_RPATH": "/tmp/evil path"}, nil},
		{"sudo yolu yazma hatası", true, map[string]string{"FAKE_WRITE_FAIL": "1", "FAKE_RPATH": ""}, nil},
		{"betik yok", false, nil, func(d string) { os.Remove(filepath.Join(d, "scripts", "run.sh")) }},
		{"betik sembolik bağ", false, nil, func(d string) {
			os.Remove(filepath.Join(d, "scripts", "run.sh"))
			os.Symlink("/etc/hosts", filepath.Join(d, "scripts", "run.sh"))
		}},
		{"betik büyük", false, nil, func(d string) {
			os.WriteFile(filepath.Join(d, "scripts", "run.sh"), bytes.Repeat([]byte("a"), 262145), 0o755)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, k := range []string{"FAKE_RC", "FAKE_ERR", "FAKE_RPATH", "FAKE_WRITE_FAIL"} {
				t.Setenv(k, "")
			}
			dir := synthScriptPkg(t, c.sudo)
			if c.mod != nil {
				c.mod(dir)
			}
			sc := scenario{name: c.name, pkgDir: dir, action: "durum", user: "deploy", profile: "ubuntu-26.04", vars: "\x00", env: c.env, becomeExe: ""}
			sh, gv := runScenario(t, root, w, sc)
			compareRuns(t, c.name, sh, gv)
			// uzak geçici dosya her çıkışta silinmeli (geçerli yol varsa)
			if c.name == "sudo yolu" {
				calls, _ := os.ReadFile(filepath.Join(w.shared, "calls.log"))
				if !strings.Contains(string(calls), "REMOTE-RM rm -f /tmp/sshpocket-AbCdEfGh") {
					t.Errorf("uzak geçici dosya silinmedi: %s", calls)
				}
			}
		})
	}
}

// Regresyon (gerçek VM denemesinde bulundu): girdi akışı HİÇ kapanmıyorsa (terminal) alt süreç bittiğinde koşu da bitmeli.
func TestExecDoesNotHangOnOpenStdin(t *testing.T) {
	root := repoRoot(t)
	w := newWorld(t)
	_ = root
	pr, pw, err := os.Pipe() // yazan uç açık kalır: okuma sonsuza dek bloklar
	if err != nil {
		t.Fatal(err)
	}
	defer pw.Close()
	defer pr.Close()
	util := filepath.Join(root, "components/utility/0.1.0")
	info := infoOf(t, filepath.Join(util, "component.json"))
	state := t.TempDir()
	var so, se bytes.Buffer
	ec := &ExecCtx{PkgDir: util, BinDir: w.bin, Server: "srv", Host: "10.0.0.5", User: "deploy", Port: "22", Key: w.key, Prefix: "SRV", Profile: "ubuntu-26.04",
		Gates: &Gates{StateDir: state}, KnownHosts: w.known, Stdin: pr, Stdout: &so, Stderr: &se, Ctx: context.Background()}
	t.Setenv("FAKE_ANS_OUT", "SSHPOCKET_RESULT utility.durum timezone=UTC")
	done := make(chan ExecResult, 1)
	go func() { done <- ExecAction(ec, info, "utility", "0.1.0", "durum") }()
	select {
	case res := <-done:
		if res.RC != 0 {
			t.Fatalf("rc=%d stderr=%q", res.RC, se.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("alt süreç bitti ama koşu terminal girdisi bekleyerek takıldı")
	}
}

type blockingReader struct{ r *os.File }

func (b blockingReader) Read(p []byte) (int, error) { return b.r.Read(p) } // *os.File DEĞİL: boru yolu denenir

func TestExecDoesNotHangOnOpenNonFileStdin(t *testing.T) {
	root := repoRoot(t)
	w := newWorld(t)
	pr, pw, _ := os.Pipe()
	defer pw.Close()
	defer pr.Close()
	util := filepath.Join(root, "components/utility/0.1.0")
	info := infoOf(t, filepath.Join(util, "component.json"))
	var so, se bytes.Buffer
	ec := &ExecCtx{PkgDir: util, BinDir: w.bin, Server: "srv", Host: "10.0.0.5", User: "deploy", Port: "22", Key: w.key, Prefix: "SRV", Profile: "ubuntu-26.04",
		Gates: &Gates{StateDir: t.TempDir()}, KnownHosts: w.known, Stdin: blockingReader{pr}, Stdout: &so, Stderr: &se, Ctx: context.Background()}
	done := make(chan int, 1)
	go func() { done <- ExecAction(ec, info, "utility", "0.1.0", "durum").RC }()
	select {
	case rc := <-done:
		if rc != 0 {
			t.Fatalf("rc=%d %q", rc, se.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("dosya olmayan açık girdide takıldı")
	}
}
