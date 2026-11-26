// Sunucu sayfası: bileşen ayarları ("settings", bkz. docs/ayar-semasi-taslak.md): istenen değer ile sunucudan SON OKUNAN değerin karşılaştırması.
// Hiçbir şey çalıştırmaz ve sunucuya bağlanmaz; yalnız bildirimi, değer dosyasını ve kayıtlı sonuçları okur.
package main

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"keylab/internal/manifest"
)

// settingDecl: component.json "settings" öğesi (kurulumda manifest doğrulayıcısından geçmiştir).
type settingDecl struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Var         string   `json:"var"`
	Description string   `json:"description"`
	Risk        []string `json:"risk"`
	RiskNote    string   `json:"risk_note"`
	ApplyVia    []string `json:"apply_via"`
	Card        string   `json:"card"`
	State       *struct {
		Action      string   `json:"action"`
		Key         string   `json:"key"`
		Keys        []string `json:"keys"`
		Compare     string   `json:"compare"`
		StripPrefix []string `json:"strip_prefix"`
		Absent      string   `json:"absent"` // sunucudaki bu değer: ayarlanan şey orada yok
		Ignore      []string `json:"ignore"` // istenen bu değerlerden biriyse yönetilmiyor
	} `json:"state"`
}

// settingCardDecl: component.json "setting_cards" öğesi.
type settingCardDecl struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Summary     *struct {
		Action string `json:"action"`
		Items  []struct {
			Key    string `json:"key"`
			Label  string `json:"label"`
			OK     string `json:"ok"`
			MaxAbs *int64 `json:"max_abs"`
			Unit   string `json:"unit"`
		} `json:"items"`
	} `json:"summary"`
}

// SettingCard: arayüzde katalog kartı + ayar penceresi; Health, özet eyleminin SON okumasından gelen sağlık bilgisidir.
type SettingCard struct {
	ID          string       `json:"id"`
	Label       string       `json:"label"`
	Description string       `json:"description,omitempty"`
	ReadAction  string       `json:"readAction,omitempty"`
	Note        string       `json:"note,omitempty"` // sağlık bilgisi neden yok
	Stale       bool         `json:"stale,omitempty"`
	Health      []HealthItem `json:"health,omitempty"`
}

// HealthItem: tek sağlık değeri. State: "ok", "warn" ya da boş (beklenti bildirilmemiş ya da değer bilinmiyor).
type HealthItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
	State string `json:"state,omitempty"`
}

// settingCards: bildirimdeki kartları güvenli sınırlara indirir ve sağlık özetini bu sunucunun kayıtlı okumasından hesaplar.
// Başarısız ya da paketin başka sürümünden okumayla sağlık değeri GÖSTERİLMEZ (eski bilgi "iyi" sanılmasın).
func (a *App) settingCards(s Server, p Package, decls []settingCardDecl, actions []ActionInfo) []SettingCard {
	byID := map[string]ActionInfo{}
	for _, ai := range actions {
		byID[ai.ID] = ai
	}
	versions := lastVersions(filepath.Join(a.stateDir, p.ID, s.Name, "results"))
	var out []SettingCard
	for _, d := range decls {
		if !reName.MatchString(d.ID) || len(out) >= 10 {
			continue
		}
		c := SettingCard{ID: d.ID, Label: d.Label, Description: d.Description}
		if d.Summary != nil {
			read, ok := byID[d.Summary.Action]
			if ok && read.Applicable && len(read.Effects) == 0 {
				c.ReadAction = read.ID
				switch {
				case read.LastTime == "":
					c.Note = "henüz okunmadı"
				case read.LastResult == "fail":
					c.Note = "son okuma başarısız"
				case versions[read.ID] != "" && versions[read.ID] != p.Version:
					c.Note = "son okuma paketin eski sürümünden (" + versions[read.ID] + ")"
				default:
					c.Stale = changedAfter(actions, read.LastTime)
					for i, it := range d.Summary.Items {
						v, found := read.Keys[it.Key]
						if !found || i >= 6 {
							continue
						}
						h := HealthItem{Label: it.Label, Value: v, Unit: it.Unit}
						if v != "unknown" {
							h.State = healthState(v, it.OK, it.MaxAbs)
						}
						c.Health = append(c.Health, h)
					}
				}
			}
		}
		out = append(out, c)
	}
	return out
}

