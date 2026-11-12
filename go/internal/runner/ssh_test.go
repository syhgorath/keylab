package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Sahte ssh / ssh-add ile ssh yardımcıları: kabuk ve Go AYNI sahte araçları çağırır.

const fakeSSH = `#!/bin/bash
echo "ssh $*" >> "${FAKE_LOG:-/dev/null}"
cmd="${@: -1}"
case "$cmd" in
  "cat /etc/os-release") printf '%s' "$FAKE_OSREL" ;;
  "test -x /usr/bin/sudo.ws && echo SSHPOCKET_SUDO_WS=yes") printf '%s' "$FAKE_SUDOWS" ;;
  "date +%s; timedatectl show -p Timezone -p NTPSynchronized 2>/dev/null") printf '%s' "$FAKE_CLOCK" ;;
  *) printf '%s' "$FAKE_OUT" ;;
esac
[ -n "$FAKE_ERR" ] && printf '%s\n' "$FAKE_ERR" >&2
exit "${FAKE_RC:-0}"
`

const fakeSSHAdd = `#!/bin/bash
echo "ssh-add $*" >> "${FAKE_LOG:-/dev/null}"
[ "$1" = "-l" ] && { printf '%s' "$FAKE_AGENT"; exit "${FAKE_AGENT_RC:-0}"; }
exit 0
`

// fakeBin: sahte araçları içeren klasörü PATH'in başına koyar (t.Setenv: kabuk ve Go aynı ortamı görür).
func fakeBin(t *testing.T) string {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ssh"), []byte(fakeSSH), 0o755)
	os.WriteFile(filepath.Join(dir, "ssh-add"), []byte(fakeSSHAdd), 0o755)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_LOG", filepath.Join(dir, "calls.log"))
	return dir
}

func realKey(t *testing.T, dir string) (priv, pubLine string) {
	priv = filepath.Join(dir, "key")
	if out, err := exec.Command("/usr/bin/ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, out)
	}
	b, _ := os.ReadFile(priv + ".pub")
	f := strings.Fields(string(b))
	return priv, f[0] + " " + f[1]
}

func TestKnownHostMatchesShell(t *testing.T) {
	root := goldRoot(t)
	dir := t.TempDir()
	_, pub := realKey(t, dir)
	kh := filepath.Join(dir, "known_hosts")
	os.WriteFile(kh, []byte("10.0.0.5 "+pub+"\n[10.0.0.6]:2222 "+pub+"\n"), 0o600)
	for _, c := range [][2]string{{"10.0.0.5", "22"}, {"10.0.0.6", "2222"}, {"10.0.0.6", "22"}, {"10.0.0.5", "2222"}, {"10.9.9.9", "22"}} {
		for _, file := range []string{kh, filepath.Join(dir, "yok")} {
			sh := runShell(t, root, nil, "", "", "comp_gate_known_host", c[0], c[1], file)
			err := KnownHost(c[0], c[1], file)
			if (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) {
				t.Errorf("%v %s: kabuk=%d %q go=%v", c, filepath.Base(file), sh.code, sh.stderr, err)
			}
		}
	}
	// env ile varsayılan dosya
	t.Setenv("SSHPOCKET_KNOWN_HOSTS", kh)
	if err := KnownHost("10.0.0.5", "22", ""); err != nil {
		t.Errorf("env known_hosts: %v", err)
	}
}

func TestAgentMatchesShell(t *testing.T) {
	root := goldRoot(t)
	fakeBin(t)
	dir := t.TempDir()
	priv, _ := realKey(t, dir)
	fpOut, _ := exec.Command("/usr/bin/ssh-keygen", "-lf", priv).Output()
	fp := strings.Fields(string(fpOut))[1]
	for _, c := range []struct{ name, agent string }{{"ajanda", "256 " + fp + " (ED25519)\n"}, {"başka anahtar", "256 SHA256:baska (ED25519)\n"}, {"boş", ""}, {"parça", "x" + fp + "y\n"}} {
		for _, key := range []string{priv, filepath.Join(dir, "yok")} {
			t.Setenv("FAKE_AGENT", c.agent)
			sh := runShell(t, root, nil, "", "", "comp_gate_agent", "pfx", key)
			err := Agent("pfx", key, nil)
			if (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != reTmp.ReplaceAllString(err.Error(), "<T>")) {
				t.Errorf("%s %s: kabuk=%d %q go=%v", c.name, filepath.Base(key), sh.code, sh.stderr, err)
			}
		}
	}
	// addRetry ile ekleme: önce ajanda yok, addRetry sonrası var
	t.Setenv("FAKE_AGENT", "")
	added := false
	err := Agent("p", priv, func(p, k string) error { t.Setenv("FAKE_AGENT", "256 "+fp+" x\n"); added = true; return nil })
	if err != nil || !added {
		t.Errorf("addRetry sonrası: %v %v", err, added)
	}
}

