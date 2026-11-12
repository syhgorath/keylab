package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"keylab/internal/manifest"
	"keylab/internal/trust"
)

// ComponentsExec.sh'ın yerel işlevleri: ilerleme ve sonuç satırı ayrıştırma, sonuç kaydı, günlük maskeleme ve kalıcı arşiv.

// ProgressRow, bir alanın son durumudur (comp_progress_parse çıktısı; boş alanlar "-").
type ProgressRow struct {
	Alan, Sira, Toplam, Durum, Degisen, Atlanan, Kesin, Uyari, Bekle string
}

var (
	reProgAlan  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	reProgSira  = regexp.MustCompile(`^[0-9]+/[0-9]+$`)
	reTokSplit  = regexp.MustCompile(`[ \t]+`)
	reProgTrail = regexp.MustCompile(`["\\,}]+$`)
)

// ProgressParse, günlükteki `SSHPOCKET_PROGRESS alan=<ad> sira=<n>/<toplam> durum=basladi|bitti [...]` satırlarından
// alan başına son durumu çıkarır. Satır başı ve tırnak varsayılmaz; bilinmeyen anahtarlar yok sayılır, bozuk satır atlanır.
// `bitti` olmadan kalan alan "basladi" görünür. Hiç satır yoksa ok=false.
func ProgressParse(log string) (rows []ProgressRow, ok bool) {
	b, err := os.ReadFile(log)
	if err != nil || !strings.Contains(string(b), "SSHPOCKET_PROGRESS") {
		return nil, false
	}
	type st struct{ sira, top, dur, dg, at, ks, uy, bk string }
	data := map[string]*st{}
	var order []string
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.Contains(ln, "SSHPOCKET_PROGRESS") {
			continue
		}
		ln = strings.ReplaceAll(ln, "\r", "")
		i := strings.Index(ln, "SSHPOCKET_PROGRESS")
		line := ln[i+len("SSHPOCKET_PROGRESS"):]
		kv := map[string]string{}
		for _, t := range reTokSplit.Split(line, -1) {
			t = reProgTrail.ReplaceAllString(t, "")
			if e := strings.Index(t, "="); e > 0 {
				kv[t[:e]] = t[e+1:]
			}
		}
		a, d := kv["alan"], kv["durum"]
		if !reProgAlan.MatchString(a) || (d != "basladi" && d != "bitti") || !reProgSira.MatchString(kv["sira"]) {
			continue
		}
		s, seen := data[a]
		if !seen {
			s = &st{}
			data[a] = s
			order = append(order, a)
		}
		sp := strings.SplitN(kv["sira"], "/", 2)
		s.sira, s.top = sp[0], sp[1]
		if d == "bitti" || s.dur == "" {
			s.dur = d
			if d == "bitti" {
				s.dg, s.at, s.ks, s.uy = kv["degisen"], kv["atlanan"], kv["kesin"], kv["uyari"]
			}
		}
		if reDigits.MatchString(kv["bekle_tahmin"]) {
			s.bk = kv["bekle_tahmin"]
		}
	}
	v := func(x string) string {
		if x == "" {
			return "-"
		}
		return x
	}
	for _, a := range order {
		s := data[a]
		rows = append(rows, ProgressRow{a, s.sira, s.top, s.dur, v(s.dg), v(s.at), v(s.ks), v(s.uy), v(s.bk)})
	}
	return rows, true
}

// TSV, satırı comp_progress_parse çıktısı gibi sekmeyle ayrılmış verir.
func (r ProgressRow) TSV() string {
	return strings.Join([]string{r.Alan, r.Sira, r.Toplam, r.Durum, r.Degisen, r.Atlanan, r.Kesin, r.Uyari, r.Bekle}, "\t")
}

var reResultVal = regexp.MustCompile(`^[A-Za-z0-9_.:+-]{0,64}$`)

