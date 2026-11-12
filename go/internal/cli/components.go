// Package cli: `--components list | sync | update | upgrade | check-new | prune | validate` (ComponentsCLI.sh, ComponentsIndex.sh
// comp_cli_update/upgrade). Gerçek iş trust (imza/kilit/kurulum/indeks), manifest (denetim) ve report paketlerindedir; burası
// komut akışı, onaylar ve çıktı metinleridir. Çıktı metinleri kabuk motoruyla aynıdır.
package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"keylab/internal/manifest"
	"keylab/internal/report"
	"keylab/internal/trust"
)

// Components: komutların bağımlılıkları.
type Components struct {
	Dir          string // components/
	LockPath     string
	ServersDir   string
	SignersFile  string // sistem/index_signers (indeks açık mı)
	IndexBase    string
	SSHPocketVer string
	Download     trust.DownloadFunc
	Head         func(url string) string // Location başlığı (check-new); nil = ağ
	JSON         func(url string) string // GitHub API gövdesi (check-new); nil = ağ
	Only         string                  // sync: yalnız bu paket (menü); boş = hepsi
	In           io.Reader
	Out, Err     io.Writer
	Now          func() time.Time

	rd *bufio.Reader
}

func (c *Components) out(format string, a ...any) { fmt.Fprintf(c.Out, format, a...) }
func (c *Components) err(format string, a ...any) { fmt.Fprintf(c.Err, format, a...) }

func (c *Components) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Components) client() *trust.Client {
	cl := &trust.Client{ComponentsDir: c.Dir, SignersFile: c.SignersFile, BaseURL: c.IndexBase, SSHPocketVer: c.SSHPocketVer, Download: c.Download}
	return cl
}

// confirm, _comp_confirm: istemi Stderr'e yazar (kabukta read -p tty'de aynı); y/Y/e/E kabul.
func (c *Components) confirm(prompt string) bool {
	if c.rd == nil {
		var r io.Reader = c.In
		if r == nil {
			r = strings.NewReader("")
		}
		c.rd = bufio.NewReader(r)
	}
	fmt.Fprint(c.Err, prompt+" [y/N] ")
	l, err := c.rd.ReadString('\n')
	if err != nil && l == "" {
		return false
	}
	switch strings.TrimSuffix(l, "\n") {
	case "y", "Y", "e", "E":
		return true
	}
	return false
}

func (c *Components) lock() (*trust.Lock, bool) {
	l, err := trust.LoadLock(c.LockPath)
	if err != nil {
		c.err("%s\n", err)
		return nil, false
	}
	return l, true
}

// Status, comp_status: ok | missing | outdated | ahead | broken.
func (c *Components) Status(lock *trust.Lock, id string) string {
	e, _ := lock.Get(id)
	have := trust.InstalledVersion(c.Dir, id)
	if have == "" {
		return "missing"
	}
	switch trust.VersionCmp(have, e.Version) {
	case -1:
		return "outdated"
	case 1:
		if trust.VerifyInstalled(c.Dir, id, have) == nil {
			return "ahead"
		}
		return "broken"
	}
	if trust.VerifyInstalled(c.Dir, id, have) == nil {
		return "ok"
	}
	return "broken"
}

// List, comp_cli_list.
func (c *Components) List() int {
	lock, ok := c.lock()
	if !ok {
		return 1
	}
	cl := c.client()
	idx, ierr := cl.LoadCached()
	if ierr != nil && ierr != trust.ErrDisabled && !errors.Is(ierr, fs.ErrNotExist) {
		c.out("ℹ️  indeks önbelleği kullanılamıyor: %s\n", strings.SplitN(ierr.Error(), "\n", 2)[0])
	}
	ids := lock.IDs()
	if len(ids) == 0 {
		c.out("Kilit dosyasında paket yok.\n")
		return 0
	}
	c.out("Paketler (components.lock)\n")
	for _, id := range ids {
		e, _ := lock.Get(id)
		have := trust.InstalledVersion(c.Dir, id)
		var mark string
		switch c.Status(lock, id) {
		case "ok":
			mark = "✅ kurulu"
		case "missing":
			mark = "⬇️  kurulmamış"
		case "outdated":
			mark = "⬆️  güncellenmeli"
		case "ahead":
			mark = "✅ kilitten yeni (indeksle yükseltilmiş)"
		case "broken":
			mark = "❌ dosyalar değişmiş (onarım gerekir)"
		}
		hv := have
		if hv == "" {
			hv = "-"
		}
		c.out("  %-20s kilit %-8s kurulu %-8s %s\n", id, e.Version, hv, mark)
		if ierr == nil && idx != nil {
			v := cl.Eval(idx, lock, id)
			if v.Kind != "unknown" && v.Kind != "current" {
				c.out("  %-20s indeks: %s\n", "", v.Text())
			}
		}
	}
	return 0
}

