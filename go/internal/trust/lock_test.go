package trust

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, _ := filepath.Abs("../../..")
	if _, err := os.Stat(filepath.Join(root, "referans/kabuk/sistem/Helpers/Components.sh")); err != nil {
		t.Skip("kabuk motoru yok: " + root)
	}
	return root
}

type shOut struct {
	code           int
	stdout, stderr string
}

// runShell: kütüphaneleri yükleyip işlevi çağırır. env: ek ortam değişkenleri (KEY=VAL).
func runShell(t *testing.T, root string, env []string, libs []string, prelude string, fn string, args ...string) shOut {
	t.Helper()
	script := `export SISTEM_PATH="$1"; shift; for l in $LIBS; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; eval "$PRELUDE"; "$@"`
	cmd := exec.Command("bash", append([]string{"-c", script, "_", filepath.Join(root, "referans/kabuk/sistem"), fn}, args...)...)
	cmd.Env = append(os.Environ(), append(env, "LIBS="+strings.Join(libs, " "), "PRELUDE="+prelude)...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return shOut{code, so.String(), se.String()}
}

const goodSigner = `sshpocket-component namespaces="sshpocket-component" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKVJS3mGlCaOT/l8XfR0TxuKxqLUs6PpVDa9TYqm0fLn`
const sha1 = "a10e7c3d85ab14a1372ab94e2b2a7b0815613fa28f3d3cc64eb0dd9db8f3789d"

func block(id string, kv ...string) string {
	s := "[" + id + "]\n"
	for i := 0; i < len(kv); i += 2 {
		s += kv[i] + " = " + kv[i+1] + "\n"
	}
	return s
}

func full(id string) string {
	return block(id, "repo", "o/"+id, "version", "1.2.3", "sha256", sha1, "signers", goodSigner)
}

func TestLockMatchesShell(t *testing.T) {
	root := repoRoot(t)
	cases := map[string]string{
		"geçerli":            full("utility") + "\n" + full("kurulum"),
		"yorumlu":            "# başlık\n\n" + full("a") + "# son\n",
		"crlf":               strings.ReplaceAll(full("a"), "\n", "\r\n"),
		"son satır yok":      strings.TrimSuffix(full("a"), "\n"),
		"boş dosya":          "",
		"yalnız yorum":       "# x\n",
		"başlıksız":          "repo = a/b\n",
		"yinelenen paket":    full("a") + full("a"),
		"yinelenen anahtar":  block("a", "repo", "o/a", "repo", "o/b", "version", "1.2.3", "sha256", sha1, "signers", goodSigner),
		"bilinmeyen anahtar": block("a", "repo", "o/a", "version", "1.2.3", "sha256", sha1, "signers", goodSigner, "extra", "x"),
		"anlaşılmayan satır": full("a") + "çöp satır\n",
		"geçersiz kimlik":    full("A"),
		"boş kimlik":         full(""),
		"repo ..":            block("a", "repo", "o/a..b", "version", "1.2.3", "sha256", sha1, "signers", goodSigner),
		"repo kötü":          block("a", "repo", "oa", "version", "1.2.3", "sha256", sha1, "signers", goodSigner),
		"sürüm kötü":         block("a", "repo", "o/a", "version", "1.2", "sha256", sha1, "signers", goodSigner),
		"özet kısa":          block("a", "repo", "o/a", "version", "1.2.3", "sha256", "abc", "signers", goodSigner),
		"özet büyük harf":    block("a", "repo", "o/a", "version", "1.2.3", "sha256", strings.ToUpper(sha1), "signers", goodSigner),
		"signers kötü":       block("a", "repo", "o/a", "version", "1.2.3", "sha256", sha1, "signers", "ssh-rsa AAAA"),
		"signers yorumlu":    block("a", "repo", "o/a", "version", "1.2.3", "sha256", sha1, "signers", goodSigner+" bir yorum"),
		"signers ctrl":       block("a", "repo", "o/a", "version", "1.2.3", "sha256", sha1, "signers", goodSigner+" y\x01z"),
		"alan eksik":         block("a", "repo", "o/a", "version", "1.2.3", "sha256", sha1),
		"sondaki boşluk":     "[a]\nrepo = o/a   \nversion = 1.2.3\nsha256 = " + sha1 + "\nsigners = " + goodSigner + "\n",
		"= çevresi yok":      "[a]\nrepo=o/a\nversion=1.2.3\nsha256=" + sha1 + "\nsigners=" + goodSigner + "\n",
		"girintili":          "[a]\n  repo = o/a\n",
		"başlık boşluklu":    "[a ]\n",
		"büyük anahtar":      "[a]\nRepo = o/a\n",
		"başlıkta yorum":     "[a] # x\n",
		"çok paket":          full("a") + full("b") + full("c"),
	}
	for name, text := range cases {
		dir := t.TempDir()
		lf := filepath.Join(dir, "components.lock")
		os.WriteFile(lf, []byte(text), 0o600)
		sh := runShell(t, root, []string{"SSHPOCKET_COMPONENTS_LOCK=" + lf}, []string{"Components.sh"}, "", "comp_lock_validate")
		lock, err := LoadLock(lf)
		gcode := 0
		if err != nil {
			gcode = 1
		}
		if sh.code != gcode {
			t.Errorf("%s: kabuk=%d (%q) go=%d (%v)", name, sh.code, sh.stderr, gcode, err)
			continue
		}
		if err != nil {
			if strings.TrimSpace(sh.stderr) != err.Error() {
				t.Errorf("%s: ileti kabuk=%q go=%q", name, strings.TrimSpace(sh.stderr), err)
			}
			continue
		}
		ids := runShell(t, root, []string{"SSHPOCKET_COMPONENTS_LOCK=" + lf}, []string{"Components.sh"}, "", "comp_ids")
		want := strings.Fields(ids.stdout)
		got := append([]string(nil), lock.IDs()...)
		sortStr(got)
		if strings.Join(want, ",") != strings.Join(got, ",") {
			t.Errorf("%s: kimlikler kabuk=%v go=%v", name, want, got)
		}
		for _, id := range want {
			e, _ := lock.Get(id)
			for k, v := range map[string]string{"repo": e.Repo, "version": e.Version, "sha256": e.SHA256, "signers": e.Signers} {
				g := runShell(t, root, []string{"SSHPOCKET_COMPONENTS_LOCK=" + lf}, []string{"Components.sh"}, "", "comp_get", id, k)
				if strings.TrimSuffix(g.stdout, "\n") != v {
					t.Errorf("%s: %s.%s kabuk=%q go=%q", name, id, k, g.stdout, v)
				}
			}
		}
	}
	// olmayan dosya: boş kilit
	l, err := LoadLock(filepath.Join(t.TempDir(), "yok"))
	if err != nil || len(l.Entries) != 0 {
		t.Errorf("olmayan dosya: %v %v", l, err)
	}
}

func sortStr(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func TestRealLockParses(t *testing.T) {
	root := repoRoot(t)
	l, err := LoadLock(filepath.Join(root, "components.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := l.Get("linux-hardening"); !ok {
		t.Fatal("gerçek kilitte linux-hardening yok")
	}
}

func TestVersionCmpMatchesShell(t *testing.T) {
	root := repoRoot(t)
	vs := []string{"0.0.0", "0.1.0", "0.1.2", "0.10.0", "0.9.9", "1.0.0", "1.0.01", "01.0.0", "10.0.0", "2.2.0", "3.7.0", "0.13.0", "0.12.1", "99999999.0.0"}
	for _, a := range vs {
		for _, b := range vs {
			sh := runShell(t, root, nil, []string{"Components.sh", "ComponentsInstall.sh"}, "", "comp_version_cmp", a, b)
			want := strings.TrimSpace(sh.stdout)
			got := map[int]string{1: "1", 0: "0", -1: "-1"}[VersionCmp(a, b)]
			if want != got {
				t.Errorf("%s %s: kabuk=%s go=%s", a, b, want, got)
			}
		}
	}
}
