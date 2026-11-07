package trust

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// İmzalı paket indeksi (docs/package-index.md; Python karşılığı component_index.py, kabuk: ComponentsIndex.sh).
// İlkeler: sessiz otomatik kurulum yok; fail-closed; indeks yeni güven üretmez (imzacı KİLİTTEN gelir).

const (
	IndexNamespace   = "sshpocket-index"
	maxIndexBytes    = 1024 * 1024
	indexMaxDays     = 90
	indexSkew        = 86400
	defaultRequires  = "3.7.0"
	DefaultIndexBase = "https://raw.githubusercontent.com/syhgorath/sshpocket-index/main"
)

var (
	reIdxTime     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
	reIdxPlatform = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)
)

// IndexPkg, indeksteki bir paket girdisidir.
type IndexPkg struct {
	ID, Repo, Version, Asset, SHA256, Requires string
	Platforms                                  []string
	Yanked                                     bool
}

// Index, doğrulanmış indeks içeriğidir (paketler kimliğe göre sıralı).
type Index struct {
	Serial               int64
	IssuedAt, ValidUntil string
	Pkgs                 []IndexPkg
}

func (x *Index) Get(id string) (IndexPkg, bool) {
	for _, p := range x.Pkgs {
		if p.ID == id {
			return p, true
		}
	}
	return IndexPkg{}, false
}

// ParseError çıkış kodlarıyla: 1 biçim, 3 süresi dolmuş, 4 gelecekte imzalanmış.
type ParseError struct {
	Msg  string
	Code int
}

func (e *ParseError) Error() string { return e.Msg }

func bad(format string, a ...any) *ParseError { return &ParseError{fmt.Sprintf(format, a...), 1} }

// strictDecode, yinelenen anahtarları ve NaN/Infinity'yi reddeden JSON okuyucudur (nesneler map[string]any).
func strictDecode(raw []byte) (any, *ParseError) {
	if !utf8.Valid(raw) {
		return nil, bad("JSON okunamadı")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, perr := strictValue(dec)
	if perr != nil {
		return nil, perr
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, bad("JSON okunamadı")
	}
	return v, nil
}

func strictValue(dec *json.Decoder) (any, *ParseError) {
	tok, err := dec.Token()
	if err != nil {
		return nil, bad("JSON okunamadı")
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			m := map[string]any{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, bad("JSON okunamadı")
				}
				k, ok := kt.(string)
				if !ok {
					return nil, bad("JSON okunamadı")
				}
				v, perr := strictValue(dec)
				if perr != nil {
					return nil, perr
				}
				if _, dup := m[k]; dup {
					return nil, bad("yinelenen anahtar: %s", k)
				}
				m[k] = v
			}
			if _, err := dec.Token(); err != nil {
				return nil, bad("JSON okunamadı")
			}
			return m, nil
		case '[':
			l := []any{}
			for dec.More() {
				v, perr := strictValue(dec)
				if perr != nil {
					return nil, perr
				}
				l = append(l, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, bad("JSON okunamadı")
			}
			return l, nil
		}
		return nil, bad("JSON okunamadı")
	case json.Number:
		s := t.String()
		if strings.ContainsAny(s, ".eE") {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, bad("JSON okunamadı")
			}
			return f, nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			f, ferr := strconv.ParseFloat(s, 64)
			if ferr != nil {
				return nil, bad("JSON okunamadı")
			}
			return f, nil
		}
		return n, nil
	default:
		return tok, nil
	}
}