func TestDetectProfileMatchesShell(t *testing.T) {
	root := goldRoot(t)
	fakeBin(t)
	for _, p := range ordered(map[string]string{
		"ubuntu":        "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"26.04\"\n",
		"debian":        "ID=debian\nVERSION_ID=\"13\"\n",
		"büyük harf":    "ID=Ubuntu\nVERSION_ID=\"22.04\"\n",
		"ters sıra":     "VERSION_ID=\"12\"\nID=debian\n",
		"version yok":   "ID=debian\n",
		"id yok":        "VERSION_ID=\"1\"\n",
		"boş":           "",
		"kötü karakter": "ID=ub$untu\nVERSION_ID=1\n",
		"uzun":          "ID=" + strings.Repeat("a", 40) + "\nVERSION_ID=1\n",
		"tırnaklı id":   "ID=\"proxmox\"\nVERSION_ID=\"8.4\"\n",
		"eşit işaretli": "ID=a=b\nVERSION_ID=1\n",
		"ilk kazanır":   "ID=first\nID=second\nVERSION_ID=1\nVERSION_ID=2\n",
		"ID_LIKE":       "ID_LIKE=debian\nID=ubuntu\nVERSION_ID=\"26.04\"\n",
	}) {
		name, osrel := p.K, p.V
		t.Setenv("FAKE_OSREL", osrel)
		sh := runShell(t, root, nil, "", "", "comp_detect_profile", "10.0.0.5", "22", "root", "/k")
		got, _, err := DetectProfile("10.0.0.5", "22", "root", "/k")
		if (sh.code == 0) != (err == nil) || (err == nil && sh.stdout != got) {
			t.Errorf("[%s] kabuk=%d %q go=%q %v", name, sh.code, sh.stdout, got, err)
		}
	}
	t.Setenv("FAKE_OSREL", "ID=ubuntu\nVERSION_ID=1\n")
	t.Setenv("FAKE_RC", "255")
	t.Setenv("FAKE_ERR", "ssh: connect to host 10.0.0.5 port 22: Connection refused")
	if _, se, err := DetectProfile("10.0.0.5", "22", "root", "/k"); err == nil || !strings.Contains(se, "Connection refused") {
		t.Errorf("ssh hatası taşınmadı: %v %q", err, se)
	}
}

func TestDetectSudoWSMatchesShell(t *testing.T) {
	root := goldRoot(t)
	fakeBin(t)
	for _, out := range []string{"SSHPOCKET_SUDO_WS=yes\n", "", "baska\n", "x SSHPOCKET_SUDO_WS=yes\n", "SSHPOCKET_SUDO_WS=yes\nextra\n", "SSHPOCKET_SUDO_WS=no\n"} {
		for _, rc := range []string{"0", "1"} {
			t.Setenv("FAKE_SUDOWS", out)
			t.Setenv("FAKE_RC", rc)
			sh := runShell(t, root, nil, "", "", "comp_detect_sudo_ws", "h", "22", "u", "/k")
			if got := DetectSudoWS("h", "22", "u", "/k"); (sh.code == 0) != got {
				t.Errorf("%q rc=%s: kabuk=%d go=%v", out, rc, sh.code, got)
			}
		}
	}
}

var reTimes = regexp.MustCompile(`[0-9]{2}:[0-9]{2}:[0-9]{2}`)
var reDiff = regexp.MustCompile(`Fark: [0-9]+ dk [0-9]+ sn`)
var reMin = regexp.MustCompile(`yaklaşık [0-9]+ dakika`)

func normClock(s string, exactMinutes bool) string {
	s = reTimes.ReplaceAllString(s, "T")
	s = reDiff.ReplaceAllString(s, "Fark: X")
	if !exactMinutes {
		s = reMin.ReplaceAllString(s, "yaklaşık N dakika")
	}
	return s
}

