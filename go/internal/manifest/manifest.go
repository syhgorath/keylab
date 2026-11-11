package manifest

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxBytes = 256 * 1024

var (
	effectsList  = []string{"installs-package", "changes-config", "destructive"}
	confirmsList = []string{"none", "yes-no", "type-server-name"}
	runnersList  = []string{"ansible", "script"}
	menuCats     = []string{"hardening", "kurulum", "paketler"}
	statusesList = []string{"supported", "planned"}
	varScalars   = []string{"string", "bool", "int", "path", "cidr", "enum"}
)

// Not: Go'da `$` yalnızca metnin sonuna eşleşir; Python `re.match` sondaki "\n"den önce de eşleşirdi.
// Go bu yüzden sondaki satır sonunu REDDEDER (Python'dan katı; güvenlik açısından istenen).
var (
	reItem      = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	reID        = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	reVersion   = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	reName      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	rePlatform  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,29}$`)
	reProfile   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	reGroup     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
	reCmdBad    = regexp.MustCompile("[;&|`$<>(){}\\\\\"'\\n\\r\\t\\x00]")
	reTextBad   = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f]`)
	rePyRange   = regexp.MustCompile(`^>=[0-9]+\.[0-9]+(,<[0-9]+\.[0-9]+)?$`)
	reScript    = regexp.MustCompile(`^scripts/[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.sh$`)
	reScriptArg = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._=:/+,-]{0,63}$|^--?[A-Za-z0-9][A-Za-z0-9._=:/+,-]{0,63}$`)
	reOutKey    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	reFindings  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,59}\.tsv$`)
	reValue     = regexp.MustCompile(`^(<[a-z0-9_ -]{1,40}>|[A-Za-z0-9_./:@=,+-]{1,200})$`)
	reListType  = regexp.MustCompile(`^list\[([a-z]+)\]$`)
)

const varDescMax = 2000

// --- Python tür denetimi karşılıkları ---

func asStr(v any) (string, bool) { s, ok := v.(string); return s, ok }
func asInt(v any) (int64, bool)  { n, ok := v.(int64); return n, ok }
func asBool(v any) (bool, bool)  { b, ok := v.(bool); return b, ok }
func asList(v any) ([]any, bool) { l, ok := v.([]any); return l, ok }
func asObj(v any) (*obj, bool)   { o, ok := v.(*obj); return o, ok }
func isStr(v any) bool           { _, ok := v.(string); return ok }
func isInt(v any) bool           { _, ok := v.(int64); return ok }
func isBoolV(v any) bool         { _, ok := v.(bool); return ok }
func isListV(v any) bool         { _, ok := v.([]any); return ok }
func isObjV(v any) bool          { _, ok := v.(*obj); return ok }
func pyLen(s string) int         { return utf8.RuneCountInString(s) }
func inStrs(v any, set []string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}
func idx(set []string, s string) int {
	for i, x := range set {
		if x == s {
			return i
		}
	}
	return -1
}

// truthy, Python'un bool(x) karşılığıdır.
func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case int64:
		return t != 0
	case float64:
		return t != 0
	case []any:
		return len(t) > 0
	case *obj:
		return t.len() > 0
	}
	return true
}

// pyStr, "%s" % x karşılığıdır (yalnızca ileti metinleri için).
func pyStr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = pyRepV(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *obj:
		parts := make([]string, 0, len(t.keys))
		for _, k := range t.keys {
			parts = append(parts, pyRepr(k)+": "+pyRepV(t.m[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprint(v)
}

// pyRepV, bir değerin Python repr() karşılığıdır (kapsayıcı içindeki metinler tırnaklı yazılır).
func pyRepV(v any) string {
	if s, ok := v.(string); ok {
		return pyRepr(s)
	}
	return pyStr(v)
}

// pyRepr, str için "%r" karşılığıdır (yeterince yakın).
func pyRepr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		q = `"`
	}
	r := strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	out := r.Replace(s)
	if q == "'" {
		out = strings.ReplaceAll(out, "'", `\'`)
	}
	return q + out + q
}

// --- doğrulayıcı ---

type validator struct{ errs []string }

func (v *validator) err(format string, a ...any) { v.errs = append(v.errs, fmt.Sprintf(format, a...)) }

func (v *validator) textOK(x any, what string, maxlen int) bool {
	s, ok := x.(string)
	if !ok || s == "" || pyLen(s) > maxlen || reTextBad.MatchString(s) {
		v.err("%s geçersiz metin", what)
		return false
	}
	return true
}

func varTypeOK(t any) bool {
	s, ok := t.(string)
	if !ok {
		return false
	}
	if inStrs(s, varScalars) {
		return true
	}
	m := reListType.FindStringSubmatch(s)
	return m != nil && inStrs(m[1], varScalars) && m[1] != "enum"
}

type varSpec struct {
	typ string
	v   *obj
}

// Result, geçerli bildirimin düz satır çıktısıdır (Python betiğinin stdout'u).
type Result struct{ Lines []string }

