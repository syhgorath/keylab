// Package report: paket sürümü değişince "ne değişti, hangi sunucum etkileniyor" raporu (ComponentsReport.sh).
// Yalnızca okur; hiçbir şey yazmaz/değiştirmez.
//
// Bilinçli fark (kabuk hatasının düzeltilmesi): kabukta _comp_report_cap sayaç tutmaz; satır sınırı (12) hiç uygulanmaz ve
// "any" bayrağı hiç kurulmaz, bu yüzden değişken/kural farkı olsa bile "Değişken ve kural farkı yok." yazılır.
// Go sınırı uygular ve bayrağı doğru kurar. Geri kalan çıktı kabukla birebir aynıdır (gerçek paketlerle karşılaştırıldı).
package report

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sshpocket-go/internal/manifest"
	"sshpocket-go/internal/runner"
	"sshpocket-go/internal/servers"
	"sshpocket-go/internal/trust"
)

// Options: dizinler ve bölüm başına en çok satır (varsayılan 12).
type Options struct {
	ComponentsDir, ServersDir string
	Max                       int
}

func (o Options) max() int {
	if o.Max > 0 {
		return o.Max
	}
	return 12
}

// PrevVersion, comp_report_prev_version: etkin sürümden küçük en büyük, component.json'u duran sürüm.
func PrevVersion(o Options, id string) string {
	cur := trust.InstalledVersion(o.ComponentsDir, id)
	if cur == "" {
		return ""
	}
	ents, _ := os.ReadDir(filepath.Join(o.ComponentsDir, id))
	best := ""
	for _, e := range ents {
		v := e.Name()
		if st, err := os.Stat(filepath.Join(o.ComponentsDir, id, v, "component.json")); err != nil || !st.Mode().IsRegular() {
			continue
		}
		if !trust.VersionValid(v) || trust.VersionCmp(v, cur) != -1 {
			continue
		}
		if best == "" || trust.VersionCmp(v, best) == 1 {
			best = v
		}
	}
	return best
}

type varRow struct{ name, typ, def string }

func describe(mf string) []varRow {
	lines, _, _ := manifest.Describe(mf, "", false)
	var out []varRow
	for _, l := range lines {
		f := strings.Split(l, "\t")
		for len(f) < 4 {
			f = append(f, "")
		}
		out = append(out, varRow{f[0], f[1], f[3]})
	}
	return out
}

// VarsDiff, comp_report_vars_diff: ("+", ad, tür, varsayılan) · ("-", ad) · ("t", ad, eski, yeni) · ("d", ad, eski, yeni).
func VarsDiff(oldMF, newMF string) [][4]string {
	ot, od := map[string]string{}, map[string]string{}
	var ord []string
	for _, r := range describe(oldMF) {
		ot[r.name], od[r.name] = r.typ, r.def
		ord = append(ord, r.name)
	}
	nt, nd := map[string]string{}, map[string]string{}
	var nrd []string
	for _, r := range describe(newMF) {
		nt[r.name], nd[r.name] = r.typ, r.def
		nrd = append(nrd, r.name)
	}
	var out [][4]string
	for _, k := range nrd {
		if _, ok := ot[k]; !ok {
			out = append(out, [4]string{"+", k, nt[k], nd[k]})
		} else if ot[k] != nt[k] {
			out = append(out, [4]string{"t", k, ot[k], nt[k]})
		} else if od[k] != nd[k] {
			out = append(out, [4]string{"d", k, od[k], nd[k]})
		}
	}
	for _, k := range ord {
		if _, ok := nt[k]; !ok {
			out = append(out, [4]string{"-", k})
		}
	}
	return out
}

func short(t string) string {
	r := []rune(t)
	if len(r) > 40 {
		return string(r[:39]) + "…"
	}
	return t
}

func varLines(oldMF, newMF string) []string {
	var out []string
	for _, d := range VarsDiff(oldMF, newMF) {
		switch d[0] {
		case "+":
			extra := ""
			if d[3] != "" {
				extra = ", varsayılan " + short(d[3])
			}
			out = append(out, fmt.Sprintf("yeni değişken: %s (%s%s)", d[1], d[2], extra))
		case "-":
			out = append(out, "kalkan değişken: "+d[1])
		case "t":
			out = append(out, fmt.Sprintf("tür değişti: %s: %s → %s", d[1], d[2], d[3]))
		case "d":
			out = append(out, fmt.Sprintf("varsayılan değişti: %s: %s → %s", d[1], short(d[2]), short(d[3])))
		}
	}
	return out
}

// capped: en çok max satır, fazlası "… ve N tane daha".
func capped(w io.Writer, lines []string, max int) bool {
	n, extra := 0, 0
	for _, l := range lines {
		if l == "" {
			continue
		}
		n++
		if n <= max {
			fmt.Fprintf(w, "    %s\n", l)
		} else {
			extra++
		}
	}
	if extra > 0 {
		fmt.Fprintf(w, "    … ve %d tane daha\n", extra)
	}
	return n > 0
}

