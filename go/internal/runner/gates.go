// Package runner, paket eylemi çalıştırmanın YEREL kısmını taşır: durum klasörü ve sonuç kaydı, çalıştırmadan önceki
// kapılar, ilerleme/sonuç ayrıştırma, günlük maskeleme ve kalıcı günlük arşivi (ComponentsGates.sh ve ComponentsExec.sh'ın
// yerel işlevleri). ssh, Ansible ve Python ortamı (venv) bu paketin DIŞINDADIR (sonraki aşama).
package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"keylab/internal/manifest"
	"keylab/internal/trust"
)

// DefaultRequiresTTL: requires eylemlerinin tazelik süresi (sn): 30 dk.
const DefaultRequiresTTL = 1800

var (
	reNameOK     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	reActionName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	reDigits     = regexp.MustCompile(`^[0-9]+$`)
)

// NameOK, sunucu adı denetimidir.
func NameOK(s string) bool { return reNameOK.MatchString(s) }

// Gates, kapıların bağlamıdır.
type Gates struct {
	StateDir    string       // ~/.local/share/sshpocket (SSHPOCKET_STATE_DIR)
	RequiresTTL int64        // 0 → DefaultRequiresTTL
	Now         func() int64 // nil → time.Now().Unix()
}

func (g *Gates) now() int64 {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().Unix()
}

func (g *Gates) ttl() int64 {
	if g.RequiresTTL > 0 {
		return g.RequiresTTL
	}
	return DefaultRequiresTTL
}

// Mutating: effects boş değilse eylem değiştiricidir.
func Mutating(info []string, action string) bool {
	return manifest.ActionGet(info, action, "effects") != ""
}

// NotRoot: değiştirici eylemi root ile reddeder.
func NotRoot(info []string, action, user string) error {
	if user == "root" && Mutating(info, action) {
		return errors.New("Değiştirici eylem root kullanıcıyla çalıştırılmaz; yetkili bir kullanıcıyla bağlanın (sudo).")
	}
	return nil
}

// Profile: algılanan profil pakette 'supported' olmalı ('planned' ve bilinmeyen reddedilir).
func Profile(info []string, detected string) error {
	if detected == "" {
		return errors.New("Sunucunun işletim sistemi profili belirlenemedi.")
	}
	for _, line := range manifest.InfoAll(info, "profile") {
		name, st, _ := strings.Cut(line, ":")
		if name != detected {
			continue
		}
		if st == "supported" {
			return nil
		}
		return fmt.Errorf("Profil '%s' pakette '%s' durumunda; desteklenmiyor.", detected, st)
	}
	return fmt.Errorf("Profil '%s' bu paketin profilleri arasında yok.", detected)
}

