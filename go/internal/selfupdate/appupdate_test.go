package selfupdate

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type zent struct {
	name string
	data string
	mode os.FileMode
	link bool
}

func mkZip(t *testing.T, ents []zent) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, e := range ents {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		m := e.mode
		if e.link {
			m |= os.ModeSymlink
		}
		h.SetMode(m)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.data))
	}
	zw.Close()
	return b.Bytes()
}

// goodApp: KeyLab.app (yeni sürüm) girdileri: çalıştırılabilir ikili `version` ile sürümünü bildirir.
func goodApp(ver string) []zent {
	return []zent{
		{name: "KeyLab.app/", mode: os.ModeDir | 0o755}, {name: "KeyLab.app/Contents/", mode: os.ModeDir | 0o755}, {name: "KeyLab.app/Contents/MacOS/", mode: os.ModeDir | 0o755},
		{name: "KeyLab.app/Contents/MacOS/KeyLab", data: string(fakeBinary(ver)), mode: 0o755},
		{name: "KeyLab.app/Contents/Info.plist", data: "<plist/>", mode: 0o644},
		// ditto'nun AppleDouble yan dosyaları (gerçek zipte var): paketin içine YAZILMAMALI (kod imzasını bozar)
		{name: "KeyLab.app/Contents/._Info.plist", data: "appledouble", mode: 0o644}, {name: "KeyLab.app/Contents/.__CodeSignature", data: "appledouble", mode: 0o644},
		{name: "KeyLab.app/Contents/MacOS/._KeyLab", data: "appledouble", mode: 0o644}, {name: "__MACOSX/KeyLab.app/Contents/._Info.plist", data: "appledouble", mode: 0o644},
	}
}

type appFx struct {
	e       *env
	app     string
	u       *Updater
	steps   []string
	pubZip  func(ver string, zipBytes []byte, signWith, ns string)
	version string
}

