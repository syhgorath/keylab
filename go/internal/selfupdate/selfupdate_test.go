package selfupdate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/trust"
)

func needKeygen(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen yok")
	}
}

func keypair(t *testing.T, dir, name string) (priv, pub string) {
	priv = filepath.Join(dir, name)
	if o, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, o)
	}
	b, _ := os.ReadFile(priv + ".pub")
	f := strings.Fields(string(b))
	return priv, f[0] + " " + f[1]
}

func sign(t *testing.T, priv, ns string, data []byte) []byte {
	d := t.TempDir()
	f := filepath.Join(d, "d")
	os.WriteFile(f, data, 0o600)
	if o, err := exec.Command("ssh-keygen", "-Y", "sign", "-f", priv, "-n", ns, f).CombinedOutput(); err != nil {
		t.Fatalf("sign: %v %s", err, o)
	}
	b, _ := os.ReadFile(f + ".sig")
	return b
}

// sahte ikili: `version` ile sürümünü bildirir
func fakeBinary(ver string) []byte {
	return []byte("#!/bin/sh\nif [ \"$1\" = version ]; then echo 'sshpocket " + ver + "'; fi\n# " + strings.Repeat("x", 100) + "\n")
}

type env struct {
	dir, exe, signers string
	net               map[string][]byte
	priv, pub         string
	latestTag         string
}

func newEnv(t *testing.T) *env {
	needKeygen(t)
	e := &env{dir: t.TempDir(), net: map[string][]byte{}, latestTag: "v1.2.0"}
	e.priv, e.pub = keypair(t, e.dir, "k")
	e.signers = filepath.Join(e.dir, "release_signers")
	os.WriteFile(e.signers, []byte("# anahtarlar\nsshpocket-release namespaces=\"sshpocket-release\" "+e.pub+"\n"), 0o644)
	e.exe = filepath.Join(e.dir, "sshpocket")
	os.WriteFile(e.exe, fakeBinary("1.0.0"), 0o755)
	os.MkdirAll(filepath.Join(e.dir, "servers", "web1"), 0o700)
	os.WriteFile(filepath.Join(e.dir, "servers", "web1", "server.env"), []byte("IP=1.1.1.1\n"), 0o600)
	return e
}

func (e *env) publish(t *testing.T, ver, goos, arch string, bin []byte, signWith string, ns string) {
	name := fmt.Sprintf("sshpocket-%s-%s-%s", ver, goos, arch)
	url := "https://github.com/syhgorath/keylab/releases/download/v" + ver + "/" + name
	e.net[url] = bin
	e.net[url+".sig"] = sign(t, signWith, ns, bin)
}

func (e *env) updater(stdin string) (*Updater, *bytes.Buffer, *bytes.Buffer) {
	var so, se bytes.Buffer
	u := &Updater{SignersFile: e.signers, ExePath: e.exe, Version: "1.0.0", GOOS: "darwin", GOARCH: "arm64",
		In: strings.NewReader(stdin), Out: &so, Err: &se,
		Head: func(url string) string {
			if strings.HasSuffix(url, "/releases/latest") && e.latestTag != "" {
				return "https://github.com/syhgorath/keylab/releases/tag/" + e.latestTag
			}
			return ""
		},
		Download: func(url, dest string, max int64) error {
			b, ok := e.net[url]
			if !ok {
				return fmt.Errorf("404")
			}
			return os.WriteFile(dest, b, 0o600)
		},
	}
	return u, &so, &se
}

func (e *env) exeVersion() string {
	o, _ := exec.Command(e.exe, "version").Output()
	return strings.TrimSpace(string(o))
}

