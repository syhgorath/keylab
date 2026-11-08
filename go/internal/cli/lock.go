package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sshpocket-go/internal/manifest"
	"sshpocket-go/internal/trust"
)

// BAKIMCI aracı: bir paket sürümünü components.lock'a güvenle yazar (scripts/lock-component.sh karşılığı).
//
//	components lock <id> <sürüm> [--repo sahip/depo] [--signers "<satır>"] [--write]
//
// Paketi + .sig'i GitHub Release'ten (yalnızca HTTPS) indirir → arşiv denetimi → imza (ad alanı sshpocket-component) →
// component.json (kimlik+sürüm+şema) → sha256 → girdiyi üretir. --write yoksa YALNIZCA gösterir.
// Kurulamayacak bir paket (component.json yok/geçersiz) kilide ASLA yazılmaz.

// LockOptions: Archive/Sig doluysa indirme atlanır (test; doğrulamalar yine yapılır). ReleaseSigners: tek imza anahtarının açık anahtar dosyası.
type LockOptions struct {
	ID, Version, Repo, Signers string
	Write                      bool
	Archive, Sig               string
	ReleaseSigners             string
}

var reEd25519Key = regexp.MustCompile(`.*(ssh-ed25519 [A-Za-z0-9+/]+=*).*`)

// LockCommand, komut satırı bağımsız değişkenlerini ayrıştırıp LockComponent'i çalıştırır.
func (c *Components) LockCommand(args []string, archive, sig, releaseSigners string) int {
	die := func(format string, a ...any) int { c.err("HATA: "+format+"\n", a...); return 1 }
	if len(args) < 2 {
		return die(`kullanım: components lock <id> <sürüm> [--repo sahip/depo] [--signers "<satır>"] [--write]`)
	}
	o := LockOptions{ID: args[0], Version: args[1], Archive: archive, Sig: sig, ReleaseSigners: releaseSigners}
	rest := args[2:]
	for len(rest) > 0 {
		val := ""
		if len(rest) > 1 {
			val = rest[1]
		}
		switch rest[0] {
		case "--repo":
			o.Repo = val
			rest = skip(rest, 2)
		case "--signers":
			o.Signers = val
			rest = skip(rest, 2)
		case "--write":
			o.Write = true
			rest = skip(rest, 1)
		default:
			return die("bilinmeyen seçenek: %s", rest[0])
		}
	}
	return c.LockComponent(o)
}

func skip(l []string, n int) []string {
	if n > len(l) {
		n = len(l)
	}
	return l[n:]
}

