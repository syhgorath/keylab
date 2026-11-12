// Veri okuyucular: sunucular, paketler, günlükler, Lynis, bulgular (salt okunur) ve ortak türler.
// (main.go'dan mekanik olarak bölündü; davranış değişmedi.)
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"keylab/internal/findings"
	"keylab/internal/servers"
	"keylab/internal/settings"
	"keylab/internal/trust"
)

const (
	cookieName  = "sshpocket_oturum"
	maxLogBytes = 1 << 20
)

var (
	reHost = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,253}$`)
	reUser = regexp.MustCompile(`^[a-z_][a-z0-9_.-]{0,31}$`)
	rePort = regexp.MustCompile(`^[0-9]{1,5}$`)
	reName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	reFile = regexp.MustCompile(`^(logs/)?[A-Za-z0-9._-]{1,128}\.log$`)
)

type Action struct {
	Time    string `json:"time"` // UTC, ISO 8601
	Package string `json:"package"`
	Version string `json:"version"`
	Action  string `json:"action"`
	Result  string `json:"result"`
}

type Lynis struct {
	Score       int      `json:"score"`
	Warnings    int      `json:"warnings"`
	Suggestions int      `json:"suggestions"`
	Date        string   `json:"date"`
	History     []int    `json:"history"` // eskiden yeniye
	Dates       []string `json:"dates"`
}

type Server struct {
	Name     string   `json:"name"`
	Host     string   `json:"host"`
	User     string   `json:"user"`
	Port     string   `json:"port"`
	Platform string   `json:"platform"`
	OS       string   `json:"os"` // ubuntu | debian | proxmox | linux
	HasKey   bool     `json:"hasKey"`
	Up       bool     `json:"up"`
	Checked  bool     `json:"checked"` // durum en az bir kez ölçüldü mü
	Conn     string   `json:"conn"`    // "up" | "refused" (makine yanıt veriyor, port kapalı) | "down" (yanıt yok); ölçülmediyse boş
	Knock    bool     `json:"knock"`   // port knocking etkin (kapalı port = knock gerekir, 🔒)
	Valid    bool     `json:"valid"`
	Last     *Action  `json:"last"`
	Recent   []Action `json:"recent"`
	Lynis    *Lynis   `json:"lynis"`
	keyPath  string
}

type Package struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type LogFile struct {
	Package string `json:"package"`
	Server  string `json:"server"`
	File    string `json:"file"`
	Size    int64  `json:"size"`
	Mtime   string `json:"mtime"`
}

type App struct {
	serversDir string
	compDir    string
	stateDir   string // ~/.local/share/sshpocket
	token      string
	host       string
	mu         sync.Mutex
	jmu        sync.Mutex // eylem işleri
	vmu        sync.Mutex // değişken dosyası yazımı
	jobs       map[string]*Job
	running    *Job
	up         map[string]bool    // son ölçülen erişilebilirlik (sayfa beklemeden gösterilsin)
	conn       map[string]string  // son ölçülen bağlantı durumu: up | refused | down
	idxDL      trust.DownloadFunc // indeks indirme (nil: HTTPS); testte değiştirilir
	tmu        sync.Mutex         // gömülü terminal oturumları
	topen      sync.Mutex         // terminal açılışlarını sıraya sokar
	terms      map[string]*TermSession
}

func parseEnv(path string) map[string]string {
	m := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if k, v, ok := strings.Cut(l, "="); ok {
			m[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return m
}

// components.log satırı: <UTC zaman>|<paket>|<sürüm>|<eylem>|<sonuç>
func readActions(path string, limit int) []Action {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	var out []Action
	for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
		p := strings.Split(strings.TrimSpace(lines[i]), "|")
		if len(p) < 5 {
			continue
		}
		out = append(out, Action{Time: p[0], Package: p[1], Version: p[2], Action: p[3], Result: p[4]})
	}
	return out
}

// en yeni Lynis özeti ve skor geçmişi: <state>/linux-hardening/<sunucu>/out/<sunucu>/<tarih>/summary.json
func (a *App) lynis(server string) *Lynis {
	root := filepath.Join(a.stateDir, "linux-hardening", server, "out", server)
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range ents {
		if e.IsDir() && reName.MatchString(e.Name()) {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	var res *Lynis
	for _, d := range dirs {
		b, err := os.ReadFile(filepath.Join(root, d, "summary.json"))
		if err != nil {
			continue
		}
		var s struct {
			Index       int    `json:"hardening_index"`
			Warnings    int    `json:"warnings"`
			Suggestions int    `json:"suggestions"`
			Date        string `json:"date"`
		}
		if json.Unmarshal(b, &s) != nil {
			continue
		}
		if res == nil {
			res = &Lynis{}
		}
		res.History = append(res.History, s.Index)
		res.Dates = append(res.Dates, s.Date)
		res.Score, res.Warnings, res.Suggestions, res.Date = s.Index, s.Warnings, s.Suggestions, s.Date
	}
	if res != nil && len(res.History) > 40 {
		res.History = res.History[len(res.History)-40:]
		res.Dates = res.Dates[len(res.Dates)-40:]
	}
	return res
}

// only boş değilse yalnızca o sunucu okunur ve sorgulanır (sunucu sayfası diğerlerini beklemesin)
// detectOS: server.env'de OS=<profil kimliği> (ubuntu-26.04, debian-13, proxmox…) varsa o; yoksa platform ve sunucu adından tahmin (kesin değil).
func detectOS(declared, name, platform string) string {
	d := strings.ToLower(strings.TrimSpace(declared))
	for _, k := range []string{"ubuntu", "debian", "proxmox"} {
		if d == k || strings.HasPrefix(d, k+"-") {
			return k
		}
	}
	n := strings.ToLower(name)
	switch {
	case strings.HasPrefix(platform, "proxmox") || strings.Contains(n, "proxmox") || strings.Contains(n, "pve"):
		return "proxmox"
	case strings.Contains(n, "ubuntu"):
		return "ubuntu"
	case strings.Contains(n, "debian"):
		return "debian"
	}
	return "linux"
}

func (a *App) servers(probe bool, only ...string) []Server {
	ents, _ := os.ReadDir(a.serversDir)
	out := []Server{}
	for _, e := range ents {
		if !e.IsDir() || !reName.MatchString(e.Name()) || (len(only) > 0 && e.Name() != only[0]) {
			continue
		}
		dir := filepath.Join(a.serversDir, e.Name())
		env := parseEnv(filepath.Join(dir, "server.env"))
		s := Server{Name: e.Name(), Host: env["IP"], User: env["USER"], Port: env["PORT"]}
		if s.Port == "" {
			s.Port = "22"
		}
		s.Platform = parseEnv(filepath.Join(dir, "linux-hardening.vars"))["hardening_platform"]
		s.OS = detectOS(env["OS"], e.Name(), s.Platform)
		s.Knock = servers.KnockEnabled(dir)
		s.keyPath = filepath.Join(dir, "id_ed25519_"+e.Name())
		if _, err := os.Stat(s.keyPath); err == nil {
			s.HasKey = true
		}
		s.Valid = reHost.MatchString(s.Host) && reUser.MatchString(s.User) && rePort.MatchString(s.Port)
		s.Recent = readActions(filepath.Join(dir, "components.log"), 8)
		if len(s.Recent) > 0 {
			s.Last = &s.Recent[0]
		}
		s.Lynis = a.lynis(e.Name())
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if probe {
		a.probe(out)
	}
	a.mu.Lock()
	for i := range out {
		if up, ok := a.up[out[i].Name]; ok {
			out[i].Up, out[i].Checked = up, true
		}
		if c, ok := a.conn[out[i].Name]; ok {
			out[i].Conn = c
		}
	}
	a.mu.Unlock()
	return out
}

// dialState: "up" (bağlandı), "refused" (RST geldi: makine canlı, port kapalı) ya da "down" (yanıt yok / ulaşılamıyor).
// Not: güvenlik duvarı paketleri sessizce düşürüyorsa (DROP) canlı makine de zaman aşımıyla "down" görünür; knock'lı
// sunucularda beklenen durum budur ve arayüzde 🔒 olarak gösterilir. ICMP (ping) kullanılmaz: yetki ve işletim sistemine bağlı.
func dialState(addr string, timeout time.Duration) string {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		c.Close()
		return "up"
	}
	// Windows'ta syscall.ECONNREFUSED farklı bir kodla eşleşmez; iletide "refused" geçer (aktif reddedildi).
	if errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(strings.ToLower(err.Error()), "refused") {
		return "refused"
	}
	return "down"
}

// probe: sunuculara eşzamanlı TCP denemesi (en fazla 1,5 sn); sonucu önbelleğe yazar
func (a *App) probe(list []Server) {
	st := settings.Store{File: confFile(a.engineRoot())}
	if v, _ := st.Get("STATUS"); v == "off" { // sshpocket.conf STATUS=off: durum yoklaması kapalı
		return
	}
	secs := 1.0
	if v, ok := st.Get("STATUS_TIMEOUT"); ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 1 {
			secs = n
		}
	}
	var wg sync.WaitGroup
	for i := range list {
		if !list[i].Valid {
			continue
		}
		wg.Add(1)
		go func(s Server) {
			defer wg.Done()
			state := dialState(net.JoinHostPort(s.Host, s.Port), time.Duration(secs*float64(time.Second)))
			a.mu.Lock()
			a.up[s.Name] = state == "up"
			if a.conn == nil { // testlerde App bu harita olmadan kurulabilir
				a.conn = map[string]string{}
			}
			a.conn[s.Name] = state
			a.mu.Unlock()
		}(list[i])
	}
	wg.Wait()
}

func (a *App) packages() (pk []Package, serial string) {
	pk = []Package{}
	ents, _ := os.ReadDir(a.compDir)
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		// USB (exFAT/FAT) symlink tutmaz: current bir dosyadır ve içinde sürüm yazar; symlink ise hedefin adı.
		p := filepath.Join(a.compDir, e.Name(), "current")
		v := "?"
		if t, err := os.Readlink(p); err == nil {
			v = filepath.Base(t)
		} else if b, err := os.ReadFile(p); err == nil && len(b) < 64 {
			if s := strings.TrimSpace(string(b)); s != "" {
				v = s
			}
		}
		pk = append(pk, Package{ID: e.Name(), Version: v})
	}
	st := parseEnv(filepath.Join(a.compDir, ".index", "state"))
	return pk, st["serial"]
}

// kalıcı günlükler: <state>/<paket>/<sunucu>/{*.log, logs/*.log}
func (a *App) logs() []LogFile {
	out := []LogFile{}
	pkgs, _ := os.ReadDir(a.stateDir)
	for _, p := range pkgs {
		if !p.IsDir() || !reName.MatchString(p.Name()) {
			continue
		}
		srvs, _ := os.ReadDir(filepath.Join(a.stateDir, p.Name()))
		for _, s := range srvs {
			if !s.IsDir() || !reName.MatchString(s.Name()) {
				continue
			}
			base := filepath.Join(a.stateDir, p.Name(), s.Name())
			for _, sub := range []string{"", "logs"} {
				fs, _ := os.ReadDir(filepath.Join(base, sub))
				for _, f := range fs {
					if f.IsDir() || !strings.HasSuffix(f.Name(), ".log") {
						continue
					}
					info, err := f.Info()
					if err != nil || !info.Mode().IsRegular() {
						continue
					}
					name := f.Name()
					if sub != "" {
						name = sub + "/" + name
					}
					out = append(out, LogFile{Package: p.Name(), Server: s.Name(), File: name, Size: info.Size(), Mtime: info.ModTime().UTC().Format(time.RFC3339)})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mtime > out[j].Mtime })
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

type Finding struct {
	ID              string `json:"id"`
	Risk            string `json:"risk"`
	Kind            string `json:"kind"` // uyari | oneri
	Text            string `json:"text"`
	Detail          string `json:"detail"`
	Suggestion      string `json:"suggestion"`
	Area            string `json:"area"`
	Value           string `json:"value"`
	ClosesByDefault string `json:"closesByDefault"`
}

// en yeni denetim raporunun bulgular.tsv dosyası (internal/findings: sunucudan gelen veri doğrulanır, geçersiz satırlar atlanır,
// alanlar kesilir; component_findings.py ile aynı kurallar). Dönen ilk değer raporun tarih klasörünün adıdır.
func (a *App) findings(server string) (string, []Finding) {
	res, err := findings.Load(a.stateDir, "linux-hardening", server, "bulgular.tsv")
	if err != nil || res == nil {
		return "", nil
	}
	out := make([]Finding, 0, len(res.Rows))
	for _, r := range res.Rows {
		out = append(out, Finding{ID: r[0], Risk: r[1], Kind: r[2], Text: r[3], Detail: r[4], Suggestion: r[5], Area: r[6], Value: r[7], ClosesByDefault: r[8]})
	}
	return filepath.Base(filepath.Dir(res.Path)), out
}
