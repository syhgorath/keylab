package manifest

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Bu dosya sistem/Helpers/component_validate.py'nin Go karşılığıdır: paket SÖZLEŞME UYUM denetimi
// (docs/package-standard.md §12). YALNIZCA STATİK: paketten hiçbir şey çalıştırmaz.

var pkgSections = []string{"Bildirim", "Düzen", "Eylemler", "Değişkenler", "Güvenlik", "Belge"}

var (
	textExt        = map[string]bool{".yml": true, ".yaml": true, ".j2": true, ".md": true, ".json": true, ".txt": true, ".cfg": true, ".sh": true, ".py": true, ".ini": true, ".in": true, ".toml": true, "": true}
	devDirs        = []string{".venv", "venv", ".tox", ".pytest_cache", ".mypy_cache", ".ruff_cache"}
	reDigestName   = regexp.MustCompile(`(?i)_(hash|digest|checksum)$`)
	reSecretName   = regexp.MustCompile(`(?i)(pass(word|wd)?|secret|token|credential|private_?key|api_?key)`)
	reSecretFile   = regexp.MustCompile(`(^|/)(\.env|id_(ed25519|rsa|ecdsa|dsa)[^/]*|[^/]*\.pem|[^/]*\.secrets)$`)
	rePrivateKey   = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	reRollback     = regexp.MustCompile(`(?i)(geri al|rollback|restore|kurtar|recovery|undo)`)
	reRecoveryName = regexp.MustCompile(`(?i)(^|_)(rollback|recover|restore|undo|geri)`)
	reArchiveName  = regexp.MustCompile(`^([a-z][a-z0-9-]*)-([0-9]+\.[0-9]+\.[0-9]+)\.tar\.gz$`)
	reURLDep       = regexp.MustCompile(`(git\+|https?://|file:|@ )`)
)

const maxFile = 512 * 1024

type pkgResult struct{ sec, level, text string }

type pkgChecker struct {
	results []pkgResult
	skipDev []string
}

func (c *pkgChecker) ok(sec, msg string) { c.results = append(c.results, pkgResult{sec, "ok", msg}) }
func (c *pkgChecker) warn(sec, msg string) {
	c.results = append(c.results, pkgResult{sec, "warn", msg})
}
func (c *pkgChecker) fail(sec, msg string) {
	c.results = append(c.results, pkgResult{sec, "fail", msg})
}

// splitExt, Python os.path.splitext(basename) uzantısıdır (baştaki noktalar uzantı sayılmaz).
func splitExt(name string) string {
	base := filepath.Base(name)
	lead := 0
	for lead < len(base) && base[lead] == '.' {
		lead++
	}
	i := strings.LastIndex(base[lead:], ".")
	if i < 0 {
		return ""
	}
	return base[lead+i:]
}