// StateEnsure: <durum>/<paket>/<sunucu> klasörünü (0700) oluşturur ve yolunu verir.
func (g *Gates) StateEnsure(id, server string) (string, error) {
	if !trust.IDValid(id) || !NameOK(server) {
		return "", errors.New("durum klasörü girdisi geçersiz")
	}
	root := g.StateDir
	dir := filepath.Join(root, id, server)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	for _, d := range []string{root, filepath.Join(root, id), dir} {
		if err := os.Chmod(d, 0o700); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// Record: kapsamsız kayıt (bkz. RecordScoped).
func (g *Gates) Record(id, server, ver, action, res string) error {
	return g.RecordScoped(id, server, ver, action, res, "")
}

// reScope: kayıt kapsamı = "<öğe>:<mod>" (katalog öğesi ve modu); öğesiz paketlerde boş.
var reScope = regexp.MustCompile(`^[A-Za-z0-9._-]{0,64}:[A-Za-z0-9._-]{0,64}$`)

// RecordScoped: sonucu kullanıcı klasörüne işler: "<epoch>|<sürüm>|<eylem>|<ok|fail>[|<kapsam>]" (dosya 0600).
// Kapsam, `items` bildiren paketlerde eylemin hangi yazılım/mod için koşulduğudur; önkoşul (requires) denetimi YALNIZ aynı kapsamın
// kaydını sayar (docker için önizleyip nginx kurmak kapıyı geçemez).
func (g *Gates) RecordScoped(id, server, ver, action, res, scope string) error {
	if scope != "" && !reScope.MatchString(scope) {
		return errors.New("kayıt girdisi geçersiz")
	}
	if !trust.IDValid(id) || !NameOK(server) || !trust.VersionValid(ver) || !reActionName.MatchString(action) || (res != "ok" && res != "fail") {
		return errors.New("kayıt girdisi geçersiz")
	}
	dir, err := g.StateEnsure(id, server)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "results"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if scope == "" {
		_, err = fmt.Fprintf(f, "%d|%s|%s|%s\n", g.now(), ver, action, res)
	} else {
		_, err = fmt.Fprintf(f, "%d|%s|%s|%s|%s\n", g.now(), ver, action, res, scope)
	}
	return err
}

// resultRows, results dosyasının tam (satır sonlu) satırlarını (ts, sürüm, eylem, sonuç, kapsam) verir; bozuk zaman damgalıları atlar.
// Kapsam alanı eski kayıtlarda yoktur (boş).
func (g *Gates) resultRows(id, server string) [][5]string {
	b, err := os.ReadFile(filepath.Join(g.StateDir, id, server, "results"))
	if err != nil {
		return nil
	}
	var rows [][5]string
	text := string(b)
	for {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			break // satır sonsuz son parça bash `read` döngüsünde işlenmez
		}
		line := text[:i]
		text = text[i+1:]
		f := strings.SplitN(line, "|", 5)
		for len(f) < 5 {
			f = append(f, "")
		}
		if !reDigits.MatchString(f[0]) {
			continue
		}
		rows = append(rows, [5]string{f[0], f[1], f[2], f[3], f[4]})
	}
	return rows
}

// Requires: kapsamsız önkoşul denetimi (bkz. RequiresScoped).
func (g *Gates) Requires(info []string, id, server, ver, action string) error {
	return g.RequiresScoped(info, id, server, ver, action, "")
}

// RequiresScoped: requires eylemleri taze (<= TTL), başarılı VE aynı kapsamda mı. Kapsamsız (eski) kayıt yalnız kapsamsız denetimi
// karşılar: `items` bildiren pakette önizleme yeniden yapılmalıdır (güvenli yönde kapanır).
func (g *Gates) RequiresScoped(info []string, id, server, ver, action, scope string) error {
	if !NameOK(server) {
		return errors.New("geçersiz sunucu adı")
	}
	now := g.now()
	rows := g.resultRows(id, server)
	for _, req := range strings.Split(manifest.ActionGet(info, action, "requires"), ",") {
		if req == "" {
			continue
		}
		found := false
		for _, r := range rows {
			if r[1] != ver || r[2] != req || r[3] != "ok" || r[4] != scope {
				continue
			}
			ts, _ := strconv.ParseInt(r[0], 10, 64)
			if ts <= now+60 && now-ts <= g.ttl() {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("'%s' için önce '%s' başarıyla çalıştırılmalı (bu sunucu ve sürüm, son %d dakika içinde).", action, req, g.ttl()/60)
		}
	}
	return nil
}

// RequiresAge: kapsamsız (bkz. RequiresAgeScoped).
func (g *Gates) RequiresAge(id, server, ver, action string) (int64, bool) {
	return g.RequiresAgeScoped(id, server, ver, action, "")
}

// RequiresAgeScoped: eylemin bu kapsamdaki en son BAŞARILI kaydının yaşı (sn); kayıt yoksa ok=false. Gelecekteki damgalar sayılmaz.
func (g *Gates) RequiresAgeScoped(id, server, ver, action, scope string) (int64, bool) {
	if !NameOK(server) {
		return 0, false
	}
	now := g.now()
	best := int64(-1)
	for _, r := range g.resultRows(id, server) {
		if r[1] != ver || r[2] != action || r[3] != "ok" || r[4] != scope {
			continue
		}
		ts, _ := strconv.ParseInt(r[0], 10, 64)
		if ts > now+60 {
			continue
		}
		if ts > best {
			best = ts
		}
	}
	if best < 0 {
		return 0, false
	}
	return now - best, true
}

// AskFunc, bir istem gösterip tek satır yanıt okur; girdi yoksa ok=false (EOF = ret).
type AskFunc func(prompt string) (answer string, ok bool)

func yes(ans string) bool { return ans == "e" || ans == "E" || ans == "evet" || ans == "Evet" }

// Confirm: effects + uyarıları out'a yazar, confirm kuralını uygular. none: onaysız · yes-no: e/evet ·
// type-server-name: sunucu adı birebir (3 deneme). Girdi bitince (EOF) reddedilir.
func Confirm(info []string, action, server string, ask AskFunc, out io.Writer) error {
	mode := manifest.ActionGet(info, action, "confirm")
	eff := manifest.ActionGet(info, action, "effects")
	if eff == "" {
		eff = "yok (salt okunur)"
	}
	fmt.Fprintf(out, "Eylem: %s   Sunucu: %s\n", action, server)
	fmt.Fprintf(out, "Etkiler: %s\n", eff)
	for _, w := range manifest.InfoAll(info, "action."+action+".warning") {
		if w != "" {
			fmt.Fprintf(out, "  ⚠️  %s\n", w)
		}
	}
	switch mode {
	case "none":
		return nil
	case "yes-no":
		ans, ok := ask("Devam edilsin mi? [e/H]: ")
		if !ok {
			return errors.New("Onay alınamadı.")
		}
		if yes(ans) {
			return nil
		}
		return errors.New("İptal edildi.")
	case "type-server-name":
		for try := 1; try <= 3; try++ {
			ans, ok := ask(fmt.Sprintf("Onaylamak için sunucu adını yazın (%s): ", server))
			if !ok {
				return errors.New("Onay alınamadı.")
			}
			if ans == server {
				return nil
			}
			if try < 3 {
				fmt.Fprintf(out, "Sunucu adı uyuşmadı (%d/3); tekrar deneyin.\n", try)
			}
		}
		return errors.New("Sunucu adı 3 kez uyuşmadı; iptal edildi.")
	}
	return fmt.Errorf("Bilinmeyen onay kuralı: %s", mode)
}

// RequiredApplies: zorunlu değişken eksikliği bu eylemi engeller mi (değiştiriciyse ya da başka eylemin ön koşuluysa).
func RequiredApplies(info []string, action string) bool {
	if Mutating(info, action) {
		return true
	}
	for _, a := range manifest.InfoAll(info, "action") {
		if a == "" {
			continue
		}
		for _, r := range strings.Split(manifest.ActionGet(info, a, "requires"), ",") {
			if r == action {
				return true
			}
		}
	}
	return false
}

// UnmetVars: requires_vars içinden değerler dosyasında BOŞ ya da YOK olanlar.
func UnmetVars(info []string, action, vf string) []string {
	var out []string
	for _, n := range strings.Split(manifest.ActionGet(info, action, "requires_vars"), ",") {
		if n == "" {
			continue
		}
		if v, ok := manifest.VarsGet(vf, n); !ok || v == "" {
			out = append(out, n)
		}
	}
	return out
}

// RequiresVars: eksik zorunlu eylem değişkeni varsa hata (neden ve ne yapılacağı iletide).
func RequiresVars(info []string, action, vf string) error {
	miss := UnmetVars(info, action, vf)
	if len(miss) == 0 {
		return nil
	}
	return fmt.Errorf("⛔ '%s' çalıştırılmadı: değeri boş değişken: %s. Bu eylem için girilmesi zorunlu (erişim kaybını önlemek için).\n   Menüde 'Değişkenler' ekranından girin ya da %s dosyasına '%s=<değer>' satırı ekleyin.",
		action, strings.Join(miss, " "), vf, miss[0])
}

// WarnApplies: warn_if onayı yalnızca yapılandırmayı değiştiren eylemlerde sorulur (yalnız installs-package olanlarda sorulmaz).
func WarnApplies(info []string, action string) bool {
	eff := manifest.ActionGet(info, action, "effects")
	if eff == "" {
		return false
	}
	for _, e := range strings.Fields(strings.ReplaceAll(eff, ",", " ")) {
		if e != "installs-package" {
			return true
		}
	}
	return false
}

// WarnVars: warn_if eşleşen her değişken için ayrıca onay ister.
func WarnVars(names []string, ask AskFunc) error {
	for _, n := range names {
		ans, ok := ask(fmt.Sprintf("⚠️  '%s' riskli bir değerde (paket uyarısı). Yine de devam edilsin mi? [e/H]: ", n))
		if !ok {
			return errors.New("Onay alınamadı.")
		}
		if !yes(ans) {
			return errors.New("İptal edildi.")
		}
	}
	return nil
}

// ApplyVars: seçili platformun apply_vars'ından BU koşuya özel geçirilecek "ad=değer" satırları (dosyaya yazılmaz).
func ApplyVars(info []string, action, vf string) []string {
	if manifest.ActionGet(info, action, "confirm") != "type-server-name" {
		return nil
	}
	eff := "," + manifest.ActionGet(info, action, "effects") + ","
	if !strings.Contains(eff, ",changes-config,") || strings.Contains(eff, ",destructive,") {
		return nil
	}
	if manifest.ActionGet(info, action, "requires") == "" {
		return nil
	}
	pv := manifest.InfoGet(info, "platform_var")
	if pv == "" {
		return nil
	}
	pid, ok := manifest.VarsGet(vf, pv)
	if !ok || pid == "" {
		return nil
	}
	found := false
	for _, c := range manifest.InfoAll(info, "platform_choice") {
		if c == pid {
			found = true
		}
	}
	if !found {
		return nil
	}
	var out []string
	for _, kv := range manifest.InfoAll(info, "platform_choice."+pid+".apply_var") {
		if kv == "" {
			continue
		}
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := manifest.VarsGet(vf, k); ok {
			continue
		}
		skip := false
		for _, s := range manifest.InfoAll(info, "action."+action+".set") {
			if strings.HasPrefix(s, k+"=") {
				skip = true
			}
		}
		if !skip {
			out = append(out, kv)
		}
	}
	return out
}

// ApplyVarsNotice: onaydan ÖNCE gösterilecek bilgi (apply_warnings + "bu koşuya özel" satırı).
func ApplyVarsNotice(info []string, vf string, applied []string, out io.Writer) {
	if len(applied) == 0 {
		return
	}
	pid, _ := manifest.VarsGet(vf, manifest.InfoGet(info, "platform_var"))
	fmt.Fprintf(out, "ℹ️  Seçili platform (%s) için bu koşuya özel değişken verilecek (değerler dosyanıza YAZILMAZ):\n", pid)
	for _, kv := range applied {
		fmt.Fprintf(out, "    %s\n", kv)
	}
	for _, w := range manifest.InfoAll(info, "platform_choice."+pid+".apply_warning") {
		if w != "" {
			fmt.Fprintf(out, "  ⚠️  %s\n", w)
		}
	}
}

// RunSets: katalogtan seçilen yazılım/mod/parametreler için "ad=değer" satırları (runset = satır satır "ad=değer").
func RunSets(info []string, action, runset string) ([]string, error) {
	if runset == "" {
		return nil, nil
	}
	iv, mv := manifest.InfoGet(info, "item_var"), manifest.InfoGet(info, "mode_var")
	var item, mode string
	var params []string
	for _, kv := range strings.Split(runset, "\n") {
		if kv == "" {
			continue
		}
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case iv != "" && k == iv:
			item = v
		case mv != "" && k == mv:
			mode = v
		default:
			params = append(params, kv)
		}
	}
	if item != "" {
		found := false
		for _, it := range manifest.InfoAll(info, "item") {
			if strings.HasPrefix(it, item+"|") {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("Katalogda olmayan yazılım: %s", item)
		}
		if acts := manifest.InfoGet(info, "item."+item+".actions"); acts != "" {
			if !strings.Contains(","+acts+",", ","+action+",") {
				return nil, fmt.Errorf("%s için '%s' eylemi tanımlı değil.", item, action)
			}
		}
		if mode != "" {
			if !strings.Contains(","+manifest.InfoGet(info, "item."+item+".modes")+",", ","+mode+",") {
				return nil, fmt.Errorf("%s için geçersiz mod: %s", item, mode)
			}
		}
	} else if mode != "" {
		return nil, errors.New("Mod yazılımsız verilemez.")
	}
	if len(params) > 0 {
		if item == "" {
			return nil, errors.New("Parametre yazılımsız verilemez.")
		}
		pp := manifest.InfoGet(info, "item."+item+".params")
		for _, kv := range params {
			k, _, _ := strings.Cut(kv, "=")
			if !strings.Contains(","+pp+",", ","+k+",") || k == iv || k == mv {
				return nil, fmt.Errorf("Koşuya özel değişken kabul edilmedi: %s", k)
			}
		}
	}
	var out []string
	if item != "" {
		out = append(out, iv+"="+item)
	}
	if mode != "" {
		out = append(out, mv+"="+mode)
	}
	return append(out, params...), nil
}