// healthState: beklenen değer (ok) ya da mutlak sınır (maxAbs) bildirilmişse "ok"/"warn"; bildirilmemişse ya da sayı okunamıyorsa boş.
func healthState(v, want string, maxAbs *int64) string {
	good := false
	switch {
	case want != "":
		good = v == want
	case maxAbs != nil:
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(n) {
			return ""
		}
		good = math.Abs(n) <= float64(*maxAbs)
	default:
		return ""
	}
	if good {
		return "ok"
	}
	return "warn"
}

// Ayar durumları (arayüz yalnız gösterir).
const (
	setMatch      = "match"      // uyumlu
	setDrift      = "drift"      // sapma
	setUnknown    = "unknown"    // bilinmiyor: okunmadı, son okuma başarısız ya da eski paket sürümünden
	setUnmanaged  = "unmanaged"  // yönetilmiyor: istenen boş ("dokunma") ya da "ignore" değerlerinden
	setAbsent     = "absent"     // sunucuda yok (örn. uygulama kurulu değil)
	setUnreadable = "unreadable" // okunamaz: okuma bildirilmemiş ya da bu sunucuya uygulanmıyor
)

type SettingInfo struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Group       string   `json:"group"`          // değişkenin grubu: kart bildirilmemişse kart başlığı
	Card        string   `json:"card,omitempty"` // setting_cards kimliği
	Var         string   `json:"var"`
	Description string   `json:"description,omitempty"`
	Risk        []string `json:"risk,omitempty"`
	RiskNote    string   `json:"riskNote,omitempty"`
	ApplyVia    []string `json:"applyVia"`
	ReadAction  string   `json:"readAction,omitempty"`
	Desired     []string `json:"desired"`
	Actual      []string `json:"actual"`
	Status      string   `json:"status"`
	Note        string   `json:"note,omitempty"`
	ReadTime    string   `json:"readTime,omitempty"`
	Stale       bool     `json:"stale,omitempty"` // okumadan sonra değiştiren bir eylem çalıştı
}

var reNotResultChar = regexp.MustCompile(`[^A-Za-z0-9_.:+-]`)

