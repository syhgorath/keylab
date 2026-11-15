package selfupdate

// macOS uygulama paketini (KeyLab.app) kendi kendine güncelleme. Tek ikiliyi değiştirmek paketin imza mührünü bozar (denendi: codesign --verify "code has no
// resources but signature indicates they must be present"), bu yüzden .app BÜTÜNÜYLE değiştirilir: keylab-X.Y.Z-macos.zip indirilir, aynı release anahtarıyla
// imzası doğrulanır, GÜVENLİ biçimde açılır (yol kaçışı/sembolik bağ yok, boyut sınırlı), açılan paketin ikilisi beklenen sürümü bildirmeli ve imzası bozuk
// olmamalı, sonra eski paket yedeklenip yenisi yerine konur (hata olursa geri alınır).

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"keylab/internal/trust"
)

const (
	maxZip        = 100 << 20
	maxZipEntries = 2000
)

var (
	maxUnzipped int64 = 300 << 20 // testte düşürülür
	rename            = os.Rename // testte hata enjekte edilir (geri alma yolunu sınamak için)
)

// ReleaseNotes: GitHub sürüm notları (release-publish.sh bunları CHANGELOG'daki ilgili bölümden koyar). Yalnız bu reponun sürümü; HTTPS; en çok 512 KB.
func (u *Updater) ReleaseNotes(ver string) (string, error) {
	if !reVer.MatchString(ver) {
		return "", errors.New("geçersiz sürüm")
	}
	dl := u.Download
	if dl == nil {
		dl = trust.HTTPSDownload
	}
	d, err := os.MkdirTemp("", "keylab-notes.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(d)
	f := filepath.Join(d, "r.json")
	if err := dl("https://api.github.com/repos/"+u.repo()+"/releases/tags/v"+ver, f, 512<<10); err != nil {
		return "", err
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return "", err
	}
	var r struct {
		Body string `json:"body"`
	}
	if json.Unmarshal(b, &r) != nil {
		return "", errors.New("sürüm notları okunamadı")
	}
	return r.Body, nil
}

// safeUnzip: zip'i dest'e açar. Yalnız dizin ve düz dosya; mutlak yol, "..", sembolik bağ, aygıt reddedilir; `__MACOSX/` ve `.DS_Store` atlanır;
// toplam açılmış boyut ve girdi sayısı sınırlıdır. Çalıştırma bitleri korunur.
func safeUnzip(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("zip açılamadı: %v", err)
	}
	defer zr.Close()
	if len(zr.File) == 0 || len(zr.File) > maxZipEntries {
		return errors.New("zip girdi sayısı kabul edilemez")
	}
	var total int64
	root := filepath.Clean(dest) + string(filepath.Separator)
	for _, f := range zr.File {
		name := f.Name
		// ditto'nun eklediği AppleDouble yan dosyaları (._ad, .__CodeSignature) yalnız seal DIŞI genişletilmiş öznitelikleri (com.apple.provenance…) taşır; düz açılırsa paketin
		// içine fazladan dosya olarak yazılıp kod imzasını BOZAR ("a sealed resource is missing or invalid": gerçek paketle denendi). Atlanırlar; ditto ile açılan paketle aynı sonuç.
		if strings.HasPrefix(name, "__MACOSX/") || path.Base(name) == ".DS_Store" || strings.HasPrefix(path.Base(name), "._") {
			continue
		}
		if strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "\x00") {
			return fmt.Errorf("zip içinde güvensiz yol: %q", name)
		}
		for _, seg := range strings.Split(name, "/") {
			if seg == ".." {
				return fmt.Errorf("zip içinde güvensiz yol: %q", name)
			}
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target+string(filepath.Separator), root) {
			return fmt.Errorf("zip içinde güvensiz yol: %q", name)
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case mode.IsRegular():
			total += int64(f.UncompressedSize64)
			if total > maxUnzipped {
				return errors.New("zip açılmış boyut sınırını aşıyor")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			perm := mode.Perm() | 0o600
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
			if err != nil {
				rc.Close()
				return err
			}
			n, cerr := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
			rc.Close()
			if e := out.Close(); cerr == nil {
				cerr = e
			}
			if cerr != nil || n != int64(f.UncompressedSize64) {
				return errors.New("zip girdisi okunamadı")
			}
			if err := os.Chmod(target, perm); err != nil {
				return err
			}
		default:
			return fmt.Errorf("zip içinde desteklenmeyen girdi türü (sembolik bağ/aygıt): %q", name)
		}
	}
	return nil
}

// topApp: dizinde tam olarak bir *.app paketi var mı; yolunu döner.
func topApp(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var apps []string
	for _, e := range ents {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".app") {
			apps = append(apps, e.Name())
		} else if e.Name() != "__MACOSX" {
			return "", fmt.Errorf("zipte beklenmeyen üst düzey öğe: %s", e.Name())
		}
	}
	if len(apps) != 1 {
		return "", errors.New("zipte tam olarak bir .app paketi olmalı")
	}
	return filepath.Join(dir, apps[0]), nil
}

