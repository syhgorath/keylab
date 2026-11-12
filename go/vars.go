// Değişken düzenleme: doğrulama ve yazma.
// (main.go'dan mekanik olarak bölündü; davranış değişmedi.)
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"keylab/internal/manifest"
)

/* ---------- değişken düzenleme: doğrulama ve yazma sshpocket'in kendi kütüphanesiyle (menüdekiyle aynı) ---------- */

type VarSchema struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     string   `json:"default"`
	Min         string   `json:"min,omitempty"`
	Max         string   `json:"max,omitempty"`
	WarnIf      string   `json:"warnIf,omitempty"`
	Description string   `json:"description"`
	Group       string   `json:"group"`
	Values      []string `json:"values,omitempty"`
	Value       string   `json:"value"`
	Set         bool     `json:"set"`
}

var reVarName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func (a *App) engineRoot() string { return filepath.Dir(filepath.Clean(a.compDir)) }

func (a *App) manifestPath(pkg string) (string, error) {
	pkgs, _ := a.packages()
	for _, p := range pkgs {
		if p.ID == pkg && reName.MatchString(p.ID) && reVer.MatchString(p.Version) {
			return filepath.Join(a.compDir, p.ID, p.Version, "component.json"), nil
		}
	}
	return "", fmt.Errorf("paket bulunamadı")
}

// jsonScalarText: describe çıktısındaki JSON varsayılanını düz metne çevirir ("docker" → docker, [] → boş, ["a","b"] → a,b)
func jsonScalarText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return ""
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return raw
	}
	switch x := v.(type) {
	case string:
		return x
	case []any:
		parts := make([]string, 0, len(x))
		for _, i := range x {
			parts = append(parts, fmt.Sprint(i))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(x)
	}
}

// parseDescribe: component_run.py describe çıktısı (TSV: ad, tür, zorunlu, varsayılan, min, max, warn_if, açıklama, grup, değerler)
func parseDescribe(out string) []VarSchema {
	var res []VarSchema
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		c := strings.Split(l, "\t")
		if len(c) < 8 || !reVarName.MatchString(c[0]) {
			continue
		}
		for len(c) < 10 {
			c = append(c, "")
		}
		v := VarSchema{Name: c[0], Type: c[1], Required: c[2] == "yes", Default: jsonScalarText(c[3]), Min: c[4], Max: c[5], WarnIf: c[6], Description: c[7], Group: c[8]}
		if t := strings.TrimSpace(c[9]); t != "" {
			var vals []string
			if json.Unmarshal([]byte(t), &vals) == nil {
				v.Values = vals
			}
		}
		if len(v.Description) > 1200 {
			v.Description = v.Description[:1200]
		}
		res = append(res, v)
	}
	return res
}

// readVarsFile: servers/<ad>/<paket>.vars → ad=değer (menüdeki comp_vars_get ile aynı: yorumlar atlanır, tırnak soyulmaz)
func readVarsFile(path string) map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimLeft(l, " \t")
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if k, v, ok := strings.Cut(l, "="); ok && reVarName.MatchString(strings.TrimSpace(k)) {
			if _, dup := m[strings.TrimSpace(k)]; !dup {
				m[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	return m
}

func (a *App) varsFile(server, pkg string) string {
	return filepath.Join(a.serversDir, server, pkg+".vars")
}

// varSchemas: sunucunun platformuna uygun değişkenler ve sunucudaki güncel değerleri
func (a *App) varSchemas(server, pkg string) ([]VarSchema, error) {
	mf, err := a.manifestPath(pkg)
	if err != nil {
		return nil, err
	}
	vf := a.varsFile(server, pkg)
	hasVars := false
	if _, err := os.Stat(vf); err == nil {
		hasVars = true
	}
	lines, errLines, code := manifest.Describe(mf, vf, hasVars)
	if code != 0 {
		return nil, fmt.Errorf("değişken şeması okunamadı: %s", strings.TrimSpace(strings.Join(errLines, "\n")))
	}
	list := parseDescribe(strings.Join(lines, "\n") + "\n")
	cur := readVarsFile(vf)
	for i := range list {
		if v, ok := cur[list[i].Name]; ok && v != "" {
			list[i].Value, list[i].Set = v, true
		}
	}
	return list, nil
}

// saveVar: doğrular (motorun doğrulayıcısı) ve yazar. warn=… dönerse onay (ack) ister.
func (a *App) saveVar(server, pkg, name, value, action string, ack bool) (warn string, status int, err error) {
	// derinlemesine savunma: çağıran doğrulamış olsa bile ad dosya yoluna (servers/<sunucu>/<paket>.vars) girer
	if !reName.MatchString(server) || !reName.MatchString(pkg) {
		return "", http.StatusBadRequest, fmt.Errorf("geçersiz sunucu ya da paket adı")
	}
	a.jmu.Lock()
	busy := a.running != nil && !a.running.Done
	a.jmu.Unlock()
	if busy {
		return "", http.StatusConflict, fmt.Errorf("çalışan bir eylem varken değişken düzenlenemez")
	}
	a.vmu.Lock()
	defer a.vmu.Unlock()
	if !reVarName.MatchString(name) || len(value) > 1000 || strings.ContainsAny(value, "\n\r\t\x00") {
		return "", http.StatusBadRequest, fmt.Errorf("geçersiz ad ya da değer")
	}
	if st, e := os.Stat(filepath.Join(a.serversDir, server)); e != nil || !st.IsDir() {
		return "", http.StatusNotFound, fmt.Errorf("sunucu yok")
	}
	schemas, e := a.varSchemas(server, pkg)
	if e != nil {
		return "", http.StatusConflict, e
	}
	found := false
	for _, sc := range schemas {
		if sc.Name == name {
			found = true
		}
	}
	if !found {
		return "", http.StatusBadRequest, fmt.Errorf("bu değişken pakette yok ya da bu sunucunun platformuna uygulanmaz")
	}
	vf := a.varsFile(server, pkg)
	if action == "unset" {
		if e := manifest.VarsUnset(vf, name); e != nil {
			return "", http.StatusInternalServerError, e
		}
		return "", 0, nil
	}
	mf, _ := a.manifestPath(pkg)
	warnHit, errLines, code := manifest.Check(mf, name, value)
	if code != 0 {
		return "", http.StatusBadRequest, fmt.Errorf("%s", strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.Join(errLines, "\n")), "HATA: ")))
	}
	if warnHit {
		warn = name
		if !ack {
			return warn, http.StatusConflict, fmt.Errorf("uyarı: bu değer güvenlik açısından riskli; onay gerekli")
		}
	}
	if e := manifest.VarsSet(vf, name, value); e != nil {
		return warn, http.StatusInternalServerError, e
	}
	return warn, 0, nil
}
