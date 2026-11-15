package selfupdate

// Gerçek paketle uçtan uca (isteğe bağlı: KEYLAB_REAL_APP_TEST=1): go build + scripts/package-app.sh ile GERÇEK KeyLab.app zip'leri üretilir (ad-hoc imzalı),
// eski sürüm geçici bir klasöre kurulur, ApplyApp yeni zip'i (release anahtarıyla imzalı) açıp GERÇEK codesign ile doğrulayıp yerine koyar. Go'daki güvenli açma işleminin
// paketin kod imzasını BOZMADIĞINI gösterir (sahte zip bunu kanıtlayamaz). macOS ve sips/iconutil/ditto/codesign gerekir; varsayılan testlerde atlanır.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/platform"
)

func TestApplyAppWithRealBundleKeepsCodeSignatureValid(t *testing.T) {
	if os.Getenv("KEYLAB_REAL_APP_TEST") != "1" {
		t.Skip("KEYLAB_REAL_APP_TEST=1 ile çalışır")
	}
	for _, tool := range []string{"go", "codesign", "ditto", "sips", "iconutil", "plutil", "ssh-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s yok", tool)
		}
	}
	e := newEnv(t)
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	build := func(ver string) (appDir, zipPath string) {
		out := t.TempDir()
		bin := filepath.Join(out, "keylab-"+ver)
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w -X main.buildVersion="+ver, "-o", bin, ".")
		cmd.Dir = root
		if o, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build %s: %v %s", ver, err, o)
		}
		pa := exec.Command("bash", filepath.Join(root, "scripts", "package-app.sh"), bin, ver, out)
		if o, err := pa.CombinedOutput(); err != nil {
			t.Fatalf("package-app %s: %v %s", ver, err, o)
		}
		return filepath.Join(out, "KeyLab.app"), filepath.Join(out, "keylab-"+ver+"-macos.zip")
	}
	oldApp, _ := build("1.0.0")
	_, newZip := build("1.2.0")
	apps := filepath.Join(e.dir, "Apps")
	os.MkdirAll(apps, 0o755)
	target := filepath.Join(apps, "KeyLab.app")
	if o, err := exec.Command("ditto", oldApp, target).CombinedOutput(); err != nil {
		t.Fatalf("ditto: %v %s", err, o)
	}
	if err := platform.CodesignVerify(target); err != nil {
		t.Fatalf("eski paket zaten geçersiz: %v", err)
	}
	zb, _ := os.ReadFile(newZip)
	u, _, _ := e.updater("")
	url := "https://github.com/syhgorath/keylab/releases/download/v1.2.0/keylab-1.2.0-macos.zip"
	e.net[url], e.net[url+".sig"] = zb, sign(t, e.priv, Namespace, zb)
	var steps []string
	backup, err := u.ApplyApp("1.2.0", target, platform.CodesignVerify, func(s string) { steps = append(steps, s) })
	if err != nil {
		t.Fatalf("ApplyApp: %v", err)
	}
	if err := platform.CodesignVerify(target); err != nil {
		t.Fatalf("güncellenen paketin kod imzası GEÇERSİZ: %v", err)
	}
	o, err := exec.Command(filepath.Join(target, "Contents", "MacOS", "KeyLab"), "version").Output()
	if err != nil || strings.TrimSpace(string(o)) != "sshpocket 1.2.0" {
		t.Fatalf("yeni paket sürümü: %q %v", o, err)
	}
	bo, _ := exec.Command(filepath.Join(backup, "Contents", "MacOS", "KeyLab"), "version").Output()
	if strings.TrimSpace(string(bo)) != "sshpocket 1.0.0" {
		t.Fatalf("yedek eski sürüm değil: %q", bo)
	}
	if err := platform.CodesignVerify(backup); err != nil {
		t.Fatalf("yedek paketin imzası bozuldu: %v", err)
	}
	t.Logf("adımlar: %v; yedek: %s", steps, backup)
}
