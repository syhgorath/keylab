package manifest

import (
	"fmt"
	"regexp"
	"strings"
)

// settings: bileşen ayarları (bkz. docs/ayar-semasi-taslak.md, P1). İsteğe bağlı ÜST DÜZEY alan; sshpocket_api artmaz.
// Yeni bir çalıştırma yolu açmaz: ayarı uygulamak = paketin tanımlı eylemini (apply_via) mevcut kapılarıyla çalıştırmak.
// `items[].settings` BİLEREK desteklenmez: eski sürümler öğelerdeki bilinmeyen alanı reddeder (paket kurulamaz); üst düzeyde yok sayarlar.

const (
	setMax       = 40
	setCardMax   = 10
	setCardDesc  = 200
	setLabelMax  = 60
	setDescMax   = 300
	setNoteMax   = 160
	setKeysMax   = 20
	setPrefixMax = 5
	setApplyMax  = 5
	setIgnoreMax = 5
	setSumMax    = 6
	setSumLabel  = 30
	setSumUnit   = 8
)

var (
	setFields     = []string{"id", "label", "var", "description", "risk", "risk_note", "apply_via", "state", "card"}
	setCardFields = []string{"id", "label", "description", "summary"}
	setSumFields  = []string{"action", "items"}
	setItemFields = []string{"key", "label", "ok", "max_abs", "unit"}
	setStateKeys  = []string{"action", "key", "keys", "compare", "source", "strip_prefix", "absent", "ignore", "not_applicable"}
	reResultValue = regexp.MustCompile(`^[A-Za-z0-9_.:+-]{1,64}$`)
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
		if d.has("setting_cards") {
			v.err("setting_cards yalnızca settings ile birlikte kullanılır")
		}
		return nil
	}
	cards, cardOrder, out := validateSettingCards(v, d, actions)
	used := map[string]bool{}
	list, ok := asList(d.get("settings"))
	if !ok || len(list) < 1 || len(list) > setMax {
		v.err("settings 1-%d öğeli liste olmalı", setMax)
		return nil
	}
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
		if so.has("card") {
			c, ok := asStr(so.get("card"))
			if !ok || !cards[c] {
				v.err("%s: card, setting_cards içinde tanımlı bir kimlik olmalı", w)
			} else {
				used[c] = true
				lines = append(lines, "setting."+id+".card="+c)
			}
		}
		out = append(out, lines...)
	}
	for _, c := range cardOrder {
		if !used[c] {
			v.err("setting_cards: '%s' kartını kullanan ayar yok", c)
		}
	}
	return out
}

// validateSettingCards: isteğe bağlı "setting_cards" (arayüz kartları; ayar "card" alanıyla bağlanır).
func validateSettingCards(v *validator, d *obj, actions *obj) (map[string]bool, []string, []string) {
	ids := map[string]bool{}
	if !d.has("setting_cards") {
		return ids, nil, nil
	}
	list, ok := asList(d.get("setting_cards"))
	if !ok || len(list) < 1 || len(list) > setCardMax {
		v.err("setting_cards 1-%d öğeli liste olmalı", setCardMax)
		return ids, nil, nil
	}
	var order, out []string
	for i, x := range list {
		w := fmt.Sprintf("setting_cards[%d]", i)
		co, ok := asObj(x)
		if !ok {
			v.err("%s nesne olmalı", w)
			continue
		}
		for _, k := range co.keys {
			if !inStrs(k, setCardFields) {
				v.err("%s: bilinmeyen alan: %s", w, k)
			}
		}
		id, idOK := asStr(co.get("id"))
		if !idOK || !reItem.MatchString(id) || ids[id] {
			v.err("%s: id geçersiz ya da yinelenmiş", w)
			continue
		}
		ids[id] = true
		order = append(order, id)
		if v.textOK(co.get("label"), w+".label", setLabelMax) {
			out = append(out, "setting_card."+id+"="+co.get("label").(string))
		}
		if co.has("description") {
			v.textOK(co.get("description"), w+".description", setCardDesc)
		}
		if co.has("summary") {
			out = append(out, validateCardSummary(v, w+".summary", id, co.get("summary"), actions)...)
		}
	}
	return ids, order, out
}

