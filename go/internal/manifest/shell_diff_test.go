package manifest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ComponentsVars.sh / ComponentsRun.sh / ComponentsManifest.sh ile karşılaştırmalı testler (bash gerekir).

type shOut struct {
	code   int
	stdout string
	stderr string
}

// testFnDef, runBash çağrısında kütüphanelerden sonra tanımlanacak sınama işlevidir.
var testFnDef string

func runBash(t *testing.T, root string, libs []string, fn string, args ...string) shOut {
	t.Helper()
	type bGold struct {
		Code           int
		Stdout, Stderr string
	}
	g := goldDo(t, func() bGold {
		o := runBashLive(t, root, libs, fn, args...)
		return bGold{o.code, reTmp.ReplaceAllString(o.stdout, "<T>"), reTmp.ReplaceAllString(o.stderr, "<T>")}
	})
	return shOut{g.Code, g.Stdout, g.Stderr}
}

// runBashF: runBash + kabuğun çalıştıktan sonra bıraktığı dosyaların durumu (yan etkili işlevler için; altın dosyadan).
func runBashF(t *testing.T, root string, libs []string, fn string, files []string, args ...string) (shOut, []string) {
	t.Helper()
	type fGold struct {
		Code           int
		Stdout, Stderr string
		States         []string
	}
	g := goldDo(t, func() fGold {
		o := runBashLive(t, root, libs, fn, args...)
		var st []string
		for _, f := range files {
			st = append(st, fileState(f))
		}
		return fGold{o.code, reTmp.ReplaceAllString(o.stdout, "<T>"), reTmp.ReplaceAllString(o.stderr, "<T>"), st}
	})
	return shOut{g.Code, g.Stdout, g.Stderr}, g.States
}

func runBashLive(t *testing.T, root string, libs []string, fn string, args ...string) shOut {
	t.Helper()
	script := `export SISTEM_PATH="$1"; shift; for l in $LIBS; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; if [ -n "$TEST_FN_DEF" ]; then eval "$TEST_FN_DEF"; fi; "$@"`
	cmd := exec.Command("bash", append([]string{"-c", script, "_", filepath.Join(root, "referans/kabuk/sistem"), fn}, args...)...)
	cmd.Env = append(os.Environ(), "LIBS="+strings.Join(libs, " "), "TEST_FN_DEF="+testFnDef)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return shOut{code, so.String(), se.String()}
}

func fileState(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return "(yok)"
	}
	st, _ := os.Stat(p)
	return fmt.Sprintf("%o|%q", st.Mode().Perm(), b)
}