// idxEpoch, "YYYY-MM-DDTHH:MM:SSZ" biçimini UTC epoch'a çevirir (saniye 60/61 Python gibi kabul edilir).
func idxEpoch(v any, what string) (int64, *ParseError) {
	s, ok := v.(string)
	if !ok || !reIdxTime.MatchString(s) {
		return 0, bad("%s biçimi geçersiz (YYYY-MM-DDTHH:MM:SSZ bekleniyor)", what)
	}
	y, _ := strconv.Atoi(s[0:4])
	mo, _ := strconv.Atoi(s[5:7])
	d, _ := strconv.Atoi(s[8:10])
	h, _ := strconv.Atoi(s[11:13])
	mi, _ := strconv.Atoi(s[14:16])
	se, _ := strconv.Atoi(s[17:19])
	if y < 1 || mo < 1 || mo > 12 || d < 1 || h > 23 || mi > 59 || se > 61 {
		return 0, bad("%s geçerli bir tarih değil", what)
	}
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	if t.Day() != d || int(t.Month()) != mo {
		return 0, bad("%s geçerli bir tarih değil", what)
	}
	return time.Date(y, time.Month(mo), d, h, mi, se, 0, time.UTC).Unix(), nil
}

func asInt64(v any) (int64, bool) { n, ok := v.(int64); return n, ok }

// ParseIndex, imzası ÖNCEDEN doğrulanmış index.json'ı şema, süre ve içerik kurallarıyla doğrular.
func ParseIndex(raw []byte, now int64) (*Index, *ParseError) {
	if len(raw) > maxIndexBytes {
		return nil, bad("indeks 1 MiB sınırını aşıyor")
	}
	dv, perr := strictDecode(raw)
	if perr != nil {
		return nil, perr
	}
	d, ok := dv.(map[string]any)
	if !ok {
		return nil, bad("üst düzey değer nesne olmalı")
	}
	schema, ok := asInt64(d["schema"])
	if !ok {
		return nil, bad("schema yok")
	}
	if schema != 1 {
		return nil, bad("bu sshpocket indeks schema %d sürümünü anlamıyor; sshpocket'i güncelleyin", schema)
	}
	serial, ok := asInt64(d["serial"])
	if !ok || serial < 1 {
		return nil, bad("serial >= 1 olmalı")
	}
	issued, perr := idxEpoch(d["issued_at"], "issued_at")
	if perr != nil {
		return nil, perr
	}
	valid, perr := idxEpoch(d["valid_until"], "valid_until")
	if perr != nil {
		return nil, perr
	}
	if !(issued < valid && valid <= issued+indexMaxDays*86400) {
		return nil, bad("valid_until, issued_at'ten sonra ve en çok %d gün içinde olmalı", indexMaxDays)
	}
	if issued > now+indexSkew {
		return nil, &ParseError{"indeks gelecekte imzalanmış görünüyor (yerel saati kontrol edin)", 4}
	}
	if now > valid {
		return nil, &ParseError{fmt.Sprintf("indeksin süresi dolmuş (%s)", d["valid_until"]), 3}
	}
	pk, ok := d["packages"].(map[string]any)
	if !ok {
		return nil, bad("packages nesne olmalı")
	}
	idx := &Index{Serial: serial, IssuedAt: d["issued_at"].(string), ValidUntil: d["valid_until"].(string)}
	ids := make([]string, 0, len(pk))
	for k := range pk {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	for _, pid := range ids {
		if !IDValid(pid) {
			return nil, bad("geçersiz paket kimliği: %s", pid)
		}
		e, ok := pk[pid].(map[string]any)
		if !ok {
			return nil, bad("%s: girdi nesne olmalı", pid)
		}
		repo, rok := e["repo"].(string)
		if !(rok && reRepo.MatchString(repo) && !strings.Contains(repo, "..")) {
			return nil, bad("%s: repo geçersiz", pid)
		}
		ver, vok := e["version"].(string)
		if !(vok && reVersion.MatchString(ver)) {
			return nil, bad("%s: version geçersiz", pid)
		}
		if asset, _ := e["asset"].(string); asset != pid+"-"+ver+".tar.gz" {
			return nil, bad("%s: asset adı '%s-%s.tar.gz' olmalı", pid, pid, ver)
		}
		sha, sok := e["sha256"].(string)
		if !(sok && reSHA256.MatchString(sha)) {
			return nil, bad("%s: sha256 geçersiz", pid)
		}
		req := defaultRequires
		if rv, present := e["requires_sshpocket"]; present {
			s, ok := rv.(string)
			if !(ok && reVersion.MatchString(s)) {
				return nil, bad("%s: requires_sshpocket geçersiz", pid)
			}
			req = s
		}
		plats := []string{}
		if pv, present := e["platforms"]; present {
			l, ok := pv.([]any)
			if !ok {
				return nil, bad("%s: platforms geçersiz", pid)
			}
			for _, p := range l {
				s, ok := p.(string)
				if !ok || !reIdxPlatform.MatchString(s) {
					return nil, bad("%s: platforms geçersiz", pid)
				}
				plats = append(plats, s)
			}
		}
		yanked := false
		if yv, present := e["yanked"]; present {
			b, ok := yv.(bool)
			if !ok {
				return nil, bad("%s: yanked bool olmalı", pid)
			}
			yanked = b
		}
		idx.Pkgs = append(idx.Pkgs, IndexPkg{pid, repo, ver, pid + "-" + ver + ".tar.gz", sha, req, plats, yanked})
	}
	return idx, nil
}

// Lines, Python betiğinin çıktısı gibi "meta|…" / "pkg|…" satırlarını üretir (karşılaştırma ve günlük için).
func (x *Index) Lines() []string {
	out := []string{fmt.Sprintf("meta|%d|%s|%s", x.Serial, x.IssuedAt, x.ValidUntil)}
	for _, p := range x.Pkgs {
		y := 0
		if p.Yanked {
			y = 1
		}
		out = append(out, fmt.Sprintf("pkg|%s|%s|%s|%s|%s|%s|%s|%d", p.ID, p.Repo, p.Version, p.Asset, p.SHA256, p.Requires, strings.Join(p.Platforms, ","), y))
	}
	return out
}

// ---------------------------------------------------------------- istemci: durum, yenileme, karar

// DownloadFunc, url'yi dest'e indirir (en çok maxBytes); testte değiştirilebilir. Varsayılan yalnızca HTTPS.
type DownloadFunc func(url, dest string, maxBytes int64) error

// Client, indeks istemcisinin bağımlılıklarıdır.
type Client struct {
	ComponentsDir string // components/ (indeks önbelleği .index/ altında)
	SignersFile   string // sistem/index_signers
	BaseURL       string // boşsa DefaultIndexBase
	SSHPocketVer  string // çalışan sshpocket sürümü
	Now           func() int64
	Download      DownloadFunc
}

func (c *Client) now() int64 {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now().Unix()
}

func (c *Client) dir() string { return filepath.Join(c.ComponentsDir, ".index") }

func (c *Client) base() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultIndexBase
}

