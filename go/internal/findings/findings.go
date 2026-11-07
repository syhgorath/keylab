// Package findings: denetim bulguları ve puanları (component_findings.py, component_scores.py).
// Okunan dosyalar sunucudan gelir ve GÜVENİLMEZ: geçersiz satır atlanır (sayılır), denetim karakterleri boşluğa çevrilir,
// alanlar 300 karakterle kesilir, bağlantılar izlenmez. Python sürümüyle aynı sınırlar ve sıralama.
package findings

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var Columns = []string{"kimlik", "risk", "tur", "aciklama", "ayrinti", "oneri", "alan", "deger", "kapanir_varsayilan", "platform"}
var Risks = []string{"yuksek", "orta", "dusuk", "bilgi", "bilinmiyor"}

const (
	maxDepth = 5
	maxRows  = 2000
	maxField = 300
	maxBytes = 4 * 1024 * 1024
)

var (
	idRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,39}$`)
	areaRe  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	valueRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}=[^\t\r\n]{0,200}$`)
	fileRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,59}\.tsv$`)
	nameRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+\n?$`) // Python `$` sondaki satır sonundan önce de eşleşir
)

// ErrBadHeader: başlık satırı beklenen 10 sütun değil (Python çıkış kodu 3).
var ErrBadHeader = errors.New("başlık bozuk")

// ErrBadArgs: geçersiz paket/sunucu/dosya adı (Python çıkış kodu 2).
var ErrBadArgs = errors.New("geçersiz ad")

// Result: ilk satır meta + bulgular.
type Result struct {
	Epoch     int64
	Total     int
	Skipped   int
	Truncated bool
	Mapped    int
	Path      string
	Rows      [][10]string
}

// walkFiles: base altında (derinlik < maxDepth'te alt klasörlere inilir), bağlantısız klasörlerde `name` adlı düzenli (bağlantısız) dosyalar.
func walkFiles(base, name string, visit func(path string, mtime float64, size int64)) {
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		return
	}
	if st, err := os.Stat(real); err != nil || !st.IsDir() {
		return
	}
	var rec func(dir string, depth int)
	rec = func(dir string, depth int) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
		for _, e := range ents {
			if e.Name() != name || e.IsDir() {
				continue
			}
			p := filepath.Join(dir, e.Name())
			li, err := os.Lstat(p)
			if err != nil || li.Mode()&os.ModeSymlink != 0 {
				continue
			}
			st, err := os.Stat(p)
			if err != nil {
				continue
			}
			visit(p, float64(st.ModTime().UnixNano())/1e9, st.Size())
		}
		if depth >= maxDepth {
			return
		}
		for _, e := range ents {
			if !e.IsDir() { // DirEntry.IsDir bağlantıyı izlemez: bağlantılı klasör atlanır
				continue
			}
			rec(filepath.Join(dir, e.Name()), depth+1)
		}
	}
	rec(real, 0)
}

func pyWhite(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

func pyStrip(s string) string { return strings.TrimFunc(s, pyWhite) }

// pyDecode: Python bytes.decode("utf-8", errors="replace") — bozuk dizi başına tek U+FFFD (en büyük geçerli önek).
func pyDecode(b []byte) string {
	var out strings.Builder
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r != utf8.RuneError || n > 1 {
			out.WriteRune(r)
			b = b[n:]
			continue
		}
		// geçersiz: tamamlanabilir bir önek kaç bayt? (Python: maximal subpart)
		k := validPrefixLen(b)
		out.WriteRune(utf8.RuneError)
		b = b[k:]
	}
	return out.String()
}

func validPrefixLen(b []byte) int {
	c := b[0]
	var need int
	lo, hi := byte(0x80), byte(0xBF)
	switch {
	case c >= 0xC2 && c <= 0xDF:
		need = 1
	case c == 0xE0:
		need, lo = 2, 0xA0
	case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
		need = 2
	case c == 0xED:
		need, hi = 2, 0x9F
	case c == 0xF0:
		need, lo = 3, 0x90
	case c >= 0xF1 && c <= 0xF3:
		need = 3
	case c == 0xF4:
		need, hi = 3, 0x8F
	default:
		return 1
	}
	n := 1
	for i := 1; i <= need && i < len(b); i++ {
		if b[i] < lo || b[i] > hi {
			break
		}
		lo, hi = 0x80, 0xBF
		n++
	}
	return n
}