func TestVarsFileOpsMatchShell(t *testing.T) {
	root := goldRoot(t)
	inits := map[string]string{
		"yok":           "\x00",
		"bos":           "",
		"yorumlu":       "# baslik\nA=1\n  B=2\n# A=9\nC = 3\nAB=4\nA=5\n",
		"crlf":          "A=1\r\nB=2\r\n",
		"son satir yok": "A=1\nB=2",
		"bosluklu":      "\t A=  x y  \nB=\n",
	}
	type op struct {
		kind, name, val string
	}
	ops := []op{
		{"set", "a", "1"}, {"set", "A", "1"}, {"set", "a", ""}, {"set", "b", "x y"}, {"set", "b", " lead trail "}, {"set", "c", "with = eq # hash"},
		{"set", "a", "tab\there"}, {"set", "a", "nl\nhere"}, {"set", "a", "cr\rhere"}, {"set", "a", "ctl\x01x"}, {"set", "a", "del\x7fx"},
		{"set", "ünicode", "x"}, {"set", "a", "üğişçö 日本"}, {"set", "ab", "yeni"}, {"set", strings.Repeat("a", 64), "x"}, {"set", strings.Repeat("a", 65), "x"},
		{"set", "1x", "x"}, {"set", "", "x"},
		{"unset", "a", ""}, {"unset", "b", ""}, {"unset", "yok", ""}, {"unset", "A", ""},
	}
	for _, ip := range ordered(inits) {
		iname, content := ip.K, ip.V
		for _, o := range ops {
			dir := t.TempDir()
			sf, gf := filepath.Join(dir, "sh.vars"), filepath.Join(dir, "go.vars")
			if content != "\x00" {
				os.WriteFile(sf, []byte(content), 0o644)
				os.WriteFile(gf, []byte(content), 0o644)
			}
			var sh shOut
			var shStates []string
			var gerr error
			switch o.kind {
			case "set":
				sh, shStates = runBashF(t, root, []string{"ComponentsVars.sh"}, "comp_vars_set", []string{sf}, sf, o.name, o.val)
				gerr = VarsSet(gf, o.name, o.val)
			case "unset":
				sh, shStates = runBashF(t, root, []string{"ComponentsVars.sh"}, "comp_vars_unset", []string{sf}, sf, o.name)
				gerr = VarsUnset(gf, o.name)
			}
			gcode := 0
			if gerr != nil {
				gcode = 1
			}
			label := fmt.Sprintf("%s %s %q=%q", iname, o.kind, o.name, o.val)
			if sh.code != gcode {
				t.Errorf("%s: çıkış kabuk=%d go=%d (%v) stderr=%q", label, sh.code, gcode, gerr, sh.stderr)
				continue
			}
			if gerr != nil && sh.stderr != "" && strings.TrimSpace(sh.stderr) != gerr.Error() {
				t.Errorf("%s: ileti kabuk=%q go=%q", label, sh.stderr, gerr)
			}
			if a, b := shStates[0], fileState(gf); a != b {
				t.Errorf("%s: dosya farkı\n kabuk=%s\n go   =%s", label, a, b)
			}
			// get: aynı dosya durumunda aynı değer
			if o.kind == "set" && gerr == nil {
				g := runBash(t, root, []string{"ComponentsVars.sh"}, "comp_vars_get", sf, o.name)
				gv, ok := VarsGet(gf, o.name)
				if (g.code == 0) != ok || (ok && g.stdout != gv) {
					t.Errorf("%s: get kabuk=%d %q go=%v %q", label, g.code, g.stdout, ok, gv)
				}
			}
		}
	}
	// get: çeşitli adlar
	dir := t.TempDir()
	f := filepath.Join(dir, "g.vars")
	os.WriteFile(f, []byte("# c\nA=1\n  b = 2 \nab=3\nc=  x y \t\nd=\n\te=7\r\n"), 0o644)
	for _, n := range []string{"a", "b", "ab", "c", "d", "e", "yok", "A", "", "1"} {
		g := runBash(t, root, []string{"ComponentsVars.sh"}, "comp_vars_get", f, n)
		gv, ok := VarsGet(f, n)
		if (g.code == 0) != ok || (ok && g.stdout != gv) {
			t.Errorf("get %q: kabuk=%d %q go=%v %q", n, g.code, g.stdout, ok, gv)
		}
	}
	if _, ok := VarsGet(filepath.Join(dir, "yok.vars"), "a"); ok {
		t.Error("olmayan dosyadan değer okundu")
	}
}

// Kabuk motoru (awk -v) var olan dosyada ters bölü dizilerini yorumlar: `C:\temp\new` → sekme + satır sonu,
// yani değerden fazladan satır sızdırılabilir. Go değeri olduğu gibi yazar.
func TestVarsSetKeepsBackslash(t *testing.T) {
	for _, existing := range []bool{false, true} {
		f := filepath.Join(t.TempDir(), "v.vars")
		if existing {
			os.WriteFile(f, []byte("x=1\n"), 0o600)
		}
		if err := VarsSet(f, "p", `C:\temp\new`); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(f)
		want := "p=C:\\temp\\new\n"
		if existing {
			want = "x=1\n" + want
		}
		if string(b) != want {
			t.Fatalf("ters bölü bozuldu (var olan dosya=%v): %q", existing, b)
		}
		if v, _ := VarsGet(f, "p"); v != `C:\temp\new` {
			t.Fatalf("okunan %q", v)
		}
	}
}

func TestShellMangledBackslashOnExistingFile(t *testing.T) {
	root := goldRoot(t)
	f := filepath.Join(t.TempDir(), "v.vars")
	os.WriteFile(f, []byte("x=1\n"), 0o600)
	runBash(t, root, []string{"ComponentsVars.sh"}, "comp_vars_set", f, "p", `a\nq=evil`)
	b, _ := os.ReadFile(f)
	t.Logf("kabuk sonucu: %q", b)
	if !strings.Contains(string(b), "\nq=evil") {
		t.Skip("bu awk ters bölüyü yorumlamıyor; kabuk hatası bu makinede görünmüyor")
	}
	g := filepath.Join(t.TempDir(), "g.vars")
	os.WriteFile(g, []byte("x=1\n"), 0o600)
	VarsSet(g, "p", `a\nq=evil`)
	gb, _ := os.ReadFile(g)
	if strings.Count(string(gb), "\n") != 2 {
		t.Fatalf("Go fazladan satır üretti: %q", gb)
	}
}

