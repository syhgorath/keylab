package trust

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Paketin güvenli kurulumu (ComponentsInstall.sh).
//   components/<id>/<sürüm>/        kurulu içerik (imzası doğrulanmış)
//   components/<id>/<sürüm>.files   kurulumdaki dosya özetleri (her çalıştırmadan önce yeniden doğrulanır)
//   components/<id>/current         etkin sürüm (düz metin)
// Eski sürümler kalır. Geri sürüme düşülmez. Kullanıcı verisi içermez.

const (
	maxDownload   = 50 * 1024 * 1024
	maxUnpacked   = 50 * 1024 * 1024
	maxArchiveEnt = 5000
)

// HTTPSDownload: yalnızca HTTPS (yönlendirmelerde de), TLS ≥ 1.2, boyut ve süre sınırlı.
func HTTPSDownload(url, dest string, maxBytes int64) error {
	if !strings.HasPrefix(url, "https://") {
		return errors.New("yalnızca HTTPS")
	}
	cl := &http.Client{
		Timeout: 180 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return errors.New("yönlendirme HTTPS değil")
			}
			if len(via) > 10 {
				return errors.New("çok fazla yönlendirme")
			}
			return nil
		},
	}
	resp, err := cl.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if n > maxBytes {
		os.Remove(dest)
		return errors.New("dosya boyut sınırını aşıyor")
	}
	return nil
}

// CheckArchive, arşiv içeriğini AÇMADAN denetler: boş değil, mutlak yol ve ".." yok, her ad prefix ya da prefix/…
// ile başlar, yalnızca düz dosya ve dizin vardır (symlink, hardlink, aygıt, fifo reddedilir).
func CheckArchive(archive, prefix string) error {
	f, err := os.Open(archive)
	if err != nil {
		return errors.New("arşiv okunamadı")
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return errors.New("arşiv okunamadı")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	types := map[byte]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("arşiv okunamadı")
		}
		if h.Typeflag == tar.TypeXGlobalHeader { // git archive / GitHub sürüm arşivleri: pax genel üst verisi, dosya değildir
			continue
		}
		names = append(names, h.Name)
		types[typeChar(h.Typeflag)] = true
	}
	if len(names) == 0 {
		return errors.New("arşiv boş")
	}
	for _, name := range names {
		switch {
		case strings.HasPrefix(name, "/"):
			return fmt.Errorf("mutlak yol: %s", name)
		case strings.Contains(name, ".."):
			return fmt.Errorf("şüpheli yol: %s", name)
		case name == prefix || strings.HasPrefix(name, prefix+"/"):
		default:
			return fmt.Errorf("beklenmeyen üst dizin: %s", name)
		}
	}
	// yalnızca düz dosya ve dizin (tar -tv ilk karakterlerinin sıralı kümesi: "-", "d", "-d")
	var set []byte
	for c := range types {
		set = append(set, c)
	}
	sort.Slice(set, func(i, j int) bool { return set[i] < set[j] })
	switch string(set) {
	case "-", "d", "-d":
		return nil
	}
	return fmt.Errorf("arşivde düz dosya/dizin dışında öğe var (%s)", set)
}

func typeChar(t byte) byte {
	switch t {
	case tar.TypeReg, tar.TypeRegA:
		return '-'
	case tar.TypeDir:
		return 'd'
	case tar.TypeSymlink:
		return 'l'
	case tar.TypeLink:
		return 'h'
	case tar.TypeChar:
		return 'c'
	case tar.TypeBlock:
		return 'b'
	case tar.TypeFifo:
		return 'p'
	}
	return '?'
}

// extractTo, CheckArchive'dan geçmiş arşivi dest'e açar; dosya kipinden grup/diğer yazma izni silinir, özel bitler atılır.
func extractTo(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var total int64
	count := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		count++
		if count > maxArchiveEnt {
			return errors.New("arşivde çok fazla öğe")
		}
		target := filepath.Join(dest, filepath.FromSlash(h.Name))
		if rel, err := filepath.Rel(dest, target); err != nil || strings.HasPrefix(rel, "..") {
			return errors.New("güvensiz yol")
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			total += h.Size
			if total > maxUnpacked {
				return errors.New("arşiv çok büyük")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(h.Mode).Perm() &^ 0o022
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode|0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, io.LimitReader(tr, h.Size)); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			os.Chmod(target, mode)
		default:
			return errors.New("desteklenmeyen öğe")
		}
	}
}

// DigestTree, dizindeki dosyaların özet listesidir: "<sha256>  ./yol" satırları, yola göre bayt sıralı
// (find . -type f ! -name .DS_Store | LC_ALL=C sort).
func DigestTree(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() && d.Name() != ".DS_Store" {
			rel, _ := filepath.Rel(dir, p)
			paths = append(paths, "./"+filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]string, 0, len(paths))
	for _, rel := range paths {
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(rel[2:])))
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, hex.EncodeToString(h.Sum(nil))+"  "+rel)
	}
	return out, nil
}

