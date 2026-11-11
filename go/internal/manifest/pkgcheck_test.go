package manifest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// component_validate.py ile karşılaştırmalı testler. Python'un os.walk sırası belirsiz olduğundan
// güvenlik ihlali içeren denemelerde tek ihlal kullanılır.

func runValidatePy(t *testing.T, root string, args ...string) pyOut {
	t.Helper()
	cmd := exec.Command("python3", append([]string{"-I", filepath.Join(root, "referans/kabuk/sistem/Helpers/component_validate.py")}, args...)...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return pyOut{code, strings.Split(strings.TrimRight(so.String(), "\n"), "\n"), splitLines(se.String())}
}

func cmpValidate(t *testing.T, label, root, arg string, strict, unsigned bool) {
	t.Helper()
	args := []string{arg}
	if strict {
		args = append(args, "--strict")
	}
	if unsigned {
		args = append(args, "--unsigned")
	}
	py := runValidatePy(t, root, args...)
	lines, code := ValidatePackage(arg, strict, unsigned)
	// Python çıktısı "...\n\nSonuç" biçimindedir; yalnızca sondaki boş satırı kırpar
	if code != py.code || !reflect.DeepEqual(lines, py.stdout) {
		t.Errorf("%s: fark\n--- python (kod %d)\n%s\n--- go (kod %d)\n%s", label, py.code, strings.Join(py.stdout, "\n"), code, strings.Join(lines, "\n"))
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if out, err := exec.Command("cp", "-R", src, dst).CombinedOutput(); err != nil {
		t.Fatalf("cp: %v %s", err, out)
	}
}

func TestValidatePackageRealDirs(t *testing.T) {
	root := repoRoot(t)
	for _, p := range realManifests(t, root) {
		dir := filepath.Dir(p)
		rel, _ := filepath.Rel(root, dir)
		cmpValidate(t, rel, root, dir, false, false)
		cmpValidate(t, rel+" --strict", root, dir, true, false)
	}
}

func TestValidatePackageMutations(t *testing.T) {
	root := repoRoot(t)
	type mutation struct {
		name string
		fn   func(t *testing.T, d string)
	}
	rm := func(rel string) func(*testing.T, string) {
		return func(t *testing.T, d string) {
			if err := os.RemoveAll(filepath.Join(d, rel)); err != nil {
				t.Fatal(err)
			}
		}
	}
	write := func(rel, content string) func(*testing.T, string) {
		return func(t *testing.T, d string) {
			p := filepath.Join(d, rel)
			os.MkdirAll(filepath.Dir(p), 0o755)
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	edit := func(rel string, from, to string) func(*testing.T, string) {
		return func(t *testing.T, d string) {
			p := filepath.Join(d, rel)
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), from) {
				t.Skipf("%s içinde %q yok", rel, from)
			}
			os.WriteFile(p, []byte(strings.Replace(string(b), from, to, 1)), 0o644)
		}
	}
	muts := []mutation{
		{"VERSION yok", rm("VERSION")},
		{"VERSION farkli", write("VERSION", "9.9.9\n")},
		{"requirements yok", rm("requirements.txt")},
		{"requirements bos", write("requirements.txt", "# bos\n")},
		{"requirements hash yok", write("requirements.txt", "ansible-core==2.17.0\n")},
		{"requirements surum yok", write("requirements.txt", "ansible-core --hash=sha256:aa\n")},
		{"requirements git", write("requirements.txt", "foo==1.0 --hash=sha256:aa git+https://x/y\n")},
		{"requirements -e", write("requirements.txt", "-e .\nfoo==1.0 \\\n  --hash=sha256:aa\n")},
		{"requirements devam satiri", write("requirements.txt", "foo==1.0 \\\n  --hash=sha256:aa \\\n  --hash=sha256:bb\n")},
		{"ansible.cfg yok", rm("ansible.cfg")},
		{"changelog baslik yok", write("CHANGELOG.md", "# degisiklikler\n")},
		{"changelog yok", rm("CHANGELOG.md")},
		{"gizli .env", write(".env", "A=1\n")},
		{"gizli pem", write("files/x.pem", "x\n")},
		{"gizli id_rsa", write("files/id_rsa_test", "x\n")},
		{"ozel anahtar blogu", write("README.md", "-----BEGIN OPENSSH PRIVATE KEY-----\nx\n")},
		{"belge geri alma yok", func(t *testing.T, d string) {
			write("README.md", "# paket\nhicbir sey\n")(t, d)
			rm("docs")(t, d)
			rm("CHANGELOG.md")(t, d)
			rm("LICENSE")(t, d)
			for _, f := range []string{"README.en.md", "CONTRIBUTING.md", "HOWTO.md", "SECURITY.md"} {
				rm(f)(t, d)
			}
		}},
		{"dev klasoru", write(".venv/lib/x.py", "x=1\n")},
		{"dev klasoru + gizli icinde", func(t *testing.T, d string) { write(".venv/.env", "A=1\n")(t, d) }},
		{"playbook yok", func(t *testing.T, d string) { rm("playbooks")(t, d) }},
		{"sembolik dosya", func(t *testing.T, d string) {
			if err := os.Symlink("/etc/hosts", filepath.Join(d, "bag")); err != nil {
				t.Fatal(err)
			}
		}},
		{"sembolik dizin", func(t *testing.T, d string) {
			if err := os.Symlink("/tmp", filepath.Join(d, "bagdizin")); err != nil {
				t.Fatal(err)
			}
		}},
		{"bos component.json", write("component.json", "")},
		{"bozuk component.json", write("component.json", "{")},
		{"component.json yok", rm("component.json")},
		{"component.yml surum farki", write("component.yml", "version: 0.0.1\n")},
		{"component.json id", edit("component.json", `"id": `, `"idx": `)},
		{"component.json surum", edit("component.json", `"version": "`, `"version": "7.`)},
	}
	for _, src := range []string{"components/utility/0.1.0", "components/kurulum/0.1.0", "components/linux-hardening/0.13.0", "components/linux-hardening/0.3.0"} {
		if _, err := os.Stat(filepath.Join(root, src)); err != nil {
			t.Logf("atlandı (paket yok; components/ eşitlenmemiş): %s", src)
			continue
		}
		for _, m := range muts {
			t.Run(src+"/"+m.name, func(t *testing.T) {
				d := filepath.Join(t.TempDir(), "pkg")
				copyTree(t, filepath.Join(root, src), d)
				m.fn(t, d)
				cmpValidate(t, src+" "+m.name, root, d, false, false)
				cmpValidate(t, src+" "+m.name+" --strict", root, d, true, false)
			})
		}
	}
}

func makeArchive(t *testing.T, path string, files map[string]string, extra func(tw *tar.Writer)) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sortStrings(names)
	for _, n := range names {
		tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(files[n])), Typeflag: tar.TypeReg})
		tw.Write([]byte(files[n]))
	}
	if extra != nil {
		extra(tw)
	}
	tw.Close()
	gz.Close()
	os.WriteFile(path, buf.Bytes(), 0o644)
}

