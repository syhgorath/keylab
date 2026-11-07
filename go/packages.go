// Paketler sayfası: USB sürümleri, kilit tabanı, sunucu başına son kullanım.
// (main.go'dan mekanik olarak bölündü; davranış değişmedi.)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

/* ---------- Paketler sayfası: USB sürümleri, kilit tabanı, sunucu başına son kullanım ---------- */

type PkgUse struct {
	Server  string `json:"server"`
	Version string `json:"version"`
	Action  string `json:"action"`
	Result  string `json:"result"`
	Time    string `json:"time"`
}

type PkgOverview struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Current     string   `json:"current"`
	Versions    []string `json:"versions"`
	Floor       string   `json:"floor"`
	Repo        string   `json:"repo"`
	Uses        []PkgUse `json:"uses"`
}

// "0.13.0" gibi sürümleri sayısal karşılaştırır (a<b ise -1)
func cmpVer(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &x)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &y)
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// components.lock: [paket] bölümleri; yalnızca repo ve version okunur (imza anahtarı/özet gösterilmez)
func (a *App) readLock() map[string]map[string]string {
	out := map[string]map[string]string{}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Clean(a.compDir)), "components.lock"))
	if err != nil {
		return out
	}
	cur := ""
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			cur = strings.Trim(l, "[]")
			if !reName.MatchString(cur) {
				cur = ""
			} else {
				out[cur] = map[string]string{}
			}
			continue
		}
		if cur == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if k, v, ok := strings.Cut(l, "="); ok {
			k = strings.TrimSpace(k)
			if k == "repo" || k == "version" {
				out[cur][k] = strings.TrimSpace(v)
			}
		}
	}
	return out
}

func (a *App) packageOverview() []PkgOverview {
	pkgs, _ := a.packages()
	lock := a.readLock()
	servers := a.servers(false)
	out := []PkgOverview{}
	for _, p := range pkgs {
		if !reName.MatchString(p.ID) {
			continue
		}
		o := PkgOverview{ID: p.ID, Current: p.Version, Floor: lock[p.ID]["version"], Repo: lock[p.ID]["repo"]}
		ents, _ := os.ReadDir(filepath.Join(a.compDir, p.ID))
		for _, e := range ents {
			if e.IsDir() && reVer.MatchString(e.Name()) && !strings.HasSuffix(e.Name(), ".files") {
				o.Versions = append(o.Versions, e.Name())
			}
		}
		sort.Slice(o.Versions, func(i, j int) bool { return cmpVer(o.Versions[i], o.Versions[j]) > 0 })
		if reVer.MatchString(p.Version) {
			if b, err := os.ReadFile(filepath.Join(a.compDir, p.ID, p.Version, "component.json")); err == nil {
				var c struct{ Name, Description string }
				if json.Unmarshal(b, &c) == nil {
					o.Name, o.Description = c.Name, c.Description
				}
			}
		}
		for _, s := range servers {
			for _, act := range readActions(filepath.Join(a.serversDir, s.Name, "components.log"), 100000) {
				if act.Package == p.ID {
					o.Uses = append(o.Uses, PkgUse{Server: s.Name, Version: act.Version, Action: act.Action, Result: act.Result, Time: act.Time})
					break
				}
			}
		}
		out = append(out, o)
	}
	return out
}
