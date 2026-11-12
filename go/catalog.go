// Sunucu sayfası: paket eylem kataloğu (component.json) ve son sonuçlar.
// (main.go'dan mekanik olarak bölündü; davranış değişmedi.)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"keylab/internal/manifest"
)

/* ---------- sunucu sayfası: paket eylem kataloğu (component.json) + son sonuçlar ---------- */

type ActionInfo struct {
	ID           string            `json:"id"`
	Effects      []string          `json:"effects"`
	RequiresSud  bool              `json:"requiresSudo"`
	Confirm      string            `json:"confirm"`
	Requires     []string          `json:"requires"`
	Warnings     []string          `json:"warnings"`
	Platforms    []string          `json:"platforms"`
	RequiresVars []string          `json:"requiresVars,omitempty"`
	MissingVars  []string          `json:"missingVars,omitempty"`
	Applicable   bool              `json:"applicable"`
	Write        bool              `json:"write"` // değişiklik yapar (yıkıcı değil): onaylı akışla macOS Terminal'de çalıştırılabilir
	Destructive  bool              `json:"destructive"`
	Prereqs      []Prereq          `json:"prereqs,omitempty"`
	PrereqsOK    bool              `json:"prereqsOk"`
	Terminal     bool              `json:"terminal"`         // sudo parolası gerektiren salt okunur eylem: macOS Terminal'de çalıştırılabilir
	Runnable     bool              `json:"runnable"`         // arayüzden çalıştırılabilir mi (şimdilik yalnızca salt okunur, sudo'suz, onaysız, önkoşulsuz)
	Reason       string            `json:"reason,omitempty"` // çalıştırılamıyorsa nedeni
	LastTime     string            `json:"lastTime,omitempty"`
	LastResult   string            `json:"lastResult,omitempty"`
	Keys         map[string]string `json:"keys,omitempty"`
}

// Prereq: eylemin requires listesindeki bir ön koşul eylemin durumu (motor: son BAŞARILI kayıt <= 30 dk)
type Prereq struct {
	Action string `json:"action"`
	OK     bool   `json:"ok"`
	AgeSec int64  `json:"ageSec"`
	Result string `json:"result,omitempty"`
}

const prereqTTL = 30 * 60 // sn; sshpocket COMP_REQUIRES_TTL ile aynı

type VarInfo struct {
	Name        string   `json:"name"`
	Group       string   `json:"group"`
	Type        string   `json:"type"`
	Values      []string `json:"values,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description"`
	Value       string   `json:"value"`
	Set         bool     `json:"set"`
	Needed      bool     `json:"needed"` // bir eylem bu değişkeni zorunlu kılıyor
}

// ItemInfo: paketin yazılım kataloğu öğesi (component.json "items"; bkz. docs/package-standard.md). Arayüz katalog sayfasını bundan çizer.
type ItemInfo struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Modes       []string `json:"modes,omitempty"`
	Actions     []string `json:"actions,omitempty"`
	Params      []string `json:"params,omitempty"`
	// States: öğenin eylemleri için BU öğenin kapsamında hesaplanmış durum (önkoşul, son sonuç, çalıştırılabilirlik); arayüz öğe düğmelerini bundan çizer.
	States map[string]ActionInfo `json:"states,omitempty"`
}