var reSignerLine = regexp.MustCompile(`^[^#\s]`)

// Enabled, index_signers'ta en az bir (yorum olmayan) satır var mı.
func (c *Client) Enabled() bool {
	b, err := os.ReadFile(c.SignersFile)
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(b), "\n") {
		if reSignerLine.MatchString(l) {
			return true
		}
	}
	return false
}

func (c *Client) verifySig(indexFile, sigFile string) bool {
	st, err := os.ReadFile(c.SignersFile)
	if err != nil {
		return false
	}
	sig, err := os.ReadFile(sigFile)
	if err != nil {
		return false
	}
	f, err := os.Open(indexFile)
	if err != nil {
		return false
	}
	defer f.Close()
	return Verify(ParseAllowedSigners(string(st)), IndexNamespace, IndexNamespace, sig, f) == nil
}

func (c *Client) stateGet(key string) string {
	b, err := os.ReadFile(filepath.Join(c.dir(), "state"))
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, key+"=") {
			v := strings.Join(strings.Fields(strings.TrimPrefix(l, key+"=")), "")
			switch key {
			case "serial":
				if regexp.MustCompile(`^[0-9]+$`).MatchString(v) {
					return v
				}
			case "sha256":
				if reSHA256.MatchString(v) {
					return v
				}
			}
			return ""
		}
	}
	return ""
}

