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

type PackageInfo struct {
	ID          string       `json:"id"`
	Version     string       `json:"version"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Vars        []VarInfo    `json:"vars"`
	Actions     []ActionInfo `json:"actions"`
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
func lastResults(path string) map[string][2]string {
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
		var sec int64
		if _, err := fmt.Sscanf(p[0], "%d", &sec); err != nil {
			continue
		}
		out[p[2]] = [2]string{time.Unix(sec, 0).UTC().Format(time.RFC3339), p[3]}
	}
	return out
}

func (a *App) packageInfos(s Server) []PackageInfo {
	pkgs, _ := a.packages()
	out := []PackageInfo{}
	for _, p := range pkgs {
		if !reName.MatchString(p.ID) || !reVer.MatchString(p.Version) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.compDir, p.ID, p.Version, "component.json"))
		if err != nil {
			continue
		}
		var c struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Actions     map[string]struct {
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
			continue
		}
		info := PackageInfo{ID: p.ID, Version: p.Version, Name: c.Name, Description: c.Description, Vars: []VarInfo{}}
		sdir := filepath.Join(a.stateDir, p.ID, s.Name)
		last := lastResults(filepath.Join(sdir, "results"))
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
		out = append(out, info)
	}
	return out
}