// normResult: sonuç satırı sözleşmesinin karakter ve uzunluk kuralı (paket gerçek değeri bu kuralla temizleyip bildirir).
func normResult(s string) string {
	s = reNotResultChar.ReplaceAllString(s, "_")
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

func splitList(s, sep string) []string {
	out := []string{}
	for _, x := range strings.Split(s, sep) {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// settingMatches: istenen (ham yazım) ile gerçek (sonuç satırı yazımı) değerleri compare kuralıyla karşılaştırır.
func settingMatches(compare string, desired, actual, prefixes []string) bool {
	if compare == "present" {
		return len(actual) > 0
	}
	want := make([]string, 0, len(desired))
	for _, d := range desired {
		for _, p := range prefixes {
			if rest, ok := strings.CutPrefix(d, p); ok {
				d = strings.TrimSpace(rest)
				break
			}
		}
		want = append(want, normResult(d))
	}
	got := slices.Clone(actual)
	if compare == "set" {
		slices.Sort(want)
		slices.Sort(got)
		want, got = slices.Compact(want), slices.Compact(got)
	}
	return slices.Equal(want, got)
}

// settingInfos: bildirimdeki ayarları bu sunucu için değerlendirir. actions: aynı paketin hesaplanmış eylem bilgileri.
func (a *App) settingInfos(s Server, p Package, decls []settingDecl, groups map[string]string, actions []ActionInfo) []SettingInfo {
	if len(decls) == 0 {
		return nil
	}
	if len(decls) > 40 {
		decls = decls[:40]
	}
	names := make([]string, 0, len(decls))
	for _, d := range decls {
		names = append(names, d.Var)
	}
	valuesPath := filepath.Join(a.serversDir, s.Name, p.ID+".vars")
	effective, verr := manifest.EffectiveVars(filepath.Join(a.compDir, p.ID, p.Version, "component.json"), valuesPath, nil, names...)
	byID := map[string]ActionInfo{}
	for _, ai := range actions {
		byID[ai.ID] = ai
	}
	versions := lastVersions(filepath.Join(a.stateDir, p.ID, s.Name, "results"))

	out := []SettingInfo{}
	for _, d := range decls {
		if !reName.MatchString(d.ID) || !reName.MatchString(d.Var) {
			continue
		}
		info := SettingInfo{ID: d.ID, Label: d.Label, Group: groups[d.Var], Var: d.Var, Description: d.Description, Risk: d.Risk, RiskNote: d.RiskNote,
			Card: d.Card, ApplyVia: d.ApplyVia, Desired: splitList(effective[d.Var], ","), Actual: []string{}}
		if info.ApplyVia == nil {
			info.ApplyVia = []string{}
		}
		readable := false
		var read ActionInfo
		if d.State != nil {
			read, readable = byID[d.State.Action]
			readable = readable && read.Applicable && len(read.Effects) == 0
		}
		if readable {
			info.ReadAction = read.ID
		}
		switch {
		case verr != nil:
			info.Status, info.Note = setUnknown, "değişken dosyası geçersiz: istenen değer okunamadı"
		case !readable:
			info.Status = setUnreadable
			if d.State != nil { // okuma bildirilmiş ama bu sunucuda çalışmaz: arayüz bunu "yalnız seçenek"ten ayırır
				info.Note = "bu sunucuda okunamıyor"
			}
		default:
			keys, compare := d.State.Keys, d.State.Compare
			if d.State.Key != "" {
				keys = []string{d.State.Key}
			}
			if compare == "" {
				compare = "exact"
			}
			info.ReadTime = read.LastTime
			info.Stale = read.LastTime != "" && changedAfter(actions, read.LastTime)
			found, unknown, absent := false, false, false
			for _, k := range keys {
				v, ok := read.Keys[k]
				if !ok {
					continue
				}
				found = true
				if v == "unknown" { // paket "okuyamadım" diyor
					unknown, info.Actual = true, []string{}
					break
				}
				if d.State.Absent != "" && v == d.State.Absent {
					absent = true
					break
				}
				// tek anahtarlı liste "a+b" biçimindedir
				if d.State.Key != "" && (compare == "set" || compare == "ordered") {
					if v != "yok" {
						info.Actual = append(info.Actual, splitList(v, "+")...)
					}
				} else if v != "" {
					info.Actual = append(info.Actual, v)
				}
			}
			// çok anahtarlı listede hiçbir anahtarın olmaması "boş liste" demektir (başarılı ve güncel sürümden bir okuma varsa)
			if len(d.State.Keys) > 0 && len(read.Keys) > 0 {
				found = true
			}
			switch {
			case read.LastTime == "":
				info.Status, info.Note = setUnknown, "henüz okunmadı"
			case read.LastResult == "fail":
				info.Status, info.Note, info.Actual = setUnknown, "son okuma başarısız", []string{}
			case versions[read.ID] != "" && versions[read.ID] != p.Version:
				info.Status, info.Note, info.Actual = setUnknown, "son okuma paketin eski sürümünden ("+versions[read.ID]+")", []string{}
			case !found || unknown:
				info.Status, info.Note = setUnknown, "okuma bu değeri bildirmedi"
			case absent:
				info.Status = setAbsent
			case settingMatches(compare, info.Desired, info.Actual, d.State.StripPrefix):
				info.Status = setMatch
			default:
				info.Status = setDrift
			}
		}
		// istenen boş ya da "ignore" değeri = "dokunma": henüz okunmamış olsa da sapma/bilinmiyor değildir
		if verr == nil && readable && allIn(info.Desired, d.State.Ignore) {
			info.Status, info.Note = setUnmanaged, ""
		}
		out = append(out, info)
	}
	return out
}

// allIn: her öğe listede mi (boş küme için doğru: istenen boşsa yönetilmiyor).
func allIn(items, list []string) bool {
	for _, x := range items {
		if !slices.Contains(list, x) {
			return false
		}
	}
	return true
}

// changedAfter:verilen zamandan sonra değişiklik yapan (effects dolu) bir eylem çalıştı mı (arayüzdeki "güncel olmayabilir" kuralı).
func changedAfter(actions []ActionInfo, t string) bool {
	for _, x := range actions {
		if len(x.Effects) > 0 && x.LastTime > t { // RFC3339 UTC: metin sırası zaman sırasıdır
			return true
		}
	}
	return false
}

// lastVersions: eylem başına son kaydın paket sürümü (results: epoch|sürüm|eylem|sonuç[|kapsam]).
func lastVersions(path string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if p := strings.Split(l, "|"); len(p) >= 4 {
			out[p[2]] = p[1]
		}
	}
	return out
}
