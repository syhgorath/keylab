package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Bu dosya sistem/Helpers/component_run.py'nin Go karşılığıdır: değişken değerlerinin doğrulanması, describe,
// check, vars (çalıştırıcı değişken dosyası), platform ve env. Hata iletileri ve çıktılar birebir korunur.

const maxValuesBytes = 64 * 1024

var (
	reCtrl     = regexp.MustCompile(`[\x00-\x1f\x7f]`)
	reInt      = regexp.MustCompile(`^-?[0-9]{1,12}$`)
	reList     = regexp.MustCompile(`^list\[(.+)\]$`)
	reCleanSep = regexp.MustCompile(`[\t\r\n\x00-\x1f\x7f]+`)
	reSafeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// errList, Python betiğindeki global `errors` listesidir.
type errList struct{ msgs []string }

func (e *errList) add(format string, a ...any) { e.msgs = append(e.msgs, fmt.Sprintf(format, a...)) }

func readJSON(path string, e *errList) any {
	st, err := os.Stat(path)
	if err == nil && st.Size() > maxBytes {
		e.add("component.json çok büyük")
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		e.add("component.json okunamadı: %v", err)
		return nil
	}
	v, derr := decode(data)
	if derr != nil {
		e.add("component.json okunamadı: %v", derr)
		return nil
	}
	return v
}

// pySplitLines, Python str.splitlines() ayraçlarını uygular.
func pySplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', ' ', ' ':
			out = append(out, cur.String())
			cur.Reset()
		case '\r':
			out = append(out, cur.String())
			cur.Reset()
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func pyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == ' ' || (r >= '\t' && r <= '\r') || (r >= 0x1c && r <= 0x1f) || r == 0x85 || r == 0xa0 ||
			r == 0x1680 || (r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000
	})
}

