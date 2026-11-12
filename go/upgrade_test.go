package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"keylab/internal/trust"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Yerel anahtarla imzalanmış sahte bir "yeni sürüm" ile GUI yükseltme akışı (USB'ye ve ağa dokunmaz).

type upgEnv struct {
	root, comp string
	app        *App
	priv       string
	served     map[string][]byte // taban ad → içerik
}

func upgTarGz(t *testing.T, id, ver string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	p := id + "-" + ver
	add := func(name, body string, dir bool) {
		h := &tar.Header{Name: name, Mode: 0o644, Typeflag: tar.TypeReg, Size: int64(len(body))}
		if dir {
			h.Typeflag, h.Mode, h.Size = tar.TypeDir, 0o755, 0
		}
		tw.WriteHeader(h)
		if !dir {
			tw.Write([]byte(body))
		}
	}
	add(p+"/", "", true)
	add(p+"/component.json", fmt.Sprintf(`{"id":%q,"version":%q}`, id, ver), false)
	add(p+"/VERSION", ver+"\n", false)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func upgSign(t *testing.T, priv, ns string, data []byte) []byte {
	d := t.TempDir()
	f := filepath.Join(d, "x")
	os.WriteFile(f, data, 0o600)
	if out, err := exec.Command("ssh-keygen", "-Y", "sign", "-f", priv, "-n", ns, f).CombinedOutput(); err != nil {
		t.Fatalf("sign: %v %s", err, out)
	}
	b, _ := os.ReadFile(f + ".sig")
	return b
}

func hexSHA(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func newUpgEnv(t *testing.T) *upgEnv {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen yok")
	}
	root := t.TempDir()
	e := &upgEnv{root: root, comp: filepath.Join(root, "components"), priv: filepath.Join(root, "key"), served: map[string][]byte{}}
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", e.priv).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, out)
	}
	pubB, _ := os.ReadFile(e.priv + ".pub")
	f := strings.Fields(string(pubB))
	pub := f[0] + " " + f[1]
	os.WriteFile(filepath.Join(root, "index_signers"), []byte("sshpocket-index namespaces=\"sshpocket-index\" "+pub+"\n"), 0o600)
	os.WriteFile(filepath.Join(root, "VERSION"), []byte("3.7.0\n"), 0o644)

	// 0.1.0 kurulu (kilit tabanı), 0.1.1 indekste
	v0 := upgTarGz(t, "utility", "0.1.0")
	v1 := upgTarGz(t, "utility", "0.1.1")
	lock := fmt.Sprintf("[utility]\nrepo = o/utility\nversion = 0.1.0\nsha256 = %s\nsigners = sshpocket-component namespaces=\"sshpocket-component\" %s\n", hexSHA(v0), pub)
	os.WriteFile(filepath.Join(root, "components.lock"), []byte(lock), 0o600)

	idx, _ := json.Marshal(map[string]any{"schema": 1, "serial": 5, "issued_at": time.Now().UTC().Add(-time.Hour).Format("2006-01-02T15:04:05Z"),
		"valid_until": time.Now().UTC().Add(30 * 24 * time.Hour).Format("2006-01-02T15:04:05Z"),
		"packages":    map[string]any{"utility": map[string]any{"repo": "o/utility", "version": "0.1.1", "asset": "utility-0.1.1.tar.gz", "sha256": hexSHA(v1)}}})
	e.served["index.json"] = idx
	e.served["index.json.sig"] = upgSign(t, e.priv, "sshpocket-index", idx)
	e.served["utility-0.1.1.tar.gz"] = v1
	e.served["utility-0.1.1.tar.gz.sig"] = upgSign(t, e.priv, "sshpocket-component", v1)

	// 0.1.0'ı doğrudan kur (kilit sürümü)
	os.MkdirAll(filepath.Join(e.comp, "utility", "0.1.0"), 0o755)
	os.WriteFile(filepath.Join(e.comp, "utility", "0.1.0", "component.json"), []byte(`{"id":"utility","version":"0.1.0"}`), 0o644)
	os.WriteFile(filepath.Join(e.comp, "utility", "current"), []byte("0.1.0\n"), 0o644)

	e.app = &App{compDir: e.comp, jobs: map[string]*Job{}}
	e.app.idxDL = func(url, dest string, max int64) error {
		b, ok := e.served[filepath.Base(url)]
		if !ok {
			return fmt.Errorf("yok: %s", url)
		}
		return os.WriteFile(dest, b, 0o600)
	}
	return e
}

func waitJob(t *testing.T, a *App, j *Job) string {
	for i := 0; i < 400; i++ {
		a.jmu.Lock()
		done := j.Done
		out := string(j.buf)
		a.jmu.Unlock()
		if done {
			return out
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("iş bitmedi")
	return ""
}

func TestUpgradeFlow(t *testing.T) {
	// başarılı yükseltme
	e := newUpgEnv(t)
	j, code, err := e.app.startUpgrade("utility", "0.1.1")
	if err != nil {
		t.Fatalf("başlamadı: %d %v", code, err)
	}
	out := waitJob(t, e.app, j)
	if j.Exit != 0 || !strings.Contains(out, "kuruldu: utility 0.1.1") {
		t.Fatalf("yükseltme başarısız (çıkış %d):\n%s", j.Exit, out)
	}
	if v, _ := os.ReadFile(filepath.Join(e.comp, "utility", "current")); strings.TrimSpace(string(v)) != "0.1.1" {
		t.Errorf("current=%q", v)
	}
	if _, err := os.Stat(filepath.Join(e.comp, "utility", "0.1.0")); err != nil {
		t.Error("eski sürüm klasörü silinmiş")
	}
	// aynı sürüm yeniden istenirse: artık güncel → reddedilir
	j2, _, _ := e.app.startUpgrade("utility", "0.1.1")
	if o := waitJob(t, e.app, j2); j2.Exit == 0 || !strings.Contains(o, "yok") {
		t.Errorf("güncelken yükseltme reddedilmeli:\n%s", o)
	}

	cases := []struct {
		name    string
		version string
		mutate  func(e *upgEnv)
		wantErr string
	}{
		{"onaylanan sürüm değişmiş", "0.1.9", func(e *upgEnv) {}, "indeks değişmiş"},
		{"arşiv bozulmuş", "0.1.1", func(e *upgEnv) {
			b := append([]byte(nil), e.served["utility-0.1.1.tar.gz"]...)
			b[len(b)/2] ^= 1
			e.served["utility-0.1.1.tar.gz"] = b
		}, "özet beklenenle uyuşmuyor"},
		{"arşiv imzasız (başka içerik, doğru özet)", "0.1.1", func(e *upgEnv) {
			e.served["utility-0.1.1.tar.gz.sig"] = upgSign(t, e.priv, "sshpocket-index", e.served["utility-0.1.1.tar.gz"]) // yanlış ad alanı
		}, "imza doğrulanamadı"},
		{"imza dosyası yok", "0.1.1", func(e *upgEnv) { delete(e.served, "utility-0.1.1.tar.gz.sig") }, "indirme başarısız"},
		{"indeks imzası bozuk", "0.1.1", func(e *upgEnv) {
			e.served["index.json"] = bytes.Replace(e.served["index.json"], []byte(`"serial":5`), []byte(`"serial":6`), 1)
		}, "indeks imzası geçersiz"},
		{"kilitte olmayan paket", "0.1.1", func(e *upgEnv) {}, "kurulabilir ya da yükseltilebilir sürüm yok"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newUpgEnv(t)
			c.mutate(e)
			pkg := "utility"
			if c.name == "kilitte olmayan paket" {
				pkg = "baska"
			}
			j, code, err := e.app.startUpgrade(pkg, c.version)
			if err != nil {
				t.Fatalf("başlamadı: %d %v", code, err)
			}
			out := waitJob(t, e.app, j)
			if j.Exit == 0 || !strings.Contains(out, c.wantErr) {
				t.Fatalf("beklenen hata %q, çıkış %d:\n%s", c.wantErr, j.Exit, out)
			}
			if v, _ := os.ReadFile(filepath.Join(e.comp, "utility", "current")); strings.TrimSpace(string(v)) != "0.1.0" {
				t.Errorf("başarısız yükseltme current'ı değiştirdi: %q", v)
			}
			if _, err := os.Stat(filepath.Join(e.comp, "utility", "0.1.1")); err == nil {
				t.Error("başarısız yükseltme sürüm klasörü bıraktı")
			}
		})
	}
	// geçersiz girdi ve aynı anda tek iş
	e = newUpgEnv(t)
	if _, code, err := e.app.startUpgrade("Utility", "0.1.1"); err == nil || code != 400 {
		t.Errorf("geçersiz kimlik: %d %v", code, err)
	}
	if _, code, err := e.app.startUpgrade("utility", "1.x"); err == nil || code != 400 {
		t.Errorf("geçersiz sürüm: %d %v", code, err)
	}
}

// Canlı deneme (yalnızca SSHPOCKET_LIVE=1): GERÇEK imzalı indeks ve GERÇEK utility sürümü GitHub'dan HTTPS ile
// indirilip GEÇİCİ bir motor ağacına kurulur; USB'ye ve gerçek components/ klasörüne yazılmaz.
func TestLiveUpgradeRealUtility(t *testing.T) {
	if os.Getenv("SSHPOCKET_LIVE") != "1" {
		t.Skip("SSHPOCKET_LIVE=1 değil")
	}
	src, _ := filepath.Abs("..")
	tmp := t.TempDir()
	for _, f := range []string{"index_signers", "components.lock", "VERSION"} {
		b, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			t.Skip(err)
		}
		os.WriteFile(filepath.Join(tmp, f), b, 0o600)
	}
	// utility 0.1.0'ı gerçek imzalı sürümden kur (aynı Go yolu: kilit sürümü)
	a := &App{compDir: tmp + "/components", jobs: map[string]*Job{}}
	lock, err := trust.LoadLock(tmp + "/components.lock")
	if err != nil {
		t.Fatal(err)
	}
	if msg, err := trust.Install(a.compDir, lock, "utility", nil); err != nil {
		t.Fatalf("kilit sürümü kurulamadı: %v", err)
	} else {
		t.Log(msg)
	}
	j, code, err := a.startUpgrade("utility", "0.1.2")
	if err != nil {
		t.Fatalf("%d %v", code, err)
	}
	out := waitJob(t, a, j)
	t.Logf("çıkış %d\n%s", j.Exit, out)
	if j.Exit != 0 {
		t.Fatal("canlı yükseltme başarısız")
	}
	if err := trust.VerifyInstalled(a.compDir, "utility", "0.1.2"); err != nil {
		t.Fatal(err)
	}
}