// ValidateFile, component.json dosyasını okuyup doğrular. wantID/wantVer boş değilse paketle eşleşmesi aranır.
// Dönüş: satırlar, hata iletileri (sırasıyla), çıkış kodu (0 geçerli, 1 geçersiz).
func ValidateFile(path, wantID, wantVer string) (lines, errs []string, code int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, []string{"bildirim okunamadı"}, 1
	}
	defer f.Close()
	buf := make([]byte, maxBytes+1)
	n := 0
	for n < len(buf) {
		m, rerr := f.Read(buf[n:])
		n += m
		if rerr != nil {
			break
		}
	}
	if n > maxBytes {
		return nil, []string{"bildirim çok büyük"}, 1
	}
	return Validate(buf[:n], wantID, wantVer)
}

// Validate, ham component.json baytlarını doğrular.
func Validate(raw []byte, wantID, wantVer string) (lines, errs []string, code int) {
	root, derr := decode(raw)
	if derr != nil {
		return nil, []string{"bildirim geçerli JSON değil"}, 1
	}
	d, ok := asObj(root)
	if !ok {
		return nil, []string{"bildirim bir nesne olmalı"}, 1
	}
	v := &validator{}

	// --- kimlik, sürüm, sözleşme sürümü ---
	pid, ver, api := d.get("id"), d.get("version"), d.get("sshpocket_api")
	if s, ok := asStr(pid); !ok || !reID.MatchString(s) {
		v.err("id geçersiz")
	}
	if s, ok := asStr(ver); !ok || !reVersion.MatchString(s) {
		v.err("version geçersiz")
	}
	if wantID != "" && pyStr(pid) != wantID {
		v.err("id paketle uyuşmuyor")
	}
	if wantVer != "" && pyStr(ver) != wantVer {
		v.err("version paketle uyuşmuyor")
	}
	apiN, apiIsInt := asInt(api)
	if !apiIsInt {
		v.err("sshpocket_api tamsayı olmalı")
	} else if apiN != 1 {
		v.err("sshpocket_api %d desteklenmiyor (bu sshpocket: %s); daha yeni bir sshpocket gerekebilir", apiN, "1")
	}
	if rf := d.get("required_features"); rf != nil {
		if l, ok := asList(rf); !ok || len(l) > 0 {
			v.err("bilinmeyen zorunlu özellik (required_features): bu sshpocket sürümü desteklemiyor")
		}
	}

	runner := d.get("runner")
	if !inStrs(runner, runnersList) {
		v.err("runner 'ansible' ya da 'script' olmalı")
	}
	runnerS, _ := asStr(runner)
	for _, k := range []string{"name", "description"} {
		if d.has(k) {
			v.textOK(d.get(k), k, 500)
		}
	}
	group := d.get("inventory_group")
	if group != nil {
		if s, ok := asStr(group); !ok || !reGroup.MatchString(s) {
			v.err("inventory_group geçersiz")
		}
	}

	// --- kontrol makinesi gereksinimi ---
	reqPy := ""
	reqV := d.getDef("requires", newObj())
	if req, ok := asObj(reqV); !ok {
		v.err("requires nesne olmalı")
	} else {
		ctrlV := req.getDef("control", newObj())
		if ctrl, ok := asObj(ctrlV); !ok {
			v.err("requires.control nesne olmalı")
		} else if ctrl.has("python") {
			if s, ok := asStr(ctrl.get("python")); ok && rePyRange.MatchString(s) {
				reqPy = s
			} else {
				v.err("requires.control.python geçersiz (örn. \">=3.12\" veya \">=3.12,<4.0\")")
			}
		}
	}

	// --- profiller ---
	var profLines []string
	if pl, ok := asList(d.get("profiles")); !ok || len(pl) == 0 {
		v.err("profiles boş olamaz")
	} else {
		seen := map[string]bool{}
		for _, p := range pl {
			po, ok := asObj(p)
			if !ok {
				v.err("profil geçersiz (id, status: supported|planned)")
				continue
			}
			id, idOK := asStr(po.get("id"))
			if !idOK || !reProfile.MatchString(id) || !inStrs(po.get("status"), statusesList) {
				v.err("profil geçersiz (id, status: supported|planned)")
				continue
			}
			if seen[id] {
				v.err("yinelenen profil: %s", id)
			}
			seen[id] = true
			st, _ := asStr(po.get("status"))
			profLines = append(profLines, "profile="+id+":"+st)
		}
	}

	// --- eylemler ---
	actions, ok := asObj(d.get("actions"))
	var actLines []string
	if !ok || actions.len() == 0 {
		v.err("actions boş olamaz")
		actions = newObj()
	}
	// değişken bildirimleri (outputs.dir_var denetimi için)
	decl := map[string]any{}
	if vl, ok := asList(d.getDef("vars", []any{})); ok {
		for _, vv := range vl {
			if vo, ok := asObj(vv); ok {
				if n, ok := asStr(vo.get("name")); ok {
					decl[n] = vo.getDef("type", "string")
				}
			}
		}
	}
	for _, name := range actions.keys {
		a := actions.get(name)
		if !reName.MatchString(name) {
			v.err("eylem adı geçersiz: %s", pyRepr(name))
			continue
		}
		ao, ok := asObj(a)
		if !ok {
			v.err("eylem nesne olmalı: %s", name)
			continue
		}
		cmd, cmdOK := asStr(ao.get("cmd"))
		if !cmdOK || strings.TrimSpace(cmd) == "" || pyLen(cmd) > 400 || reCmdBad.MatchString(cmd) {
			v.err("%s: cmd geçersiz (kabuk meta karakteri/satır sonu yok, boş olamaz)", name)
			cmd = ""
		}
		if runnerS == "script" && cmd != "" {
			sw := strings.Fields(cmd)
			if !reScript.MatchString(sw[0]) || strings.Contains(sw[0], "..") {
				v.err("%s: script runner için cmd 'scripts/<ad>.sh [argümanlar]' ile başlamalı", name)
				cmd = ""
			} else {
				bad := false
				for _, w := range sw[1:] {
					if !reScriptArg.MatchString(w) {
						bad = true
					}
				}
				if bad || len(sw) > 9 {
					v.err("%s: script argümanları düz sözcük olmalı (en çok 8; harf/rakam ve . _ = : / + , -)", name)
					cmd = ""
				}
			}
		}
		var effects []string
		effOK := false
		if el, ok := asList(ao.get("effects")); ok {
			effOK = true
			seenE := map[string]bool{}
			for _, e := range el {
				es, ok := asStr(e)
				if !ok || !inStrs(es, effectsList) {
					effOK = false
					break
				}
				seenE[es] = true
			}
			if effOK && len(seenE) != len(el) {
				effOK = false
			}
			if effOK {
				for _, e := range el {
					effects = append(effects, e.(string))
				}
			}
		}
		if !effOK {
			v.err("%s: effects geçersiz (%s)", name, strings.Join(effectsList, "|"))
			effects = nil
		}
		for _, k := range []string{"requires_sudo", "interactive"} {
			if ao.has(k) && !isBoolV(ao.get(k)) {
				v.err("%s: %s bool olmalı", name, k)
			}
		}
		if ao.has("platforms") {
			if !platformListOK(ao.get("platforms"), true) {
				v.err("%s: platforms 1-10 benzersiz, küçük harfli kimlikten oluşan liste olmalı", name)
			}
		}
		confirm := "none"
		confV := ao.getDef("confirm", "none")
		if !inStrs(confV, confirmsList) {
			v.err("%s: confirm geçersiz (%s)", name, strings.Join(confirmsList, "|"))
		} else {
			confirm = confV.(string)
		}
		minimum := "none"
		hasDestr := false
		for _, e := range effects {
			if e == "destructive" {
				hasDestr = true
			}
		}
		if hasDestr {
			minimum = "type-server-name"
		} else if len(effects) > 0 {
			minimum = "yes-no"
		}
		effective := confirm
		if idx(confirmsList, minimum) > idx(confirmsList, confirm) {
			effective = minimum
		}
		var warns []any
		if wl, ok := asList(ao.getDef("warnings", []any{})); !ok || len(wl) > 20 {
			v.err("%s: warnings liste olmalı", name)
		} else {
			warns = wl
		}
		for _, w := range warns {
			v.textOK(w, name+": warning", 500)
		}
		actLines = append(actLines, "action="+name)
		actLines = append(actLines, "action."+name+".cmd="+cmd)
		actLines = append(actLines, "action."+name+".effects="+strings.Join(effects, ","))
		actLines = append(actLines, "action."+name+".sudo="+yesNo(ao.get("requires_sudo") == true))
		actLines = append(actLines, "action."+name+".interactive="+yesNo(ao.get("interactive") == true))
		actLines = append(actLines, "action."+name+".confirm="+effective)
		actLines = append(actLines, "action."+name+".confirm_raised="+yesNo(effective != confirm))
		actLines = append(actLines, fmt.Sprintf("action.%s.warnings=%d", name, len(warns)))
		if pl, ok := asList(ao.get("platforms")); ok && len(pl) > 0 {
			allStr := true
			var ps []string
			for _, x := range pl {
				s, ok := asStr(x)
				if !ok {
					allStr = false
					break
				}
				ps = append(ps, s)
			}
			if allStr {
				actLines = append(actLines, "action."+name+".platforms="+strings.Join(ps, ","))
			}
		}
		for _, w := range warns {
			if s, ok := asStr(w); ok {
				actLines = append(actLines, "action."+name+".warning="+s)
			}
		}
		// outputs (§8)
		if ao.get("outputs") != nil {
			outs, ok := asObj(ao.get("outputs"))
			if !ok {
				v.err("%s: outputs nesne olmalı", name)
			} else {
				dv := outs.get("dir_var")
				if dv != nil && runnerS == "script" {
					v.err("%s: outputs.dir_var script runner'da desteklenmez (betik sunucuda çalışır; sonuç SSHPOCKET_RESULT satırıyla gelir)", name)
				} else if dv != nil {
					dvs, isS := asStr(dv)
					if !isS || decl[dvs] != "path" {
						v.err("%s: outputs.dir_var 'path' türünde bildirilmiş bir değişken olmalı", name)
					} else {
						actLines = append(actLines, "action."+name+".outputs.dir_var="+dvs)
					}
				}
				if fmtV := outs.get("format"); fmtV != nil {
					if fmtV != "keyvalue-line" {
						v.err("%s: outputs.format yalnızca 'keyvalue-line' olabilir", name)
					} else {
						okeys, ok := asList(outs.get("keys"))
						keysOK := ok && len(okeys) > 0 && len(okeys) <= 20
						seenK := map[string]bool{}
						var ks []string
						if keysOK {
							for _, k := range okeys {
								s, ok := asStr(k)
								if !ok || !reOutKey.MatchString(s) {
									keysOK = false
									break
								}
								seenK[s] = true
								ks = append(ks, s)
							}
							if keysOK && len(seenK) != len(okeys) {
								keysOK = false
							}
						}
						if !keysOK {
							v.err("%s: outputs.keys geçersiz (küçük harf/rakam/_ ; en çok 20, yinelenmez)", name)
						} else {
							actLines = append(actLines, "action."+name+".outputs.keys="+strings.Join(ks, ","))
						}
						want := "SSHPOCKET_RESULT " + pyStr(d.get("id")) + "." + name
						if outs.has("prefix") && outs.get("prefix") != want {
							v.err("%s: outputs.prefix '%s' olmalı (sshpocket türetir; paketteki farklıysa reddedilir)", name, want)
						}
						actLines = append(actLines, "action."+name+".outputs.format=keyvalue-line")
					}
				} else if outs.has("keys") || outs.has("prefix") {
					v.err("%s: outputs.keys/prefix için outputs.format gerekir", name)
				}
				if outs.has("summary") {
					v.textOK(outs.get("summary"), name+": outputs.summary", 500)
				}
				if outs.has("findings") {
					fs, ok := asStr(outs.get("findings"))
					if !(ok && reFindings.MatchString(fs) && !strings.Contains(fs, "..")) {
						v.err("%s: outputs.findings düz dosya adı olmalı (.tsv; '/' yok; en çok 64 karakter)", name)
					} else {
						actLines = append(actLines, "action."+name+".outputs.findings="+fs)
					}
				}
			}
		}
	}

	// requires: var olan eylemlere, kendine ve döngüsüz
	graph := map[string][]string{}
	var graphOrder []string
	for _, name := range actions.keys {
		ao, ok := asObj(actions.get(name))
		if !ok {
			continue
		}
		reqL, ok := asList(ao.getDef("requires", []any{}))
		allStr := ok
		if ok {
			for _, r := range reqL {
				if !isStr(r) {
					allStr = false
				}
			}
		}
		if !ok || !allStr {
			v.err("%s: requires liste olmalı", name)
			continue
		}
		var known []string
		for _, r := range reqL {
			rs := r.(string)
			if !actions.has(rs) {
				v.err("%s: requires bilinmeyen eylem: %s", name, rs)
			}
			if rs == name {
				v.err("%s: requires kendini gösteremez", name)
			}
			if actions.has(rs) {
				known = append(known, rs)
			}
		}
		graph[name] = known
		graphOrder = append(graphOrder, name)
		mine := ao.get("platforms")
		for _, r := range known {
			ro, rok := asObj(actions.get(r))
			var theirs any
			if rok {
				theirs = ro.get("platforms")
			}
			if tl, ok := asList(theirs); ok {
				ml, mok := asList(mine)
				bad := !mok
				if mok {
					for _, x := range ml {
						if !listHas(tl, x) {
							bad = true
						}
					}
				}
				if bad {
					v.err("%s: requires %s, ama %s bu eylemin platformlarının hepsinde yok", name, r, r)
				}
			}
		}
		actLines = append(actLines, "action."+name+".requires="+strings.Join(known, ","))
	}
	if hasCycle(graph, graphOrder) {
		v.err("requires döngüsü var")
	}

	// --- değişkenler ---
	var varLines []string
	seenVars := map[string]bool{}
	varSpecs := map[string]varSpec{}
	varsL, ok := asList(d.getDef("vars", []any{}))
	if !ok {
		v.err("vars liste olmalı")
		varsL = nil
	}
	for _, vv := range varsL {
		typed := 1
		var vo *obj
		if s, ok := asStr(vv); ok { // eski biçim: yalnızca ad
			vo = newObj()
			vo.set("name", s)
			vo.set("type", "string")
			typed = 0
		} else if o, ok := asObj(vv); ok {
			vo = o
		}
		var n string
		nameOK := false
		if vo != nil {
			if s, ok := asStr(vo.get("name")); ok && reName.MatchString(s) {
				n = s
				nameOK = true
			}
		}
		if !nameOK {
			v.err("vars: değişken adı geçersiz")
			continue
		}
		if seenVars[n] {
			v.err("vars: yinelenen değişken: %s", n)
		}
		seenVars[n] = true
		tV := vo.getDef("type", "string")
		if !varTypeOK(tV) {
			v.err("%s: değişken türü geçersiz", n)
			continue
		}
		t := tV.(string)
		if vo.has("required") && !isBoolV(vo.get("required")) {
			v.err("%s: required bool olmalı", n)
		}
		if vo.has("group") {
			g, ok := asStr(vo.get("group"))
			if !ok || strings.TrimSpace(g) == "" || pyLen(g) > 40 || reTextBad.MatchString(g) {
				v.err("%s: group 1-40 karakterlik düz metin olmalı", n)
			}
		}
		if vo.has("platforms") {
			if !platformListOK(vo.get("platforms"), false) {
				v.err("%s: platforms 1-10 küçük harfli kimlikten oluşan liste olmalı", n)
			}
		}
		if vo.has("description") {
			v.textOK(vo.get("description"), n+": description", varDescMax)
		}
		if t == "enum" {
			vals, ok := asList(vo.get("values"))
			bad := !ok || len(vals) == 0
			if !bad {
				for _, x := range vals {
					s, ok := asStr(x)
					if !ok || reTextBad.MatchString(s) {
						bad = true
					}
				}
			}
			if bad {
				v.err("%s: enum için values gerekir", n)
			}
		}
		if t == "int" {
			lo, hi := vo.get("min"), vo.get("max")
			if lo != nil && !isInt(lo) {
				v.err("%s: min tamsayı olmalı", n)
			}
			if hi != nil && !isInt(hi) {
				v.err("%s: max tamsayı olmalı", n)
			}
			loN, loOK := asInt(lo)
			hiN, hiOK := asInt(hi)
			if loOK && hiOK && loN > hiN {
				v.err("%s: min > max", n)
			}
			if dv := vo.get("default"); dv != nil {
				dn, dok := asInt(dv)
				if !dok || (loOK && dn < loN) || (hiOK && dn > hiN) {
					v.err("%s: default türü/aralığı geçersiz", n)
				}
			}
		}
		if t == "bool" {
			if vo.has("default") && !isBoolV(vo.get("default")) {
				v.err("%s: bool default geçersiz", n)
			}
			if vo.has("warn_if") && !isBoolV(vo.get("warn_if")) {
				v.err("%s: warn_if bool olmalı", n)
			}
		}
		varSpecs[n] = varSpec{t, vo}
		varLines = append(varLines, fmt.Sprintf("var=%s:%s:%s:%d", n, t, yesNo(vo.get("required") == true), typed))
	}

	// --- platform_var ---
	if d.has("platform_var") {
		pv, ok := asStr(d.get("platform_var"))
		sp, found := varSpecs[pv]
		if !ok || !found || (sp.typ != "string" && sp.typ != "enum") {
			v.err("platform_var tanımlı bir string/enum değişkenin adı olmalı")
		}
	}

	// --- eylem düzeyi sabit değişkenler (action.set) ---
	for _, name := range actions.keys {
		ao, ok := asObj(actions.get(name))
		if !ok || !ao.has("set") {
			continue
		}
		sv, ok := asObj(ao.get("set"))
		if !ok || sv.len() == 0 || sv.len() > 8 {
			v.err("%s: set boş olmayan (en çok 8) nesne olmalı", name)
			continue
		}
		var dirVar any
		if oo, ok := asObj(ao.get("outputs")); ok {
			dirVar = oo.get("dir_var")
		}
		keys := append([]string(nil), sv.keys...)
		sort.Strings(keys)
		for _, k := range keys {
			val := sv.get(k)
			sp, found := varSpecs[k]
			if !found {
				v.err("%s: set bilinmeyen değişken: %s", name, k)
				continue
			}
			if ds, ok := asStr(dirVar); ok && k == ds {
				v.err("%s: set, outputs.dir_var değişkenini (%s) ezemez", name, k)
				continue
			}
			okVal := false
			switch sp.typ {
			case "bool":
				okVal = isBoolV(val)
			case "int":
				if n, ok := asInt(val); ok {
					okVal = true
					if lo, ok := asInt(sp.v.get("min")); ok && n < lo {
						okVal = false
					}
					if hi, ok := asInt(sp.v.get("max")); ok && n > hi {
						okVal = false
					}
				}
			case "enum":
				if s, ok := asStr(val); ok {
					if vl, ok := asList(sp.v.getDef("values", []any{})); ok && listHas(vl, s) {
						okVal = true
					}
				}
			case "string", "path":
				okVal = scalarStrOK(val)
			}
			if !okVal {
				v.err("%s: set %s için geçersiz değer/tür (yalnızca bool, int, enum, string, path; liste yok)", name, k)
				continue
			}
			actLines = append(actLines, "action."+name+".set="+k+"="+valText(val))
		}
	}

	// --- platform seçimi ve eylem düzeyi zorunlu değişkenler ---
	var platLines []string
	if s, ok := asStr(d.get("platform_var")); ok {
		if _, found := varSpecs[s]; found {
			platLines = append(platLines, "platform_var="+s)
		}
	}
	declared := func(n any, what string, want ...string) bool {
		s, ok := asStr(n)
		sp, found := varSpecs[s]
		if !ok || !found {
			nm := "?"
			if ok {
				nm = s
			}
			v.err("%s: tanımlı bir değişken adı olmalı (%s)", what, nm)
			return false
		}
		if len(want) > 0 && idx(want, sp.typ) < 0 {
			v.err("%s: %s için tür %s olmalı", what, s, strings.Join(want, "/"))
			return false
		}
		return true
	}
	scalarOK := func(n string, val any) bool {
		sp := varSpecs[n]
		switch sp.typ {
		case "bool":
			return isBoolV(val)
		case "enum":
			if s, ok := asStr(val); ok {
				if vl, ok := asList(sp.v.getDef("values", []any{})); ok {
					return listHas(vl, s)
				}
			}
			return false
		case "string", "path":
			return scalarStrOK(val)
		}
		return false
	}

	for _, name := range actions.keys {
		ao, ok := asObj(actions.get(name))
		if !ok || !ao.has("requires_vars") {
			continue
		}
		rv, ok := asList(ao.get("requires_vars"))
		if !ok || len(rv) == 0 || len(rv) > 8 || len(uniqStr(rv)) != len(rv) {
			v.err("%s: requires_vars 1-8 benzersiz değişken adından oluşan liste olmalı", name)
			continue
		}
		allOK := true
		for _, n := range rv { // Python all(): ilk başarısızlıkta durur
			if !declared(n, name+": requires_vars") {
				allOK = false
				break
			}
		}
		if allOK {
			var ns []string
			for _, n := range rv {
				ns = append(ns, n.(string))
			}
			actLines = append(actLines, "action."+name+".requires_vars="+strings.Join(ns, ","))
		}
	}

	if d.get("onboarding_vars") != nil {
		ob, ok := asList(d.get("onboarding_vars"))
		if !ok || len(ob) > 8 || len(uniqStr(ob)) != len(ob) {
			v.err("onboarding_vars en çok 8 benzersiz değişken adından oluşan liste olmalı")
		} else {
			for _, n := range ob {
				if declared(n, "onboarding_vars") {
					s := n.(string)
					if varSpecs[s].v.has("warn_if") {
						v.err("onboarding_vars: %s riskli değer (warn_if) taşıyor; sunucu eklerken sorulamaz", s)
					} else {
						platLines = append(platLines, "onboarding_var="+s)
					}
				}
			}
		}
	}

	if d.get("platform_choices") != nil {
		pc, ok := asList(d.get("platform_choices"))
		if !ok || len(pc) == 0 || len(pc) > 10 {
			v.err("platform_choices 1-10 öğeli liste olmalı")
			pc = nil
		}
		seenIDs := map[string]bool{}
		for i, cv := range pc {
			w := fmt.Sprintf("platform_choices[%d]", i)
			c, ok := asObj(cv)
			var cid string
			idOK := false
			if ok {
				if s, ok := asStr(c.get("id")); ok && rePlatform.MatchString(s) && !seenIDs[s] {
					cid, idOK = s, true
				}
			}
			if !idOK {
				v.err("%s: id benzersiz, küçük harfli kimlik olmalı", w)
				continue
			}
			seenIDs[cid] = true
			label, lok := asStr(c.get("label"))
			if !lok || strings.TrimSpace(label) == "" || pyLen(label) > 60 || reTextBad.MatchString(label) {
				v.err("%s: label 1-60 karakterlik düz metin olmalı", w)
				continue
			}
			if c.has("description") {
				v.textOK(c.get("description"), w+": description", varDescMax)
			}
			if c.has("auto_detect") && !isBoolV(c.get("auto_detect")) {
				v.err("%s: auto_detect bool olmalı", w)
			}
			platLines = append(platLines, "platform_choice="+cid)
			platLines = append(platLines, "platform_choice."+cid+".label="+label)
			if ds, ok := asStr(c.get("description")); ok && ds != "" {
				platLines = append(platLines, "platform_choice."+cid+".description="+ds)
			}
			platLines = append(platLines, "platform_choice."+cid+".auto_detect="+yesNo(c.get("auto_detect") != false))
			for _, kt := range [][2]string{{"defaults", "default"}, {"apply_vars", "apply_var"}} {
				key, tag := kt[0], kt[1]
				dvv := c.get(key)
				if dvv == nil {
					continue
				}
				dv, ok := asObj(dvv)
				if !ok || dv.len() > 8 {
					v.err("%s: %s en çok 8 değişkenli nesne olmalı (boş nesne kabul edilir)", w, key)
					continue
				}
				keys := append([]string(nil), dv.keys...)
				sort.Strings(keys)
				for _, k := range keys {
					if !declared(k, w+": "+key) {
						continue
					}
					sp := varSpecs[k]
					if sp.v.has("warn_if") && key == "defaults" {
						v.err("%s: defaults %s riskli değer (warn_if) taşıyan değişkeni yazamaz", w, k)
					} else if key == "apply_vars" && sp.typ != "bool" && sp.typ != "enum" {
						v.err("%s: apply_vars %s için tür bool/enum olmalı", w, k)
					} else if !scalarOK(k, dv.get(k)) {
						v.err("%s: %s %s için geçersiz değer/tür", w, key, k)
					} else {
						platLines = append(platLines, "platform_choice."+cid+"."+tag+"="+k+"="+valText(dv.get(k)))
					}
				}
			}
			aw := c.get("apply_warnings")
			if truthy(c.get("apply_vars")) && (!isListV(aw) || len(aw.([]any)) == 0 || len(aw.([]any)) > 10) {
				v.err("%s: apply_vars doluysa apply_warnings (1-10 metin) zorunlu", w)
			} else if aw != nil {
				al, ok := asList(aw)
				if !ok || len(al) > 10 {
					v.err("%s: apply_warnings liste olmalı", w)
				} else {
					for _, t := range al {
						if v.textOK(t, w+": apply_warnings", 500) {
							platLines = append(platLines, "platform_choice."+cid+".apply_warning="+t.(string))
						}
					}
				}
			}
		}
	}

	// --- invoke.append ---
	var invLines []string
	invV := d.getDef("invoke", newObj())
	if runnerS == "script" && (truthy(invV) || d.get("inventory_group") != nil) {
		v.err("invoke ve inventory_group yalnızca ansible runner içindir")
		invV = newObj()
	}
	var app []any
	if io, ok := asObj(invV); ok {
		if al, ok := asList(io.getDef("append", []any{})); ok {
			app = al
		} else {
			v.err("invoke.append liste olmalı")
		}
	} else {
		v.err("invoke.append liste olmalı")
	}
	for i, ev := range app {
		e, ok := asObj(ev)
		arg, aok := "", false
		if ok {
			arg, aok = asStr(e.get("arg"))
		}
		if !ok || !aok || !strings.HasPrefix(arg, "-") || reCmdBad.MatchString(arg) || strings.Contains(arg, " ") {
			v.err("invoke.append[%d]: arg geçersiz", i)
			continue
		}
		val := e.get("value")
		valS := ""
		if val != nil {
			s, ok := asStr(val)
			if !ok || !reValue.MatchString(s) {
				v.err("invoke.append[%d]: value geçersiz", i)
				continue
			}
			valS = s
		}
		for _, k := range []string{"required", "interactive"} {
			if e.has(k) && !isBoolV(e.get(k)) {
				v.err("invoke.append[%d]: %s bool olmalı", i, k)
			}
		}
		invLines = append(invLines, "invoke="+arg+"|"+valS+"|"+yesNo(e.get("required") == true)+"|"+yesNo(e.get("interactive") == true))
	}

	// --- items: yazılım kataloğu (S2) ---
	var itemLines []string
	if d.get("items") != nil {
		its, ok := asList(d.get("items"))
		if !ok || len(its) < 1 || len(its) > 30 {
			v.err("items 1-30 öğeli liste olmalı")
			its = nil
		}
		iv, mv := d.get("item_var"), d.get("mode_var")
		ivS, ivOK := asStr(iv)
		if sp, f := varSpecs[ivS]; !(ivOK && f && (sp.typ == "string" || sp.typ == "enum")) {
			v.err("items için item_var tanımlı bir string/enum değişkenin adı olmalı")
			ivS = ""
		}
		mvS := ""
		if mv != nil {
			s, ok := asStr(mv)
			sp, f := varSpecs[s]
			if !(ok && f && (sp.typ == "string" || sp.typ == "enum") && s != ivS) {
				v.err("mode_var tanımlı, item_var'dan farklı bir string/enum değişkenin adı olmalı")
			} else {
				mvS = s
			}
		}
		if ivS != "" {
			itemLines = append(itemLines, "item_var="+ivS)
		}
		if mvS != "" {
			itemLines = append(itemLines, "mode_var="+mvS)
		}
		seenItems := map[string]bool{}
		for i, itv := range its {
			w := fmt.Sprintf("items[%d]", i)
			it, ok := asObj(itv)
			if !ok {
				v.err("%s nesne olmalı", w)
				continue
			}
			for _, k := range it.keys {
				switch k {
				case "id", "label", "description", "modes", "actions", "params":
				default:
					v.err("%s: bilinmeyen alan: %s", w, k)
				}
			}
			iid, iidOK := asStr(it.get("id"))
			if !(iidOK && reItem.MatchString(iid)) || seenItems[iid] {
				v.err("%s: id geçersiz ya da yinelenmiş", w)
				continue
			}
			seenItems[iid] = true
			if ivS != "" && varSpecs[ivS].typ == "enum" {
				if vl, ok := asList(varSpecs[ivS].v.getDef("values", []any{})); !ok || !listHas(vl, iid) {
					v.err("%s: %s, item_var enum değerlerinde yok", w, iid)
				}
			}
			if v.textOK(it.get("label"), w+".label", 60) {
				itemLines = append(itemLines, "item="+iid+"|"+it.get("label").(string))
			}
			if it.has("description") && v.textOK(it.get("description"), w+".description", 500) {
				itemLines = append(itemLines, "item."+iid+".description="+it.get("description").(string))
			}
			if it.get("modes") != nil {
				modes, ok := asList(it.get("modes"))
				mOK := ok && len(modes) >= 1 && len(modes) <= 5
				if mOK {
					for _, m := range modes {
						if s, ok := asStr(m); !ok || !reItem.MatchString(s) {
							mOK = false
						}
					}
					if mOK && len(uniqStr(modes)) != len(modes) {
						mOK = false
					}
				}
				if !mOK {
					v.err("%s: modes 1-5 benzersiz küçük harfli kimlik olmalı", w)
				} else if mvS == "" {
					v.err("%s: modes için mode_var gerekir", w)
				} else {
					if varSpecs[mvS].typ == "enum" {
						vl, _ := asList(varSpecs[mvS].v.getDef("values", []any{}))
						for _, m := range modes {
							if !listHas(vl, m) {
								v.err("%s: mod %s, mode_var enum değerlerinde yok", w, m.(string))
							}
						}
					}
					var ms []string
					for _, m := range modes {
						ms = append(ms, m.(string))
					}
					itemLines = append(itemLines, "item."+iid+".modes="+strings.Join(ms, ","))
				}
			}
			if it.get("actions") != nil {
				acts, ok := asList(it.get("actions"))
				aOK := ok && len(acts) > 0
				if aOK {
					for _, a := range acts {
						if s, ok := asStr(a); !ok || !actions.has(s) {
							aOK = false
						}
					}
				}
				if !aOK {
					v.err("%s: actions boş olmayan, tanımlı eylem adları listesi olmalı", w)
				} else {
					var as []string
					for _, a := range acts {
						as = append(as, a.(string))
					}
					for _, a := range as {
						if ao, ok := asObj(actions.get(a)); ok {
							if rl, ok := asList(ao.get("requires")); ok {
								for _, r := range rl {
									if !listHas(acts, r) {
										v.err("%s: %s eyleminin ön koşulu %s da actions içinde olmalı", w, a, pyStr(r))
									}
								}
							}
						}
					}
					itemLines = append(itemLines, "item."+iid+".actions="+strings.Join(as, ","))
				}
			}
			if it.get("params") != nil {
				prm, ok := asList(it.get("params"))
				pOK := ok && len(prm) <= 12
				if pOK {
					for _, x := range prm {
						if s, ok := asStr(x); !ok {
							pOK = false
						} else if _, f := varSpecs[s]; !f {
							pOK = false
						}
					}
				}
				if !pOK {
					v.err("%s: params en çok 12 tanımlı değişken adı olmalı", w)
				} else {
					var ps []string
					for _, x := range prm {
						ps = append(ps, x.(string))
					}
					itemLines = append(itemLines, "item."+iid+".params="+strings.Join(ps, ","))
				}
			}
		}
	} else if d.has("item_var") || d.has("mode_var") {
		v.err("item_var/mode_var yalnızca items ile birlikte kullanılır")
	}

	// --- menu ---
	var menuLines []string
	if d.get("menu") != nil {
		mn, ok := asObj(d.get("menu"))
		if !ok {
			v.err("menu nesne olmalı")
		} else {
			for _, k := range mn.keys {
				switch k {
				case "category", "title", "order":
				default:
					v.err("menu: bilinmeyen alan: %s", k)
				}
			}
			cat := mn.getDef("category", "paketler")
			if !inStrs(cat, menuCats) {
				v.err("menu.category şunlardan biri olmalı: %s", strings.Join(menuCats, ", "))
			} else {
				menuLines = append(menuLines, "menu.category="+cat.(string))
			}
			if mn.has("title") {
				if v.textOK(mn.get("title"), "menu.title", 60) {
					menuLines = append(menuLines, "menu.title="+mn.get("title").(string))
				}
			}
			if mn.has("order") {
				o, ok := asInt(mn.get("order"))
				if !ok || o < 0 || o > 999 {
					v.err("menu.order 0-999 arası tamsayı olmalı")
				} else {
					menuLines = append(menuLines, fmt.Sprintf("menu.order=%d", o))
				}
			}
		}
	}

	// --- sections (isteğe bağlı, yalnız Go: kabuk/Python şartnamesinde yok; arayüz kartları) ---
	secLines := validateSections(v, d, actions, varSpecs)

	if len(v.errs) > 0 {
		return nil, v.errs, 1
	}
	out := []string{"id=" + pid.(string), "version=" + ver.(string), fmt.Sprintf("sshpocket_api=%d", apiN), "runner=" + runnerS}
	if gs, ok := asStr(group); ok && gs != "" {
		out = append(out, "inventory_group="+gs)
	}
	if reqPy != "" {
		out = append(out, "requires.python="+reqPy)
	}
	for _, part := range [][]string{profLines, actLines, varLines, platLines, invLines, itemLines, menuLines, secLines} {
		out = append(out, part...)
	}
	return out, nil, 0
}