// resolveLockPath: components.lock sembolik bağsa yazarken bağı EZMEMEK için hedef yola çözülür.
func resolveLockPath(p string) string {
	if fi, err := os.Lstat(p); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if t, err := os.Readlink(p); err == nil {
			if !filepath.IsAbs(t) {
				t = filepath.Join(filepath.Dir(p), t)
			}
			return filepath.Clean(t)
		}
	}
	return p
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// firstReleaseKey: release_signers içindeki ilk yorum olmayan satırdan açık anahtar.
func firstReleaseKey(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if l == "" || l[0] == '#' || !strings.Contains(l, "ssh-ed25519 ") {
			continue
		}
		if m := reEd25519Key.FindStringSubmatch(l); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

// LockComponent, işin kendisi. Çıkış kodu: 0 başarılı, 1 hata.
func (c *Components) LockComponent(o LockOptions) int {
	die := func(format string, a ...any) int { c.err("HATA: "+format+"\n", a...); return 1 }
	if !trust.IDValid(o.ID) {
		return die("geçersiz paket kimliği")
	}
	if !trust.VersionValid(o.Version) {
		return die("geçersiz sürüm (X.Y.Z)")
	}
	lockPath := resolveLockPath(c.LockPath)
	var lock *trust.Lock
	var oldText string
	if b, err := os.ReadFile(lockPath); err == nil {
		oldText = string(b)
		l, lerr := trust.LoadLock(lockPath)
		if lerr != nil {
			return die("mevcut components.lock geçersiz")
		}
		lock = l
	} else {
		lock = &trust.Lock{}
	}
	cur, _ := lock.Get(o.ID)
	repo := o.Repo
	if repo == "" {
		repo = cur.Repo
	}
	if repo == "" {
		return die("depo bilinmiyor: --repo sahip/depo verin")
	}
	if !trust.RepoValid(repo) {
		return die("geçersiz depo adı")
	}
	signers := o.Signers
	if signers == "" {
		signers = cur.Signers
	}
	if signers == "" {
		key := firstReleaseKey(o.ReleaseSigners)
		if key == "" {
			return die("imzacı anahtarı bulunamadı (%s); --signers verin", o.ReleaseSigners)
		}
		signers = `sshpocket-component namespaces="sshpocket-component" ` + key
	}
	if !trust.SignersValid(signers) {
		return die("signers satırı geçersiz")
	}

	work, err := os.MkdirTemp("", "sshpocket-lock.")
	if err != nil {
		return die("geçici klasör açılamadı")
	}
	defer os.RemoveAll(work)
	archive, sigFile := filepath.Join(work, "pkg.tar.gz"), filepath.Join(work, "pkg.tar.gz.sig")
	if o.Archive != "" {
		if o.Sig == "" {
			return die("LOCK_COMPONENT_SIG gerekli")
		}
		if copyFile(o.Archive, archive) != nil || copyFile(o.Sig, sigFile) != nil {
			return die("arşiv/imza kopyalanamadı")
		}
	} else {
		dl := c.Download
		if dl == nil {
			dl = trust.HTTPSDownload
		}
		url := fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s-%s.tar.gz", repo, o.Version, o.ID, o.Version)
		if dl(url, archive, 50*1024*1024) != nil {
			return die("indirilemedi: %s", url)
		}
		if dl(url+".sig", sigFile, 50*1024*1024) != nil {
			return die("imza indirilemedi: %s.sig", url)
		}
	}

	prefix := o.ID + "-" + o.Version
	if trust.CheckArchive(archive, prefix) != nil {
		return die("arşiv denetimi başarısız")
	}
	sgn, serr := trust.SignersFromLockLine(signers)
	sigBytes, rerr := os.ReadFile(sigFile)
	af, oerr := os.Open(archive)
	if serr != nil || rerr != nil || oerr != nil {
		return die("imza doğrulanamadı (signers/ad alanı/paket uyuşmuyor)")
	}
	verr := trust.Verify(sgn, trust.ComponentNamespace, trust.ComponentNamespace, sigBytes, af)
	af.Close()
	if verr != nil {
		return die("imza doğrulanamadı (signers/ad alanı/paket uyuşmuyor)")
	}

	x := filepath.Join(work, "x")
	if err := os.Mkdir(x, 0o700); err != nil || trust.ExtractArchive(archive, x) != nil {
		return die("arşiv açılamadı")
	}
	mf := filepath.Join(x, prefix, "component.json")
	if _, err := os.Stat(mf); err != nil {
		return die("paket component.json içermiyor; kurulamaz, kilide yazılmadı (bkz. linux-hardening#30)")
	}
	if _, errs, code := manifest.ValidateFile(mf, o.ID, o.Version); code != 0 {
		for _, e := range errs {
			c.err("%s\n", e)
		}
		return die("component.json geçersiz (yukarıdaki nedenler); kilide yazılmadı")
	}

	sha, err := trust.FileSHA256(archive)
	if err != nil {
		return die("özet hesaplanamadı")
	}
	entry := trust.LockEntry{ID: o.ID, Repo: repo, Version: o.Version, SHA256: sha, Signers: signers}
	c.out("%s\n", entry.Text())
	if !o.Write {
		c.out("(yalnızca gösterildi; yazmak için --write)\n")
		return 0
	}

	text := trust.ReplaceLockEntry(oldText, entry)
	newFile := filepath.Join(work, "lock.new")
	if err := os.WriteFile(newFile, []byte(text), 0o644); err != nil {
		return die("geçici dosya yazılamadı")
	}
	if _, err := trust.LoadLock(newFile); err != nil {
		return die("üretilen kilit dosyası geçersiz; yazılmadı")
	}
	tmp := lockPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return die("components.lock yazılamadı")
	}
	if err := os.Rename(tmp, lockPath); err != nil {
		os.Remove(tmp)
		return die("components.lock yazılamadı")
	}
	c.out("✅ components.lock güncellendi: %s %s\n", o.ID, o.Version)
	return 0
}
