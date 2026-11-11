package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// component_run.py ile karşılaştırmalı testler (describe, check, vars, platform, env, net-*).

func runRunPy(t *testing.T, root string, args ...string) pyOut {
	t.Helper()
	cmd := exec.Command("python3", append([]string{"-I", filepath.Join(root, "referans/kabuk/sistem/Helpers/component_run.py")}, args...)...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return pyOut{code, splitLines(so.String()), splitLines(se.String())}
}

func eq(t *testing.T, label string, py pyOut, lines, errs []string, code int) {
	t.Helper()
	if code != py.code || !reflect.DeepEqual(lines, py.stdout) || !reflect.DeepEqual(errs, py.stderr) {
		t.Errorf("%s: fark\n  python: kod=%d out=%q err=%q\n  go:     kod=%d out=%q err=%q", label, py.code, py.stdout, py.stderr, code, lines, errs)
	}
}

func writeTemp(t *testing.T, dir, name, content string) string {
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDescribePlatformMatchPython(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	for _, p := range realManifests(t, root) {
		rel, _ := filepath.Rel(root, p)
		py := runRunPy(t, root, "describe", p)
		l, e, c := Describe(p, "", false)
		eq(t, rel+" describe", py, l, e, c)

		data, _ := os.ReadFile(p)
		var m map[string]any
		json.Unmarshal(data, &m)
		pv, _ := m["platform_var"].(string)
		var ids []string
		if pc, ok := m["platform_choices"].([]any); ok {
			for _, c := range pc {
				ids = append(ids, c.(map[string]any)["id"].(string))
			}
		}
		for _, id := range append(ids, "", "bilinmeyen") {
			vf := writeTemp(t, dir, "v.vars", fmt.Sprintf("# not\n%s=%s\nother = 1\n", pv, id))
			py := runRunPy(t, root, "describe", p, vf)
			l, e, c := Describe(p, vf, true)
			eq(t, rel+" describe vars="+id, py, l, e, c)
			py = runRunPy(t, root, "platform", p, vf)
			pl, pe, pc := Platform(p, vf)
			eq(t, rel+" platform vars="+id, py, nonEmpty(pl), pe, pc)
		}
		py = runRunPy(t, root, "describe", p, filepath.Join(dir, "yok.vars"))
		l, e, c = Describe(p, filepath.Join(dir, "yok.vars"), true)
		eq(t, rel+" describe yok", py, l, e, c)
	}
}

// nonEmpty: Python boş platformda boş satır yazar; splitLines bunu nil yapar, Go tarafını da aynı biçime getirir.
func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func init() { _ = nonEmpty }

var checkRaws = []string{"", " ", "true", "yes", "No", "FALSE", "maybe", "5", "-5", "0", "99999999999999", "abc", "10.0.0.0/24", "10.0.0.1", "10.0.0.1/24",
	"10.0.0.1/255.255.255.0", "10.0.0.1/0.0.0.255", "10.0.0.1/33", "10.0.0.1/", "::1", "fe80::/64", "fe80::1/129", "300.1.1.1", "01.2.3.4", "1.2.3.4/08",
	"10.0.0.0/24/5", "a,b", "a,,b", ",", "a, b ,c", "10.0.0.0/24,192.168.0.0/16", "x\x01y", "/tmp/x", strings.Repeat("a", 1025), "ü,ö", "22", "ssh", "0.0.0.0/0", "::/0", "2001:db8::/32"}

func pickedManifest(p string) bool {
	return strings.Contains(p, "utility/0.1.0") || strings.Contains(p, "kurulum/0.1.0") || strings.Contains(p, "linux-hardening/0.13.0") || strings.Contains(p, "linux-hardening/0.3.0")
}

func TestCheckMatchesPython(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	root := repoRoot(t)
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	defer wg.Wait()
	for _, p := range realManifests(t, root) {
		if !pickedManifest(p) {
			continue
		}
		rel, _ := filepath.Rel(root, p)
		data, _ := os.ReadFile(p)
		var m map[string]any
		json.Unmarshal(data, &m)
		vs, _ := m["vars"].([]any)
		raws := append([]string(nil), checkRaws...)
		for _, v := range vs {
			vo, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if vals, ok := vo["values"].([]any); ok {
				for _, x := range vals {
					raws = append(raws, x.(string), x.(string)+","+x.(string))
				}
			}
		}
		for _, v := range vs {
			name := ""
			switch t := v.(type) {
			case string:
				name = t
			case map[string]any:
				name, _ = t["name"].(string)
			}
			for _, raw := range raws {
				wg.Add(1)
				sem <- struct{}{}
				go func(p, rel, name, raw string) {
					defer wg.Done()
					defer func() { <-sem }()
					py := runRunPy(t, root, "check", p, name, raw)
					warn, e, c := Check(p, name, raw)
					var out []string
					if warn {
						out = []string{"warn=" + name}
					}
					eq(t, fmt.Sprintf("%s check %s %q", rel, name, raw), py, out, e, c)
				}(p, rel, name, raw)
			}
		}
		py := runRunPy(t, root, "check", p, "olmayan_degisken", "x")
		_, e, c := Check(p, "olmayan_degisken", "x")
		eq(t, rel+" check olmayan", py, nil, e, c)
	}
}

func TestVarsMatchesPython(t *testing.T) {
	root := repoRoot(t)
	rng := rand.New(rand.NewSource(7))
	for _, p := range realManifests(t, root) {
		if !pickedManifest(p) {
			continue
		}
		rel, _ := filepath.Rel(root, p)
		data, _ := os.ReadFile(p)
		var m map[string]any
		json.Unmarshal(data, &m)
		type vspec struct {
			name, typ string
			vals      []string
		}
		var specs []vspec
		for _, v := range m["vars"].([]any) {
			vo, ok := v.(map[string]any)
			if !ok {
				continue
			}
			s := vspec{name: vo["name"].(string)}
			s.typ, _ = vo["type"].(string)
			if s.typ == "" {
				s.typ = "string"
			}
			if vl, ok := vo["values"].([]any); ok {
				for _, x := range vl {
					s.vals = append(s.vals, x.(string))
				}
			}
			specs = append(specs, s)
		}
		sample := func(s vspec, bad bool) string {
			base := strings.TrimSuffix(strings.TrimPrefix(s.typ, "list["), "]")
			one := map[string]string{"string": "abc", "path": "/tmp/x", "bool": "true", "int": "5", "cidr": "10.0.0.0/24", "enum": "x"}[base]
			if base == "enum" && len(s.vals) > 0 {
				one = s.vals[0]
			}
			if bad {
				one = map[string]string{"string": "a\x01b", "path": "p\x02", "bool": "perhaps", "int": "zzz", "cidr": "999.9.9.9/99", "enum": "NOPE"}[base]
			}
			if strings.HasPrefix(s.typ, "list[") {
				return one + "," + one
			}
			return one
		}
		for trial := 0; trial < 60; trial++ {
			var buf strings.Builder
			buf.WriteString("# deneme\n")
			for _, s := range specs {
				switch rng.Intn(5) {
				case 0: // yok
				case 1:
					fmt.Fprintf(&buf, "%s=\n", s.name)
				case 2:
					fmt.Fprintf(&buf, "%s=%s\n", s.name, sample(s, true))
				default:
					fmt.Fprintf(&buf, "%s=%s\n", s.name, sample(s, false))
				}
			}
			if rng.Intn(6) == 0 {
				buf.WriteString("bilinmeyen_x=1\n")
			}
			if rng.Intn(8) == 0 {
				buf.WriteString("satir bozuk\n")
			}
			dir := t.TempDir()
			vf := writeTemp(t, dir, "v.vars", buf.String())
			noReq := rng.Intn(2) == 0
			var forced []string
			if len(specs) > 0 && rng.Intn(3) == 0 {
				s := specs[rng.Intn(len(specs))]
				forced = []string{"--set", s.name + "=" + sample(s, rng.Intn(2) == 0)}
			}
			args := []string{"vars", p, vf, filepath.Join(dir, "py.json")}
			opt := VarsOptions{OutPath: filepath.Join(dir, "go.json"), Forced: map[string]string{}}
			if noReq {
				args = append(args, "--no-required")
				opt.NoReq = true
			}
			if forced != nil {
				args = append(args, forced...)
				k, v, _ := strings.Cut(forced[1], "=")
				opt.Forced[k] = v
			}
			py := runRunPy(t, root, args...)
			warns, errs, code := BuildVars(p, vf, opt)
			var out []string
			for _, w := range warns {
				out = append(out, "warn="+w)
			}
			if code == 0 {
				out = append(out, "ok")
			}
			eq(t, fmt.Sprintf("%s vars #%d\n%s", rel, trial, buf.String()), py, out, errs, code)
			if code == 0 && py.code == 0 {
				a, _ := os.ReadFile(filepath.Join(dir, "py.json"))
				b, _ := os.ReadFile(filepath.Join(dir, "go.json"))
				if !bytes.Equal(a, b) {
					t.Errorf("%s vars #%d: JSON farkı\n--py--\n%s\n--go--\n%s", rel, trial, a, b)
				}
				st, _ := os.Stat(filepath.Join(dir, "go.json"))
				if st.Mode().Perm() != 0o600 {
					t.Errorf("izin %v", st.Mode().Perm())
				}
				// env: aynı vars.json üzerinde
				el, ee, ec := Env(filepath.Join(dir, "go.json"), "srv", "apply", "ubuntu-26.04")
				epy := runRunPy(t, root, "env", filepath.Join(dir, "py.json"), "srv", "apply", "ubuntu-26.04")
				eq(t, rel+" env", epy, el, ee, ec)
			}
		}
	}
}

func TestEnvBadNames(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	vj := writeTemp(t, dir, "v.json", `{"a_b": "it's \"q\" $x", "n": 5, "l": [1,"x",true], "t": true}`)
	for _, args := range [][3]string{{"srv", "apply", "p"}, {"-bad", "apply", "p"}, {"srv", "a b", "p"}, {"srv", "apply", ""}} {
		py := runRunPy(t, root, "env", vj, args[0], args[1], args[2])
		l, e, c := Env(vj, args[0], args[1], args[2])
		eq(t, fmt.Sprint(args), py, l, e, c)
	}
	for i, body := range []string{`{"A": 1}`, `{"a": 1.5}`, `{"a": null}`, `{"a": {"b": 1}}`, `{"a": "x\u0001y"}`, `[1]`, `{"a": ["é", 2.5]}`} {
		f := writeTemp(t, dir, fmt.Sprintf("b%d.json", i), body)
		py := runRunPy(t, root, "env", f, "srv", "apply", "p")
		l, _, c := Env(f, "srv", "apply", "p")
		// hata iletisinde Python'un okuma hatası metni değişebilir; çıkış kodu ve satırlar aynı olmalı
		if c != py.code || (c == 0 && !reflect.DeepEqual(l, py.stdout)) {
			t.Errorf("%s: fark py=%d %q go=%d %q", body, py.code, py.stdout, c, l)
		}
	}
}

func TestNetMatchesPython(t *testing.T) {
	root := repoRoot(t)
	addrs := []string{"10.0.0.5", "192.168.1.200", "::1", "fe80::1", "2001:db8::5", "300.1.1.1", "", "1.2.3", "01.2.3.4", "10.0.0.5/24"}
	for _, a := range addrs {
		py := runRunPy(t, root, "net-valid", a)
		s, c := NetValid(a)
		var out []string
		if c == 0 {
			out = []string{s}
		}
		eq(t, "net-valid "+a, py, out, nil, c)
		for _, plen := range []string{"0", "8", "24", "32", "33", "64", "128", "129", "-1", "x", ""} {
			py := runRunPy(t, root, "net-suggest", a, plen)
			l, c := NetSuggest(a, plen)
			eq(t, "net-suggest "+a+" "+plen, py, l, nil, c)
		}
		for _, mk := range []string{"255.255.255.0", "255.255.0.0", "0xffffff00", "0xffff0000", "255.0.255.0", "0x0", "0xffffffff", "ffff:ffff:ffff:ffff::", "0xffffffffffffffff0000000000000000", "x", "", "255.255.255.255", "0.0.0.0"} {
			py := runRunPy(t, root, "net-prefix", a, mk)
			s, c := NetPrefix(a, mk)
			var out []string
			if c == 0 {
				out = []string{s}
			}
			eq(t, "net-prefix "+a+" "+mk, py, out, nil, c)
		}
	}
	lists := []string{"", "10.0.0.0/24", "10.0.0.0/24,192.168.0.0/16", "10.0.0.0/8", "10.1.0.0/16,10.2.0.0/16", "fe80::/64", "10.0.0.0/24,fe80::/10", "bozuk", "10.0.0.0/24,"}
	news := []string{"10.0.0.0/24", "10.0.0.5", "10.0.0.7/24", "10.0.0.0/16", "10.0.0.0/8", "192.168.5.0/24", "fe80::1", "fe80::/10", "2001:db8::/32", "x", "10.0.0.0/255.255.255.0", "0.0.0.0/0", "10.0.0.0/0.0.0.255"}
	for _, l := range lists {
		for _, n := range news {
			py := runRunPy(t, root, "net-add", l, n)
			out, c := NetAdd(l, n)
			eq(t, "net-add "+l+" + "+n, py, out, nil, c)
		}
	}
}