// ErrDisabled: indeks özelliği kapalı (hata sayılmaz).
var ErrDisabled = errors.New("paket indeksi kapalı (sistem/index_signers boş)")

// Refresh: indir → imza → ayrıştır/şema/süre → seri → durumu ilerlet. Hata durumunda hiçbir dosya değişmez (fail-closed).
func (c *Client) Refresh() (*Index, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	dl := c.Download
	if dl == nil {
		dl = HTTPSDownload
	}
	tmp, err := os.MkdirTemp("", "sshpocket-index.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	base := c.base()
	ix, sg := filepath.Join(tmp, "index.json"), filepath.Join(tmp, "index.json.sig")
	if dl(base+"/index.json", ix, maxIndexBytes) != nil || dl(base+"/index.json.sig", sg, 4096) != nil {
		return nil, fmt.Errorf("indeks alınamadı: %s", base)
	}
	if !c.verifySig(ix, sg) {
		return nil, errors.New("indeks imzası geçersiz; güncelleme yapılmadı")
	}
	raw, err := os.ReadFile(ix)
	if err != nil {
		return nil, err
	}
	idx, perr := ParseIndex(raw, c.now())
	if perr != nil {
		return nil, errors.New(perr.Msg + "\nindeks reddedildi; güncelleme yapılmadı")
	}
	sha, _ := FileSHA256(ix)
	stSerial, stSHA := c.stateGet("serial"), c.stateGet("sha256")
	if stSerial != "" {
		ss, _ := strconv.ParseInt(stSerial, 10, 64)
		if idx.Serial < ss {
			return nil, fmt.Errorf("eski indeks (seri %d < %d): geri alma saldırısı olabilir; reddedildi", idx.Serial, ss)
		}
		if idx.Serial == ss && stSHA != "" && sha != stSHA {
			return nil, fmt.Errorf("aynı seri numarası (seri %d) iki farklı içerikle yayımlanmış: reddedildi", ss)
		}
	}
	dir := c.dir()
	wfail := fmt.Errorf("indeks durumu yazılamadı (%s); güncelleme yapılmadı", dir)
	if os.MkdirAll(dir, 0o755) != nil {
		return nil, wfail
	}
	sigRaw, _ := os.ReadFile(sg)
	files := []struct {
		name string
		data []byte
	}{{"index.json", raw}, {"index.json.sig", sigRaw}, {"state", []byte(fmt.Sprintf("serial=%d\nsha256=%s\n", idx.Serial, sha))}}
	for _, f := range files {
		if os.WriteFile(filepath.Join(dir, f.name+".tmp"), f.data, 0o644) != nil {
			return nil, wfail
		}
	}
	for _, f := range files {
		if os.Rename(filepath.Join(dir, f.name+".tmp"), filepath.Join(dir, f.name)) != nil {
			return nil, wfail
		}
	}
	return idx, nil
}

// LoadCached: son kabul edilen kopyayı yeniden doğrular (imza + süre + durumla aynı seri); ağ yok.
func (c *Client) LoadCached() (*Index, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	dir := c.dir()
	ix, sg := filepath.Join(dir, "index.json"), filepath.Join(dir, "index.json.sig")
	if _, err := os.Stat(ix); err != nil {
		return nil, os.ErrNotExist
	}
	if _, err := os.Stat(sg); err != nil {
		return nil, os.ErrNotExist
	}
	if !c.verifySig(ix, sg) {
		return nil, errors.New("önbellekteki indeksin imzası geçersiz")
	}
	raw, _ := os.ReadFile(ix)
	idx, perr := ParseIndex(raw, c.now())
	if perr != nil {
		return nil, perr
	}
	if strconv.FormatInt(idx.Serial, 10) != c.stateGet("serial") {
		return nil, errors.New("önbellekteki indeks durumla uyuşmuyor")
	}
	return idx, nil
}

// Verdict, bir paketin indeks karşısındaki kararıdır.
type Verdict struct {
	Kind   string // unknown | yanked | needs-sshpocket | other-repo | below-floor | conflict | upgrade | install | current
	Detail string
}

// ReadVersionFile, sshpocket'in VERSION dosyasından sürümü okur (boşluklar atılır); yoksa boş.
func ReadVersionFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(string(b)), "")
}