type PackageInfo struct {
	ID          string       `json:"id"`
	Version     string       `json:"version"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	ItemVar     string       `json:"itemVar,omitempty"` // "items" varsa: yazılım seçimini taşıyan değişken
	ModeVar     string       `json:"modeVar,omitempty"`
	Items       []ItemInfo   `json:"items,omitempty"`
	Vars        []VarInfo    `json:"vars"`
	Actions     []ActionInfo `json:"actions"`
}

// cleanItems: component.json'dan gelen "items"ı güvenli sınırlara indirir (kimlikler reName, metinler kısaltılır, listeler sınırlanır).
func cleanItems(in []ItemInfo) []ItemInfo {
	var out []ItemInfo
	clip := func(s string, n int) string {
		if r := []rune(s); len(r) > n {
			return string(r[:n])
		}
		return s
	}
	ids := func(l []string, max int) []string {
		var o []string
		for _, x := range l {
			if reName.MatchString(x) && len(o) < max {
				o = append(o, x)
			}
		}
		return o
	}
	for _, it := range in {
		if !reName.MatchString(it.ID) || len(out) >= 30 {
			continue
		}
		out = append(out, ItemInfo{ID: it.ID, Label: clip(it.Label, 80), Description: clip(it.Description, 400),
			Modes: ids(it.Modes, 5), Actions: ids(it.Actions, 20), Params: ids(it.Params, 12)})
	}
	return out
}

var reAction = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

var reVer = regexp.MustCompile(`^[0-9A-Za-z._-]{1,32}$`)

// eylemlerin gösterim sırası (component.json sözlüğü sırasız); bilinmeyenler sona, alfabetik
var actionOrder = []string{"durum", "audit", "preview", "kur", "apply", "yukselt", "geri_yukle",
	"rollback_status", "rollback_disarm", "rollback_now", "pve_firewall_preview", "pve_firewall_apply",
	"kaldir_onizle", "kaldir", "kaldir_geri_yukle"}

func orderIndex(id string) int {
	for i, a := range actionOrder {
		if a == id {
			return i
		}
	}
	return len(actionOrder)
}

// result.<eylem>: ilk satır epoch, sonrası anahtar=değer
func readResultFile(path string) (string, map[string]string) {
	b, err := os.ReadFile(path)
	if err != nil || len(b) > 8192 {
		return "", nil
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	ts := ""
	if len(lines) > 0 {
		var sec int64
		if _, err := fmt.Sscanf(strings.TrimSpace(lines[0]), "%d", &sec); err == nil && sec > 0 {
			ts = time.Unix(sec, 0).UTC().Format(time.RFC3339)
			lines = lines[1:]
		}
	}
	keys := map[string]string{}
	for _, l := range lines {
		if k, v, ok := strings.Cut(l, "="); ok && len(k) <= 64 && len(v) <= 200 {
			keys[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return ts, keys
}

// results: <epoch>|<sürüm>|<eylem>|<ok|fail>; eylem başına son satır
// lastResults: eylem başına son kayıt; yalnız verilen kapsamdakiler sayılır (kapsam: `items` bildiren pakette "<öğe>:<mod>", yoksa boş).
func lastResults(path, scope string) map[string][2]string {
	out := map[string][2]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		p := strings.Split(l, "|")
		if len(p) < 4 {
			continue
		}
		rowScope := ""
		if len(p) >= 5 {
			rowScope = p[4]
		}
		if rowScope != scope {
			continue
		}
		var sec int64
		if _, err := fmt.Sscanf(p[0], "%d", &sec); err != nil {
			continue
		}
		out[p[2]] = [2]string{time.Unix(sec, 0).UTC().Format(time.RFC3339), p[3]}
	}
	return out
}

func (a *App) packageInfos(s Server) []PackageInfo { return a.packageInfosFor(s, "") }

// packageInfosFor: item doluysa kapsam (önkoşul rozetleri, son sonuçlar) o katalog öğesi için hesaplanır; boşsa şimdiki seçim için.
func (a *App) packageInfosFor(s Server, item string) []PackageInfo {
	pkgs, _ := a.packages()
	out := []PackageInfo{}
	for _, p := range pkgs {
		if info, ok := a.buildPackageInfo(s, p, item); ok {
			out = append(out, info)
		}
	}
	return out
}

func (a *App) buildPackageInfo(s Server, p Package, itemOverride string) (PackageInfo, bool) {
	if !reName.MatchString(p.ID) || !reVer.MatchString(p.Version) {
		return PackageInfo{}, false
	}
	b, err := os.ReadFile(filepath.Join(a.compDir, p.ID, p.Version, "component.json"))
	if err != nil {
		return PackageInfo{}, false
	}
	var c struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ItemVar     string `json:"item_var"`
		ModeVar     string `json:"mode_var"`
		Items       []struct {
			ID          string   `json:"id"`
			Label       string   `json:"label"`
			Description string   `json:"description"`
			Modes       []string `json:"modes"`
			Actions     []string `json:"actions"`
			Params      []string `json:"params"`
		} `json:"items"`
		Actions map[string]struct {
			Effects      []string `json:"effects"`
			RequiresSudo bool     `json:"requires_sudo"`
			Confirm      string   `json:"confirm"`
			Requires     []string `json:"requires"`
			Warnings     []string `json:"warnings"`
			Platforms    []string `json:"platforms"`
			RequiresVars []string `json:"requires_vars"`
		} `json:"actions"`
		Vars []struct {
			Name        string          `json:"name"`
			Group       string          `json:"group"`
			Type        string          `json:"type"`
			Values      []string        `json:"values"`
			Default     json.RawMessage `json:"default"`
			Description string          `json:"description"`
		} `json:"vars"`
	}
	if json.Unmarshal(b, &c) != nil {
		return PackageInfo{}, false
	}
	info := PackageInfo{ID: p.ID, Version: p.Version, Name: c.Name, Description: c.Description, Vars: []VarInfo{}}
	if reName.MatchString(c.ItemVar) && len(c.Items) > 0 {
		info.ItemVar = c.ItemVar
		if reName.MatchString(c.ModeVar) {
			info.ModeVar = c.ModeVar
		}
		raw := make([]ItemInfo, 0, len(c.Items))
		for _, it := range c.Items {
			raw = append(raw, ItemInfo{ID: it.ID, Label: it.Label, Description: it.Description, Modes: it.Modes, Actions: it.Actions, Params: it.Params})
		}
		info.Items = cleanItems(raw)
	}
	sdir := filepath.Join(a.stateDir, p.ID, s.Name)
	// kapsam: `items` bildiren pakette şimdiki seçim (değer dosyası/varsayılan); okunamazsa hiçbir kayıtla eşleşmeyen kapsam
	scope := ""
	if info.ItemVar != "" {
		scope = "\x00"
		names := []string{info.ItemVar}
		if info.ModeVar != "" {
			names = append(names, info.ModeVar)
		}
		var forced map[string]string
		if itemOverride != "" {
			forced = map[string]string{info.ItemVar: itemOverride} // bu öğe için hesapla (seçim değişkeninin değerinden bağımsız)
		}
		if ev, err := manifest.EffectiveVars(filepath.Join(a.compDir, p.ID, p.Version, "component.json"), filepath.Join(a.serversDir, s.Name, p.ID+".vars"), forced, names...); err == nil {
			scope = ev[info.ItemVar] + ":" + ev[info.ModeVar]
		}
	}
	last := lastResults(filepath.Join(sdir, "results"), scope)
	// sunucuya özel değişkenler: servers/<ad>/<paket>.vars (anahtar=değer); yalnızca ayarlı olanlar ve eylemlerin zorunlu kıldıkları gösterilir
	set := parseEnv(filepath.Join(a.serversDir, s.Name, p.ID+".vars"))
	needed := map[string]bool{}
	for _, x := range c.Actions {
		for _, v := range x.RequiresVars {
			needed[v] = true
		}
	}
	for _, v := range c.Vars {
		val, isSet := set[v.Name]
		if !reName.MatchString(v.Name) || (!isSet && !needed[v.Name]) {
			continue
		}
		def := strings.Trim(string(v.Default), `"`)
		if def == "null" {
			def = ""
		}
		vi := VarInfo{Name: v.Name, Group: v.Group, Type: v.Type, Values: v.Values, Default: def, Description: v.Description, Value: val, Set: isSet && val != "", Needed: needed[v.Name]}
		if len(vi.Description) > 600 {
			vi.Description = vi.Description[:600]
		}
		if len(vi.Value) > 300 {
			vi.Value = vi.Value[:300]
		}
		info.Vars = append(info.Vars, vi)
	}
	for id, x := range c.Actions {
		if !reName.MatchString(id) {
			continue
		}
		ai := ActionInfo{ID: id, Effects: x.Effects, RequiresSud: x.RequiresSudo, Confirm: x.Confirm, Requires: x.Requires, Warnings: x.Warnings, Platforms: x.Platforms, Applicable: len(x.Platforms) == 0}
		for _, pl := range x.Platforms {
			if pl == s.Platform {
				ai.Applicable = true
			}
		}
		for _, e := range x.Effects {
			if e == "destructive" {
				ai.Destructive = true
			}
		}
		ai.PrereqsOK = true
		for _, rq := range x.Requires {
			pr := Prereq{Action: rq}
			if r, ok := last[rq]; ok {
				pr.Result = r[1]
				if t, err := time.Parse(time.RFC3339, r[0]); err == nil {
					pr.AgeSec = int64(time.Since(t).Seconds())
					pr.OK = r[1] == "ok" && pr.AgeSec <= prereqTTL
				}
			}
			if !pr.OK {
				ai.PrereqsOK = false
			}
			ai.Prereqs = append(ai.Prereqs, pr)
		}
		ai.RequiresVars = x.RequiresVars
		for _, rv := range x.RequiresVars {
			if val, ok := set[rv]; !ok || strings.TrimSpace(val) == "" {
				ai.MissingVars = append(ai.MissingVars, rv)
			}
		}
		ai.Runnable, ai.Terminal, ai.Write, ai.Reason = classify(ai)
		ts, keys := readResultFile(filepath.Join(sdir, "result."+id))
		ai.Keys = keys
		ai.LastTime = ts
		if r, ok := last[id]; ok {
			ai.LastTime, ai.LastResult = r[0], r[1]
		}
		info.Actions = append(info.Actions, ai)
	}
	sort.Slice(info.Actions, func(i, j int) bool {
		oi, oj := orderIndex(info.Actions[i].ID), orderIndex(info.Actions[j].ID)
		if oi != oj {
			return oi < oj
		}
		return info.Actions[i].ID < info.Actions[j].ID
	})
	if itemOverride == "" && len(info.Items) > 0 {
		for i := range info.Items {
			sub, ok := a.buildPackageInfo(s, p, info.Items[i].ID)
			if !ok {
				continue
			}
			st := map[string]ActionInfo{}
			for _, ai := range sub.Actions {
				for _, want := range info.Items[i].Actions {
					if ai.ID == want {
						st[ai.ID] = ai
					}
				}
			}
			info.Items[i].States = st
		}
	}
	return info, true
}