// parseValues: "ad=değer" satırlı değerler dosyası. Sonuç haritadır; hatalar e'ye eklenir.
func parseValues(path string, e *errList) map[string]string {
	out := map[string]string{}
	if path == "" || path == "/dev/null" {
		return out
	}
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out
		}
		e.add("değerler dosyası okunamadı: %v", err)
		return out
	}
	if st.Size() > maxValuesBytes {
		e.add("değerler dosyası çok büyük")
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(data) {
		e.add("değerler dosyası okunamadı: %v", err)
		return out
	}
	for i, line := range pySplitLines(string(data)) {
		s := pyStrip(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		k, v, found := strings.Cut(s, "=")
		if !found {
			e.add("değerler dosyası satır %d: 'ad=değer' biçiminde değil", i+1)
			continue
		}
		k = pyStrip(k)
		if !reName.MatchString(k) {
			e.add("değerler dosyası satır %d: geçersiz değişken adı", i+1)
			continue
		}
		if _, dup := out[k]; dup {
			e.add("değerler dosyası: yinelenen değişken: %s", k)
			continue
		}
		out[k] = pyStrip(v)
	}
	return out
}

// scalar: ham metni tür değerine çevirir; geçersizse hata ekler ve ok=false.
func scalar(t, raw string, spec *obj, name string, e *errList) (any, bool) {
	switch t {
	case "string", "path":
		if reCtrl.MatchString(raw) || pyLen(raw) > 1024 {
			e.add("%s: değer geçersiz (denetim karakteri/çok uzun)", name)
			return nil, false
		}
		return raw, true
	case "bool":
		switch strings.ToLower(raw) {
		case "true", "yes":
			return true, true
		case "false", "no":
			return false, true
		}
		e.add("%s: true/false olmalı", name)
		return nil, false
	case "int":
		if !reInt.MatchString(raw) {
			e.add("%s: tamsayı olmalı", name)
			return nil, false
		}
		n, _ := strconv.ParseInt(raw, 10, 64)
		if lo, ok := asInt(spec.get("min")); ok && n < lo {
			e.add("%s: %d en az %d olmalı", name, n, lo)
			return nil, false
		}
		if hi, ok := asInt(spec.get("max")); ok && n > hi {
			e.add("%s: %d en çok %d olmalı", name, n, hi)
			return nil, false
		}
		return n, true
	case "cidr":
		if _, ok := parseNetwork(raw); !ok {
			e.add("%s: geçerli bir CIDR değil", name)
			return nil, false
		}
		return raw, true
	case "enum":
		vals, _ := asList(spec.get("values"))
		for _, x := range vals {
			if x == raw {
				return raw, true
			}
		}
		e.add("%s: izin verilen değerlerden biri olmalı", name)
		return nil, false
	}
	e.add("%s: bilinmeyen tür", name)
	return nil, false
}

func typed(t, raw string, spec *obj, name string, e *errList) (any, bool) {
	if m := reList.FindStringSubmatch(t); m != nil {
		var items []string
		if pyStrip(raw) != "" {
			for _, x := range strings.Split(raw, ",") {
				items = append(items, pyStrip(x))
			}
		}
		for _, x := range items {
			if x == "" {
				e.add("%s: listede boş öğe var", name)
				return nil, false
			}
		}
		res := []any{}
		for _, x := range items {
			v, ok := scalar(m[1], x, spec, name, e)
			if !ok {
				return nil, false
			}
			res = append(res, v)
		}
		return res, true
	}
	return scalar(t, raw, spec, name, e)
}

// Decl, bildirimdeki değişkenleri ekleme sırasıyla tutar (yinelenen ad: sonuncu belirtim, sıra iki kez).
type Decl struct {
	Specs map[string]*obj
	Order []string
}

func declaredVars(path string, e *errList) *Decl {
	d := readJSON(path, e)
	out := &Decl{Specs: map[string]*obj{}}
	if do, ok := asObj(d); ok {
		vs := do.getDef("vars", []any{})
		if l, ok := asList(vs); ok {
			for _, v := range l {
				var vo *obj
				if s, ok := asStr(v); ok {
					vo = newObj()
					vo.set("name", s)
					vo.set("type", "string")
				} else if o, ok := asObj(v); ok {
					vo = o
				}
				if vo != nil {
					if n, ok := asStr(vo.get("name")); ok {
						out.Specs[n] = vo
						out.Order = append(out.Order, n)
					}
				}
			}
		}
	} else if d != nil {
		e.add("component.json nesne olmalı")
	}
	return out
}

func cleanText(s string) string {
	return pyStrip(reCleanSep.ReplaceAllString(s, " "))
}

func readVarsFile(path string) map[string]string {
	vals := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return vals
	}
	for _, line := range strings.SplitAfter(string(data), "\n") {
		if strings.Contains(line, "=") && !strings.HasPrefix(strings.TrimLeft(line, " \t\r\n\v\f"), "#") {
			k, v, _ := strings.Cut(strings.TrimSuffix(line, "\n"), "=")
			vals[pyStrip(k)] = pyStrip(v)
		}
	}
	return vals
}

func currentPlatform(manifestPath string, decl *Decl, varsPath string, e *errList) string {
	d := readJSON(manifestPath, e)
	do, ok := asObj(d)
	if !ok {
		return ""
	}
	pv, ok := asStr(do.get("platform_var"))
	if !ok {
		return ""
	}
	spec, found := decl.Specs[pv]
	if !found {
		return ""
	}
	val := ""
	if varsPath != "" {
		val = readVarsFile(varsPath)[pv]
	}
	if val == "" {
		if dv, ok := asStr(spec.get("default")); ok {
			val = dv
		}
	}
	return val
}

// Platform: `platform <component.json> [<vars>]` çıktısı (tek satır) ve çıkış kodu.
func Platform(manifestPath, varsPath string) (line string, errs []string, code int) {
	e := &errList{}
	decl := declaredVars(manifestPath, e)
	if len(e.msgs) > 0 {
		return "", prefixErrs(e.msgs), 1
	}
	return cleanText(currentPlatform(manifestPath, decl, varsPath, e)), nil, 0
}

func prefixErrs(m []string) []string {
	out := make([]string, len(m))
	for i, s := range m {
		out[i] = "HATA: " + s
	}
	return out
}

