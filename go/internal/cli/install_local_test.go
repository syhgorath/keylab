package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// install-local: ağsız, yerel arşivden kurulum. Güven kapıları indirmeyle aynı olmalı (imza, özet, kilit); --force aynı sürümü yeniler.
// Test anahtarı tek kullanımlıktır; gerçek sunucuya/imza anahtarına dokunulmaz.

type ilEnv struct {
	t         *testing.T
	dir       string // üst klasör: components/ ve components.lock burada
	comp      string
	priv      string
	signers   string // kilitteki signers satırı
	out, errb *bytes.Buffer
}

func newILEnv(t *testing.T) *ilEnv {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen yok")
	}
	e := &ilEnv{t: t, dir: t.TempDir(), out: &bytes.Buffer{}, errb: &bytes.Buffer{}}
	e.comp = filepath.Join(e.dir, "components")
	os.MkdirAll(e.comp, 0o755)
	e.priv = filepath.Join(e.dir, "k")
	if o, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", e.priv).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, o)
	}
	pub, _ := os.ReadFile(e.priv + ".pub")
	f := strings.Fields(string(pub))
	e.signers = fmt.Sprintf(`sshpocket-component namespaces="sshpocket-component" %s %s`, f[0], f[1])
	return e
}

// pkg: <id>-<ver>/component.json (+ icerik dosyası) içeren, imzalı arşiv üretir; (arşiv yolu, imza yolu, sha256) döner.
func (e *ilEnv) pkg(id, ver, content string) (string, string, string) {
	e.t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	root := id + "-" + ver + "/"
	tw.WriteHeader(&tar.Header{Name: root, Typeflag: tar.TypeDir, Mode: 0o755})
	for name, body := range map[string]string{
		"component.json": fmt.Sprintf("{\n  \"id\": %q,\n  \"version\": %q\n}\n", id, ver),
		"icerik.txt":     content,
	} {
		tw.WriteHeader(&tar.Header{Name: root + name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	arch := filepath.Join(e.dir, fmt.Sprintf("%s-%s-%d.tar.gz", id, ver, time.Now().UnixNano()))
	os.WriteFile(arch, buf.Bytes(), 0o600)
	if o, err := exec.Command("ssh-keygen", "-Y", "sign", "-f", e.priv, "-n", "sshpocket-component", arch).CombinedOutput(); err != nil {
		e.t.Fatalf("sign: %v %s", err, o)
	}
	sum := sha256.Sum256(buf.Bytes())
	return arch, arch + ".sig", hex.EncodeToString(sum[:])
}

func (e *ilEnv) writeLock(id, ver, sha string) {
	e.t.Helper()
	txt := fmt.Sprintf("[%s]\nrepo = test/%s\nversion = %s\nsha256 = %s\nsigners = %s\n", id, id, ver, sha, e.signers)
	if err := os.WriteFile(filepath.Join(e.dir, "components.lock"), []byte(txt), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *ilEnv) run(args ...string) int {
	e.out.Reset()
	e.errb.Reset()
	c := &Components{Dir: e.comp, LockPath: filepath.Join(e.dir, "components.lock"), Out: e.out, Err: e.errb, Now: func() time.Time { return time.Unix(1700000000, 0) }}
	return c.InstallLocal(args)
}

func (e *ilEnv) installedContent(id, ver string) string {
	b, _ := os.ReadFile(filepath.Join(e.comp, id, ver, "icerik.txt"))
	return string(b)
}

func TestInstallLocalInstallsSignedArchiveWithoutNetwork(t *testing.T) {
	e := newILEnv(t)
	arch, sig, sha := e.pkg("demo", "1.0.0", "bir")
	e.writeLock("demo", "1.0.0", sha)
	if rc := e.run("demo", arch, sig); rc != 0 || !strings.Contains(e.out.String(), "kuruldu: demo 1.0.0") {
		t.Fatalf("rc=%d out=%q err=%q", rc, e.out, e.errb)
	}
	if e.installedContent("demo", "1.0.0") != "bir" {
		t.Fatal("içerik kurulmadı")
	}
	if b, _ := os.ReadFile(filepath.Join(e.comp, "demo", "current")); strings.TrimSpace(string(b)) != "1.0.0" {
		t.Fatalf("current: %q", b)
	}
}

// Güven kapıları atlanmaz: imzadan sonra değiştirilmiş arşiv, kilitte olmayan paket ve yanlış imzacı kurulmaz.
func TestInstallLocalKeepsAllTrustGates(t *testing.T) {
	e := newILEnv(t)
	arch, sig, sha := e.pkg("demo", "1.0.0", "bir")
	e.writeLock("demo", "1.0.0", sha)

	// 1) imzadan sonra bozulmuş arşiv
	os.WriteFile(arch, append(mustRead(t, arch), []byte("zehir")...), 0o600)
	if rc := e.run("demo", arch, sig); rc == 0 {
		t.Fatalf("değiştirilmiş arşiv kuruldu: %q", e.out)
	}
	if _, err := os.Stat(filepath.Join(e.comp, "demo", "current")); err == nil {
		t.Fatal("bozuk arşivden sonra paket kurulu")
	}

	// 2) kilitte olmayan paket
	arch2, sig2, _ := e.pkg("baska", "1.0.0", "x")
	if rc := e.run("baska", arch2, sig2); rc == 0 || !strings.Contains(e.errb.String(), "kilit dosyasında yok") {
		t.Fatalf("kilitte olmayan paket: rc=%d err=%q", rc, e.errb)
	}

	// 3) başka bir anahtarla imzalanmış (kilitteki imzacı değil) arşiv
	other := filepath.Join(e.dir, "baska-anahtar")
	if o, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", other).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, o)
	}
	arch3, sig3, sha3 := e.pkg("demo", "1.0.0", "iki")
	e.writeLock("demo", "1.0.0", sha3)
	os.Remove(sig3) // ssh-keygen mevcut .sig'i ezmez: önce sil, sonra başka anahtarla imzala
	if o, err := exec.Command("ssh-keygen", "-Y", "sign", "-f", other, "-n", "sshpocket-component", arch3).CombinedOutput(); err != nil {
		t.Fatalf("sign: %v %s", err, o)
	}
	if rc := e.run("demo", arch3, arch3+".sig"); rc == 0 {
		t.Fatalf("yanlış imzacıyla kuruldu: %q", e.out)
	}
}

func TestInstallLocalUsageAndForce(t *testing.T) {
	e := newILEnv(t)
	if rc := e.run("demo", "yalniz-iki"); rc != 2 {
		t.Fatalf("eksik argüman rc=%d", rc)
	}

	arch, sig, sha := e.pkg("demo", "1.0.0", "bir")
	e.writeLock("demo", "1.0.0", sha)
	if rc := e.run("demo", arch, sig); rc != 0 {
		t.Fatalf("ilk kurulum: %q %q", e.out, e.errb)
	}

	// aynı sürüm, yeni içerik (geliştirme döngüsü): --force yoksa "zaten kurulu", içerik DEĞİŞMEZ
	arch2, sig2, sha2 := e.pkg("demo", "1.0.0", "iki")
	e.writeLock("demo", "1.0.0", sha2)
	if rc := e.run("demo", arch2, sig2); rc != 0 || !strings.Contains(e.out.String(), "zaten kurulu") || e.installedContent("demo", "1.0.0") != "bir" {
		t.Fatalf("--force'suz: rc=%d out=%q içerik=%q", rc, e.out, e.installedContent("demo", "1.0.0"))
	}

	// --force: yenisi kurulur, eskisi SİLİNMEZ, kenara alınır
	if rc := e.run("--force", "demo", arch2, sig2); rc != 0 || !strings.Contains(e.out.String(), "kuruldu: demo 1.0.0") || e.installedContent("demo", "1.0.0") != "iki" {
		t.Fatalf("--force: rc=%d out=%q err=%q içerik=%q", rc, e.out, e.errb, e.installedContent("demo", "1.0.0"))
	}
	ents, _ := os.ReadDir(filepath.Join(e.comp, "demo"))
	var kenar bool
	for _, d := range ents {
		if strings.HasPrefix(d.Name(), "1.0.0.bozuk-") && d.IsDir() {
			kenar = true
			if b, _ := os.ReadFile(filepath.Join(e.comp, "demo", d.Name(), "icerik.txt")); string(b) != "bir" {
				t.Fatalf("kenara alınan içerik: %q", b)
			}
		}
	}
	if !kenar {
		t.Fatal("eski sürüm kenara alınmadı (silinmiş olabilir)")
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