func sortStrings(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func TestValidatePackageArchives(t *testing.T) {
	root := repoRoot(t)
	// gerçek paket → <id>-<sürüm>.tar.gz
	for _, src := range []string{"components/utility/0.1.0", "components/kurulum/0.1.0"} {
		if _, err := os.Stat(filepath.Join(root, src)); err != nil {
			t.Logf("atlandı (paket yok; components/ eşitlenmemiş): %s", src)
			continue
		}
		parts := strings.Split(src, "/")
		id, ver := parts[1], parts[2]
		tmp := t.TempDir()
		pk := filepath.Join(tmp, id+"-"+ver)
		copyTree(t, filepath.Join(root, src), pk)
		arc := filepath.Join(tmp, id+"-"+ver+".tar.gz")
		if out, err := exec.Command("tar", "czf", arc, "-C", tmp, id+"-"+ver).CombinedOutput(); err != nil {
			t.Fatalf("tar: %v %s", err, out)
		}
		cmpValidate(t, id+" arşiv imzasız", root, arc, false, false)
		cmpValidate(t, id+" arşiv --unsigned", root, arc, false, true)
		os.WriteFile(arc+".sig", []byte("sig"), 0o644)
		cmpValidate(t, id+" arşiv .sig", root, arc, false, false)
	}
	man := `{"id":"abc","version":"1.0.0","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"ubuntu-26.04","status":"supported"}],"actions":{"durum":{"cmd":"ansible-playbook playbooks/x.yml","effects":[]}}}`
	tmp := t.TempDir()
	cases := map[string]struct {
		name  string
		files map[string]string
		extra func(*tar.Writer)
		exact bool // iletinin tamamı Python ile aynı olmalı (özel durumlar)
	}{
		"kök yanlış":      {"abc-1.0.0.tar.gz", map[string]string{"baska/component.json": man}, nil, true},
		"iki kök":         {"abc-1.0.0.tar.gz", map[string]string{"abc-1.0.0/component.json": man, "x/y": "1"}, nil, true},
		"boş ad":          {"kotu.tar.gz", map[string]string{"abc-1.0.0/component.json": man}, nil, true},
		"geçerli minimal": {"abc-1.0.0.tar.gz", map[string]string{"abc-1.0.0/component.json": man, "abc-1.0.0/VERSION": "1.0.0\n", "abc-1.0.0/requirements.txt": "a==1 --hash=sha256:aa\n", "abc-1.0.0/playbooks/x.yml": "- hosts: all\n"}, nil, true},
		"geçişli yol":     {"abc-1.0.0.tar.gz", map[string]string{"abc-1.0.0/../x": "1"}, nil, true},
		"mutlak yol":      {"abc-1.0.0.tar.gz", map[string]string{"/etc/x": "1"}, nil, true},
		"sembolik üye": {"abc-1.0.0.tar.gz", map[string]string{"abc-1.0.0/component.json": man}, func(tw *tar.Writer) {
			tw.WriteHeader(&tar.Header{Name: "abc-1.0.0/bag", Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink, Mode: 0o777})
		}, true},
	}
	for name, c := range cases {
		arc := filepath.Join(tmp, c.name)
		makeArchive(t, arc, c.files, c.extra)
		cmpValidate(t, "arşiv "+name, root, arc, false, true)
	}
	// olmayan yol
	cmpValidate(t, "olmayan", root, filepath.Join(tmp, "yok-1.0.0.tar.gz"), false, false)
}

func TestSplitExtMatchesPython(t *testing.T) {
	root := repoRoot(t)
	_ = root
	for name, want := range map[string]string{".env": "", "a.yml": ".yml", "a.b.c": ".c", "README": "", "..x": "", ".a.b": ".b", "x.": ".", "dir/f.SH": ".SH"} {
		if got := splitExt(name); got != want {
			t.Errorf("splitExt(%q)=%q want %q", name, got, want)
		}
	}
	_ = json.Marshal
	_ = fmt.Sprint
}