// Describe: değişken başına bir TSV satırı. varsPath verilmişse (hasVars) platforma göre süzülür.
func Describe(manifestPath, varsPath string, hasVars bool) (lines, errs []string, code int) {
	e := &errList{}
	decl := declaredVars(manifestPath, e)
	if len(e.msgs) > 0 {
		return nil, prefixErrs(e.msgs), 1
	}
	plat := ""
	if hasVars {
		plat = currentPlatform(manifestPath, decl, varsPath, e)
	}
	groupOf := func(name string) string {
		if g, ok := asStr(decl.Specs[name].get("group")); ok {
			return cleanText(g)
		}
		return ""
	}
	var groups []string
	for _, name := range decl.Order {
		g := groupOf(name)
		if g != "" && idxOf(groups, g) < 0 {
			groups = append(groups, g)
		}
	}
	sortKey := func(name string) int {
		if g := groupOf(name); g != "" {
			return idxOf(groups, g) + 1
		}
		return 0
	}
	order := append([]string(nil), decl.Order...)
	sort.SliceStable(order, func(i, j int) bool {
		ki, kj := sortKey(order[i]), sortKey(order[j])
		if ki != kj {
			return ki < kj
		}
		return idxOf(decl.Order, order[i]) < idxOf(decl.Order, order[j])
	})
	for _, name := range order {
		v := decl.Specs[name]
		if pls, ok := asList(v.get("platforms")); ok && plat != "" && !listHas(pls, plat) {
			continue
		}
		dflt := ""
		if v.has("default") {
			dflt = dumps(v.get("default"), -1, false, false)
		}
		warn := ""
		if v.has("warn_if") {
			warn = dumps(v.get("warn_if"), -1, false, false)
		}
		mn, mx := "", ""
		if n, ok := asInt(v.get("min")); ok {
			mn = strconv.FormatInt(n, 10)
		}
		if n, ok := asInt(v.get("max")); ok {
			mx = strconv.FormatInt(n, 10)
		}
		grp := groupOf(name)
		vals := ""
		if vl, ok := asList(v.get("values")); ok {
			vals = dumps(vl, -1, false, true)
		}
		typ := v.getDef("type", "string")
		desc := v.getDef("description", "")
		lines = append(lines, strings.Join([]string{name, cleanText(pyStr(typ)), yesNo(v.get("required") == true),
			cleanText(dflt), mn, mx, cleanText(warn), cleanText(pyStr(desc)), grp, cleanText(vals)}, "\t"))
	}
	return lines, nil, 0
}

func idxOf(l []string, s string) int {
	for i, x := range l {
		if x == s {
			return i
		}
	}
	return -1
}

// Check: tek değeri doğrular. warn=true ise stdout'a "warn=<ad>" yazılmalıdır.
func Check(manifestPath, name, raw string) (warn bool, errs []string, code int) {
	e := &errList{}
	decl := declaredVars(manifestPath, e)
	spec, found := decl.Specs[name]
	if !found {
		e.add("%s: pakette tanımlı bir değişken değil", name)
	} else if raw == "" {
		e.add("%s: değer boş", name)
	} else {
		t, _ := asStr(spec.getDef("type", "string"))
		v, ok := typed(t, raw, spec, name, e)
		if ok && spec.has("warn_if") {
			if b, isB := v.(bool); isB && spec.get("warn_if") == b {
				warn = true
			}
		}
	}
	if len(e.msgs) > 0 {
		return false, prefixErrs(e.msgs), 1
	}
	return warn, nil, 0
}

// VarsOptions, `vars` komutunun seçenekleridir.
type VarsOptions struct {
	Forced  map[string]string // --set ad=değer
	NoReq   bool              // --no-required
	OutPath string
}