// pySplitLines: Python str.splitlines() (anahtar yok): \n \r \r\n \v \f \x1c \x1d \x1e \x85    .
func pySplitLines(s string) []string {
	var lines []string
	start := 0
	rs := []rune(s)
	var cur []rune
	_ = start
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			lines = append(lines, string(cur))
			cur = cur[:0]
		case '\r':
			lines = append(lines, string(cur))
			cur = cur[:0]
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
		default:
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		lines = append(lines, string(cur))
	}
	return lines
}

func clean(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			b.WriteByte(' ')
		} else {
			b.WriteRune(r)
		}
	}
	s := pyStrip(b.String())
	rs := []rune(s)
	if len(rs) > maxField {
		rs = rs[:maxField]
	}
	return string(rs)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func valid(r [10]string) bool {
	kimlik, risk, tur, alan, deger, kapanir, platform := r[0], r[1], r[2], r[6], r[7], r[8], r[9]
	return idRe.MatchString(kimlik) && contains(Risks, risk) && (tur == "uyari" || tur == "oneri") &&
		(platform == "ubuntu" || platform == "proxmox" || platform == "hepsi") &&
		(kapanir == "" || kapanir == "evet" || kapanir == "hayir") &&
		(alan == "" || areaRe.MatchString(alan)) && (deger == "" || valueRe.MatchString(deger))
}

// Load, component_findings.py main: <state>/<pkg>/<server>/**/<file> (en yeni). Dosya yoksa (nil, nil).
func Load(state, pkg, server, fname string) (*Result, error) {
	if !nameRe.MatchString(pkg) || !nameRe.MatchString(server) || !fileRe.MatchString(fname) || strings.Contains(fname, "..") {
		return nil, ErrBadArgs
	}
	var best string
	var bestM float64
	found := false
	walkFiles(filepath.Join(state, pkg, server), fname, func(p string, m float64, _ int64) {
		if !found || m > bestM {
			best, bestM, found = p, m, true
		}
	})
	if !found {
		return nil, nil
	}
	f, err := os.Open(best)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, nil
	}
	body := raw
	if len(body) > maxBytes {
		body = body[:maxBytes]
	}
	lines := pySplitLines(pyDecode(body))
	if len(lines) == 0 {
		return nil, ErrBadHeader
	}
	hdr := strings.Split(lines[0], "\t")
	if len(hdr) > len(Columns) {
		hdr = hdr[:len(Columns)]
	}
	if len(hdr) != len(Columns) {
		return nil, ErrBadHeader
	}
	for i := range hdr {
		if pyStrip(hdr[i]) != Columns[i] {
			return nil, ErrBadHeader
		}
	}
	res := &Result{Path: best, Epoch: int64(bestM)}
	var rows [][10]string
	for _, line := range lines[1:] {
		if pyStrip(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < len(Columns) {
			res.Skipped++
			continue
		}
		var r [10]string
		for i := 0; i < 10; i++ {
			r[i] = clean(cols[i])
		}
		if !valid(r) {
			res.Skipped++
			continue
		}
		rows = append(rows, r)
	}
	res.Truncated = len(rows) > maxRows || len(raw) > maxBytes
	idx := func(r [10]string) int {
		for i, x := range Risks {
			if x == r[1] {
				return i
			}
		}
		return len(Risks)
	}
	sort.SliceStable(rows, func(i, j int) bool { return idx(rows[i]) < idx(rows[j]) })
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	for _, r := range rows {
		if r[6] != "" {
			res.Mapped++
		}
	}
	res.Total = len(rows)
	res.Rows = rows
	return res, nil
}

// Format, Python çıktısı: "#\x1f…" başlık satırı + \x1f ayraçlı satırlar.
func (r *Result) Format() string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	t := 0
	if r.Truncated {
		t = 1
	}
	b.WriteString("#\x1f" + itoa(r.Epoch) + "\x1f" + itoa(int64(r.Total)) + "\x1f" + itoa(int64(r.Skipped)) + "\x1f" + itoa(int64(t)) + "\x1f" + itoa(int64(r.Mapped)) + "\x1f" + r.Path + "\n")
	for _, row := range r.Rows {
		b.WriteString(strings.Join(row[:], "\x1f") + "\n")
	}
	return b.String()
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		d = append([]byte{'-'}, d...)
	}
	return string(d)
}