// ParseResult: eylemin `SSHPOCKET_RESULT <paket>.<eylem> k=v ...` son satırını doğrular ve "k=v" satırları verir.
// outputs.format keyvalue-line değilse boş ve hatasız döner.
func ParseResult(info []string, action, log string) ([]string, error) {
	if manifest.ActionGet(info, action, "outputs.format") != "keyvalue-line" {
		return nil, nil
	}
	id := manifest.InfoGet(info, "id")
	keys := "," + manifest.ActionGet(info, action, "outputs.keys") + ","
	b, _ := os.ReadFile(log)
	var line string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.Contains(l, "SSHPOCKET_RESULT") {
			line = l
		}
	}
	line = strings.ReplaceAll(line, "\r", "")
	if line == "" {
		return nil, fmt.Errorf("Sonuç satırı bulunamadı (SSHPOCKET_RESULT).")
	}
	rest := "SSHPOCKET_RESULT" + line[strings.Index(line, "SSHPOCKET_RESULT")+len("SSHPOCKET_RESULT"):]
	for _, cut := range []string{`\n`, `"`, `'`} {
		if i := strings.Index(rest, cut); i >= 0 {
			rest = rest[:i]
		}
	}
	toks := strings.Fields(strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r' {
			return 0xE000 // IFS=' ': yalnızca boşluk ayırıcıdır; diğer boşluklar sözcük içinde kalır
		}
		return r
	}, rest))
	for i, t := range toks {
		toks[i] = strings.Map(func(r rune) rune {
			if r == 0xE000 {
				return '\t'
			}
			return r
		}, t)
	}
	if len(toks) < 2 || toks[0] != "SSHPOCKET_RESULT" || toks[1] != id+"."+action {
		return nil, fmt.Errorf("Sonuç satırı beklenen önekle ('SSHPOCKET_RESULT %s.%s') başlamıyor.", id, action)
	}
	var out []string
	seen := ","
	for _, tok := range toks[2:] {
		k, v, hasEq := strings.Cut(tok, "=")
		if !hasEq {
			v = tok
			k = tok
		}
		if !hasEq || !strings.Contains(keys, ","+k+",") || !reResultVal.MatchString(v) || strings.Contains(seen, ","+k+",") {
			return nil, fmt.Errorf("Sonuç satırı geçersiz ('%s' bilinmiyor, yinelenmiş ya da değeri uygunsuz).", k)
		}
		seen += k + ","
		out = append(out, k+"="+v)
	}
	return out, nil
}

// SaveResult: sonucu "result.<eylem>" dosyasına yazar (ilk satır epoch; 0600, atomik). Boş sonuçta hiçbir şey yazılmaz.
func SaveResult(sdir, action string, result []string, now int64) error {
	if !reActionName.MatchString(action) {
		return fmt.Errorf("geçersiz eylem")
	}
	if len(result) == 0 {
		return nil
	}
	tmp, err := os.CreateTemp(sdir, "result.")
	if err != nil {
		return err
	}
	name := tmp.Name()
	tmp.Chmod(0o600)
	_, err = tmp.WriteString(strconv.FormatInt(now, 10) + "\n" + strings.Join(result, "\n") + "\n")
	tmp.Close()
	if err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, filepath.Join(sdir, "result."+action)); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// LastResult: ilk satır epoch, sonra "anahtar=değer" satırları (en çok 40 satır); kayıt yoksa ok=false.
func LastResult(stateDir, id, server, action string) ([]string, bool) {
	if !reActionName.MatchString(action) || !NameOK(server) || !trust.IDValid(id) {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(stateDir, id, server, "result."+action))
	if err != nil {
		return nil, false
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[:40]
	}
	return lines, true
}

// ---------------------------------------------------------------- günlük maskeleme

var (
	rePrivBegin = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	rePrivEnd   = regexp.MustCompile(`-----END [A-Z ]*PRIVATE KEY-----`)
	// POSIX en uzun eşleşme (sed -E ile aynı): anahtar sözcük + en çok 20 harf/_/boşluk + ":" ya da "=" + boşluklar + değer
	reSecretLine = regexp.MustCompilePOSIX(`(([Bb][Ee][Cc][Oo][Mm][Ee]|[Ss][Uu][Dd][Oo]|[Pp][Aa][Ss][Ss][Ww][Oo][Rr][Dd]|[Pp][Aa][Ss][Ss][Ww][Dd]|[Pp][Aa][Rr][Oo][Ll][Aa]|[Tt][Oo][Kk][Ee][Nn]|[Ss][Ee][Cc][Rr][Ee][Tt]|[Aa][Pp][Ii][_-]?[Kk][Ee][Yy])[A-Za-z_ ]{0,20}[:=][ ]*)[^ ].*`)
	reTokenLine  = regexp.MustCompile(`(ghp_|github_pat_|glpat-|xox[bp]-)[A-Za-z0-9_-]{8,}`)
)