// readOnlyAction: tanımlı ve effects'i boş eylem.
func readOnlyAction(actions *obj, name string) bool {
	ao, ok := asObj(actions.get(name))
	if !ok {
		return false
	}
	eff, ok := asList(ao.get("effects"))
	return ok && len(eff) == 0
}

// validateCardSummary: kartın sağlık özeti (salt okunur eylemin sonuç anahtarları; beklenen değer ya da mutlak sınır).
func validateCardSummary(v *validator, w, id string, x any, actions *obj) []string {
	so, ok := asObj(x)
	if !ok {
		v.err("%s nesne olmalı", w)
		return nil
	}
	for _, k := range so.keys {
		if !inStrs(k, setSumFields) {
			v.err("%s: bilinmeyen alan: %s", w, k)
		}
	}
	action, ok := asStr(so.get("action"))
	if !ok || !readOnlyAction(actions, action) {
		v.err("%s.action tanımlı, salt okunur (effects boş) bir eylem olmalı", w)
		return nil
	}
	items, ok := asList(so.get("items"))
	if !ok || len(items) < 1 || len(items) > setSumMax {
		v.err("%s.items 1-%d öğeli liste olmalı", w, setSumMax)
		return nil
	}
	declared := actionOutKeys(actions, action)
	seen := map[string]bool{}
	var keys []string
	for i, x := range items {
		iw := fmt.Sprintf("%s.items[%d]", w, i)
		it, ok := asObj(x)
		if !ok {
			v.err("%s nesne olmalı", iw)
			continue
		}
		for _, k := range it.keys {
			if !inStrs(k, setItemFields) {
				v.err("%s: bilinmeyen alan: %s", iw, k)
			}
		}
		key, ok := asStr(it.get("key"))
		switch {
		case !ok || idx(declared, key) < 0:
			v.err("%s.key, %s eyleminin outputs.keys listesinde olmalı", iw, action)
		case seen[key]:
			v.err("%s.key yinelenmiş: %s", iw, key)
		default:
			seen[key] = true
			keys = append(keys, key)
		}
		v.textOK(it.get("label"), iw+".label", setSumLabel)
		if it.has("ok") && it.has("max_abs") {
			v.err("%s: 'ok' ile 'max_abs' birlikte verilemez", iw)
		}
		if it.has("ok") {
			v.textOK(it.get("ok"), iw+".ok", 64)
		}
		if it.has("max_abs") {
			if n, ok := asInt(it.get("max_abs")); !ok || n < 0 {
				v.err("%s.max_abs sıfır ya da pozitif tamsayı olmalı", iw)
			}
		}
		if it.has("unit") {
			v.textOK(it.get("unit"), iw+".unit", setSumUnit)
		}
	}
	return []string{"setting_card." + id + ".read=" + action, "setting_card." + id + ".keys=" + strings.Join(keys, ",")}
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
	if st.has("absent") {
		a, ok := asStr(st.get("absent"))
		switch {
		case !ok || !reResultValue.MatchString(a):
			v.err("%s.state: absent sonuç satırı değeri olmalı (1-64 karakter; harf, rakam, _ . : + -)", w)
		case !st.has("key"):
			v.err("%s.state: absent yalnızca 'key' ile kullanılır", w)
		}
	}
	if st.has("not_applicable") {
		absent, _ := asStr(st.get("absent"))
		vals, ok := uniqueStrings(st.get("not_applicable"), setIgnoreMax)
		for _, s := range vals {
			ok = ok && reResultValue.MatchString(s) && s != absent && s != "unknown"
		}
		switch {
		case !ok:
			v.err("%s.state: not_applicable en çok %d sonuç satırı değeri olmalı ('unknown' ve absent değeri olamaz)", w, setIgnoreMax)
		case !st.has("key"):
			v.err("%s.state: not_applicable yalnızca 'key' ile kullanılır", w)
		}
	}
	if st.has("ignore") {
		vals, ok := uniqueStrings(st.get("ignore"), setIgnoreMax)
		for _, s := range vals {
			ok = ok && s != "" && pyLen(s) <= 64 && !reTextBad.MatchString(s)
		}
		if !ok {
			v.err("%s.state: ignore en çok %d kısa metin olmalı", w, setIgnoreMax)
		}
	}
	return []string{"setting." + id + ".read=" + action, "setting." + id + ".keys=" + strings.Join(keys, ","), "setting." + id + ".compare=" + compare}
}
