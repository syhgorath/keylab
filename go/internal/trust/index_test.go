package trust

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

const testNow = 1793000000 // 2026-10-26

func baseIndex() map[string]any {
	return map[string]any{
		"schema": 1, "serial": 2, "issued_at": "2026-10-25T00:00:00Z", "valid_until": "2026-12-01T00:00:00Z",
		"packages": map[string]any{
			"utility": map[string]any{"repo": "o/utility", "version": "0.1.2", "asset": "utility-0.1.2.tar.gz", "sha256": sha1, "requires_sshpocket": "3.7.0", "platforms": []any{"ubuntu-26.04", "debian-13"}},
			"kurulum": map[string]any{"repo": "o/kurulum", "version": "0.1.0", "asset": "kurulum-0.1.0.tar.gz", "sha256": sha1, "yanked": true},
		},
	}
}

func runIndexPy(t *testing.T, root string, data []byte, now int64) (int, []string, string) {
	t.Helper()
	f, _ := os.CreateTemp("", "idx-*.json")
	defer os.Remove(f.Name())
	f.Write(data)
	f.Close()
	cmd := exec.Command("python3", "-I", filepath.Join(root, "referans/kabuk/sistem/Helpers/component_index.py"), f.Name(), fmt.Sprint(now))
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	var lines []string
	if s := strings.TrimRight(so.String(), "\n"); s != "" {
		lines = strings.Split(s, "\n")
	}
	return code, lines, strings.TrimSpace(se.String())
}

func cmpIndex(t *testing.T, label, root string, data []byte, now int64) {
	t.Helper()
	pc, pl, pe := runIndexPy(t, root, data, now)
	idx, perr := ParseIndex(data, now)
	gc, gl, ge := 0, []string(nil), ""
	if perr != nil {
		gc, ge = perr.Code, perr.Msg
	} else {
		gl = idx.Lines()
	}
	if strings.Contains(string(data), "NaN") || strings.Contains(string(data), "Infinity") {
		if pc != gc {
			t.Errorf("%s: kod py=%d go=%d", label, pc, gc)
		}
		return
	}
	if pc != gc || !reflect.DeepEqual(pl, gl) || pe != ge {
		t.Errorf("%s: fark\n  py: kod=%d out=%q err=%q\n  go: kod=%d out=%q err=%q\n  veri=%s", label, pc, pl, pe, gc, gl, ge, data)
	}
}