func TestVarsMissingMatchesShell(t *testing.T) {
	root := repoRoot(t)
	for _, p := range realManifests(t, root) {
		dir := t.TempDir()
		vf := filepath.Join(dir, "x.vars")
		for _, content := range []string{"", "\x00", "bilinmeyen=1\n"} {
			if content != "\x00" {
				os.WriteFile(vf, []byte(content), 0o644)
			} else {
				os.Remove(vf)
			}
			sh := runBashLive(t, root, []string{"ComponentsRun.sh", "ComponentsVars.sh"}, "comp_vars_missing", p, vf)
			want := splitLines(sh.stdout)
			got := VarsMissing(p, vf)
			if !reflect.DeepEqual(got, want) {
				rel, _ := filepath.Rel(root, p)
				t.Errorf("%s %q: kabuk=%q go=%q", rel, content, want, got)
			}
		}
		// zorunlu değişkenleri doldurunca boşalmalı
		if miss := VarsMissing(p, vf); len(miss) > 0 {
			for _, n := range miss {
				VarsSet(vf, n, "x")
			}
			sh := runBashLive(t, root, []string{"ComponentsRun.sh", "ComponentsVars.sh"}, "comp_vars_missing", p, vf)
			if got := VarsMissing(p, vf); !reflect.DeepEqual(got, splitLines(sh.stdout)) {
				t.Errorf("doldurma sonrası fark: kabuk=%q go=%q", sh.stdout, got)
			}
		}
	}
}

func TestVarsSaveMatchesShell(t *testing.T) {
	root := repoRoot(t)
	p := filepath.Join(root, "components/utility/0.1.0/component.json")
	dir := t.TempDir()
	for _, c := range [][2]string{{"utility_timezone", "Europe/Istanbul"}, {"utility_backup", "evet"}, {"utility_backup", "true"}, {"yok", "x"}, {"utility_timezone", ""}} {
		sf, gf := filepath.Join(dir, "s.vars"), filepath.Join(dir, "g.vars")
		os.Remove(sf)
		os.Remove(gf)
		sh := runBashLive(t, root, []string{"ComponentsRun.sh", "ComponentsVars.sh"}, "comp_vars_save", p, sf, c[0], c[1])
		warn, err := VarsSave(p, gf, c[0], c[1])
		gcode := 0
		if err != nil {
			gcode = 1
		}
		if sh.code != gcode || fileState(sf) != fileState(gf) || (warn != strings.Contains(sh.stdout, "warn=")) {
			t.Errorf("%v: kabuk=%d %q go=%d warn=%v err=%v", c, sh.code, sh.stdout, gcode, warn, err)
		}
	}
}