func readText(path string) string {
	st, err := os.Stat(path)
	if err != nil || st.Size() > maxFile {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ToValidUTF8(string(b), "�")
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
func isLink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

func (c *pkgChecker) skipped(name string) bool {
	for _, x := range c.skipDev {
		if x == name {
			return true
		}
	}
	return false
}

// walkFiles, os.walk(followlinks=False) gibi: dizindeki dosyalar (sembolik bağlar dahil), sonra alt dizinler (bağlı dizinler girilmez).
func (c *pkgChecker) walkFiles(root string, fn func(path string)) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
	var subs []string
	for _, e := range ents {
		p := filepath.Join(root, e.Name())
		if e.Type()&os.ModeSymlink != 0 {
			if isDir(p) {
				continue // os.walk bunu dirs listesine koyar, içine girmez
			}
			fn(p)
			continue
		}
		if e.IsDir() {
			switch e.Name() {
			case ".git", "node_modules", "__pycache__":
				continue
			}
			if c.skipped(e.Name()) {
				continue
			}
			subs = append(subs, p)
			continue
		}
		fn(p)
	}
	for _, s := range subs {
		c.walkFiles(s, fn)
	}
}

func (c *pkgChecker) corpus(root string, subdirs []string) string {
	var out []string
	for _, sd := range subdirs {
		p := filepath.Join(root, sd)
		if isFile(p) {
			out = append(out, readText(p))
		} else if isDir(p) {
			c.walkFiles(p, func(f string) {
				if textExt[strings.ToLower(splitExt(f))] {
					out = append(out, readText(f))
				}
			})
		}
	}
	return strings.Join(out, "\n")
}

func parseInfo(lines []string) map[string][]string {
	info := map[string][]string{}
	for _, line := range lines {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		info[k] = append(info[k], v)
	}
	return info
}

func first(info map[string][]string, key string, def ...string) string {
	if v, ok := info[key]; ok && len(v) > 0 {
		return v[0]
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

// safeExtract: arşivi YALNIZCA güvenliyse açar.
func safeExtract(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	type ent struct {
		h    *tar.Header
		data []byte
	}
	var ents []ent
	var total int64
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(ents) >= 5000 {
			return fmt.Errorf("arşivde çok fazla öğe")
		}
		name := h.Name
		if strings.HasPrefix(name, "/") {
			return fmt.Errorf("güvensiz yol: %s", name)
		}
		for _, part := range strings.Split(name, "/") {
			if part == ".." {
				return fmt.Errorf("güvensiz yol: %s", name)
			}
		}
		switch h.Typeflag {
		case tar.TypeSymlink, tar.TypeLink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			return fmt.Errorf("bağ/aygıt dosyası yasak: %s", name)
		}
		total += h.Size
		if total > 50*1024*1024 {
			return fmt.Errorf("arşiv çok büyük (açılmış > 50 MB)")
		}
		var data []byte
		if h.Typeflag == tar.TypeReg || h.Typeflag == tar.TypeRegA {
			data, err = io.ReadAll(io.LimitReader(tr, 50*1024*1024+1))
			if err != nil {
				return err
			}
		}
		ents = append(ents, ent{h, data})
	}
	for _, e := range ents {
		target := filepath.Join(dest, filepath.FromSlash(e.h.Name))
		switch e.h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(e.h.Mode).Perm()
			if err := os.WriteFile(target, e.data, mode); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveInput: kök dizin, kimlik/sürüm ipucu, geçici dizin, arşiv yolu.
func (c *pkgChecker) resolveInput(arg string) (root, idHint, verHint, tmp, archive string) {
	if isDir(arg) {
		abs, _ := filepath.Abs(arg)
		return abs, "", "", "", ""
	}
	if isFile(arg) {
		base := filepath.Base(arg)
		m := reArchiveName.FindStringSubmatch(base)
		if m == nil {
			c.fail("Düzen", fmt.Sprintf("arşiv adı '<id>-<sürüm>.tar.gz' olmalı (örn. paket-1.0.0.tar.gz): %s", base))
			return "", "", "", "", arg
		}
		tmp, err := os.MkdirTemp("", "sshpocket-validate-")
		if err != nil {
			c.fail("Düzen", fmt.Sprintf("arşiv güvenli açılamadı: %v", err))
			return "", m[1], m[2], "", arg
		}
		os.Chmod(tmp, 0o700)
		if err := safeExtract(arg, tmp); err != nil {
			c.fail("Düzen", fmt.Sprintf("arşiv güvenli açılamadı: %v", err))
			return "", m[1], m[2], tmp, arg
		}
		ents, _ := os.ReadDir(tmp)
		var top []string
		for _, e := range ents {
			top = append(top, e.Name())
		}
		want := m[1] + "-" + m[2]
		if len(top) != 1 || top[0] != want || !isDir(filepath.Join(tmp, want)) {
			sort.Strings(top)
			found := strings.Join(top, ", ")
			if found == "" {
				found = "boş"
			}
			c.fail("Düzen", fmt.Sprintf("arşiv kökü tek klasör '%s/' olmalı (bulundu: %s)", want, found))
			return "", m[1], m[2], tmp, arg
		}
		c.ok("Düzen", fmt.Sprintf("arşiv kökü '%s/'", want))
		return filepath.Join(tmp, want), m[1], m[2], tmp, arg
	}
	c.fail("Düzen", "bulunamadı: "+arg)
	return "", "", "", "", ""
}

func (c *pkgChecker) checkManifest(root, idHint, verHint string) map[string][]string {
	mf := filepath.Join(root, "component.json")
	if !isFile(mf) {
		c.fail("Bildirim", "component.json yok (paket kökünde olmalı)")
		return nil
	}
	lines, errs, code := ValidateFile(mf, idHint, verHint)
	if code != 0 {
		text := strings.Join(errs, "\n")
		if pyStrip(text) == "" {
			text = "bildirim doğrulanamadı"
		}
		for _, l := range pySplitLines(pyStrip(text)) {
			c.fail("Bildirim", pyStrip(l))
		}
		return nil
	}
	info := parseInfo(lines)
	c.ok("Bildirim", fmt.Sprintf("component.json geçerli (id=%s, sürüm=%s, runner=%s)", first(info, "id"), first(info, "version"), first(info, "runner")))
	var sup []string
	for _, v := range info["profile"] {
		if strings.HasSuffix(v, ":supported") {
			sup = append(sup, v)
		}
	}
	if len(sup) == 0 {
		c.fail("Bildirim", "en az bir 'supported' profil gerekir (yalnızca planned/boş)")
	} else {
		c.ok("Bildirim", "desteklenen profil var: "+strings.Join(sup, ", "))
	}
	return info
}

var (
	reYmlVersion = regexp.MustCompile(`(?m)^version:\s*['"]?([0-9.]+)`)
)

func (c *pkgChecker) checkLayout(root string, info map[string][]string) {
	ver := first(info, "version")
	vf := filepath.Join(root, "VERSION")
	if !isFile(vf) {
		c.fail("Düzen", "VERSION dosyası yok (sürüm üçlüsü: VERSION, component.json, etiket)")
	} else if v := pyStrip(readText(vf)); v == ver {
		c.ok("Düzen", fmt.Sprintf("VERSION = component.json sürümü (%s)", ver))
	} else {
		c.fail("Düzen", fmt.Sprintf("VERSION '%s' ≠ component.json sürümü '%s'", v, ver))
	}
	if cy := filepath.Join(root, "component.yml"); isFile(cy) {
		if m := reYmlVersion.FindStringSubmatch(readText(cy)); m != nil && m[1] != ver {
			c.warn("Düzen", fmt.Sprintf("component.yml sürümü '%s' ≠ component.json '%s' (JSON yml'den türetilmeli)", m[1], ver))
		}
	}
	if cl := filepath.Join(root, "CHANGELOG.md"); isFile(cl) {
		if !regexp.MustCompile(`(?m)^## \[` + regexp.QuoteMeta(ver) + `\]`).MatchString(readText(cl)) {
			c.warn("Düzen", fmt.Sprintf("CHANGELOG.md'de [%s] başlığı yok", ver))
		}
	}
	if first(info, "runner") == "ansible" {
		rq := filepath.Join(root, "requirements.txt")
		if !isFile(rq) {
			c.fail("Düzen", "requirements.txt yok (runner=ansible; hash'li, sabit sürümlü olmalı)")
		} else {
			c.checkRequirements(rq)
		}
		if !isFile(filepath.Join(root, "ansible.cfg")) {
			c.warn("Düzen", "ansible.cfg yok (çalışma dizini paket kökü; ayarlar oradan okunur)")
		}
	}
}

func firstWord(l string) string {
	f := strings.Fields(l)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func (c *pkgChecker) checkRequirements(path string) {
	raw := readText(path)
	var logical []string
	cur := ""
	for _, line := range pySplitLines(raw) {
		s := strings.TrimRightFunc(line, isPySpace)
		if strings.HasPrefix(strings.TrimLeftFunc(s, isPySpace), "#") || pyStrip(s) == "" {
			continue
		}
		if strings.HasSuffix(s, "\\") {
			cur += s[:len(s)-1] + " "
		} else {
			logical = append(logical, pyStrip(cur+s))
			cur = ""
		}
	}
	if pyStrip(cur) != "" {
		logical = append(logical, pyStrip(cur))
	}
	var reqs []string
	for _, l := range logical {
		if !strings.HasPrefix(l, "-") {
			reqs = append(reqs, l)
		}
	}
	var bad []string
	for _, l := range reqs {
		if !strings.Contains(l, "==") {
			bad = append(bad, firstWord(l)+": sabit sürüm (==) yok")
		} else if !strings.Contains(l, "--hash=sha256:") {
			bad = append(bad, firstWord(l)+": --hash=sha256 yok")
		}
		if reURLDep.MatchString(l) {
			bad = append(bad, firstWord(l)+": URL/VCS bağımlılığı yasak")
		}
	}
	for _, l := range logical {
		if strings.HasPrefix(l, "-e ") || strings.HasPrefix(l, "-r ") || strings.HasPrefix(l, "--editable") {
			bad = append(bad, "-e/-r yönergeleri yasak")
			break
		}
	}
	if len(reqs) == 0 {
		bad = append(bad, "hiç bağımlılık yok (ansible-core bekleniyordu)")
	}
	if len(bad) > 0 {
		for _, b := range bad {
			c.fail("Düzen", "requirements.txt: "+b)
		}
	} else {
		c.ok("Düzen", fmt.Sprintf("requirements.txt: %d bağımlılık, hepsi sabit sürümlü ve hash'li", len(reqs)))
	}
}

func isPySpace(r rune) bool {
	return r == ' ' || (r >= '\t' && r <= '\r') || (r >= 0x1c && r <= 0x1f) || r == 0x85 || r == 0xa0 ||
		r == 0x1680 || (r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000
}

var codeDirs = []string{"playbooks", "roles", "vars", "inventory", "templates", "files", "scripts"}

func (c *pkgChecker) checkActions(root string, info map[string][]string) bool {
	pid := first(info, "id")
	runner := first(info, "runner")
	code := c.corpus(root, codeDirs)
	mutatingWithoutDocs := false
	for _, a := range info["action"] {
		cmd := first(info, "action."+a+".cmd")
		effects := first(info, "action."+a+".effects")
		sudo := first(info, "action."+a+".sudo")
		nwarn, _ := strconv.Atoi(first(info, "action."+a+".warnings", "0"))
		var requires []string
		for _, r := range strings.Split(first(info, "action."+a+".requires"), ",") {
			if r != "" {
				requires = append(requires, r)
			}
		}
		mutating := effects != ""
		label := a + " (salt okunur)"
		if mutating {
			label = a + " (değiştirir: " + effects + ")"
		}
		var issues []string
		toks := strings.Fields(cmd)
		if runner == "ansible" {
			if len(toks) == 0 || toks[0] != "ansible-playbook" {
				c.fail("Eylemler", a+": cmd 'ansible-playbook' ile başlamalı")
				continue
			}
			var pbs []string
			for _, t := range toks[1:] {
				if strings.HasSuffix(t, ".yml") || strings.HasSuffix(t, ".yaml") {
					pbs = append(pbs, t)
				}
			}
			if len(pbs) == 0 {
				c.fail("Eylemler", a+": cmd bir playbook (.yml) içermiyor")
				continue
			}
			for _, pb := range pbs {
				dotdot := false
				for _, part := range strings.Split(pb, "/") {
					if part == ".." {
						dotdot = true
					}
				}
				if strings.HasPrefix(pb, "/") || dotdot {
					c.fail("Eylemler", fmt.Sprintf("%s: playbook yolu paket içinde ve göreli olmalı (%s)", a, pb))
				} else if !isFile(filepath.Join(root, pb)) {
					c.fail("Eylemler", fmt.Sprintf("%s: playbook yok: %s", a, pb))
				}
			}
		} else if runner == "script" {
			script := ""
			if len(toks) > 0 {
				script = filepath.Join(root, toks[0])
			}
			if len(toks) == 0 || !isFile(script) {
				w := "(boş)"
				if len(toks) > 0 {
					w = toks[0]
				}
				c.fail("Eylemler", fmt.Sprintf("%s: betik yok: %s", a, w))
				continue
			}
			if isLink(script) {
				c.fail("Eylemler", fmt.Sprintf("%s: betik sembolik bağ olamaz: %s", a, toks[0]))
				continue
			}
			if st, _ := os.Stat(script); st != nil && st.Size() > 262144 {
				c.fail("Eylemler", fmt.Sprintf("%s: betik 256 KB'tan büyük: %s", a, toks[0]))
				continue
			}
		}
		if mutating {
			if sudo != "yes" {
				issues = append(issues, "requires_sudo belirtilmemiş")
			}
			if nwarn == 0 {
				issues = append(issues, "hiç warning yok (neyi değiştirdiğini/önizlemenin göstermediklerini yazın)")
			}
			var readonly []string
			for _, r := range requires {
				if first(info, "action."+r+".effects") == "" {
					readonly = append(readonly, r)
				}
			}
			configChanging := false
			for _, e := range strings.Split(effects, ",") {
				if e != "installs-package" {
					configChanging = true
				}
			}
			if len(readonly) == 0 && configChanging && !reRecoveryName.MatchString(a) {
				issues = append(issues, "salt okunur bir 'preview' ön koşulu yok (geri alma/kurtarma eylemleri bilerek ön koşulsuz olabilir)")
			}
			mutatingWithoutDocs = true
		}
		if first(info, "action."+a+".outputs.format") != "" {
			tag := "SSHPOCKET_RESULT " + pid + "." + a
			if !strings.Contains(code, tag) {
				issues = append(issues, fmt.Sprintf("outputs.format var ama playbook/rol/betiklerde '%s' çıktısı bulunamadı", tag))
			}
		}
		if len(issues) > 0 {
			for _, i := range issues {
				c.warn("Eylemler", a+": "+i)
			}
		} else {
			c.ok("Eylemler", label)
		}
	}
	return mutatingWithoutDocs
}

func (c *pkgChecker) checkVars(root string, info map[string][]string) {
	varLines := info["var"]
	if len(varLines) == 0 {
		c.ok("Değişkenler", "değişken tanımı yok")
		return
	}
	code := c.corpus(root, codeDirs)
	for _, vl := range varLines {
		p := strings.Split(vl, ":")
		for len(p) < 4 {
			p = append(p, "")
		}
		name, vtype, req, typed := p[0], p[1], p[2], p[3]
		var notes []string
		if typed == "0" {
			notes = append(notes, "eski biçim (yalnızca ad; tür/varsayılan/açıklama üst verisi yok)")
		}
		isDigest := reSecretName.MatchString(name) && reDigestName.MatchString(name)
		if reSecretName.MatchString(name) && !isDigest && vtype != "bool" && vtype != "int" {
			notes = append(notes, "adı parola/anahtar/token izlenimi veriyor: gizli değer değişken OLAMAZ (standart §6)")
		}
		if !strings.Contains(code, name) && !strings.Contains(code, "SSHPOCKET_VAR_"+strings.ToUpper(name)) {
			notes = append(notes, "playbook/rol/vars/betik içinde kullanılmıyor")
		}
		if len(notes) > 0 {
			for _, n := range notes {
				c.warn("Değişkenler", name+": "+n)
			}
		} else {
			extra1, extra2 := "", ""
			if req == "yes" {
				extra1 = ", zorunlu"
			}
			if isDigest && vtype != "bool" && vtype != "int" {
				extra2 = ", parola ÖZETİ: düz parola olamaz"
			}
			c.ok("Değişkenler", fmt.Sprintf("%s (%s%s%s)", name, vtype, extra1, extra2))
		}
	}
}

func (c *pkgChecker) checkSecurity(root string) {
	bad := false
	c.walkFiles(root, func(f string) {
		rel, _ := filepath.Rel(root, f)
		rel = filepath.ToSlash(rel)
		if isLink(f) {
			c.fail("Güvenlik", "sembolik bağ yasak (kurulumda reddedilir): "+rel)
			bad = true
			return
		}
		if reSecretFile.MatchString(rel) {
			c.fail("Güvenlik", "gizli bilgi dosyası paket içinde: "+rel)
			bad = true
		} else if textExt[strings.ToLower(splitExt(f))] && rePrivateKey.MatchString(readText(f)) {
			c.fail("Güvenlik", "özel anahtar bloğu bulundu: "+rel)
			bad = true
		}
	})
	var walkDirs func(d string)
	walkDirs = func(d string) {
		ents, err := os.ReadDir(d)
		if err != nil {
			return
		}
		sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
		var subs []string
		for _, e := range ents {
			p := filepath.Join(d, e.Name())
			if e.Type()&os.ModeSymlink != 0 {
				if isDir(p) && !c.skipped(e.Name()) {
					rel, _ := filepath.Rel(root, p)
					c.fail("Güvenlik", "sembolik bağ yasak: "+filepath.ToSlash(rel))
					bad = true
				}
				continue
			}
			if e.IsDir() && !c.skipped(e.Name()) {
				subs = append(subs, p)
			}
		}
		for _, s := range subs {
			walkDirs(s)
		}
	}
	walkDirs(root)
	if !bad {
		c.ok("Güvenlik", "sembolik bağ, özel anahtar ve gizli bilgi dosyası yok")
	}
}

func (c *pkgChecker) checkDocs(root string, mutating bool, archive string, unsigned bool) {
	var subs []string
	ents, _ := os.ReadDir(root)
	for _, e := range ents {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			subs = append(subs, e.Name())
		}
	}
	if isDir(filepath.Join(root, "docs")) {
		subs = append(subs, "docs")
	}
	md := c.corpus(root, subs)
	if mutating {
		if reRollback.MatchString(md) {
			c.ok("Belge", "değiştiren eylemler için geri alma/kurtarma anlatılmış görünüyor (belgede anahtar sözcük bulundu)")
		} else {
			c.warn("Belge", "değiştiren eylem var ama README/docs'ta geri alma/kurtarma yolu anlatılmamış (kapsamı dürüstçe yazın)")
		}
	} else {
		c.ok("Belge", "değiştiren eylem yok; geri alma belgesi gerekmiyor")
	}
	if archive != "" && unsigned {
		c.ok("Belge", "imza henüz beklenmiyor (--unsigned)")
	} else if archive != "" {
		if isFile(archive + ".sig") {
			c.ok("Belge", "imza dosyası yanında (.sig); imzayı doğrulamak için scripts/lock-component.sh")
		} else {
			c.warn("Belge", fmt.Sprintf("%s.sig yok (paket 'ssh-keygen -Y sign -n sshpocket-component' ile imzalanmalı)", filepath.Base(archive)))
		}
	}
}

func (c *pkgChecker) render(strict bool) ([]string, int) {
	icon := map[string]string{"ok": "✅", "warn": "⚠️ ", "fail": "❌"}
	nfail, nwarn := 0, 0
	for _, r := range c.results {
		switch r.level {
		case "fail":
			nfail++
		case "warn":
			nwarn++
		}
	}
	var out []string
	for _, sec := range pkgSections {
		var rows []pkgResult
		for _, r := range c.results {
			if r.sec == sec {
				rows = append(rows, r)
			}
		}
		if len(rows) == 0 {
			continue
		}
		out = append(out, sec)
		for _, r := range rows {
			out = append(out, fmt.Sprintf("  %s %s", icon[r.level], r.text))
		}
		out = append(out, "")
	}
	switch {
	case nfail > 0:
		out = append(out, fmt.Sprintf("Sonuç: ❌ uyumsuz (%d hata, %d uyarı)", nfail, nwarn))
	case nwarn > 0 && strict:
		out = append(out, fmt.Sprintf("Sonuç: ❌ --strict: %d uyarı hata sayıldı", nwarn))
	case nwarn > 0:
		out = append(out, fmt.Sprintf("Sonuç: ✅ uyumlu (%d uyarı: bakmanız önerilir)", nwarn))
	default:
		out = append(out, "Sonuç: ✅ uyumlu")
	}
	code := 0
	if nfail > 0 || (strict && nwarn > 0) {
		code = 1
	}
	return out, code
}

// ValidatePackage, `--components validate <dizin|arşiv> [--strict] [--unsigned]` çıktısını ve çıkış kodunu üretir.
func ValidatePackage(arg string, strict, unsigned bool) (lines []string, code int) {
	c := &pkgChecker{}
	root, idHint, verHint, tmp, archive := c.resolveInput(arg)
	if tmp != "" {
		defer os.RemoveAll(tmp)
	}
	if root != "" && archive == "" {
		c.skipDev = append(c.skipDev, devDirs...)
		var found []string
		for _, x := range devDirs {
			if isDir(filepath.Join(root, x)) {
				found = append(found, x)
			}
		}
		if len(found) > 0 {
			c.ok("Düzen", fmt.Sprintf("geliştirme klasörleri atlandı (%s): dizin modu; yayın arşivinde bulunmamalı, arşiv modu atlamaz", strings.Join(found, ", ")))
		}
	}
	lines = append(lines, "sshpocket --components validate "+arg, "")
	if root != "" {
		info := c.checkManifest(root, idHint, verHint)
		mutating := false
		if info != nil {
			c.checkLayout(root, info)
			mutating = c.checkActions(root, info)
			c.checkVars(root, info)
		}
		c.checkSecurity(root)
		c.checkDocs(root, mutating, archive, unsigned)
	}
	out, code := c.render(strict)
	return append(lines, out...), code
}