// InstalledVersion, components/<id>/current içindeki etkin sürümdür; yoksa boş.
func InstalledVersion(componentsDir, id string) string {
	if !IDValid(id) {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(componentsDir, id, "current"))
	if err != nil {
		return ""
	}
	first := strings.SplitN(string(b), "\n", 2)[0]
	v := strings.Join(strings.Fields(first), "")
	if VersionValid(v) {
		return v
	}
	return ""
}

// Eval, comp_index_eval karşılığıdır.
func (c *Client) Eval(idx *Index, lock *Lock, id string) Verdict {
	p, ok := idx.Get(id)
	l, lok := lock.Get(id)
	if !ok || p.Version == "" || !lok {
		return Verdict{"unknown", ""}
	}
	have := InstalledVersion(c.ComponentsDir, id)
	me := c.SSHPocketVer
	switch {
	case p.Yanked:
		return Verdict{"yanked", p.Version}
	case me != "" && VersionValid(me) && VersionCmp(p.Requires, me) == 1:
		return Verdict{"needs-sshpocket", p.Requires}
	case p.Repo != l.Repo:
		return Verdict{"other-repo", p.Repo}
	case VersionCmp(p.Version, l.Version) == -1:
		return Verdict{"below-floor", p.Version}
	case p.Version == l.Version && p.SHA256 != l.SHA256:
		return Verdict{"conflict", p.Version}
	case have == "":
		return Verdict{"install", p.Version}
	case VersionCmp(p.Version, have) == 1:
		return Verdict{"upgrade", p.Version}
	}
	return Verdict{"current", p.Version}
}

// Row, rapor satırıdır.
type Row struct {
	ID, Installed string
	Verdict
}

// Report, indeksteki her paket için (kilitte olanlar) karar satırı verir; unknown atlanır.
func (c *Client) Report(idx *Index, lock *Lock) []Row {
	var rows []Row
	for _, p := range idx.Pkgs {
		v := c.Eval(idx, lock, p.ID)
		if v.Kind == "unknown" {
			continue
		}
		rows = append(rows, Row{p.ID, InstalledVersion(c.ComponentsDir, p.ID), v})
	}
	return rows
}

// Text, kararın insan okunur satırıdır (comp_index_verdict_text).
func (v Verdict) Text() string {
	switch v.Kind {
	case "upgrade":
		return fmt.Sprintf("⬆️  yükseltilebilir → %s", v.Detail)
	case "install":
		return fmt.Sprintf("📥 kurulabilir → %s (kurmak için: upgrade <paket>)", v.Detail)
	case "current":
		return "✅ güncel"
	case "yanked":
		return fmt.Sprintf("🚫 indeks %s sürümünü önermiyor (yanked)", v.Detail)
	case "needs-sshpocket":
		return fmt.Sprintf("⏸️  sshpocket %s gerekli", v.Detail)
	case "other-repo":
		return fmt.Sprintf("❌ indeks başka depo gösteriyor (%s); yok sayıldı", v.Detail)
	case "below-floor":
		return fmt.Sprintf("⚠️  indeks sürümü (%s) kilit tabanının altında; yok sayıldı", v.Detail)
	case "conflict":
		return fmt.Sprintf("❌ indeks ve kilit aynı sürümde çelişiyor (%s); yükseltilmez", v.Detail)
	}
	return "–"
}