func trimNL(s string) string { return strings.TrimRight(s, "\n") }

// VerifyInstalled: kurulu sürümün dosyaları kurulumdaki gibi mi?
func VerifyInstalled(componentsDir, id, ver string) error {
	base := filepath.Join(componentsDir, id)
	st, err := os.Stat(filepath.Join(base, ver))
	fb, ferr := os.ReadFile(filepath.Join(base, ver+".files"))
	if err != nil || !st.IsDir() || ferr != nil {
		return fmt.Errorf("kurulu paket bulunamadı: %s %s", id, ver)
	}
	cur, err := DigestTree(filepath.Join(base, ver))
	if err != nil {
		return err
	}
	if trimNL(strings.Join(cur, "\n")) != trimNL(string(fb)) {
		return fmt.Errorf("kurulu paketin dosyaları değişmiş: %s %s", id, ver)
	}
	return nil
}

// manifestField, component.json üst düzey alanını (metin ya da tamsayı) okur.
func manifestField(path, key string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var d map[string]any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&d); err != nil {
		return "", false
	}
	switch v := d[key].(type) {
	case string:
		return v, true
	case json.Number:
		if _, err := strconv.ParseInt(v.String(), 10, 64); err == nil {
			return v.String(), true
		}
	}
	return "", false
}

