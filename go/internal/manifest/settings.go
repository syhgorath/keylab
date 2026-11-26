package manifest

import (
	"fmt"
	"strings"
)

// settings: bileşen ayarları (bkz. docs/ayar-semasi-taslak.md, P1). İsteğe bağlı ÜST DÜZEY alan; sshpocket_api artmaz.
// Yeni bir çalıştırma yolu açmaz: ayarı uygulamak = paketin tanımlı eylemini (apply_via) mevcut kapılarıyla çalıştırmak.
// `items[].settings` BİLEREK desteklenmez: eski sürümler öğelerdeki bilinmeyen alanı reddeder (paket kurulamaz); üst düzeyde yok sayarlar.

const (
	setMax       = 40
	setLabelMax  = 60
	setDescMax   = 300
	setNoteMax   = 160
	setKeysMax   = 20
	setPrefixMax = 5
	setApplyMax  = 5
)

var (
	setFields     = []string{"id", "label", "var", "description", "risk", "risk_note", "apply_via", "state"}
	setStateKeys  = []string{"action", "key", "keys", "compare", "source", "strip_prefix"}
	setRisks      = []string{"access-loss", "restarts-service", "time-jump", "data-loss"}
	setComparisns = []string{"exact", "set", "ordered", "present"}
)

// uniqueStrings, x 1-max öğeli, yinelenmeyen metin listesiyse onu döndürür.
func uniqueStrings(x any, max int) ([]string, bool) {
	l, ok := asList(x)
	if !ok || len(l) < 1 || len(l) > max {
		return nil, false
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(l))
	for _, e := range l {
		s, ok := asStr(e)
		if !ok || seen[s] {
			return nil, false
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, true
}

// actionOutKeys: eylemin outputs.keys listesi.
func actionOutKeys(actions *obj, name string) []string {
	ao, ok := asObj(actions.get(name))
	if !ok {
		return nil
	}
	outs, ok := asObj(ao.get("outputs"))
	if !ok {
		return nil
	}
	l, _ := asList(outs.get("keys"))
	var out []string
	for _, k := range l {
		if s, ok := asStr(k); ok {
			out = append(out, s)
		}
	}
	return out
}

// validateSettings, "settings" alanını denetler ve çıktı satırlarını üretir:
// setting.<id>=<etiket>, setting.<id>.var|apply|risk|described|read|keys|compare=...
func validateSettings(v *validator, d *obj, actions *obj, varSpecs map[string]varSpec) []string {
	if !d.has("settings") {
		return nil
	}
	list, ok := asList(d.get("settings"))
	if !ok || len(list) < 1 || len(list) > setMax {
		v.err("settings 1-%d öğeli liste olmalı", setMax)
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for i, x := range list {
		w := fmt.Sprintf("settings[%d]", i)
		so, ok := asObj(x)
		if !ok {
			v.err("%s nesne olmalı", w)
			continue
		}
		for _, k := range so.keys {
			if !inStrs(k, setFields) {
				v.err("%s: bilinmeyen alan: %s", w, k)
			}
		}
		id, idOK := asStr(so.get("id"))
		if !idOK || !reItem.MatchString(id) {
			v.err("%s: id küçük harfle başlayan, 1-32 karakterlik kimlik olmalı", w)
			continue
		}
		if seen[id] {
			v.err("%s: yinelenen id: %s", w, id)
			continue
		}
		seen[id] = true
		var lines []string
		if v.textOK(so.get("label"), w+".label", setLabelMax) {
			lines = append(lines, "setting."+id+"="+so.get("label").(string))
		}
		vn, vnOK := asStr(so.get("var"))
		spec, declared := varSpecs[vn]
		if !vnOK || !declared {
			v.err("%s: var, vars[] içinde tanımlı bir değişkenin adı olmalı", w)
		} else {
			lines = append(lines, "setting."+id+".var="+vn)
		}
		described := false
		if so.has("description") {
			described = v.textOK(so.get("description"), w+".description", setDescMax)
		} else if declared {
			s, ok := asStr(spec.v.get("description"))
			described = ok && s != ""
		}
		lines = append(lines, "setting."+id+".described="+yesNo(described))
		if so.has("risk") {
			rs, ok := uniqueStrings(so.get("risk"), len(setRisks))
			for _, r := range rs {
				ok = ok && inStrs(r, setRisks)
			}
			if !ok {
				v.err("%s: risk şunlardan oluşan, yinelenmeyen liste olmalı: %s", w, strings.Join(setRisks, ", "))
			} else {
				lines = append(lines, "setting."+id+".risk="+strings.Join(rs, ","))
			}
		}
		if so.has("risk_note") {
			v.textOK(so.get("risk_note"), w+".risk_note", setNoteMax)
		}
		apply, aok := uniqueStrings(so.get("apply_via"), setApplyMax)
		for _, a := range apply {
			aok = aok && actions.has(a)
		}
		if !aok {
			v.err("%s: apply_via 1-%d tanımlı eylem adından oluşan, yinelenmeyen liste olmalı", w, setApplyMax)
		} else {
			lines = append(lines, "setting."+id+".apply="+strings.Join(apply, ","))
		}
		if so.has("state") {
			lines = append(lines, validateSettingState(v, w, id, so.get("state"), actions)...)
		}
		out = append(out, lines...)
	}
	return out
}

// validateSettingState: sunucudaki gerçek değerin kaynağı (salt okunur eylem + sonuç satırı anahtarları).
func validateSettingState(v *validator, w, id string, x any, actions *obj) []string {
	st, ok := asObj(x)
	if !ok {
		v.err("%s: state nesne olmalı", w)
		return nil
	}
	for _, k := range st.keys {
		if !inStrs(k, setStateKeys) {
			v.err("%s.state: bilinmeyen alan: %s", w, k)
		}
	}
	if st.has("source") && st.get("source") != "result" {
		v.err("%s.state: source yalnızca 'result' olabilir (bu sürümde 'summary' yok)", w)
	}
	action, ok := asStr(st.get("action"))
	if !ok || !actions.has(action) {
		v.err("%s.state: action tanımlı bir eylem olmalı", w)
		return nil
	}
	ao, _ := asObj(actions.get(action))
	readOnly := false
	if ao != nil {
		eff, ok := asList(ao.get("effects"))
		readOnly = ok && len(eff) == 0
	}
	if !readOnly {
		v.err("%s.state: action salt okunur olmalı (effects boş): %s", w, action)
	}
	if st.has("key") == st.has("keys") {
		v.err("%s.state: 'key' ya da 'keys' alanlarından tam biri gerekir", w)
		return nil
	}
	var keys []string
	if st.has("key") {
		k, ok := asStr(st.get("key"))
		if !ok || k == "" {
			v.err("%s.state: key metin olmalı", w)
			return nil
		}
		keys = []string{k}
	} else {
		ks, ok := uniqueStrings(st.get("keys"), setKeysMax)
		if !ok {
			v.err("%s.state: keys 1-%d yinelenmeyen anahtar adından oluşmalı", w, setKeysMax)
			return nil
		}
		keys = ks
	}
	declared := actionOutKeys(actions, action)
	for _, k := range keys {
		if idx(declared, k) < 0 {
			v.err("%s.state: '%s' anahtarı %s eyleminin outputs.keys listesinde yok", w, k, action)
		}
	}
	compare := "exact"
	if st.has("compare") {
		c, ok := asStr(st.get("compare"))
		if !ok || !inStrs(c, setComparisns) {
			v.err("%s.state: compare şunlardan biri olmalı: %s", w, strings.Join(setComparisns, ", "))
		} else {
			compare = c
		}
	}
	if st.has("keys") && compare == "exact" {
		v.err("%s.state: keys ile compare 'set', 'ordered' ya da 'present' olmalı", w)
	}
	if st.has("strip_prefix") {
		ps, ok := uniqueStrings(st.get("strip_prefix"), setPrefixMax)
		for _, p := range ps {
			ok = ok && p != "" && pyLen(p) <= 20 && !reTextBad.MatchString(p)
		}
		if !ok {
			v.err("%s.state: strip_prefix en çok %d kısa metin olmalı", w, setPrefixMax)
		}
	}
	return []string{"setting." + id + ".read=" + action, "setting." + id + ".keys=" + strings.Join(keys, ","), "setting." + id + ".compare=" + compare}
}
