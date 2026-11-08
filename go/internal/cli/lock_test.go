package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// lock-component.sh (kabuk) ile `components lock` (Go) yan yana: aynı girdi → aynı çıkış kodu, stdout, HATA satırları ve components.lock baytları.

const lockManifest = `{
 "id":"demo","version":"0.1.0","sshpocket_api":1,"runner":"ansible","inventory_group":"demo","name":"Demo","description":"d",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "requires":{"control":{"python":">=3.12","packages":["ansible-core"]},"target":{"python":">=3.9"}},
 "actions":{"preview":{"cmd":"ansible-playbook playbooks/preview.yml --check --diff","effects":[],"requires_sudo":true,"confirm":"none","warnings":["w"]}},
 "vars":[]}`

type lockFx struct {
	root, tmp         string
	archive, sig      string // geçerli, imzalı
	badSig            string // başka anahtarla imzalı
	noManifest, noSig string
	wrongID, wrongSig string // component.json içindeki id uyuşmuyor
	signers, otherKey string
	relSigners        string
}

func sh(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func lockFixtures(t *testing.T) *lockFx {
	t.Helper()
	root, _ := filepath.Abs("../../..")
	if _, err := os.Stat(filepath.Join(root, "referans/kabuk/scripts/lock-component.sh")); err != nil {
		t.Skip("kabuk motoru yok")
	}
	for _, b := range []string{"bash", "ssh-keygen", "tar", "python3"} {
		if _, err := exec.LookPath(b); err != nil {
			t.Skip(b + " yok")
		}
	}
	tmp := t.TempDir()
	f := &lockFx{root: root, tmp: tmp}
	keyOf := func(name string) (priv string, line string) {
		priv = filepath.Join(tmp, name)
		sh(t, tmp, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", priv)
		pub, _ := os.ReadFile(priv + ".pub")
		fs := strings.Fields(string(pub))
		return priv, `sshpocket-component namespaces="sshpocket-component" ` + fs[0] + " " + fs[1]
	}
	k1, s1 := keyOf("k1")
	k2, s2 := keyOf("k2")
	f.signers, f.otherKey = s1, s2

	// mk: tag'e özel temiz bir derleme klasöründe arşiv üretir ve signKey ile imzalar.
	mk := func(tag, manifestID string, withManifest bool, signKey string) (arch, sig string) {
		d := filepath.Join(tmp, "build-"+tag)
		pk := filepath.Join(d, "demo-0.1.0")
		if err := os.MkdirAll(pk, 0o755); err != nil {
			t.Fatal(err)
		}
		if withManifest {
			m := strings.Replace(lockManifest, `"id":"demo"`, `"id":"`+manifestID+`"`, 1)
			os.WriteFile(filepath.Join(pk, "component.json"), []byte(m), 0o644)
		}
		os.WriteFile(filepath.Join(pk, "README.md"), []byte("x\n"), 0o644)
		arch = filepath.Join(tmp, tag+".tar.gz")
		sh(t, d, "tar", "czf", arch, "demo-0.1.0")
		sh(t, tmp, "ssh-keygen", "-Y", "sign", "-q", "-f", signKey, "-n", "sshpocket-component", arch)
		return arch, arch + ".sig"
	}
	f.archive, f.sig = mk("ok", "demo", true, k1)
	bad := filepath.Join(tmp, "bad-same.tar.gz")
	b, _ := os.ReadFile(f.archive)
	os.WriteFile(bad, b, 0o644)
	sh(t, tmp, "ssh-keygen", "-Y", "sign", "-q", "-f", k2, "-n", "sshpocket-component", bad)
	f.badSig = bad + ".sig"
	f.noManifest, f.noSig = mk("nomanifest", "demo", false, k1)
	f.wrongID, f.wrongSig = mk("wrongid", "baska", true, k1)

	f.relSigners = filepath.Join(tmp, "release_signers")
	pub, _ := os.ReadFile(k1 + ".pub")
	pf := strings.Fields(string(pub))
	os.WriteFile(f.relSigners, []byte("# yorum satırı\n\nsshpocket-release namespaces=\"sshpocket-release\" "+pf[0]+" "+pf[1]+"\n"), 0o644)
	return f
}

type lockCase struct {
	name     string
	args     []string
	lockText *string // nil = dosya yok
	symlink  bool    // lock, gerçek dosyaya sembolik bağ
	archive  string  // "" = f.archive
	sig      string
	noArch   bool
}

type lockRes struct {
	code     int
	stdout   string
	hata     string // yalnız "HATA:" satırları
	lock     string
	lockSeen bool
	isLink   bool
}

func hataLines(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "HATA:") {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func (f *lockFx) setup(t *testing.T, c lockCase) (lockPath, realPath string) {
	d := t.TempDir()
	realPath = filepath.Join(d, "real.lock")
	lockPath = filepath.Join(d, "components.lock")
	if c.lockText != nil {
		os.WriteFile(realPath, []byte(*c.lockText), 0o644)
	}
	if c.symlink {
		if err := os.Symlink("real.lock", lockPath); err != nil {
			t.Fatal(err)
		}
	} else {
		realPath = lockPath
		if c.lockText != nil {
			os.WriteFile(lockPath, []byte(*c.lockText), 0o644)
		}
	}
	return
}

func (f *lockFx) read(lockPath, realPath string) lockRes {
	var r lockRes
	if b, err := os.ReadFile(realPath); err == nil {
		r.lock, r.lockSeen = string(b), true
	}
	if fi, err := os.Lstat(lockPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		r.isLink = true
	}
	return r
}

func (f *lockFx) runShell(t *testing.T, c lockCase) lockRes {
	lockPath, realPath := f.setup(t, c)
	arch, sig := f.archive, f.sig
	if c.archive != "" {
		arch, sig = c.archive, c.sig
	}
	cmd := exec.Command("bash", append([]string{filepath.Join(f.root, "referans/kabuk/scripts/lock-component.sh")}, c.args...)...)
	cmd.Env = append(os.Environ(), "SSHPOCKET_COMPONENTS_LOCK="+lockPath, "LOCK_RELEASE_SIGNERS="+f.relSigners)
	if !c.noArch {
		cmd.Env = append(cmd.Env, "LOCK_COMPONENT_ARCHIVE="+arch, "LOCK_COMPONENT_SIG="+sig)
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	r := f.read(lockPath, realPath)
	r.code, r.stdout, r.hata = code, so.String(), hataLines(se.String())
	return r
}

func (f *lockFx) runGo(t *testing.T, c lockCase) lockRes {
	lockPath, realPath := f.setup(t, c)
	arch, sig := f.archive, f.sig
	if c.archive != "" {
		arch, sig = c.archive, c.sig
	}
	if c.noArch {
		arch, sig = "", ""
	}
	var so, se bytes.Buffer
	comp := &Components{LockPath: lockPath, Out: &so, Err: &se,
		Download: func(url, dest string, max int64) error { return os.ErrNotExist }}
	code := comp.LockCommand(c.args, arch, sig, f.relSigners)
	r := f.read(lockPath, realPath)
	r.code, r.stdout, r.hata = code, so.String(), hataLines(se.String())
	return r
}

func sp(s string) *string { return &s }

func TestLockMatchesShell(t *testing.T) {
	f := lockFixtures(t)
	existing := "# components.lock — elle yazılan yorum\n\n[utility]\nrepo = o/utility\nversion = 0.1.1\nsha256 = " +
		strings.Repeat("a", 64) + "\nsigners = " + f.signers + "\n\n\n\n[demo]\nrepo = eski/demo\nversion = 0.0.9\nsha256 = " +
		strings.Repeat("b", 64) + "\nsigners = " + f.signers + "\n\n[kurulum]\nrepo = o/kurulum\nversion = 0.1.0\nsha256 = " +
		strings.Repeat("c", 64) + "\nsigners = " + f.signers + "\n"
	noNewline := strings.TrimSuffix(existing, "\n")
	onlyDemo := "[demo]\nrepo = eski/demo\nversion = 0.0.9\nsha256 = " + strings.Repeat("b", 64) + "\nsigners = " + f.signers + "\n"
	cases := []lockCase{
		{name: "yalnız göster (--repo, anahtar release_signers'tan)", args: []string{"demo", "0.1.0", "--repo", "o/demo"}},
		{name: "yeni dosyaya yaz", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--write"}},
		{name: "var olan girdiyi orta yerde değiştir", args: []string{"demo", "0.1.0", "--write"}, lockText: &existing},
		{name: "son satır sonu yok", args: []string{"demo", "0.1.0", "--write"}, lockText: &noNewline},
		{name: "tek girdi yerine geç", args: []string{"demo", "0.1.0", "--write"}, lockText: &onlyDemo},
		{name: "girdi yok, ekle", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--write"}, lockText: sp(strings.Replace(existing, "[demo]", "[baska]", 1))},
		{name: "açık --signers", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--signers", f.signers, "--write"}},
		{name: "sembolik bağlı kilit", args: []string{"demo", "0.1.0", "--write"}, lockText: &existing, symlink: true},
		{name: "yanlış anahtarla imzalı", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--write"}, archive: f.archive, sig: f.badSig},
		{name: "signers başka anahtar", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--signers", f.otherKey, "--write"}},
		{name: "component.json yok", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--write"}, archive: f.noManifest, sig: f.noSig},
		{name: "component.json kimliği uyuşmuyor", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--write"}, archive: f.wrongID, sig: f.wrongSig},
		{name: "mevcut girdinin signers'ı geçerli (release_signers'a bakılmaz)", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--write"},
			lockText: sp("[demo]\nrepo = o/demo\nversion = 0.0.9\nsha256 = " + strings.Repeat("b", 64) + "\nsigners = " + f.otherKey + "\n")},
		{name: "depo yok", args: []string{"demo", "0.1.0", "--write"}},
		{name: "depo geçersiz", args: []string{"demo", "0.1.0", "--repo", "bozuk", "--write"}},
		{name: "kimlik geçersiz", args: []string{"Demo", "0.1.0"}},
		{name: "sürüm geçersiz", args: []string{"demo", "0.1"}},
		{name: "bilinmeyen seçenek", args: []string{"demo", "0.1.0", "--yaz"}},
		{name: "argüman eksik", args: []string{"demo"}},
		{name: "signers satırı geçersiz", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--signers", "bozuk"}},
		{name: "mevcut kilit geçersiz", args: []string{"demo", "0.1.0", "--repo", "o/demo", "--write"}, lockText: sp("bu bir kilit değil\n")},
		{name: "sürüm arşivle uyuşmuyor", args: []string{"demo", "0.2.0", "--repo", "o/demo", "--write"}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			s, g := f.runShell(t, c), f.runGo(t, c)
			s.hata = strings.ReplaceAll(s.hata, "lock-component.sh", "components lock") // kasıtlı fark: komut adı
			if s.code != g.code {
				t.Errorf("çıkış kodu: kabuk=%d go=%d\nkabuk HATA: %q\ngo HATA: %q", s.code, g.code, s.hata, g.hata)
			}
			if s.stdout != g.stdout {
				t.Errorf("stdout farklı\nkabuk:\n%s\ngo:\n%s", s.stdout, g.stdout)
			}
			if s.hata != g.hata {
				t.Errorf("HATA satırı farklı\nkabuk: %q\ngo:    %q", s.hata, g.hata)
			}
			if s.lock != g.lock || s.lockSeen != g.lockSeen {
				t.Errorf("components.lock farklı\nkabuk:\n%s\ngo:\n%s", s.lock, g.lock)
			}
			if s.isLink != g.isLink {
				t.Errorf("sembolik bağ korunumu farklı: kabuk=%v go=%v", s.isLink, g.isLink)
			}
			if c.symlink && !g.isLink {
				t.Error("sembolik bağ ezildi")
			}
		})
	}
}

// Yazılan kilidi okuyucu da kabul etmeli ve kurulum akışı onunla çalışmalı (üretim → tüketim).
func TestLockWrittenFileLoads(t *testing.T) {
	f := lockFixtures(t)
	lockPath := filepath.Join(t.TempDir(), "components.lock")
	var so, se bytes.Buffer
	c := &Components{LockPath: lockPath, Out: &so, Err: &se}
	if code := c.LockCommand([]string{"demo", "0.1.0", "--repo", "o/demo", "--write"}, f.archive, f.sig, f.relSigners); code != 0 {
		t.Fatalf("kod %d: %s", code, se.String())
	}
	l, ok := (&Components{LockPath: lockPath, Err: &se}).lock()
	if !ok {
		t.Fatalf("yazılan kilit okunamadı: %s", se.String())
	}
	e, found := l.Get("demo")
	if !found || e.Version != "0.1.0" || e.Repo != "o/demo" {
		t.Fatalf("girdi beklenmedik: %+v", e)
	}
}

// İndirme yolu (kabuk karşılaştırmasında arşiv ortam değişkeniyle verildiği için ayrıca): URL biçimi, başarı, iki hata iletisi.
func TestLockDownloadPath(t *testing.T) {
	f := lockFixtures(t)
	run := func(dl func(url, dest string, max int64) error) (int, string, string, string) {
		lockPath := filepath.Join(t.TempDir(), "components.lock")
		var so, se bytes.Buffer
		c := &Components{LockPath: lockPath, Out: &so, Err: &se, Download: dl}
		code := c.LockCommand([]string{"demo", "0.1.0", "--repo", "o/demo", "--write"}, "", "", f.relSigners)
		b, _ := os.ReadFile(lockPath)
		return code, so.String(), se.String(), string(b)
	}
	var urls []string
	good := func(url, dest string, max int64) error {
		urls = append(urls, url)
		src := f.archive
		if strings.HasSuffix(url, ".sig") {
			src = f.sig
		}
		return copyFile(src, dest)
	}
	code, _, se, lock := run(good)
	if code != 0 || !strings.Contains(lock, "[demo]") {
		t.Fatalf("indirme başarılı olmalıydı: kod %d\n%s", code, se)
	}
	want := []string{"https://github.com/o/demo/releases/download/v0.1.0/demo-0.1.0.tar.gz", "https://github.com/o/demo/releases/download/v0.1.0/demo-0.1.0.tar.gz.sig"}
	if strings.Join(urls, "\n") != strings.Join(want, "\n") {
		t.Errorf("URL'ler beklenenden farklı: %v", urls)
	}

	code, _, se, lock = run(func(url, dest string, max int64) error { return os.ErrNotExist })
	if code != 1 || !strings.Contains(se, "HATA: indirilemedi: "+want[0]) || lock != "" {
		t.Errorf("arşiv indirilemeyince hata ve kilide yazmama beklenirdi: kod %d, %q, kilit %q", code, se, lock)
	}
	code, _, se, lock = run(func(url, dest string, max int64) error {
		if strings.HasSuffix(url, ".sig") {
			return os.ErrNotExist
		}
		return copyFile(f.archive, dest)
	})
	if code != 1 || !strings.Contains(se, "HATA: imza indirilemedi: "+want[1]) || lock != "" {
		t.Errorf("imza indirilemeyince hata beklenirdi: kod %d, %q, kilit %q", code, se, lock)
	}
}
