package findings

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// component_scores.py: <state>/<pkg>/<server>/**/summary.json → araç başına en yeni 2 puan.
// Ortak özet: {"tool":"Lynis","date":"2026-10-25T08:44:34Z","score":83}; `score` yoksa `hardening_index` (Lynis).
// Yalnızca sonlu sayı (0..1000) ve [A-Za-z0-9 ._-]{1,20} araç adı kabul edilir.

var (
	toolRe = regexp.MustCompile(`^[A-Za-z0-9 ._-]{1,20}\n?$`)
	dateRe = regexp.MustCompile(`^([0-9]{4})-([0-9]{1,2})-([0-9]{1,2})T([0-9]{1,2}):([0-9]{1,2}):([0-9]{1,2})Z$`)
)

// Score: tek puan satırı. Value Python'un yazdığı biçimdedir ("83", "83.5", "83.0").
type Score struct {
	Tool  string
	Epoch int64
	Value string
	Num   float64
}

// number: Python number(): bool/sayı olmayan, sonlu olmayan veya 0..1000 dışı → ok=false.
func number(v any) (string, float64, bool) {
	var f float64
	switch x := v.(type) {
	case json.Number:
		f, _ = strconv.ParseFloat(string(x), 64) // taşan değer ±Inf döner (Python float('inf') gibi) ve aşağıda elenir
	default:
		return "", 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > 1000 {
		return "", 0, false
	}
	if f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10), f, true
	}
	s := strconv.FormatFloat(f, 'f', 1, 64) // doğru yuvarlama (Python round(x, 1) ile aynı: ikilik değer üzerinden)
	r, _ := strconv.ParseFloat(s, 64)
	return s, r, true
}

func parseEpoch(v any, fallback int64) int64 {
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	m := dateRe.FindStringSubmatch(s)
	if m == nil {
		return fallback
	}
	n := func(i int) int { x, _ := strconv.Atoi(m[i]); return x }
	y, mo, d, h, mi, sec := n(1), n(2), n(3), n(4), n(5), n(6)
	if mo < 1 || mo > 12 || d < 1 || h > 23 || mi > 59 || sec > 59 || y < 1 {
		return fallback
	}
	t := time.Date(y, time.Month(mo), d, h, mi, sec, 0, time.UTC)
	if t.Day() != d { // 31 Şubat gibi
		return fallback
	}
	return t.Unix()
}

// decodeLenient: Python json.load NaN/Infinity/-Infinity sabitlerini kabul eder; Go etmez. Dize dışındaki bu sözcükleri null'a çevirip yeniden dener.
func decodeLenient(b []byte) (map[string]any, bool) {
	dec := func(b []byte) (map[string]any, bool) {
		var m map[string]any
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.UseNumber()
		if d.Decode(&m) != nil || m == nil {
			return nil, false
		}
		if _, err := d.Token(); err == nil { // sonda fazladan veri
			return nil, false
		}
		return m, true
	}
	if m, ok := dec(b); ok {
		return m, true
	}
	var out []byte
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out = append(out, c)
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			out = append(out, c)
			continue
		}
		rest := string(b[i:])
		switch {
		case strings.HasPrefix(rest, "-Infinity"):
			out = append(out, "null"...)
			i += len("-Infinity") - 1
		case strings.HasPrefix(rest, "Infinity"):
			out = append(out, "null"...)
			i += len("Infinity") - 1
		case strings.HasPrefix(rest, "NaN"):
			out = append(out, "null"...)
			i += len("NaN") - 1
		default:
			out = append(out, c)
		}
	}
	return dec(out)
}

// Scores, component_scores.py main. Geçersiz paket/sunucu adı → ErrBadArgs.
func Scores(state, pkg, server string) ([]Score, error) {
	if !nameRe.MatchString(pkg) || !nameRe.MatchString(server) {
		return nil, ErrBadArgs
	}
	var rows []Score
	walkFiles(filepath.Join(state, pkg, server), "summary.json", func(p string, m float64, _ int64) {
		b, err := os.ReadFile(p)
		if err != nil {
			return
		}
		data, ok := decodeLenient(b)
		if !ok {
			return
		}
		sv, sn, sok := number(data["score"])
		lv, ln, lok := number(data["hardening_index"])
		if !sok {
			sv, sn, sok = lv, ln, lok
		}
		if !sok {
			return
		}
		tool, isStr := data["tool"].(string)
		if !isStr || !toolRe.MatchString(tool) {
			if lok {
				tool = "Lynis"
			} else {
				tool = "denetim"
			}
		}
		rows = append(rows, Score{tool, parseEpoch(data["date"], int64(m)), sv, sn})
	})
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Epoch > rows[j].Epoch })
	per := map[string][]Score{}
	for _, r := range rows {
		if len(per[r.Tool]) < 2 {
			per[r.Tool] = append(per[r.Tool], r)
		}
	}
	var tools []string
	for t := range per {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	var out []Score
	for _, t := range tools {
		out = append(out, per[t]...)
	}
	return out, nil
}

// Format: "<araç>\t<epoch>\t<puan>\n" satırları.
func FormatScores(s []Score) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteString(r.Tool + "\t" + strconv.FormatInt(r.Epoch, 10) + "\t" + r.Value + "\n")
	}
	return b.String()
}
