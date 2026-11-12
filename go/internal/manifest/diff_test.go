package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"keylab/internal/shellgold"
)

// Kabuk motorundaki Python doğrulayıcı referanstır: aynı girdide çıkış kodu, stdout ve stderr aynı olmalı.
// Bilinçli fark: Go, Python'un `$` ile kabul ettiği sondaki satır sonunu reddeder (TestTrailingNewlineStricter).

// goldRoot: altın dosyalı testler için kök; oynatmada kabuk/python gerekmez, yalnız kayıt modunda gerekir.
func goldRoot(t *testing.T) string {
	t.Helper()
	if os.Getenv("KEYLAB_KAYIT_KABUK") == "1" {
		return repoRoot(t)
	}
	root, _ := filepath.Abs("../../..")
	return root
}

var (
	goldOnce  sync.Once
	goldStore *shellgold.Store
	reTmp     = regexp.MustCompile(`(?:/private)?(?:/var/folders/[^/\s]+/[^/\s]+/T|/tmp)/[^\s"']+`) // geçici yollar altın dosyaya girmesin
)

func goldDo[T any](t *testing.T, f func() T) T {
	t.Helper()
	goldOnce.Do(func() { goldStore = shellgold.Open(t, "shell") })
	return shellgold.Do(t, goldStore, goldStore.Next(t), f)
}

// pyGold: python/kabuk çalışmasının kaydedilen biçimi (anahtar: test adı + sıra).
type pyGold struct {
	Code           int
	Stdout, Stderr []string
}

func normSS(ss []string, extra string) []string {
	if len(ss) == 0 {
		return nil
	}
	out := make([]string, len(ss))
	for i, l := range ss {
		if extra != "" {
			l = strings.ReplaceAll(l, extra, "<A>")
		}
		out[i] = reTmp.ReplaceAllString(l, "<T>")
	}
	return out
}

// ordered: harita gezintisini deterministik yapar (altın dosya anahtarı sıraya bağlı).
type kv[V any] struct {
	K string
	V V
}

func ordered[V any](m map[string]V) []kv[V] {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]kv[V], 0, len(keys))
	for _, k := range keys {
		out = append(out, kv[V]{k, m[k]})
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "referans/kabuk/sistem/Helpers/component_manifest.py")); err != nil {
		t.Skip("kabuk motoru bulunamadı: " + root)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 yok")
	}
	return root
}

type pyOut struct {
	code   int
	stdout []string
	stderr []string
}