func TestWarnClockSkewMatchesShell(t *testing.T) {
	root := goldRoot(t)
	fakeBin(t)
	dir := t.TempDir()
	priv, _ := realKey(t, dir)
	now := func() int64 { var n int64; fmt.Sscan(strings.TrimSpace(mustRun(t, "date", "+%s")), &n); return n }()
	type skewCase struct {
		off  int64
		tail string
	}
	for _, p := range ordered(map[string]skewCase{
		"uyarı yok": {10, "Timezone=Europe/Istanbul\nNTPSynchronized=yes\n"}, "60 sn": {60, ""},
		"10 dk geride": {-600, "Timezone=Europe/Istanbul\nNTPSynchronized=yes\n"}, "10 dk ileride ntp kapalı": {600, "Timezone=UTC\nNTPSynchronized=no\n"},
		"tam 1 saat": {-3600, "Timezone=Europe/Istanbul\nNTPSynchronized=yes\n"}, "3 saat": {10800, "NTPSynchronized=yes\n"},
		"yarım saat + 1 dk": {1860, ""}, "kötü saat dilimi": {-600, "Timezone=Bad Zone$\n"}, "kötü epoch": {0, ""},
	}) {
		name, c := p.K, p.V
		epoch := fmt.Sprintf("%d\n", now+c.off)
		if name == "kötü epoch" {
			epoch = "abc\n"
		}
		t.Setenv("FAKE_CLOCK", epoch+c.tail)
		sh := runShell(t, root, nil, "", "", "comp_warn_clock_skew", "10.0.0.5", "2222", "root", priv)
		var sb strings.Builder
		WarnClockSkew("10.0.0.5", "2222", "root", priv, nil, &sb)
		// ±1-2 sn kayması: dakika sayıları yuvarlama sınırında değilse aynı; zaman damgaları normalize
		a, b := normClock(sh.stderr, false), normClock(reTmp.ReplaceAllString(sb.String(), "<T>"), false)
		if a != b {
			t.Errorf("[%s]\n kabuk=%q\n go   =%q", name, a, b)
		}
	}
	// hata: ssh başarısız → sessiz
	t.Setenv("FAKE_RC", "255")
	t.Setenv("FAKE_CLOCK", fmt.Sprintf("%d\n", now+9999))
	var sb strings.Builder
	WarnClockSkew("h", "22", "u", priv, nil, &sb)
	if sb.Len() != 0 {
		t.Errorf("ssh hatasında çıktı: %q", sb.String())
	}
}

func mustRun(t *testing.T, name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestSSHHintMatchesShell(t *testing.T) {
	root := goldRoot(t)
	dir := t.TempDir()
	texts := []string{
		"", "WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!\nx", "Offending ED25519 key in /home/u/.ssh/known_hosts:3", "No ED25519 host key is known for 10.0.0.5 and you have requested strict checking.",
		"Host key verification failed.", "Received disconnect: Too many authentication failures", "root@h: Permission denied (publickey).", "ssh: connect to host h port 22: Connection timed out",
		"Operation timed out", "ssh: connect to host h port 22: No route to host", "Network is unreachable", "ssh: connect to host h port 22: Connection refused",
		"ssh: Could not resolve hostname foo: Name or service not known", "Connection reset by peer", "Connection closed by remote host", "kex_exchange_identification: read: Connection reset",
		"bilinmeyen bir hata\nikinci satır", strings.Repeat("a", 300), "ctrl\x01\x07chars\x1b[31mred", "çok\nsatırlı\x00veri", "\nilk boş\nikinci", "   \n",
		"Permission denied ve timed out birlikte", "Connection refused ve Permission denied",
	}
	for i, tx := range texts {
		f := filepath.Join(dir, fmt.Sprintf("e%d", i))
		os.WriteFile(f, []byte(tx), 0o600)
		sh := runShell(t, root, nil, "", "", "comp_ssh_hint", f)
		got := SSHHint([]byte(tx))
		if want := strings.TrimSuffix(sh.stdout, "\n"); want != got {
			t.Errorf("#%d %q:\n kabuk=%q\n go   =%q", i, tx, want, got)
		}
	}
}
