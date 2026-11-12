package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

const fakePyTmpl = `#!/bin/bash
case "$1" in
  -c) case "$2" in *sys.version_info*) echo "%s"; exit 0 ;; *) exit "${FAKE_VENVPY_RC:-0}" ;; esac ;;
  -m) case "$2" in
        venv) [ "$FAKE_VENV_FAIL" = 1 ] && exit 1; mkdir -p "$3/bin" && cp "$0" "$3/bin/python"; exit 0 ;;
        pip) echo "pip $*" >> "${FAKE_LOG:-/dev/null}"; [ -n "$FAKE_ANSIBLE_SRC" ] && { cp "$FAKE_ANSIBLE_SRC" "$(dirname "$0")/ansible-playbook"; chmod +x "$(dirname "$0")/ansible-playbook"; }; exit "${FAKE_PIP_RC:-0}" ;;
      esac ;;
esac
exit 0
`

func fakePythons(t *testing.T, versions map[string]string) {
	dir := t.TempDir()
	for name, ver := range versions {
		os.WriteFile(filepath.Join(dir, name), []byte(fmt.Sprintf(fakePyTmpl, ver)), 0o755)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_LOG", filepath.Join(dir, "calls.log"))
}

func TestPythonInRangeMatchesShell(t *testing.T) {
	root := goldRoot(t)
	for _, v := range []string{"3.12.0", "3.12.4", "3.11.9", "3.13.1", "4.0.0", "3.9.6", "2.7.18", "3.12", "x", "", "3.12.4.1", "03.12.0", "3.100.0"} {
		for _, r := range []string{">=3.12", ">=3.12,<4.0", ">=3.10,<3.13", ">=3.12,<3.12", ">=3", "3.12", ">=3.12,", ">=3.12,<4", ""} {
			sh := runShell(t, root, nil, "", "", "comp_python_in_range", v, r)
			if got := PythonInRange(v, r); (sh.code == 0) != got {
				t.Errorf("%q %q: kabuk=%d go=%v", v, r, sh.code, got)
			}
		}
	}
}

func TestRequirementsHashedMatchesShell(t *testing.T) {
	root := goldRoot(t)
	h := "--hash=sha256:" + strings.Repeat("a", 64)
	cases := map[string]string{
		"geçerli":         "pkg==1.0 \\\n  " + h + "\n",
		"iki paket":       "a==1 \\\n  " + h + " \\\n  " + h + "\nb==2.0.1 \\\n  " + h + "\n",
		"extras":          "a[x,y]==1 \\\n  " + h + "\n",
		"marker":          "a==1 ; python_version >= '3.9' \\\n  " + h + "\n",
		"yorum + boş":     "# yorum\n\na==1 \\\n  " + h + "\n",
		"crlf":            "a==1 \\\r\n  " + h + "\r\n",
		"hash yok":        "a==1\n",
		"son hash yok":    "a==1 \\\n  " + h + "\nb==2\n",
		"ara hash yok":    "a==1 \\\nb==2 \\\n  " + h + "\n",
		"başıboş hash":    h + "\n",
		"-r":              "-r other.txt\n",
		"-e":              "-e .\n",
		"index-url":       "--index-url https://x\na==1 \\\n  " + h + "\n",
		"sürüm yok":       "a>=1 \\\n  " + h + "\n",
		"url":             "a @ https://x/a.whl \\\n  " + h + "\n",
		"boş":             "",
		"yalnız yorum":    "# x\n",
		"kısa hash":       "a==1 \\\n  --hash=sha256:abc\n",
		"büyük harf hash": "a==1 \\\n  --hash=sha256:" + strings.Repeat("A", 64) + "\n",
		"satır sonu yok":  "a==1 \\\n  " + h,
		"girintili":       "   a==1 \\\n      " + h + "\n",
		"sha512":          "a==1 \\\n  --hash=sha512:" + strings.Repeat("a", 128) + "\n",
	}
	dir := t.TempDir()
	for _, p := range ordered(cases) {
		name, content := p.K, p.V
		f := filepath.Join(dir, "r.txt")
		os.WriteFile(f, []byte(content), 0o600)
		sh := runShell(t, root, nil, "", "", "comp_requirements_hashed", f)
		err := RequirementsHashed(f)
		if (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) {
			t.Errorf("[%s] kabuk=%d %q go=%v", name, sh.code, sh.stderr, err)
		}
	}
	sh := runShell(t, root, nil, "", "", "comp_requirements_hashed", filepath.Join(dir, "yok"))
	if err := RequirementsHashed(filepath.Join(dir, "yok")); (sh.code == 0) != (err == nil) || strings.TrimSpace(sh.stderr) != err.Error() {
		t.Errorf("olmayan dosya: %q %v", sh.stderr, err)
	}
}

func TestPickPythonAndEnvIDMatchShell(t *testing.T) {
	root := goldRoot(t)
	fakePythons(t, map[string]string{"py-old": "3.9.1", "py-mid": "3.11.2", "py-new": "3.13.0", "py-broken": ""})
	// (boş aday listesi makinedeki python'lara bağlı: altın dosyaya girmez)
	for _, cands := range []string{"py-old py-mid py-new", "py-new py-mid", "py-old", "yok py-broken py-mid", "py-broken"} {
		for _, rng := range []string{">=3.12", ">=3.10,<3.12", ">=3.14"} {
			t.Setenv("SSHPOCKET_PYTHON_CANDIDATES", cands)
			sh := runShell(t, root, nil, "", "", "comp_pick_python", rng)
			got, ok := PickPython(rng)
			got = reTmp.ReplaceAllString(got, "<T>")
			if (sh.code == 0) != ok || (ok && sh.stdout != got) {
				t.Errorf("%q %q: kabuk=%d %q go=%v %q", cands, rng, sh.code, sh.stdout, ok, got)
			}
		}
	}
	dir := t.TempDir()
	req := filepath.Join(dir, "requirements.txt")
	os.WriteFile(req, []byte("a==1 \\\n  --hash=sha256:"+strings.Repeat("b", 64)+"\n"), 0o600)
	// env_id işletim sistemi/mimariye bağlıdır (macOS ve Linux'ta farklı özet): altın dosyaya girmez; kabuk motoru varsa canlı karşılaştırılır,
	// yoksa yalnız biçim denetlenir.
	for _, pv := range []string{"3.12.4", "3.13.0", ""} {
		got, err := EnvID(req, pv)
		if err != nil || !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(got) {
			t.Errorf("env_id %q: go=%q %v", pv, got, err)
			continue
		}
		if lroot, ok := liveRoot(); ok {
			sh := runShellLive(t, lroot, nil, "", "", "comp_env_id", req, pv)
			if strings.TrimSpace(sh.stdout) != got {
				t.Errorf("env_id %q: kabuk=%q go=%q", pv, sh.stdout, got)
			}
		}
	}
	// saplama: SSHPOCKET_PY_STUB_PATH ile verilen yol xcode-select -p başarısızsa atlanır (macOS dışı: asla stub değil)
	// (işletim sistemine bağlı: altın dosyaya girmez; yalnız kabuk motoru varsa karşılaştırılır)
	if lroot, ok := liveRoot(); ok {
		sh := runShellLive(t, lroot, []string{"SSHPOCKET_PY_STUB_PATH=/usr/bin/python3"}, "", "", "comp_python_is_stub", "/usr/bin/python3")
		if got := func() bool {
			t.Setenv("SSHPOCKET_PY_STUB_PATH", "/usr/bin/python3")
			return PythonIsStub("/usr/bin/python3")
		}(); (sh.code == 0) != got {
			t.Errorf("stub: kabuk=%d go=%v", sh.code, got)
		}
	}
	t.Setenv("SSHPOCKET_PY_STUB_PATH", "/usr/bin/python3")
	if PythonIsStub("/baska/python3") {
		t.Error("başka yol stub sayıldı")
	}
}

func venvSnapshot(cache string) map[string]string {
	out := map[string]string{}
	filepath.WalkDir(cache, func(p string, d os.DirEntry, err error) error {
		if err != nil || p == cache {
			return nil
		}
		rel, _ := filepath.Rel(cache, p)
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		b, _ := os.ReadFile(p)
		if strings.HasSuffix(rel, "bin/python") {
			b = []byte("COPY") // sahte python kopyası
		}
		out[rel] = string(b)
		return nil
	})
	return out
}

func TestVenvEnsureMatchesShell(t *testing.T) {
	root := goldRoot(t)
	h := "--hash=sha256:" + strings.Repeat("c", 64)
	for _, c := range []struct {
		name string
		env  map[string]string
		req  string
		pre  func(cache, envid string)
	}{
		{"başarı", nil, "a==1 \\\n  " + h + "\n", nil},
		{"venv oluşturulamadı", map[string]string{"FAKE_VENV_FAIL": "1"}, "a==1 \\\n  " + h + "\n", nil},
		{"pip başarısız", map[string]string{"FAKE_PIP_RC": "1"}, "a==1 \\\n  " + h + "\n", nil},
		{"çevrimdışı", map[string]string{"FAKE_PIP_RC": "1", "SSHPOCKET_OFFLINE": "1"}, "a==1 \\\n  " + h + "\n", nil},
		{"hash'siz requirements", nil, "a==1\n", nil},
		{"sağlıksız venv silinir", nil, "a==1 \\\n  " + h + "\n", func(cache, envid string) {
			d := filepath.Join(cache, "venv", "pk-"+envid)
			os.MkdirAll(d, 0o755)
			os.WriteFile(filepath.Join(d, "yari"), []byte("x"), 0o644)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, k := range []string{"FAKE_VENV_FAIL", "FAKE_PIP_RC", "SSHPOCKET_OFFLINE"} {
				t.Setenv(k, "")
			}
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			fakePythons(t, map[string]string{"python3.12": "3.12.4"})
			t.Setenv("SSHPOCKET_PYTHON_CANDIDATES", "python3.12")
			pkg := t.TempDir()
			os.WriteFile(filepath.Join(pkg, "requirements.txt"), []byte(c.req), 0o600)
			envid, _ := EnvID(filepath.Join(pkg, "requirements.txt"), "3.12.4")
			shCache, goCache := t.TempDir(), t.TempDir()
			if c.pre != nil {
				c.pre(shCache, envid)
				c.pre(goCache, envid)
			}
			type venvGold struct {
				Code           int
				Stdout, Stderr string
				Snap           map[string]string
			}
			// ortam kimliği (16 onaltılık) işletim sistemi/mimariye bağlıdır: yer tutucuya çevrilir
			reID := regexp.MustCompile(`[0-9a-f]{16}`)
			cn := func(s, cache string) string {
				return reID.ReplaceAllString(reTmp.ReplaceAllString(strings.ReplaceAll(s, cache, "<C>"), "<T>"), "<ID>")
			}
			snapN := func(m map[string]string, cache string) map[string]string {
				o := map[string]string{}
				for k, v := range m {
					o[cn(k, cache)] = cn(v, cache)
				}
				return o
			}
			t.Setenv("SSHPOCKET_CACHE_DIR", shCache)
			shg := goldDo(t, func() venvGold {
				x := runShellLive(t, root, nil, "", "", "comp_venv_ensure", "pk", pkg, ">=3.12")
				return venvGold{x.code, cn(x.stdout, shCache), cn(x.stderr, shCache), snapN(venvSnapshot(shCache), shCache)}
			})
			sh := shOut{shg.Code, shg.Stdout, shg.Stderr}
			t.Setenv("SSHPOCKET_CACHE_DIR", goCache)
			dir, err := VenvEnsure("pk", pkg, ">=3.12", "")
			if (sh.code == 0) != (err == nil) {
				t.Fatalf("kabuk=%d (%q) go=%v", sh.code, sh.stderr, err)
			}
			if err != nil {
				// kabuk, başarısız temizlikte olmayan klasör için `rm` hatası da basar (gürültü); asıl ileti son satırdır
				lines := strings.Split(strings.TrimSpace(sh.stderr), "\n")
				if last := lines[len(lines)-1]; last != cn(err.Error(), goCache) {
					t.Errorf("ileti kabuk=%q go=%q", last, err)
				}
			}
			if err == nil && sh.stdout != cn(dir, goCache) {
				t.Errorf("dizin kabuk=%q go=%q", sh.stdout, dir)
			}
			if a, b := shg.Snap, snapN(venvSnapshot(goCache), goCache); !reflect.DeepEqual(a, b) {
				t.Errorf("önbellek farkı\n kabuk=%v\n go   =%v", a, b)
			}
			if err == nil {
				// ikinci çağrı: hiçbir şey kurulmaz, aynı dizin; lookup de bulur
				d2, err2 := VenvEnsure("pk", pkg, ">=3.12", "")
				l, ok := VenvLookup("pk", pkg, ">=3.12")
				if err2 != nil || d2 != dir || !ok || l != dir {
					t.Errorf("ikinci çağrı/lookup: %v %q %v %q", err2, d2, ok, l)
				}
			}
		})
	}
}