func runPy(t *testing.T, root string, data []byte, args ...string) pyOut {
	t.Helper()
	f, err := os.CreateTemp("", "cm-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Write(data)
	f.Close()
	cmd := exec.Command("python3", append([]string{"-I", filepath.Join(root, "referans/kabuk/sistem/Helpers/component_manifest.py"), f.Name()}, args...)...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err = cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return pyOut{code, splitLines(so.String()), splitLines(se.String())}
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func compare(t *testing.T, label string, root string, data []byte) {
	t.Helper()
	py := runPy(t, root, data)
	lines, errs, code := Validate(data, "", "")
	if code != py.code || !reflect.DeepEqual(lines, py.stdout) || !reflect.DeepEqual(errs, py.stderr) {
		t.Errorf("%s: fark\n  python: kod=%d out=%q err=%q\n  go:     kod=%d out=%q err=%q", label, py.code, py.stdout, py.stderr, code, lines, errs)
	}
}

func realManifests(t *testing.T, root string) []string {
	m, _ := filepath.Glob(filepath.Join(root, "components/*/*/component.json"))
	sort.Strings(m)
	if len(m) == 0 {
		t.Skip("örnek bildirim yok")
	}
	return m
}

func TestRealManifestsMatchPython(t *testing.T) {
	root := repoRoot(t)
	for _, p := range realManifests(t, root) {
		data, _ := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		compare(t, rel, root, data)
		// beklenen kimlik/sürüm eşleşmesi
		lines, _, code := Validate(data, "yok", "")
		py := runPy(t, root, data, "yok")
		if code != py.code || !reflect.DeepEqual(lines, py.stdout) {
			t.Errorf("%s: id uyuşmazlığı farkı", rel)
		}
	}
}

// --- mutasyon: her yolda değeri bozup Python ile karşılaştır ---

type mut struct {
	path []any // string anahtar ya da int dizin
	del  bool
	val  any
}

func walk(v any, path []any, depth int, out *[][]any) {
	*out = append(*out, append([]any(nil), path...))
	if depth >= 6 {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walk(t[k], append(path, k), depth+1, out)
		}
	case []any:
		for i, e := range t {
			if i >= 3 {
				break
			}
			walk(e, append(path, i), depth+1, out)
		}
	}
}

func apply(root any, m mut) any {
	var rec func(v any, p []any) any
	rec = func(v any, p []any) any {
		if len(p) == 0 {
			return m.val
		}
		switch t := v.(type) {
		case map[string]any:
			k := p[0].(string)
			if len(p) == 1 && m.del {
				delete(t, k)
				return t
			}
			t[k] = rec(t[k], p[1:])
			return t
		case []any:
			i := p[0].(int)
			if len(p) == 1 && m.del {
				return append(t[:i:i], t[i+1:]...)
			}
			t[i] = rec(t[i], p[1:])
			return t
		}
		return v
	}
	return rec(root, m.path)
}

func deepCopy(v any) any {
	b, _ := json.Marshal(v)
	var o any
	json.Unmarshal(b, &o)
	return o
}

var junk = []any{nil, true, false, 0, 1, -1, 1000, 1.5, "", "x", "UPPER", "a b", "yes-no", "destructive", "path", "bool", "enum", "list[string]", "ansible", "script",
	[]any{}, []any{"x"}, []any{"x", "x"}, []any{1}, map[string]any{}, map[string]any{"a": "b"}, strings.Repeat("a", 300)}

func TestMutationsMatchPython(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	root := repoRoot(t)
	all := realManifests(t, root)
	pick := map[string]bool{"utility/0.1.0": true, "kurulum/0.1.0": true, "linux-hardening/0.13.0": true, "linux-hardening/0.3.0": true, "linux-hardening/0.8.0": true}
	type job struct {
		label string
		data  []byte
	}
	var jobs []job
	for _, p := range all {
		rel, _ := filepath.Rel(filepath.Join(root, "components"), filepath.Dir(p))
		if !pick[rel] {
			continue
		}
		data, _ := os.ReadFile(p)
		var base any
		if err := json.Unmarshal(data, &base); err != nil {
			t.Fatal(err)
		}
		var paths [][]any
		walk(base, nil, 0, &paths)
		for _, path := range paths {
			if len(path) == 0 {
				continue
			}
			ms := []mut{{path: path, del: true}}
			for _, j := range junk {
				ms = append(ms, mut{path: path, val: j})
			}
			for mi, m := range ms {
				b, _ := json.Marshal(apply(deepCopy(base), m))
				jobs = append(jobs, job{fmt.Sprintf("%s %v #%d", rel, path, mi), b})
			}
		}
	}
	t.Logf("%d mutasyon", len(jobs))
	var wg sync.WaitGroup
	ch := make(chan job)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				compare(t, j.label, root, j.data)
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
}

func TestTrailingNewlineStricter(t *testing.T) {
	root := repoRoot(t)
	data, _ := os.ReadFile(filepath.Join(root, "components/utility/0.1.0/component.json"))
	mod := bytes.Replace(data, []byte(`"id": "utility"`), []byte(`"id": "utility\n"`), 1)
	if bytes.Equal(mod, data) {
		t.Skip("id satırı biçimi farklı")
	}
	_, _, code := Validate(mod, "", "")
	if code == 0 {
		t.Fatal("sondaki satır sonu kabul edildi")
	}
}