// ApplyApp: ver sürümünün macOS paketini indirir, imzasını doğrular, açar, doğrular ve appPath'in yerine koyar; eskisi yedeklenir (backup yolu döner).
// verifyApp: açılan paketin kod imzası denetimi (nil: atlanır). progress: adım bildirimi (nil olabilir). Yeniden başlatma çağıranın işidir.
func (u *Updater) ApplyApp(ver, appPath string, verifyApp func(string) error, progress func(step string)) (backup string, err error) {
	say := func(s string) {
		if progress != nil {
			progress(s)
		}
	}
	if !reVer.MatchString(ver) {
		return "", errors.New("geçersiz sürüm")
	}
	if !strings.HasSuffix(appPath, ".app") {
		return "", errors.New("hedef bir .app paketi değil")
	}
	if st, serr := os.Stat(appPath); serr != nil || !st.IsDir() {
		return "", fmt.Errorf("uygulama paketi bulunamadı: %s", appPath)
	}
	parent := filepath.Dir(appPath)
	signersText, _ := os.ReadFile(u.SignersFile)
	signers := trust.ParseAllowedSigners(string(signersText))
	if len(signers) == 0 {
		return "", errors.New("imza anahtarı tanımlı değil; güncelleme kapalı")
	}
	work, werr := os.MkdirTemp(parent, ".keylab-update-") // AYNI birimde: rename atomik olsun
	if werr != nil {
		return "", fmt.Errorf("uygulamanın klasörü yazılabilir değil (%s): %v", parent, werr)
	}
	defer os.RemoveAll(work)
	dl := u.Download
	if dl == nil {
		dl = trust.HTTPSDownload
	}
	zipPath, sigPath := filepath.Join(work, "paket.zip"), filepath.Join(work, "paket.zip.sig")
	base := "https://github.com/" + u.repo() + "/releases/download/v" + ver + "/keylab-" + ver + "-macos.zip"
	say("indiriliyor")
	if dl(base, zipPath, maxZip) != nil || dl(base+".sig", sigPath, 4096) != nil {
		return "", errors.New("indirme başarısız")
	}
	say("imza doğrulanıyor")
	sigRaw, rerr := os.ReadFile(sigPath)
	f, oerr := os.Open(zipPath)
	if rerr != nil || oerr != nil || trust.Verify(signers, Principal, Namespace, sigRaw, f) != nil {
		if f != nil {
			f.Close()
		}
		return "", errors.New("İMZA DOĞRULANAMADI: paket kurulmadı (sahte ya da bozuk olabilir)")
	}
	f.Close()
	say("açılıyor")
	ext := filepath.Join(work, "x")
	if err := os.Mkdir(ext, 0o755); err != nil {
		return "", err
	}
	if err := safeUnzip(zipPath, ext); err != nil {
		return "", err
	}
	newApp, err := topApp(ext)
	if err != nil {
		return "", err
	}
	bin := filepath.Join(newApp, "Contents", "MacOS", "KeyLab")
	if st, serr := os.Stat(bin); serr != nil || !st.Mode().IsRegular() {
		return "", errors.New("paketin içinde çalıştırılabilir ikili yok")
	}
	say("doğrulanıyor")
	reported, verr := u.runVersion(bin)
	if verr != nil || reported != "sshpocket "+ver {
		return "", fmt.Errorf("paketteki ikilinin bildirdiği sürüm (%s) istenenle (%s) uyuşmuyor, kurulmadı", strings.TrimPrefix(reported, "sshpocket "), ver)
	}
	if verifyApp != nil {
		if err := verifyApp(newApp); err != nil {
			return "", err
		}
	}
	say("yerleştiriliyor")
	bdir := filepath.Join(parent, ".keylab-update-backup")
	if err := os.MkdirAll(bdir, 0o700); err != nil {
		return "", fmt.Errorf("yedek klasörü oluşturulamadı: %v", err)
	}
	if ents, _ := os.ReadDir(bdir); len(ents) > 0 { // yalnız bir önceki yedek tutulur
		for _, e := range ents {
			os.RemoveAll(filepath.Join(bdir, e.Name()))
		}
	}
	backup = filepath.Join(bdir, "KeyLab-"+u.Version+".app")
	if err := rename(appPath, backup); err != nil {
		return "", fmt.Errorf("eski paket yedeklenemedi (uygulama başka bir yerden salt okunur çalışıyor olabilir): %v", err)
	}
	if err := rename(newApp, appPath); err != nil {
		if rb := rename(backup, appPath); rb != nil {
			return "", fmt.Errorf("yeni paket yerleştirilemedi (%v) ve eski paket geri konamadı: %v; yedek: %s", err, rb, backup)
		}
		return "", fmt.Errorf("yeni paket yerleştirilemedi: %v (eski paket geri konuldu)", err)
	}
	return backup, nil
}