// Mask, günlükten parola/anahtar/token izlerini siler: NUL baytları atılır, özel anahtar blokları tek satırla
// değiştirilir, anahtar sözcüklü `ad: değer` satırlarının değeri ve bilinen token önekleri "****" olur.
// Her çıktı satırı satır sonuyla biter. Kapanmamış özel anahtar bloğu dosya sonuna kadar gizlenir.
func Mask(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	if text == "" {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	var out strings.Builder
	inBlock := false
	for _, l := range lines {
		if inBlock {
			if rePrivEnd.MatchString(l) {
				inBlock = false
				out.WriteString("[özel anahtar bloğu gizlendi]\n")
			}
			continue
		}
		if rePrivBegin.MatchString(l) {
			inBlock = true
			continue
		}
		l = reSecretLine.ReplaceAllString(l, "${1}****")
		l = reTokenLine.ReplaceAllString(l, "${1}****")
		out.WriteString(l + "\n")
	}
	if inBlock {
		out.WriteString("[özel anahtar bloğu gizlendi]\n")
	}
	return out.String()
}

// ---------------------------------------------------------------- kalıcı günlük arşivi

// ArchiveLog, last-run.log'u <durum>/logs/<YYYYmmdd-HHMMSS>-<eylem>.log olarak arşivler (dizin 0700, dosya 0600;
// başlık + maskelenmiş gövde + sonuç). Başarısızlıkta <durum>/last-fail-log'a yolu yazar (başarıda siler).
// Sunucu/paket başına son keep (varsayılan 50) günlük kalır. last-run.log yoksa boş yol ve hata yok döner.
func ArchiveLog(sdir, id, ver, action, server string, rc int, t0, t1 int64, keep int) (string, error) {
	lastRun := filepath.Join(sdir, "last-run.log")
	raw, err := os.ReadFile(lastRun)
	if err != nil {
		return "", nil
	}
	if !reActionName.MatchString(action) {
		return "", fmt.Errorf("geçersiz eylem")
	}
	if keep < 1 {
		keep = 50
	}
	ldir := filepath.Join(sdir, "logs")
	if err := os.MkdirAll(ldir, 0o700); err != nil {
		return "", err
	}
	os.Chmod(ldir, 0o700)
	tm := time.Unix(t0, 0)
	name := tm.Format("20060102-150405") + "-" + action
	f := filepath.Join(ldir, name+".log")
	for n := 1; ; {
		if _, err := os.Lstat(f); err != nil {
			break
		}
		n++
		f = filepath.Join(ldir, fmt.Sprintf("%s-%d.log", name, n))
	}
	res := "fail"
	if rc == 0 {
		res = "ok"
	}
	switch rc {
	case 129, 130, 143:
		res = fmt.Sprintf("fail — kesildi (sinyal %d)", rc-128)
	}
	var cmdline string
	var body []string
	for _, l := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		if strings.HasPrefix(l, "komut: ") {
			if cmdline == "" {
				cmdline = strings.TrimSuffix(Mask(strings.TrimPrefix(l, "komut: ")), "\n")
			}
			continue
		}
		body = append(body, l)
	}
	if cmdline == "" {
		cmdline = "?"
	}
	var sb strings.Builder
	sb.WriteString("# sshpocket eylem günlüğü\n")
	fmt.Fprintf(&sb, "# zaman: %s (yerel) / %s (UTC)\n", tm.Format("2006-01-02 15:04:05 -0700"), tm.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&sb, "# paket: %s %s\n# eylem: %s\n# sunucu: %s\n", id, ver, action, server)
	fmt.Fprintf(&sb, "# komut: %s\n# süre: %ds\n", cmdline, t1-t0)
	sb.WriteString("# ----\n")
	if len(body) > 0 && !(len(body) == 1 && body[0] == "") {
		sb.WriteString(Mask(strings.Join(body, "\n") + "\n"))
	}
	sb.WriteString("# ----\n")
	fmt.Fprintf(&sb, "# çıkış kodu: %d\n# sonuç: %s\n", rc, res)
	if err := os.WriteFile(f, []byte(sb.String()), 0o600); err != nil {
		os.Remove(f)
		return "", err
	}
	os.Chmod(f, 0o600)
	failFile := filepath.Join(sdir, "last-fail-log")
	if rc != 0 {
		os.WriteFile(failFile, []byte(f+"\n"), 0o600)
	} else {
		os.Remove(failFile)
	}
	ents, _ := os.ReadDir(ldir)
	var logs []string
	for _, e := range ents {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".log") {
			logs = append(logs, e.Name())
		}
	}
	if len(logs) > keep {
		sort.Strings(logs)
		for _, n := range logs[:len(logs)-keep] {
			os.Remove(filepath.Join(ldir, n))
		}
	}
	return f, nil
}