func TestUpdateSuccess(t *testing.T) {
	e := newEnv(t)
	e.publish(t, "1.2.0", "darwin", "arm64", fakeBinary("1.2.0"), e.priv, Namespace)
	u, so, _ := e.updater("y\ny\n")
	code, updated := u.Run(false)
	if code != 0 || !updated || e.exeVersion() != "sshpocket 1.2.0" {
		t.Fatalf("kod=%d updated=%v sürüm=%q\n%s", code, updated, e.exeVersion(), so)
	}
	if b, _ := os.ReadFile(filepath.Join(e.dir, ".update-backup", "sshpocket-1.0.0")); !bytes.Contains(b, []byte("sshpocket 1.0.0")) {
		t.Error("yedek eski ikili değil")
	}
	if st, _ := os.Stat(e.exe); st.Mode().Perm() != 0o755 {
		t.Errorf("izin %v", st.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(e.dir, "servers", "web1", "server.env")); err != nil {
		t.Error("kullanıcı verisi kayboldu")
	}
	ents, _ := os.ReadDir(e.dir)
	for _, x := range ents {
		if strings.HasPrefix(x.Name(), ".sshpocket.new.") {
			t.Errorf("geçici dosya kaldı: %s", x.Name())
		}
	}
}

func TestUpdateUniversalPreferredOnDarwin(t *testing.T) {
	e := newEnv(t)
	e.publish(t, "1.2.0", "darwin", "universal", fakeBinary("1.2.0"), e.priv, Namespace)
	u, _, _ := e.updater("y\ny\n")
	if code, _ := u.Run(false); code != 0 || e.exeVersion() != "sshpocket 1.2.0" {
		t.Fatalf("universal varlık kullanılmadı")
	}
}

func TestUpdateRefusals(t *testing.T) {
	type tc struct {
		name  string
		setup func(t *testing.T, e *env)
		in    string
		check bool
		code  int
		msg   string
	}
	good := func(t *testing.T, e *env) {
		e.publish(t, "1.2.0", "darwin", "arm64", fakeBinary("1.2.0"), e.priv, Namespace)
	}
	cases := []tc{
		{"yanlış anahtarla imza", func(t *testing.T, e *env) {
			p2, _ := keypair(t, e.dir, "other")
			e.publish(t, "1.2.0", "darwin", "arm64", fakeBinary("1.2.0"), p2, Namespace)
		}, "y\ny\n", false, 1, "İMZA DOĞRULANAMADI"},
		{"yanlış ad alanı", func(t *testing.T, e *env) {
			e.publish(t, "1.2.0", "darwin", "arm64", fakeBinary("1.2.0"), e.priv, "sshpocket-component")
		}, "y\ny\n", false, 1, "İMZA DOĞRULANAMADI"},
		{"imzadan sonra değiştirilmiş ikili", func(t *testing.T, e *env) {
			good(t, e)
			url := "https://github.com/syhgorath/keylab/releases/download/v1.2.0/sshpocket-1.2.0-darwin-arm64"
			e.net[url] = append(e.net[url], []byte("# zehir\n")...)
		}, "y\ny\n", false, 1, "İMZA DOĞRULANAMADI"},
		{"imza dosyası yok", func(t *testing.T, e *env) {
			good(t, e)
			delete(e.net, "https://github.com/syhgorath/keylab/releases/download/v1.2.0/sshpocket-1.2.0-darwin-arm64.sig")
		}, "y\ny\n", false, 1, "İndirme başarısız"},
		{"varlık yok", func(t *testing.T, e *env) {}, "y\ny\n", false, 1, "İndirme başarısız"},
		{"içerideki sürüm etiketle uyuşmuyor", func(t *testing.T, e *env) {
			e.publish(t, "1.2.0", "darwin", "arm64", fakeBinary("1.1.0"), e.priv, Namespace) // eski imzalı sürüm yeniden sunuluyor
		}, "y\ny\n", false, 1, "uyuşmuyor"},
		{"anahtar tanımlı değil", func(t *testing.T, e *env) {
			good(t, e)
			os.WriteFile(e.signers, []byte("# boş\n"), 0o644)
		}, "y\ny\n", false, 1, "İMZA DOĞRULANAMADI"},
		{"ilk onay n", good, "n\n", false, 1, "İptal edildi."},
		{"ikinci onay n", good, "y\nn\n", false, 1, "İptal edildi."},
		{"girdi yok", good, "", false, 1, "İptal edildi."},
		{"zaten güncel", func(t *testing.T, e *env) { e.latestTag = "v1.0.0" }, "", false, 0, "Güncelsiniz"},
		{"geri sürüm", func(t *testing.T, e *env) { e.latestTag = "v0.9.0" }, "", false, 0, "geri sürüme düşülmez"},
		{"ağ yok", func(t *testing.T, e *env) { e.latestTag = "" }, "", false, 1, "Son sürüm öğrenilemedi"},
		{"--check yeni sürüm var", good, "", true, 10, "Yeni sürüm var"},
	}
	for _, c := range cases {
		e := newEnv(t)
		c.setup(t, e)
		before, _ := os.ReadFile(e.exe)
		u, so, se := e.updater(c.in)
		code, updated := u.Run(c.check)
		after, _ := os.ReadFile(e.exe)
		if code != c.code || updated || !strings.Contains(so.String()+se.String(), c.msg) {
			t.Errorf("%s: kod=%d updated=%v\n%s%s", c.name, code, updated, so, se)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("%s: reddedilen güncellemede ikili DEĞİŞTİ", c.name)
		}
		if _, err := os.Stat(filepath.Join(e.dir, ".update-backup")); err == nil {
			t.Errorf("%s: reddedilen güncellemede yedek klasörü oluştu", c.name)
		}
	}
}

func TestUpdateUnwritableDirKeepsOldBinary(t *testing.T) {
	e := newEnv(t)
	e.publish(t, "1.2.0", "darwin", "arm64", fakeBinary("1.2.0"), e.priv, Namespace)
	os.Chmod(e.dir, 0o555)
	defer os.Chmod(e.dir, 0o755)
	if os.Geteuid() == 0 {
		t.Skip("root yazma korumasını aşar")
	}
	u, so, se := e.updater("y\ny\n")
	code, updated := u.Run(false)
	os.Chmod(e.dir, 0o755)
	if code != 1 || updated || e.exeVersion() != "sshpocket 1.0.0" || !strings.Contains(so.String()+se.String(), "Güncelleme başarısız") {
		t.Fatalf("kod=%d updated=%v sürüm=%q\n%s%s", code, updated, e.exeVersion(), so, se)
	}
}

func TestLatestRequiresExactTagURL(t *testing.T) {
	for loc, want := range map[string]string{
		"https://github.com/syhgorath/keylab/releases/tag/v1.2.3":   "1.2.3",
		"https://github.com/baska/repo/releases/tag/v1.2.3":         "",
		"https://github.com/syhgorath/keylab/releases/tag/1.2.3":    "",
		"https://github.com/syhgorath/keylab/releases/tag/v1.2":     "",
		"http://github.com/syhgorath/keylab/releases/tag/v1.2.3":    "",
		"https://github.com/syhgorath/keylab/releases/tag/v1.2.3/x": "",
		"": "",
	} {
		u := &Updater{Head: func(string) string { return loc }}
		got, _ := u.Latest()
		if got != want {
			t.Errorf("%q: %q != %q", loc, got, want)
		}
	}
}

var _ = trust.VersionCmp

func TestAssetNamesNewThenOld(t *testing.T) {
	u := &Updater{GOOS: "darwin", GOARCH: "arm64"}
	got := strings.Join(u.assetNames("1.2.3"), " ")
	want := "keylab-1.2.3-darwin-universal keylab-1.2.3-darwin-arm64 sshpocket-1.2.3-darwin-universal sshpocket-1.2.3-darwin-arm64"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if DefaultRepo != "syhgorath/keylab" {
		t.Fatal(DefaultRepo)
	}
}