// BuildVars: çalıştırıcı değişken dosyasını (JSON, 0600, O_EXCL) yazar. Dönüş: warn adları (sıralı), hatalar, çıkış kodu.
func BuildVars(manifestPath, valuesPath string, opt VarsOptions) (warns, errs []string, code int) {
	e := &errList{}
	d := readJSON(manifestPath, e)
	declared := map[string]*obj{}
	var declOrder []string
	if do, ok := asObj(d); ok {
		if l, ok := asList(do.getDef("vars", []any{})); ok {
			for _, v := range l {
				var vo *obj
				if s, ok := asStr(v); ok {
					vo = newObj()
					vo.set("name", s)
					vo.set("type", "string")
				} else if o, ok := asObj(v); ok {
					vo = o
				}
				if vo != nil {
					if n, ok := asStr(vo.get("name")); ok {
						if _, dup := declared[n]; !dup {
							declOrder = append(declOrder, n)
						}
						declared[n] = vo
					}
				}
			}
		}
	} else if d != nil {
		e.add("component.json nesne olmalı")
	}

	values := parseValues(valuesPath, e)
	for k, v := range opt.Forced {
		values[k] = v
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := declared[k]; !ok {
			e.add("%s: pakette tanımlı bir değişken değil", k)
		}
	}

	result := map[string]any{}
	require := !opt.NoReq
	for _, name := range declOrder {
		spec := declared[name]
		t, _ := asStr(spec.getDef("type", "string"))
		var v any
		raw, in := values[name]
		switch {
		case in && raw != "":
			tv, ok := typed(t, raw, spec, name, e)
			if !ok {
				continue
			}
			v = tv
		case spec.has("default"):
			v = spec.get("default")
		case in && reList.MatchString(t):
			v = []any{}
		}
		isEmptyList := false
		if l, ok := v.([]any); ok && len(l) == 0 {
			isEmptyList = true
		}
		if v == nil || (spec.get("required") == true && isEmptyList) {
			if spec.get("required") == true && require {
				e.add("%s: zorunlu değişken eksik", name)
			}
			continue
		}
		result[name] = v
		if b, isB := v.(bool); isB && spec.has("warn_if") && spec.get("warn_if") == b {
			warns = append(warns, name)
		}
	}
	if len(e.msgs) > 0 {
		return nil, prefixErrs(e.msgs), 1
	}
	f, err := os.OpenFile(opt.OutPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, []string{"HATA: değişken dosyası yazılamadı: " + err.Error()}, 1
	}
	defer f.Close()
	if _, err := f.WriteString(dumps(result, 2, true, false) + "\n"); err != nil {
		return nil, []string{"HATA: değişken dosyası yazılamadı: " + err.Error()}, 1
	}
	sort.Strings(warns)
	return warns, nil, 0
}

// shQuote, Python shlex.quote karşılığıdır.
func shQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !(r < 128 && (r == '_' || r == '@' || r == '%' || r == '+' || r == '=' || r == ':' || r == ',' || r == '.' || r == '/' || r == '-' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// Env: script runner için sunucuda betiğe verilecek `export` satırları.
func Env(varsJSON, server, action, profile string) (lines, errs []string, code int) {
	for _, p := range [][2]string{{"sunucu", server}, {"eylem", action}, {"profil", profile}} {
		if !reSafeName.MatchString(p[1]) {
			return nil, []string{"HATA: " + p[0] + " adı geçersiz"}, 1
		}
	}
	e := &errList{}
	data := readJSON(varsJSON, e)
	do, ok := asObj(data)
	if len(e.msgs) > 0 || !ok {
		return nil, []string{"HATA: değişken dosyası okunamadı"}, 1
	}
	lines = []string{"export SSHPOCKET_SERVER=" + shQuote(server), "export SSHPOCKET_ACTION=" + shQuote(action), "export SSHPOCKET_PROFILE=" + shQuote(profile)}
	names := append([]string(nil), do.keys...)
	sort.Strings(names)
	for _, name := range names {
		v := do.get(name)
		if !reName.MatchString(name) {
			return nil, []string{"HATA: değişken adı geçersiz: " + pyRepr(name)}, 1
		}
		var txt string
		switch t := v.(type) {
		case bool:
			txt = valText(t)
		case []any:
			parts := make([]string, len(t))
			for i, x := range t {
				parts[i] = pyStr(x)
			}
			txt = strings.Join(parts, ",")
		case int64, string:
			txt = pyStr(t)
		default:
			return nil, []string{"HATA: " + name + ": desteklenmeyen değer türü"}, 1
		}
		if reCtrl.MatchString(txt) {
			return nil, []string{"HATA: " + name + ": denetim karakteri içeriyor"}, 1
		}
		lines = append(lines, "export SSHPOCKET_VAR_"+strings.ToUpper(name)+"="+shQuote(txt))
	}
	return lines, nil, 0
}