func quarantine(dir, id, ver string, stamp string) {
	base := filepath.Join(dir, id)
	if st, err := os.Stat(filepath.Join(base, ver)); err == nil && st.IsDir() {
		os.Rename(filepath.Join(base, ver), filepath.Join(base, ver+".bozuk-"+stamp))
	}
	if st, err := os.Stat(filepath.Join(base, ver+".files")); err == nil && st.Mode().IsRegular() {
		os.Rename(filepath.Join(base, ver+".files"), filepath.Join(base, ver+".files.bozuk-"+stamp))
	}
	os.Remove(filepath.Join(base, "current"))
}

func (c *Components) reportChanged(ids []string) {
	for _, id := range ids {
		c.out("\n")
		o := report.Options{ComponentsDir: c.Dir, ServersDir: c.ServersDir}
		report.Pkg(c.Out, c.Err, o, id, report.PrevVersion(o, id))
	}
}

// Sync, comp_cli_sync: kilitteki sürümü kurar/onarır.
func (c *Components) Sync(yes bool) int {
	lock, ok := c.lock()
	if !ok {
		return 1
	}
	rc := 0
	var reported []string
	for _, id := range lock.IDs() {
		if c.Only != "" && id != c.Only {
			continue
		}
		e, _ := lock.Get(id)
		want := e.Version
		switch c.Status(lock, id) {
		case "ok":
			continue
		case "ahead":
			c.out("⚠️  %s: kurulu sürüm kilitten yeni; dokunulmadı.\n", id)
			continue
		case "missing", "outdated":
			if !yes && !c.confirm(fmt.Sprintf("%s %s indirilip imzası doğrulanarak kurulsun mu?", id, want)) {
				c.out("Atlandı: %s\n", id)
				rc = 1
				continue
			}
		case "broken":
			c.out("❌ %s: kurulu dosyalar değişmiş (kurulumdan sonra düzenlenmiş olabilir).\n", id)
			if !yes && !c.confirm(fmt.Sprintf("Bozuk kurulum kenara alınıp %s %s yeniden kurulsun mu?", id, want)) {
				c.out("Atlandı: %s\n", id)
				rc = 1
				continue
			}
			quarantine(c.Dir, id, trust.InstalledVersion(c.Dir, id), c.now().Format("20060102-150405"))
		}
		oldv := trust.InstalledVersion(c.Dir, id)
		msg, err := trust.Install(c.Dir, lock, id, c.Download)
		if err != nil {
			c.err("%s\n", err)
			c.out("⚠️  %s kurulamadı\n", id)
			rc = 1
			continue
		}
		c.out("%s\n", msg)
		c.out("✅ %s %s\n", id, want)
		if oldv != "" && oldv != want {
			reported = append(reported, id)
		}
	}
	c.reportChanged(reported)
	return rc
}

var reTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func (c *Components) latestVersion(repo string) (string, bool) {
	if !trust.RepoValid(repo) {
		return "", false
	}
	head := c.Head
	if head == nil {
		head = httpHead
	}
	if loc := head("https://github.com/" + repo + "/releases/latest"); loc != "" {
		i := strings.LastIndex(loc, "/")
		tag := loc[i+1:]
		if !reTag.MatchString(tag) || loc != "https://github.com/"+repo+"/releases/tag/"+tag {
			return "", false
		}
		return tag[1:], true
	}
	js := c.JSON
	if js == nil {
		js = httpJSON
	}
	body := js("https://api.github.com/repos/" + repo + "/releases?per_page=1")
	var rel []struct {
		Tag string `json:"tag_name"`
	}
	if json.Unmarshal([]byte(body), &rel) != nil || len(rel) == 0 || !reTag.MatchString(rel[0].Tag) {
		return "", false
	}
	return rel[0].Tag[1:], true
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// HTTPHead: HTTPS başlık isteği; Location değerini verir (yönlendirme izlenmez).
func HTTPHead(url string) string { return httpHead(url) }

func httpHead(url string) string {
	if !strings.HasPrefix(url, "https://") {
		return ""
	}
	resp, err := httpClient().Head(url)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	return resp.Header.Get("Location")
}

func httpJSON(url string) string {
	if !strings.HasPrefix(url, "https://") {
		return ""
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return string(b)
}

// CheckNew, comp_cli_check_new: yalnızca bilgi.
func (c *Components) CheckNew() int {
	lock, ok := c.lock()
	if !ok {
		return 1
	}
	ids := lock.IDs()
	if len(ids) == 0 {
		c.out("Kilit dosyasında paket yok.\n")
		return 0
	}
	for _, id := range ids {
		e, _ := lock.Get(id)
		latest, ok := c.latestVersion(e.Repo)
		if !ok {
			c.out("ℹ️  %s: yayın bilgisi alınamadı\n", id)
			continue
		}
		if trust.VersionCmp(latest, e.Version) == 1 {
			c.out("🆕 %s: %s yayınlanmış (sshpocket'ın kilidi: %s).\n", id, latest, e.Version)
			c.out("   Yeni sürüm, yeni bir sshpocket sürümüyle (kilit güncellenince) gelir; şimdi kurulmaz.\n")
		} else {
			c.out("✅ %s: güncel (%s)\n", id, e.Version)
		}
	}
	return 0
}

// Prune, comp_cli_prune.
func (c *Components) Prune(args []string) int {
	yes, dry := false, false
	for _, a := range args {
		switch a {
		case "--yes":
			yes = true
		case "--dry-run":
			dry = true
		default:
			c.err("kullanım: --components prune [--yes] [--dry-run]\n")
			return 2
		}
	}
	ents, _ := os.ReadDir(c.Dir)
	var ids []string
	for _, e := range ents {
		li, err := os.Lstat(filepath.Join(c.Dir, e.Name()))
		if err != nil || !li.IsDir() || li.Mode()&os.ModeSymlink != 0 || !trust.IDValid(e.Name()) {
			continue
		}
		ids = append(ids, e.Name())
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		c.out("Kurulu paket yok.\n")
		return 0
	}
	lock, _ := trust.LoadLock(c.LockPath)
	type rm struct{ id, v string }
	var removals []rm
	for _, id := range ids {
		base := filepath.Join(c.Dir, id)
		cur := trust.InstalledVersion(c.Dir, id)
		if cur == "" {
			c.out("  %s: etkin sürüm (current) yok; dokunulmadı\n", id)
			continue
		}
		lockv := ""
		if lock != nil {
			if e, ok := lock.Get(id); ok {
				lockv = e.Version
			}
		}
		vers := versionDirs(base)
		prev := ""
		for _, v := range vers {
			if trust.VersionCmp(v, cur) == -1 && (prev == "" || trust.VersionCmp(v, prev) == 1) {
				prev = v
			}
		}
		var rmList []string
		keep := 0
		for _, v := range vers {
			if v == cur || v == prev || v == lockv {
				keep++
			} else {
				rmList = append(rmList, v)
			}
		}
		if len(rmList) == 0 {
			c.out("  %s: temiz (kalan %d sürüm; etkin %s)\n", id, keep, cur)
			continue
		}
		pv, lv := prev, lockv
		if pv == "" {
			pv = "-"
		}
		if lv == "" {
			lv = "-"
		}
		c.out("  %s: etkin %s, önceki %s, kilit %s KALIR; silinecek: %s \n", id, cur, pv, lv, strings.Join(rmList, " "))
		for _, v := range rmList {
			removals = append(removals, rm{id, v})
		}
	}
	if len(removals) == 0 {
		c.out("Silinecek eski sürüm yok.\n")
		return 0
	}
	if dry {
		c.out("(--dry-run: hiçbir şey silinmedi)\n")
		return 0
	}
	if !yes && !c.confirm(fmt.Sprintf("%d eski sürüm klasörü silinsin mi?", len(removals))) {
		c.out("Atlandı.\n")
		return 1
	}
	for _, r := range removals {
		base := filepath.Join(c.Dir, r.id)
		li, err := os.Lstat(filepath.Join(base, r.v))
		if err != nil || !li.IsDir() || li.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if os.RemoveAll(filepath.Join(base, r.v)) == nil {
			os.Remove(filepath.Join(base, r.v+".files"))
			c.out("silindi: %s %s\n", r.id, r.v)
		}
	}
	return 0
}

// versionDirs: <X.Y.Z> adlı, bağlantı olmayan klasörler (sürüm sırasında, ad sırasıyla).
func versionDirs(base string) []string {
	ents, _ := os.ReadDir(base)
	var out []string
	for _, e := range ents {
		li, err := os.Lstat(filepath.Join(base, e.Name()))
		if err != nil || !li.IsDir() || li.Mode()&os.ModeSymlink != 0 || !trust.VersionValid(e.Name()) {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// Update, comp_cli_update: imzalı indeksi yeniler ve paket başına kararı yazar.
func (c *Components) Update() int {
	if _, ok := c.lock(); !ok {
		return 1
	}
	lock, _ := trust.LoadLock(c.LockPath)
	cl := c.client()
	idx, err := cl.Refresh()
	if err == trust.ErrDisabled {
		c.out("ℹ️  paket indeksi kapalı (sistem/index_signers boş).\n")
		return 0
	}
	if err != nil {
		c.err("%s\n", err)
		return 1
	}
	c.out("Paket indeksi (seri %d, geçerlilik: %s)\n", idx.Serial, idx.ValidUntil)
	up, inst := c.indexReport(cl, idx, lock)
	if len(up) > 0 {
		c.out("\nKurulu paketleri yükseltmek için: keylab components upgrade\n")
	}
	if len(inst) > 0 {
		c.out("\nKurulu olmayan paketi kurmak için adını verin: keylab components upgrade <paket>\n")
	}
	return 0
}

func (c *Components) indexReport(cl *trust.Client, idx *trust.Index, lock *trust.Lock) (upgradable, installable []string) {
	for _, r := range cl.Report(idx, lock) {
		have := r.Installed
		if have == "" {
			have = "-"
		}
		c.out("  %-20s kurulu %-8s %s\n", r.ID, have, r.Verdict.Text())
		switch r.Kind {
		case "upgrade":
			upgradable = append(upgradable, r.ID)
		case "install":
			installable = append(installable, r.ID)
		}
	}
	return
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// Upgrade, comp_cli_upgrade: adsız çağrı yalnız KURULU paketleri yükseltir; kurulu olmayan yalnız adı verilirse kurulur.
func (c *Components) Upgrade(args []string) int {
	yes := false
	var want []string
	for _, a := range args {
		switch {
		case a == "--yes":
			yes = true
		case strings.HasPrefix(a, "-"):
			c.err("kullanım: --components upgrade [--yes] [paket…]\n")
			return 2
		default:
			if !trust.IDValid(a) {
				c.err("geçersiz paket kimliği: %s\n", a)
				return 2
			}
			want = append(want, a)
		}
	}
	if _, ok := c.lock(); !ok {
		return 1
	}
	lock, _ := trust.LoadLock(c.LockPath)
	cl := c.client()
	idx, err := cl.Refresh()
	if err == trust.ErrDisabled {
		c.out("ℹ️  paket indeksi kapalı (sistem/index_signers boş).\n")
		return 0
	}
	if err != nil {
		c.err("%s\n", err)
		return 1
	}
	up, inst := c.indexReport(cl, idx, lock)
	cands := append([]string{}, up...)
	for _, id := range want {
		if contains(inst, id) {
			cands = append(cands, id)
		}
		if !contains(cands, id) {
			c.out("ℹ️  %s: indekste kurulabilir ya da yükseltilebilir sürüm yok.\n", id)
		}
	}
	if len(cands) == 0 {
		c.out("Yükseltilebilir paket yok.\n")
		if len(inst) > 0 {
			c.out("Kurulu olmayan paketi kurmak için adını verin: upgrade <paket>\n")
		}
		return 0
	}
	ok, skipped, failed := 0, 0, 0
	var reported []string
	for _, id := range cands {
		if len(want) > 0 && !contains(want, id) {
			continue
		}
		v := cl.Eval(idx, lock, id)
		oldv := trust.InstalledVersion(c.Dir, id)
		ov := oldv
		if ov == "" {
			ov = "kurulu değil"
		}
		p, _ := idx.Get(id)
		if !yes && !c.confirm(fmt.Sprintf("%s %s → %s indirilip imzası doğrulanarak kurulsun mu? (kaynak: %s)", id, ov, v.Detail, p.Repo)) {
			c.out("Atlandı: %s\n", id)
			skipped++
			continue
		}
		msg, ierr := trust.InstallFromIndex(c.Dir, lock, idx, id, c.Download)
		if ierr != nil {
			c.err("%s\n", ierr)
			c.out("⚠️  %s kurulamadı (eski sürüm olduğu gibi duruyor)\n", id)
			failed++
			continue
		}
		c.out("%s\n", msg)
		newv := trust.InstalledVersion(c.Dir, id)
		c.out("✅ %s %s\n", id, newv)
		ok++
		if oldv != "" && oldv != newv {
			reported = append(reported, id)
		}
	}
	c.reportChanged(reported)
	c.out("yükseltilen: %d, atlanan: %d, hata: %d\n", ok, skipped, failed)
	if failed == 0 && skipped == 0 {
		return 0
	}
	return 1
}

// Validate, comp_cli_validate: statik paket denetimi.
func (c *Components) Validate(args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		c.err("kullanım: keylab components validate <kurulu-paket-kimliği | dizin | arşiv.tar.gz> [--strict]\n")
		return 2
	}
	target, rest := args[0], args[1:]
	strict, unsigned := false, false
	for _, a := range rest {
		switch a {
		case "--strict":
			strict = true
		case "--unsigned":
			unsigned = true
		}
	}
	if _, err := os.Stat(target); err != nil && trust.IDValid(target) {
		ver := trust.InstalledVersion(c.Dir, target)
		if ver == "" {
			c.err("kurulu paket yok ve '%s' bir dizin/arşiv değil\n", target)
			return 2
		}
		target = filepath.Join(c.Dir, target, ver)
	}
	lines, code := manifest.ValidatePackage(target, strict, unsigned)
	for _, l := range lines {
		c.out("%s\n", l)
	}
	return code
}