func TestInventoryMatchesShell(t *testing.T) {
	root := goldRoot(t)
	type c struct{ group, alias, host, user, port, key, become string }
	base := c{"Grup", "srv1", "10.0.0.5", "root", "22", "/home/u/.ssh/id", ""}
	cases := []c{base}
	mut := func(f func(*c)) {
		x := base
		f(&x)
		cases = append(cases, x)
	}
	mut(func(x *c) { x.become = "/usr/bin/sudo.ws" })
	mut(func(x *c) { x.become = "/bin/evil" })
	mut(func(x *c) { x.group = "1bad" })
	mut(func(x *c) { x.group = "" })
	mut(func(x *c) { x.alias = "-bad" })
	mut(func(x *c) { x.alias = "a b" })
	mut(func(x *c) { x.host = "host name" })
	mut(func(x *c) { x.host = "::1" })
	mut(func(x *c) { x.host = "fe80::1" })
	mut(func(x *c) { x.user = "9x" })
	mut(func(x *c) { x.user = "ok_user.1-x" })
	mut(func(x *c) { x.port = "0" })
	mut(func(x *c) { x.port = "65535" })
	mut(func(x *c) { x.port = "65536" })
	mut(func(x *c) { x.port = "x" })
	mut(func(x *c) { x.port = "" })
	mut(func(x *c) { x.key = "relative/key" })
	mut(func(x *c) { x.key = "/a'b" })
	mut(func(x *c) { x.key = "/a\"b" })
	mut(func(x *c) { x.key = `/a\b` })
	mut(func(x *c) { x.key = "/a b/ç" })
	mut(func(x *c) { x.key = "" })
	for i, cs := range cases {
		dir := t.TempDir()
		sf, gf := filepath.Join(dir, "sh.ini"), filepath.Join(dir, "go.ini")
		sh, shStates := runBashF(t, root, []string{"ComponentsRun.sh"}, "comp_run_write_inventory", []string{sf}, sf, cs.group, cs.alias, cs.host, cs.user, cs.port, cs.key, cs.become)
		err := WriteInventory(gf, cs.group, cs.alias, cs.host, cs.user, cs.port, cs.key, cs.become)
		gcode := 0
		if err != nil {
			gcode = 1
		}
		if sh.code != gcode || shStates[0] != fileState(gf) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) {
			t.Errorf("#%d %+v: kabuk=%d %q go=%v\n kabuk dosya=%s\n go dosya   =%s", i, cs, sh.code, sh.stderr, err, shStates[0], fileState(gf))
		}
	}
	// var olan dosyanın üzerine yazmaz
	dir := t.TempDir()
	sf, gf := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(sf, []byte("x"), 0o600)
	os.WriteFile(gf, []byte("x"), 0o600)
	sh := runBash(t, root, []string{"ComponentsRun.sh"}, "comp_run_write_inventory", sf, "G", "s", "h", "u", "22", "/k", "")
	err := WriteInventory(gf, "G", "s", "h", "u", "22", "/k", "")
	if sh.code == 0 || err == nil || strings.TrimSpace(sh.stderr) != err.Error() {
		t.Errorf("var olan dosya: kabuk=%d %q go=%v", sh.code, sh.stderr, err)
	}
}

func TestBuildArgvMatchesShell(t *testing.T) {
	root := repoRoot(t)
	libs := []string{"ComponentsManifest.sh", "ComponentsRun.sh"}
	testFnDef = `th() { local info="$1" action="$2"; comp_run_build_argv "$info" "$action" /tmp/inv.ini srvx /tmp/vars.json; local rc=$?; printf '%s\0' "${COMP_RUN_ARGV[@]}"; printf 'INT=%s\n' "$COMP_RUN_INTERACTIVE"; return $rc; }`
	defer func() { testFnDef = "" }()
	check := func(label string, info []string, action string) {
		t.Helper()
		sh := runBashLive(t, root, libs, "th", strings.Join(info, "\n"), action)
		argv, inter, err := BuildArgv(info, action, "/tmp/inv.ini", "srvx", "/tmp/vars.json")
		gcode := 0
		if err != nil {
			gcode = 1
		}
		var shArgv []string
		shInter := false
		if i := strings.LastIndex(sh.stdout, "INT="); i >= 0 {
			shInter = strings.HasPrefix(sh.stdout[i:], "INT=1")
			body := sh.stdout[:i]
			if body != "" {
				shArgv = strings.Split(strings.TrimSuffix(body, "\x00"), "\x00")
			}
		}
		if sh.code != gcode {
			t.Errorf("%s: çıkış kabuk=%d (%q) go=%d (%v)", label, sh.code, sh.stderr, gcode, err)
			return
		}
		if gcode != 0 {
			if strings.TrimSpace(sh.stderr) != err.Error() {
				t.Errorf("%s: ileti kabuk=%q go=%q", label, sh.stderr, err)
			}
			return
		}
		if !reflect.DeepEqual(argv, shArgv) || inter != shInter {
			t.Errorf("%s: kabuk=%q int=%v go=%q int=%v", label, shArgv, shInter, argv, inter)
		}
	}
	for _, p := range realManifests(t, root) {
		lines, _, code := ValidateFile(p, "", "")
		if code != 0 {
			continue
		}
		rel, _ := filepath.Rel(root, p)
		for _, a := range InfoAll(lines, "action") {
			check(rel+" "+a, lines, a)
		}
		check(rel+" bilinmeyen", lines, "yok_eylem")
	}
	syn := func(extra ...string) []string {
		return append([]string{"runner=ansible", "action=x", "action.x.cmd=ansible-playbook playbooks/x.yml", "action.x.sudo=no", "action.x.interactive=no"}, extra...)
	}
	check("invoke arg", syn("invoke=--tags|t1|no|no"), "x")
	check("invoke bayrak", syn("invoke=--check||no|no"), "x")
	check("invoke interaktif", syn("invoke=--ask-vault-pass||no|yes"), "x")
	check("invoke -e atlanir", syn("invoke=-e|a=b|no|no"), "x")
	check("invoke yer tutucu", syn("invoke=--foo|<kullanici>|no|no"), "x")
	check("sudo ekler", []string{"runner=ansible", "action=x", "action.x.cmd=ansible-playbook p.yml", "action.x.sudo=yes"}, "x")
	check("sudo -K var", []string{"runner=ansible", "action=x", "action.x.cmd=ansible-playbook p.yml", "action.x.sudo=yes", "invoke=-K||no|no"}, "x")
	check("sudo ask var", []string{"runner=ansible", "action=x", "action.x.cmd=ansible-playbook p.yml", "action.x.sudo=yes", "invoke=--ask-become-pass||no|no"}, "x")
	for _, bad := range []string{"-i", "--inventory", "--limit", "-l", "-e", "--extra-vars", "--inventory=x", "--limit=x", "--extra-vars=x", "-i=x", "-l=x", "-e=x"} {
		check("yasak "+bad, []string{"runner=ansible", "action=x", "action.x.cmd=ansible-playbook " + bad + " p.yml"}, "x")
	}
	check("script runner", []string{"runner=script", "action=x", "action.x.cmd=scripts/a.sh"}, "x")
	check("komut yok", []string{"runner=ansible", "action=x"}, "x")
}

