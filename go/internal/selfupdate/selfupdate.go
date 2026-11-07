// Package selfupdate: tek ikilinin imzalı kendini güncellemesi (Update.sh'ın tek-ikili karşılığı).
//
// Güvenlik modeli (kabuk sürümüyle aynı ilkeler):
//   - Yalnızca bu reponun GitHub Release'leri, yalnızca HTTPS.
//   - İkili, kök klasördeki sistem/release_signers içindeki anahtarla (ad alanı sshpocket-release) imzalı olmalı; imza doğrulanmadan
//     hiçbir şey çalıştırılmaz ya da yerleştirilmez. Anahtar yoksa güncelleme KAPALIDIR.
//   - Yeni ikilinin kendi bildirdiği sürüm (`version`) istenen etiketle aynı olmalı (eski imzalı sürümün yeniden sunulmasını engeller).
//   - Geri sürüme düşülmez. Yerleştirme atomiktir (aynı klasörde yeni dosya + rename); eski ikili .update-backup/ altına yedeklenir ve
//     hata olursa geri konur. Kullanıcı verisine (servers/, components/, anahtarlar) dokunulmaz.
//
// Yayın varlıkları: sshpocket-X.Y.Z-<os>-<mimari> (macOS için önce -darwin-universal) ve aynı adla .sig.
package selfupdate

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"sshpocket-go/internal/trust"
)

const (
	DefaultRepo = "syhgorath/sshpocket"
	Namespace   = "sshpocket-release"
	Principal   = "sshpocket-release"
	maxBinary   = 200 << 20
)

var reVer = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// Updater: bağımlılıklar (testte değiştirilir).
type Updater struct {
	Repo         string
	SignersFile  string
	ExePath      string // güncellenecek ikili
	BackupDir    string // varsayılan: <ikilinin klasörü>/.update-backup
	Version      string // çalışan sürüm
	GOOS, GOARCH string
	Download     trust.DownloadFunc
	Head         func(url string) string           // Location başlığı; nil = ağ
	RunVersion   func(path string) (string, error) // yeni ikilinin bildirdiği sürüm; nil = `path version`
	In           io.Reader
	Out, Err     io.Writer

	rd *bufio.Reader
}

func (u *Updater) out(f string, a ...any) { fmt.Fprintf(u.Out, f, a...) }
func (u *Updater) err(f string, a ...any) { fmt.Fprintf(u.Err, f, a...) }

func (u *Updater) repo() string {
	if u.Repo != "" {
		return u.Repo
	}
	return DefaultRepo
}

func (u *Updater) confirm(q string) bool {
	if u.rd == nil {
		var r io.Reader = u.In
		if r == nil {
			r = strings.NewReader("")
		}
		u.rd = bufio.NewReader(r)
	}
	fmt.Fprint(u.Err, q+" (y/n) ")
	l, err := u.rd.ReadString('\n')
	if err != nil && l == "" {
		return false
	}
	a := strings.TrimSuffix(l, "\n")
	return a == "y" || a == "Y"
}

// Latest, update_latest_version: yönlendirme tam olarak bu reponun etiket adresi olmalı.
func (u *Updater) Latest() (string, bool) {
	head := u.Head
	if head == nil {
		return "", false
	}
	loc := head("https://github.com/" + u.repo() + "/releases/latest")
	if loc == "" {
		return "", false
	}
	tag := loc[strings.LastIndex(loc, "/")+1:]
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(tag) || loc != "https://github.com/"+u.repo()+"/releases/tag/"+tag {
		return "", false
	}
	return tag[1:], true
}

func (u *Updater) assetNames(ver string) []string {
	goos, arch := u.GOOS, u.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}
	var names []string
	if goos == "darwin" {
		names = append(names, fmt.Sprintf("sshpocket-%s-darwin-universal", ver))
	}
	return append(names, fmt.Sprintf("sshpocket-%s-%s-%s", ver, goos, arch))
}

func (u *Updater) runVersion(path string) (string, error) {
	if u.RunVersion != nil {
		return u.RunVersion(path)
	}
	o, err := exec.Command(path, "version").Output()
	return strings.TrimSpace(string(o)), err
}

