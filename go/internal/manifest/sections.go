package manifest

import (
	"fmt"
	"strings"
)

// sections: paket kartları (bkz. go/PAKET-BOLUMLERI-TASLAK.md). İsteğe bağlı üst düzey alan; sshpocket_api artmaz.
// Alanlar yalnızca BİLGİDİR (installs/touches hiçbir şeyi çalıştırmaz, kapıları değiştirmez).

const (
	secMax        = 30
	secSummaryMax = 300
	secTitleMax   = 40
	secListMax    = 10
	secListItem   = 60
)

var secKeys = []string{"id", "title", "summary", "installs", "touches", "vars_group", "action", "plan_tokens", "order"}

// validateSections, "sections" alanını denetler ve çıktı satırlarını üretir:
// section.<id>=<başlık>, section.<id>.vars_group|action|installs|touches|plan_tokens|order=...
func validateSections(v *validator, d *obj, actions *obj, varSpecs map[string]varSpec) []string {
	if !d.has("sections") {
		return nil
	}
	list, ok := asList(d.get("sections"))
	if !ok || len(list) < 1 || len(list) > secMax {
		v.err("sections 1-%d öğeli liste olmalı", secMax)
		return nil
	}
	groups := map[string]bool{}
	for _, sp := range varSpecs {
		if g, ok := asStr(sp.v.get("group")); ok {
			groups[g] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for i, x := range list {
		w := fmt.Sprintf("sections[%d]", i)
		so, ok := asObj(x)
		if !ok {
			v.err("%s nesne olmalı", w)
			continue
		}
		for _, k := range so.keys {
			if !inStrs(k, secKeys) {
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
		w = "sections." + id
		title := ""
		if v.textOK(so.get("title"), w+".title", secTitleMax) {
			title = so.get("title").(string)
		}
		v.textOK(so.get("summary"), w+".summary", secSummaryMax)
		line := "section." + id + "=" + title

		out = append(out, line)
		for _, k := range []string{"installs", "touches", "plan_tokens"} {
			if !so.has(k) {
				continue
			}
			l, ok := asList(so.get(k))
			if !ok || len(l) > secListMax {
				v.err("%s.%s en çok %d öğeli liste olmalı", w, k, secListMax)
				continue
			}
			var items []string
			bad := false
			for _, e := range l {
				s, ok := asStr(e)
				if !ok || s == "" || pyLen(s) > secListItem || reTextBad.MatchString(s) || strings.ContainsAny(s, ",+") {
					bad = true
					break
				}
				items = append(items, s)
			}
			if bad {
				v.err("%s.%s: öğeler 1-%d karakterlik düz metin olmalı (',' ve '+' yasak)", w, k, secListItem)
				continue
			}
			if len(items) > 0 {
				out = append(out, "section."+id+"."+k+"="+strings.Join(items, ","))
			}
		}
		if so.has("vars_group") {
			g, ok := asStr(so.get("vars_group"))
			if !ok || !groups[g] {
				v.err("%s.vars_group tanımlı bir değişken grubu olmalı", w)
			} else {
				out = append(out, "section."+id+".vars_group="+g)
			}
		}
		if so.has("action") {
			a, ok := asStr(so.get("action"))
			if !ok || !actions.has(a) {
				v.err("%s.action tanımlı bir eylem adı olmalı", w)
			} else {
				out = append(out, "section."+id+".action="+a)
			}
		}
		if so.has("order") {
			o, ok := asInt(so.get("order"))
			if !ok || o < 0 || o > 999 {
				v.err("%s.order 0-999 arası tamsayı olmalı", w)
			} else {
				out = append(out, fmt.Sprintf("section.%s.order=%d", id, o))
			}
		}
	}
	return out
}