// ServerLines, comp_report_servers: bu makinedeki sunucular için etki satırları.
func ServerLines(o Options, id, newMF string, info []string) []string {
	var out []string
	for _, srv := range servers.Names(o.ServersDir) {
		dir := filepath.Join(o.ServersDir, srv)
		vf := manifest.VarsFile(dir, id)
		if st, err := os.Stat(vf); err != nil || !st.Mode().IsRegular() {
			continue // bu sunucuda paket hiç ayarlanmamış
		}
		var acts, names []string
		seen := map[string]bool{}
		for _, act := range manifest.InfoAll(info, "action") {
			rv := runner.UnmetVars(info, act, vf)
			if len(rv) == 0 {
				continue
			}
			acts = append(acts, act)
			for _, v := range rv {
				if !seen[v] {
					seen[v] = true
					names = append(names, v)
				}
			}
		}
		if len(acts) > 0 {
			out = append(out, fmt.Sprintf("%s: %s ⛔ — değeri boş: %s (Paketler › Değişkenler)", srv, strings.Join(acts, ", "), strings.Join(names, " ")))
		}
		if miss := manifest.VarsMissing(newMF, vf); len(miss) > 0 {
			out = append(out, fmt.Sprintf("%s: zorunlu değişken eksik: %s", srv, strings.Join(miss, " ")))
		}
		if onboardNeeded(info, vf) {
			out = append(out, srv+": sunucu türü seçilmemiş (paket menüsü ilk girişte soracak)")
		}
	}
	return out
}

// onboardNeeded, comp_onboard_needed: platform listesi var ama platform_var dosyada yok/boş.
func onboardNeeded(info []string, vf string) bool {
	if manifest.InfoGet(info, "platform_choice") == "" {
		return false
	}
	pv := manifest.InfoGet(info, "platform_var")
	if pv == "" {
		return false
	}
	v, _ := manifest.VarsGet(vf, pv)
	return v == ""
}

// Pkg, comp_report_pkg: eski ↔ yeni (etkin) sürüm farkı ve sunucu etkisi. old boşsa diskteki bir önceki sürüm.
// Çıkış kodu: 0 tamam/karşılaştırılacak yok, 1 hata (iletiler errw'ye).
func Pkg(w, errw io.Writer, o Options, id, old string) int {
	nw := trust.InstalledVersion(o.ComponentsDir, id)
	if nw == "" {
		fmt.Fprintf(errw, "%s: kurulu değil.\n", id)
		return 1
	}
	if old == "" {
		old = PrevVersion(o, id)
	}
	if old == "" || old == nw {
		fmt.Fprintf(w, "%s %s: karşılaştırılacak eski sürüm yok.\n", id, nw)
		return 0
	}
	omf := filepath.Join(o.ComponentsDir, id, old, "component.json")
	nmf := filepath.Join(o.ComponentsDir, id, nw, "component.json")
	oi, _, c := manifest.ValidateFile(omf, id, old)
	if c != 0 {
		fmt.Fprintf(errw, "%s %s: eski bildirim okunamadı; rapor yok.\n", id, old)
		return 1
	}
	ni, _, c := manifest.ValidateFile(nmf, id, nw)
	if c != 0 {
		fmt.Fprintf(errw, "%s %s: bildirim okunamadı; rapor yok.\n", id, nw)
		return 1
	}
	fmt.Fprintf(w, "📋 %s %s → %s: ne değişti?\n", id, old, nw)
	any := false
	if lines := varLines(omf, nmf); len(lines) > 0 {
		fmt.Fprintln(w, "  Değişkenler:")
		if capped(w, lines, o.max()) {
			any = true
		}
	}
	var rules []string
	for _, a := range manifest.InfoAll(ni, "action") {
		b := manifest.ActionGet(ni, a, "requires_vars")
		if b != "" && b != manifest.ActionGet(oi, a, "requires_vars") {
			rules = append(rules, fmt.Sprintf("%s artık şu değişken(ler) dolu olmadan çalışmaz: %s", a, b))
		}
	}
	if len(rules) > 0 {
		fmt.Fprintln(w, "  Yeni kurallar:")
		if capped(w, rules, o.max()) {
			any = true
		}
	}
	a := strings.Join(manifest.InfoAll(ni, "platform_choice"), " ")
	b := strings.Join(manifest.InfoAll(oi, "platform_choice"), " ")
	if a != "" && a != b {
		extra := ""
		if b != "" {
			extra = " (eskiden: " + b + ")"
		}
		fmt.Fprintf(w, "  Sunucu türü seçimi: %s%s\n", a, extra)
		any = true
	}
	if !any {
		fmt.Fprintln(w, "  Değişken ve kural farkı yok.")
	}
	if lines := ServerLines(o, id, nmf, ni); len(lines) > 0 {
		fmt.Fprintln(w, "  Sunucularınız:")
		capped(w, lines, o.max())
	}
	fmt.Fprintln(w, "  (Bu rapor yalnızca okur; hiçbir şey değiştirilmedi.)")
	return 0
}

// All, comp_report_all: kurulu her paket için Pkg.
func All(w, errw io.Writer, o Options) {
	ents, _ := os.ReadDir(o.ComponentsDir)
	var ids []string
	for _, e := range ents {
		if e.IsDir() && trust.IDValid(e.Name()) && trust.InstalledVersion(o.ComponentsDir, e.Name()) != "" {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		Pkg(w, errw, o, id, "")
	}
}