func newAppFx(t *testing.T) *appFx {
	t.Helper()
	e := newEnv(t)
	f := &appFx{e: e, app: filepath.Join(e.dir, "Apps", "KeyLab.app")}
	os.MkdirAll(filepath.Join(f.app, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(f.app, "Contents", "MacOS", "KeyLab"), fakeBinary("1.0.0"), 0o755)
	f.u, _, _ = e.updater("")
	f.pubZip = func(ver string, zb []byte, signWith, ns string) {
		url := "https://github.com/syhgorath/keylab/releases/download/v" + ver + "/keylab-" + ver + "-macos.zip"
		e.net[url] = zb
		e.net[url+".sig"] = sign(t, signWith, ns, zb)
	}
	return f
}

func (f *appFx) apply(ver string) (string, error) {
	return f.u.ApplyApp(ver, f.app, nil, func(s string) { f.steps = append(f.steps, s) })
}
func (f *appFx) appVersion() string {
	o, _ := exec.Command(filepath.Join(f.app, "Contents", "MacOS", "KeyLab"), "version").Output()
	return strings.TrimSpace(string(o))
}
func (f *appFx) noLeftovers(t *testing.T) {
	t.Helper()
	ents, _ := os.ReadDir(filepath.Dir(f.app))
	for _, x := range ents {
		if strings.HasPrefix(x.Name(), ".keylab-update-") && x.Name() != ".keylab-update-backup" {
			t.Errorf("geçici güncelleme klasörü kaldı: %s", x.Name())
		}
	}
}

func TestApplyAppReplacesWholeBundleAndKeepsOneBackup(t *testing.T) {
	f := newAppFx(t)
	f.pubZip("1.2.0", mkZip(t, goodApp("1.2.0")), f.e.priv, Namespace)
	backup, err := f.apply("1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if f.appVersion() != "sshpocket 1.2.0" {
		t.Fatalf("uygulama güncellenmedi: %s", f.appVersion())
	}
	if _, err := os.Stat(filepath.Join(f.app, "Contents", "Info.plist")); err != nil {
		t.Fatal("paketin diğer dosyaları (Info.plist) yerinde değil")
	}
	filepath.WalkDir(f.app, func(p string, d os.DirEntry, _ error) error {
		if strings.HasPrefix(d.Name(), "._") {
			t.Errorf("AppleDouble yan dosyası pakete yazıldı (kod imzasını bozar): %s", p)
		}
		return nil
	})
	if o, _ := exec.Command(filepath.Join(backup, "Contents", "MacOS", "KeyLab"), "version").Output(); strings.TrimSpace(string(o)) != "sshpocket 1.0.0" {
		t.Fatalf("yedek eski sürüm değil: %s", o)
	}
	if strings.Join(f.steps, ",") != "indiriliyor,imza doğrulanıyor,açılıyor,doğrulanıyor,yerleştiriliyor" {
		t.Fatalf("adımlar: %v", f.steps)
	}
	f.noLeftovers(t)
	// ikinci güncelleme: yalnız son yedek (1.2.0) kalır
	f.u.Version = "1.2.0"
	f.pubZip("1.3.0", mkZip(t, goodApp("1.3.0")), f.e.priv, Namespace)
	if _, err := f.apply("1.3.0"); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(filepath.Join(filepath.Dir(f.app), ".keylab-update-backup"))
	if len(ents) != 1 || ents[0].Name() != "KeyLab-1.2.0.app" {
		t.Fatalf("yedekler: %v", ents)
	}
}

func TestApplyAppRefusesBadSignatureTamperingAndWrongNamespace(t *testing.T) {
	for name, mut := range map[string]func(f *appFx){
		"yanlış anahtar": func(f *appFx) {
			p2, _ := keypair(t, f.e.dir, "other")
			f.pubZip("1.2.0", mkZip(t, goodApp("1.2.0")), p2, Namespace)
		},
		"yanlış ad alanı": func(f *appFx) { f.pubZip("1.2.0", mkZip(t, goodApp("1.2.0")), f.e.priv, "sshpocket-component") },
		"imzadan sonra değiştirilmiş zip": func(f *appFx) {
			f.pubZip("1.2.0", mkZip(t, goodApp("1.2.0")), f.e.priv, Namespace)
			u := "https://github.com/syhgorath/keylab/releases/download/v1.2.0/keylab-1.2.0-macos.zip"
			f.e.net[u] = append(f.e.net[u], []byte("zehir")...)
		},
	} {
		f := newAppFx(t)
		mut(f)
		if _, err := f.apply("1.2.0"); err == nil || !strings.Contains(err.Error(), "İMZA") {
			t.Fatalf("%s: beklenen imza hatası: %v", name, err)
		}
		if f.appVersion() != "sshpocket 1.0.0" {
			t.Fatalf("%s: uygulama değişti", name)
		}
		f.noLeftovers(t)
	}
}

func TestApplyAppRefusesUnsafeZipsAndLeavesNothingBehind(t *testing.T) {
	for name, ents := range map[string][]zent{
		"yol kaçışı (..)":    append(goodApp("1.2.0"), zent{name: "KeyLab.app/../../evil", data: "x", mode: 0o644}),
		"mutlak yol":         append(goodApp("1.2.0"), zent{name: "/tmp/evil-keylab", data: "x", mode: 0o644}),
		"sembolik bağ":       append(goodApp("1.2.0"), zent{name: "KeyLab.app/Contents/link", data: "/etc/passwd", mode: 0o777, link: true}),
		"üst düzeyde ek öğe": append(goodApp("1.2.0"), zent{name: "kurulum.sh", data: "x", mode: 0o755}),
		"iki paket":          append(goodApp("1.2.0"), zent{name: "Other.app/Contents/x", data: "x", mode: 0o644}),
	} {
		f := newAppFx(t)
		f.pubZip("1.2.0", mkZip(t, ents), f.e.priv, Namespace)
		if _, err := f.apply("1.2.0"); err == nil {
			t.Fatalf("%s: güvensiz zip kabul edildi", name)
		}
		if f.appVersion() != "sshpocket 1.0.0" {
			t.Fatalf("%s: uygulama değişti", name)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(f.e.dir), "evil")); err == nil {
			t.Fatalf("%s: paket dışına dosya yazıldı", name)
		}
		f.noLeftovers(t)
	}
}