func TestParseIndexBasics(t *testing.T) {
	root := repoRoot(t)
	b, _ := json.Marshal(baseIndex())
	cmpIndex(t, "geçerli", root, b, testNow)
	// zaman kapıları
	cmpIndex(t, "süresi dolmuş", root, b, 1796000000)
	cmpIndex(t, "gelecekte", root, b, 1700000000)
	cmpIndex(t, "sınır: issued+skew", root, b, 1792000000-86400)
	cmpIndex(t, "tam valid_until", root, b, 1796083200) // 2026-12-01T00:00:00Z
	cmpIndex(t, "valid_until+1", root, b, 1796083201)
	// ham metin durumları
	for name, raw := range map[string]string{
		"boş":                `{}`,
		"dizi":               `[]`,
		"yinelenen":          `{"schema":1,"schema":1}`,
		"iç yinelenen":       `{"schema":1,"serial":2,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2026-12-01T00:00:00Z","packages":{"a":{"repo":"o/a","repo":"o/b"}}}`,
		"NaN":                `{"schema":NaN}`,
		"bozuk":              `{`,
		"artık":              `{"schema":1} x`,
		"schema float":       `{"schema":1.0}`,
		"schema 2":           `{"schema":2}`,
		"schema bool":        `{"schema":true}`,
		"serial 0":           `{"schema":1,"serial":0}`,
		"tarih Şubat 30":     `{"schema":1,"serial":1,"issued_at":"2026-02-30T00:00:00Z","valid_until":"2026-03-01T00:00:00Z","packages":{}}`,
		"saniye 60":          `{"schema":1,"serial":1,"issued_at":"2026-10-25T00:00:60Z","valid_until":"2026-12-01T00:00:00Z","packages":{}}`,
		"saniye 62":          `{"schema":1,"serial":1,"issued_at":"2026-10-25T00:00:62Z","valid_until":"2026-12-01T00:00:00Z","packages":{}}`,
		"saat 24":            `{"schema":1,"serial":1,"issued_at":"2026-10-25T24:00:00Z","valid_until":"2026-12-01T00:00:00Z","packages":{}}`,
		"ay 13":              `{"schema":1,"serial":1,"issued_at":"2026-13-25T00:00:00Z","valid_until":"2026-12-01T00:00:00Z","packages":{}}`,
		"artık yıl Şubat 29": `{"schema":1,"serial":1,"issued_at":"2028-02-29T00:00:00Z","valid_until":"2028-03-10T00:00:00Z","packages":{}}`,
		"91 gün":             `{"schema":1,"serial":1,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2027-01-24T00:00:01Z","packages":{}}`,
		"90 gün":             `{"schema":1,"serial":1,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2027-01-23T00:00:00Z","packages":{}}`,
		"valid<issued":       `{"schema":1,"serial":1,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2026-10-24T00:00:00Z","packages":{}}`,
		"paket yok":          `{"schema":1,"serial":1,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2026-12-01T00:00:00Z"}`,
		"ek alan":            `{"schema":1,"serial":1,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2026-12-01T00:00:00Z","packages":{},"ek":[1,2]}`,
		"unicode paket":      `{"schema":1,"serial":1,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2026-12-01T00:00:00Z","packages":{"é":{}}}`,
		"büyük sayı":         `{"schema":1,"serial":99999999999999999999,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2026-12-01T00:00:00Z","packages":{}}`,
	} {
		if name == "büyük sayı" {
			continue // Python sınırsız tamsayıyı kabul eder; Go int64 sınırı (bilinçli fark)
		}
		cmpIndex(t, name, root, []byte(raw), testNow)
	}
	if _, perr := ParseIndex([]byte(`{"schema":1,"serial":99999999999999999999,"issued_at":"2026-10-25T00:00:00Z","valid_until":"2026-12-01T00:00:00Z","packages":{}}`), testNow); perr == nil {
		t.Error("int64 dışı seri kabul edildi")
	}
	if _, perr := ParseIndex(bytes.Repeat([]byte(" "), maxIndexBytes+1), testNow); perr == nil {
		t.Error("büyük indeks kabul edildi")
	}
}

func TestParseIndexMutationsMatchPython(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	root := repoRoot(t)
	junk := []any{nil, true, false, 0, 1, -1, 2, 1.5, "", "x", "UPPER", "o/x", "0.1.2", "1.2", "../x", "a..b/c", strings.Repeat("a", 70), sha1, "2026-10-25T00:00:00Z", "ubuntu-26.04",
		[]any{}, []any{"ubuntu"}, []any{1}, []any{"A"}, map[string]any{}, map[string]any{"a": 1}}
	base := baseIndex()
	var paths [][]string
	var walk func(v any, p []string)
	walk = func(v any, p []string) {
		paths = append(paths, append([]string(nil), p...))
		if m, ok := v.(map[string]any); ok {
			keys := make([]string, 0)
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(m[k], append(p, k))
			}
		}
	}
	walk(base, nil)
	apply := func(del bool, p []string, val any) []byte {
		raw, _ := json.Marshal(base)
		var m map[string]any
		json.Unmarshal(raw, &m)
		cur := m
		for _, k := range p[:len(p)-1] {
			cur = cur[k].(map[string]any)
		}
		if del {
			delete(cur, p[len(p)-1])
		} else {
			cur[p[len(p)-1]] = val
		}
		out, _ := json.Marshal(m)
		return out
	}
	type job struct {
		label string
		data  []byte
	}
	var jobs []job
	for _, p := range paths {
		if len(p) == 0 {
			continue
		}
		jobs = append(jobs, job{fmt.Sprint(p, " sil"), apply(true, p, nil)})
		for i, j := range junk {
			jobs = append(jobs, job{fmt.Sprintf("%v #%d", p, i), apply(false, p, j)})
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
				cmpIndex(t, j.label, root, j.data, testNow)
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
}