// Run, update_main. checkOnly: yalnızca bak. Çıkış kodu: 0 güncel/başarılı, 10 yeni sürüm var (checkOnly), 1 hata/iptal.
func (u *Updater) Run(checkOnly bool) (code int, updated bool) {
	cur := u.Version
	if !reVer.MatchString(cur) {
		u.out("⚠️  Mevcut sürüm okunamadı (VERSION)\n")
		return 1, false
	}
	u.out("Mevcut sürüm: %s\n", cur)
	u.out("🔍 Son sürüm kontrol ediliyor (github.com/%s)...\n", u.repo())
	latest, ok := u.Latest()
	if !ok {
		u.out("⚠️  Son sürüm öğrenilemedi (ağ yok ya da yayınlanmış sürüm yok).\n")
		return 1, false
	}
	switch trust.VersionCmp(latest, cur) {
	case 0:
		u.out("✅ Güncelsiniz (%s).\n", cur)
		return 0, false
	case -1:
		u.out("ℹ️  Uzaktaki sürüm (%s) kurulu olandan eski; geri sürüme düşülmez.\n", latest)
		return 0, false
	}
	u.out("🆕 Yeni sürüm var: %s (mevcut: %s)\n", latest, cur)
	if checkOnly {
		return 10, false
	}
	if !u.confirm("İndirilip imzası doğrulansın mı?") {
		u.out("İptal edildi.\n")
		return 1, false
	}
	signersText, _ := os.ReadFile(u.SignersFile)
	signers := trust.ParseAllowedSigners(string(signersText))
	if len(signers) == 0 {
		u.err("imza anahtarı tanımlı değil (%s)\n", u.SignersFile)
		u.out("❌ İMZA DOĞRULANAMADI. Paket KURULMADI. (Sahte veya bozuk paket olabilir.)\n")
		return 1, false
	}
	work, err := os.MkdirTemp("", "sshpocket-update.")
	if err != nil {
		u.out("⚠️  Geçici klasör oluşturulamadı\n")
		return 1, false
	}
	defer os.RemoveAll(work)
	dl := u.Download
	if dl == nil {
		dl = trust.HTTPSDownload
	}
	base := "https://github.com/" + u.repo() + "/releases/download/v" + latest + "/"
	bin, sig := filepath.Join(work, "sshpocket.new"), filepath.Join(work, "sshpocket.new.sig")
	u.out("⬇️  İndiriliyor...\n")
	got := false
	for _, name := range u.assetNames(latest) {
		if dl(base+name, bin, maxBinary) == nil && dl(base+name+".sig", sig, 4096) == nil {
			got = true
			break
		}
	}
	if !got {
		u.out("⚠️  İndirme başarısız\n")
		return 1, false
	}
	u.out("🔐 İmza doğrulanıyor...\n")
	sigRaw, rerr := os.ReadFile(sig)
	f, oerr := os.Open(bin)
	if rerr != nil || oerr != nil || trust.Verify(signers, Principal, Namespace, sigRaw, f) != nil {
		if f != nil {
			f.Close()
		}
		u.out("❌ İMZA DOĞRULANAMADI. Paket KURULMADI. (Sahte veya bozuk paket olabilir.)\n")
		return 1, false
	}
	f.Close()
	u.out("✅ İmza geçerli.\n")
	if err := os.Chmod(bin, 0o755); err != nil {
		u.out("⚠️  Dosya izni ayarlanamadı\n")
		return 1, false
	}
	reported, verr := u.runVersion(bin)
	if verr != nil || reported != "sshpocket "+latest {
		u.out("❌ İkilinin bildirdiği sürüm (%s) istenenle (%s) uyuşmuyor, kurulmadı.\n", strings.TrimPrefix(reported, "sshpocket "), latest)
		return 1, false
	}
	u.out("\n──── Güncelleme özeti ────\n  %-9s %s → %s\n  %-9s ✅ doğrulandı\n───────────────────────────\n", "sshpocket", cur, latest, "imza")
	if !u.confirm("Kurulsun mu? (sunucularınız/anahtarlarınız ve components/ korunur, yedek alınır)") {
		u.out("İptal edildi.\n")
		return 1, false
	}
	backup, err := u.apply(bin, cur)
	if err != nil {
		u.err("%s\n", err)
		u.out("❌ Güncelleme başarısız.\n")
		return 1, false
	}
	u.out("💾 Yedek: %s\n\n✅ Güncellendi: %s → %s. sshpocket'ı yeniden başlatın.\n", backup, cur, latest)
	return 0, true
}

// apply: eski ikiliyi yedekler, yenisini atomik yerleştirir; hata olursa eskisini geri koyar.
func (u *Updater) apply(newBin, oldVer string) (string, error) {
	dir := filepath.Dir(u.ExePath)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("hedef yazılabilir değil: %s", dir)
	}
	tmp, err := os.CreateTemp(dir, ".sshpocket.new.")
	if err != nil {
		return "", fmt.Errorf("hedef yazılabilir değil (USB yazma korumalı olabilir): %s", dir)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	src, err := os.Open(newBin)
	if err != nil {
		tmp.Close()
		return "", err
	}
	_, cerr := io.Copy(tmp, src)
	src.Close()
	if e := tmp.Close(); cerr == nil {
		cerr = e
	}
	if cerr != nil {
		return "", fmt.Errorf("yeni ikili yazılamadı")
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return "", err
	}
	bdir := u.BackupDir
	if bdir == "" {
		bdir = filepath.Join(dir, ".update-backup")
	}
	if err := os.MkdirAll(bdir, 0o700); err != nil {
		return "", fmt.Errorf("yedek alınamadı")
	}
	backup := filepath.Join(bdir, "sshpocket-"+oldVer)
	if err := copyFile(u.ExePath, backup, 0o755); err != nil {
		return "", fmt.Errorf("yedek alınamadı")
	}
	if err := os.Rename(tmpName, u.ExePath); err != nil {
		os.Rename(backup, u.ExePath) // yerinde değişmediyse zararsız
		return "", fmt.Errorf("ikili yerleştirilemedi: %v", err)
	}
	return backup, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
