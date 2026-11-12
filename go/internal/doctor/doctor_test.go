package doctor

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var reMB = regexp.MustCompile(`boş alan: \d+ MB`)
var reDropped = regexp.MustCompile(`(?m)^  (✅|⚠️ |❌) (curl|tar|sha256 aracı|nc|fzf|shasum).*\n`)

func shellDoctor(t *testing.T, path string, env ...string) (int, string) {
	root, _ := filepath.Abs("../../..")
	if _, err := os.Stat(filepath.Join(root, "referans/kabuk/sistem/Helpers/Doctor.sh")); err != nil {
		t.Skip("kabuk motoru yok")
	}
	script := `export SISTEM_PATH="$1"; shift; for l in UI.sh Components.sh ComponentsVenv.sh Doctor.sh; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; doctor_run "$@"`
	cmd := exec.Command("bash", "-c", script, "_", filepath.Join(root, "referans/kabuk/sistem"))
	cmd.Env = append(os.Environ(), append(env, "SSHPOCKET_DOCTOR_OFFLINE=1", "PATH="+path)...)
	var so bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &so
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return code, so.String()
}

func norm(s string) string {
	// önbellek klasörü adı bilerek değişti (kabuk: .cache/sshpocket, Go: .cache/keylab; eski klasör varsa o); karşılaştırmada eşitle
	s = strings.ReplaceAll(s, ".cache/keylab", ".cache/sshpocket")
	s = reDropped.ReplaceAllString(s, "")
	return reMB.ReplaceAllString(s, "boş alan: N MB")
}

func goDoctor(t *testing.T, path string) (int, string) {
	t.Setenv("PATH", path)
	t.Setenv("SSHPOCKET_DOCTOR_OFFLINE", "1")
	var b bytes.Buffer
	return Run(&b, Options{}), b.String()
}

func TestDoctorMatchesShell(t *testing.T) {
	base := os.Getenv("PATH")
	// 1) gerçek PATH
	// 2) ssh-copy-id ve python'suz sınırlı PATH (yalnız temel araçlar bir klasöre bağlanır)
	mk := func(names ...string) string {
		d := t.TempDir()
		for _, n := range names {
			if p, err := exec.LookPath(n); err == nil {
				os.Symlink(p, filepath.Join(d, n))
			}
		}
		return d
	}
	bash, _ := exec.LookPath("bash")
	_ = bash
	for name, path := range map[string]string{
		"gerçek PATH": base,
		"python yok":  mk("ssh", "ssh-keygen", "ssh-add", "ssh-copy-id", "security", "bash", "env", "df", "awk", "dirname", "uname", "tr", "head", "sort", "mktemp", "cat", "sed", "grep", "basename"),
		"ssh-add yok": mk("ssh", "ssh-keygen", "security", "bash", "env", "df", "awk", "dirname", "uname", "tr", "head", "sort", "mktemp", "cat", "sed", "grep", "basename"),
	} {
		sc, so := shellDoctor(t, path)
		gc, go_ := goDoctor(t, path)
		// "Sonuç" satırındaki eksik sayısı kaldırılan denetimler yüzünden farklı olabilir: yalnız kod ve satırlar (sonuç satırı hariç)
		cut := func(s string) string {
			i := strings.Index(s, "Sonuç:")
			if i < 0 {
				return s
			}
			return s[:i]
		}
		if cut(norm(so)) != cut(norm(go_)) {
			t.Errorf("%s:\nkabuk:\n%s\ngo:\n%s", name, norm(so), norm(go_))
		}
		if (sc == 0) != (gc == 0) && !strings.Contains(so, "curl bulunamadı") && !strings.Contains(so, "tar bulunamadı") {
			t.Errorf("%s: çıkış kodu kabuk=%d go=%d", name, sc, gc)
		}
	}
}