func TestApplyAppChecksVersionCodesignSizeAndTarget(t *testing.T) {
	f := newAppFx(t) // sürüm uyuşmazlığı: zip içindeki ikili 9.9.9 diyor
	f.pubZip("1.2.0", mkZip(t, goodApp("9.9.9")), f.e.priv, Namespace)
	if _, err := f.apply("1.2.0"); err == nil || !strings.Contains(err.Error(), "uyuşmuyor") || f.appVersion() != "sshpocket 1.0.0" {
		t.Fatalf("sürüm uyuşmazlığı: %v", err)
	}
	f = newAppFx(t) // codesign doğrulaması başarısız
	f.pubZip("1.2.0", mkZip(t, goodApp("1.2.0")), f.e.priv, Namespace)
	if _, err := f.u.ApplyApp("1.2.0", f.app, func(string) error { return errors.New("imza bozuk") }, nil); err == nil || f.appVersion() != "sshpocket 1.0.0" {
		t.Fatalf("codesign hatası uygulamayı değiştirdi: %v", err)
	}
	f = newAppFx(t) // açılmış boyut sınırı
	old := maxUnzipped
	maxUnzipped = 50
	t.Cleanup(func() { maxUnzipped = old })
	f.pubZip("1.2.0", mkZip(t, goodApp("1.2.0")), f.e.priv, Namespace)
	if _, err := f.apply("1.2.0"); err == nil || !strings.Contains(err.Error(), "boyut") {
		t.Fatalf("boyut sınırı: %v", err)
	}
	maxUnzipped = old
	for _, bad := range []struct{ ver, app string }{{"1.2", f.app}, {"1.2.0", filepath.Join(f.e.dir, "KeyLab")}, {"1.2.0", filepath.Join(f.e.dir, "yok.app")}} {
		if _, err := f.u.ApplyApp(bad.ver, bad.app, nil, nil); err == nil {
			t.Fatalf("geçersiz girdi kabul edildi: %+v", bad)
		}
	}
	os.WriteFile(f.e.signers, []byte("# boş\n"), 0o644) // anahtar yok → güncelleme kapalı
	if _, err := f.apply("1.2.0"); err == nil || !strings.Contains(err.Error(), "anahtarı tanımlı değil") {
		t.Fatalf("anahtarsız: %v", err)
	}
}

// Yerleştirme ikinci adımda başarısız olursa ESKİ paket geri konur (uygulama bozuk kalmaz).
func TestApplyAppRollsBackWhenPlacementFails(t *testing.T) {
	f := newAppFx(t)
	f.pubZip("1.2.0", mkZip(t, goodApp("1.2.0")), f.e.priv, Namespace)
	old, n := rename, 0
	rename = func(a, b string) error {
		n++
		if n == 2 { // yeni paketi yerine koyma adımı
			return errors.New("disk dolu")
		}
		return os.Rename(a, b)
	}
	t.Cleanup(func() { rename = old })
	_, err := f.apply("1.2.0")
	if err == nil || !strings.Contains(err.Error(), "geri konuldu") {
		t.Fatalf("hata: %v", err)
	}
	if f.appVersion() != "sshpocket 1.0.0" {
		t.Fatalf("eski paket geri konmadı: %q", f.appVersion())
	}
}

func TestReleaseNotes(t *testing.T) {
	e := newEnv(t)
	u, _, _ := e.updater("")
	e.net["https://api.github.com/repos/syhgorath/keylab/releases/tags/v1.2.0"] = []byte(`{"tag_name":"v1.2.0","body":"### Düzeltildi\n- **Bir şey**: ayrıntı"}`)
	if b, err := u.ReleaseNotes("1.2.0"); err != nil || b != "### Düzeltildi\n- **Bir şey**: ayrıntı" {
		t.Fatalf("notlar: %q %v", b, err)
	}
	if _, err := u.ReleaseNotes("1.9.9"); err == nil {
		t.Fatal("olmayan sürüm hata vermeliydi")
	}
	if _, err := u.ReleaseNotes("../x"); err == nil {
		t.Fatal("geçersiz sürüm kabul edildi")
	}
	e.net["https://api.github.com/repos/syhgorath/keylab/releases/tags/v1.3.0"] = []byte(`bozuk`)
	if _, err := u.ReleaseNotes("1.3.0"); err == nil {
		t.Fatal("bozuk JSON hata vermeliydi")
	}
	_ = fmt.Sprint
}