// --- küçük yardımcılar ---

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func valText(v any) string {
	if b, ok := v.(bool); ok {
		if b {
			return "true"
		}
		return "false"
	}
	return pyStr(v)
}

func scalarStrOK(v any) bool {
	s, ok := asStr(v)
	return ok && s != "" && pyLen(s) <= 200 && !reTextBad.MatchString(s)
}

func listHas(l []any, x any) bool {
	for _, y := range l {
		if y == x {
			return true
		}
	}
	return false
}

func uniqStr(l []any) map[string]bool {
	m := map[string]bool{}
	for _, x := range l {
		m[pyStr(x)] = true
	}
	return m
}

// platformListOK: 1-10 öğe, hepsi geçerli kimlik; unique true ise benzersiz de olmalı.
func platformListOK(x any, unique bool) bool {
	l, ok := asList(x)
	if !ok || len(l) == 0 || len(l) > 10 {
		return false
	}
	seen := map[string]bool{}
	for _, e := range l {
		s, ok := asStr(e)
		if !ok || !rePlatform.MatchString(s) {
			return false
		}
		seen[s] = true
	}
	if unique && len(seen) != len(l) {
		return false
	}
	return true
}

func hasCycle(graph map[string][]string, order []string) bool {
	state := map[string]int{}
	var visit func(n string) bool
	visit = func(n string) bool {
		state[n] = 1
		for _, m := range graph[n] {
			if state[m] == 1 || (state[m] == 0 && visit(m)) {
				return true
			}
		}
		state[n] = 2
		return false
	}
	for _, n := range order {
		if state[n] == 0 && visit(n) {
			return true
		}
	}
	return false
}