func TestNetSuggestForMatchesShell(t *testing.T) {
	root := goldRoot(t)
	for _, a := range []string{"10.0.0.5/24", "10.0.0.5", "::1", "fe80::1/64", "10.0.0.5/32", "10.0.0.5/33", "10.0.0.5/x", "x/24", "10.0.0.5/", "2001:db8::5/128", "1.2.3.4/0", "1.2.3.4/1000"} {
		sh := runBash(t, root, []string{"ComponentsRun.sh", "ComponentsVars.sh"}, "comp_net_suggest", a)
		l, c := NetSuggestFor(a)
		if (sh.code == 0) != (c == 0) || (c == 0 && !reflect.DeepEqual(l, splitLines(sh.stdout))) {
			t.Errorf("%q: kabuk=%d %q go=%d %q", a, sh.code, sh.stdout, c, l)
		}
	}
}

func TestInfoAccessorsMatchShell(t *testing.T) {
	root := goldRoot(t)
	info := []string{"id=x", "action=a", "action=b", "action.a.platforms=ubuntu-26.04,debian-13", "action.b.cmd=c d", "runner=ansible", "x=a=b"}
	for _, k := range []string{"id", "action", "x", "yok", "action.a.platforms"} {
		sh := runBash(t, root, []string{"ComponentsManifest.sh"}, "comp_info_get", strings.Join(info, "\n"), k)
		if strings.TrimSuffix(sh.stdout, "\n") != InfoGet(info, k) {
			t.Errorf("get %s: %q vs %q", k, sh.stdout, InfoGet(info, k))
		}
		sa := runBash(t, root, []string{"ComponentsManifest.sh"}, "comp_info_all", strings.Join(info, "\n"), k)
		if !reflect.DeepEqual(splitLines(sa.stdout), InfoAll(info, k)) {
			t.Errorf("all %s: %q vs %q", k, sa.stdout, InfoAll(info, k))
		}
	}
	for _, c := range [][2]string{{"a", "ubuntu-26.04"}, {"a", "debian-13"}, {"a", "arch"}, {"a", ""}, {"b", "arch"}, {"yok", "x"}} {
		sh := runBash(t, root, []string{"ComponentsManifest.sh"}, "comp_action_on_platform", strings.Join(info, "\n"), c[0], c[1])
		if (sh.code == 0) != ActionOnPlatform(info, c[0], c[1]) {
			t.Errorf("platform %v: kabuk=%d go=%v", c, sh.code, ActionOnPlatform(info, c[0], c[1]))
		}
	}
}