func installLock(base string, tries int) (release func(), err error) {
	lock := filepath.Join(base, ".install.lock")
	var other string
	for i := 0; ; i++ {
		if err := os.Mkdir(lock, 0o755); err == nil {
			break
		}
		b, _ := os.ReadFile(filepath.Join(lock, "pid"))
		other = strings.TrimSpace(string(b))
		if pid, perr := strconv.Atoi(other); perr == nil && pid > 0 && isProcessDead(pid) {
			os.RemoveAll(lock) // sahibi ölmüş (yarım kalmış kurulum)
			continue
		}
		if i >= tries {
			msg := "başka bir kurulum sürüyor"
			if other != "" {
				msg += " (pid " + other + ")"
			}
			return nil, errors.New(msg + "; bitince yeniden deneyin")
		}
		time.Sleep(500 * time.Millisecond)
	}
	os.WriteFile(filepath.Join(lock, "pid"), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
	return func() { os.RemoveAll(lock) }, nil
}

// InstallArchive: Sıra — sürüm kilitle uyumlu mu → imza+özet → geri sürüm yok → arşiv denetimi → aç → bildirim
// kimliği/sürümü → yerleştir → current. expectedSHA yoksa sürüm TAM kilit sürümü olmalı; varsa sürüm ≥ kilit sürümü ve
// kilit sürümüne eşitse özet kilitteki de olmalı. Dönüş mesajı ("kuruldu: …" / "zaten kurulu: …") ya da hata.
func InstallArchive(componentsDir string, lock *Lock, id, ver, archive, sigFile, expectedSHA string) (string, error) {
	if !IDValid(id) || !VersionValid(ver) {
		return "", errors.New("geçersiz kimlik/sürüm")
	}
	e, ok := lock.Get(id)
	if !ok {
		return "", fmt.Errorf("paket kilit dosyasında yok: %s", id)
	}
	if expectedSHA == "" {
		if ver != e.Version {
			return "", fmt.Errorf("sürüm kilitle uyuşmuyor (%s, kilit: %s)", ver, e.Version)
		}
	} else {
		if VersionCmp(ver, e.Version) == -1 {
			return "", fmt.Errorf("sürüm kilit tabanının altında (%s, kilit: %s)", ver, e.Version)
		}
		if ver == e.Version && expectedSHA != e.SHA256 {
			return "", fmt.Errorf("özet kilitle uyuşmuyor (%s %s)", id, ver)
		}
	}
	if err := VerifyArchive(lock, id, archive, sigFile, expectedSHA); err != nil {
		return "", err
	}
	if inst := InstalledVersion(componentsDir, id); inst != "" {
		switch VersionCmp(ver, inst) {
		case -1:
			return "", fmt.Errorf("geri sürüme düşülmez (%s → %s)", inst, ver)
		case 0:
			if err := VerifyInstalled(componentsDir, id, ver); err != nil {
				return "", err
			}
			return fmt.Sprintf("zaten kurulu: %s %s", id, ver), nil
		}
	}
	if err := CheckArchive(archive, id+"-"+ver); err != nil {
		return "", err
	}
	base := filepath.Join(componentsDir, id)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	release, err := installLock(base, 20)
	if err != nil {
		return "", err
	}
	defer release()
	return place(componentsDir, id, ver, archive, base)
}

func place(componentsDir, id, ver, archive, base string) (string, error) {
	if InstalledVersion(componentsDir, id) == ver {
		if err := VerifyInstalled(componentsDir, id, ver); err != nil {
			return "", err
		}
		return fmt.Sprintf("zaten kurulu: %s %s", id, ver), nil
	}
	tmp, err := os.MkdirTemp(base, ".tmp.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := extractTo(archive, tmp); err != nil {
		return "", errors.New("arşiv açılamadı")
	}
	root := filepath.Join(tmp, id+"-"+ver)
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return "", errors.New("açılan içerik beklenen biçimde değil")
	}
	mf := filepath.Join(root, "component.json")
	if v, ok := manifestField(mf, "id"); !ok || v != id {
		return "", errors.New("component.json kimlik/sürüm paketle uyuşmuyor")
	}
	if v, ok := manifestField(mf, "version"); !ok || v != ver {
		return "", errors.New("component.json kimlik/sürüm paketle uyuşmuyor")
	}
	if _, err := os.Lstat(filepath.Join(base, ver)); err == nil {
		return "", fmt.Errorf("hedef sürüm klasörü zaten var (%s); önce elle inceleyin", ver)
	}
	digests, err := DigestTree(root)
	if err != nil {
		return "", err
	}
	filesTmp := filepath.Join(base, ver+".files.tmp")
	if err := os.WriteFile(filesTmp, []byte(strings.Join(digests, "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(root, filepath.Join(base, ver)); err != nil {
		os.Remove(filesTmp)
		return "", err
	}
	os.Rename(filesTmp, filepath.Join(base, ver+".files"))
	cur := filepath.Join(base, "current.tmp")
	if err := os.WriteFile(cur, []byte(ver+"\n"), 0o644); err == nil {
		err = os.Rename(cur, filepath.Join(base, "current"))
	}
	if err != nil || InstalledVersion(componentsDir, id) != ver {
		os.RemoveAll(filepath.Join(base, ver))
		os.Remove(filepath.Join(base, ver+".files"))
		os.Remove(cur)
		return "", errors.New("etkin sürüm yazılamadı")
	}
	return fmt.Sprintf("kuruldu: %s %s", id, ver), nil
}

// Install, kilitteki sürümü indirip kurar (sürümü kilit belirler; "latest" yoktur).
func Install(componentsDir string, lock *Lock, id string, dl DownloadFunc) (string, error) {
	e, ok := lock.Get(id)
	if !ok {
		return "", fmt.Errorf("paket kilit dosyasında yok: %s", id)
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s-%s.tar.gz", e.Repo, e.Version, id, e.Version)
	return downloadAndInstall(componentsDir, lock, id, e.Version, url, "", dl)
}

// InstallFromIndex, indeksin sürümünü indirip kurar (sha256 indeksten, imza kilit imzacısından).
func InstallFromIndex(componentsDir string, lock *Lock, idx *Index, id string, dl DownloadFunc) (string, error) {
	p, ok := idx.Get(id)
	if !ok {
		return "", fmt.Errorf("indekste yok: %s", id)
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", p.Repo, p.Version, p.Asset)
	return downloadAndInstall(componentsDir, lock, id, p.Version, url, p.SHA256, dl)
}

func downloadAndInstall(componentsDir string, lock *Lock, id, ver, url, expected string, dl DownloadFunc) (string, error) {
	if dl == nil {
		dl = HTTPSDownload
	}
	work, err := os.MkdirTemp("", "sshpocket-comp.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	pkg, sig := filepath.Join(work, "pkg.tar.gz"), filepath.Join(work, "pkg.tar.gz.sig")
	if dl(url, pkg, maxDownload) != nil || dl(url+".sig", sig, maxDownload) != nil {
		return "", fmt.Errorf("indirme başarısız: %s", url)
	}
	return InstallArchive(componentsDir, lock, id, ver, pkg, sig, expected)
}

// Prepare, comp_prepare karşılığıdır: kurulu etkin sürümü kilit tabanı ve dosya bütünlüğüyle doğrular; sürümü ve
// component.json yolunu verir (bildirimin kendisi manifest.ValidateFile ile doğrulanır).
func Prepare(componentsDir string, lock *Lock, id string) (version, manifestPath string, err error) {
	if !IDValid(id) {
		return "", "", errors.New("geçersiz paket kimliği")
	}
	e, ok := lock.Get(id)
	if !ok {
		return "", "", fmt.Errorf("paket kilit dosyasında yok: %s", id)
	}
	ver := InstalledVersion(componentsDir, id)
	if ver == "" {
		return "", "", fmt.Errorf("paket kurulu değil: %s", id)
	}
	if VersionCmp(ver, e.Version) == -1 {
		return "", "", fmt.Errorf("kurulu sürüm kilit tabanının altında (%s)", ver)
	}
	if err := VerifyInstalled(componentsDir, id, ver); err != nil {
		return "", "", err
	}
	return ver, filepath.Join(componentsDir, id, ver, "component.json"), nil
}
