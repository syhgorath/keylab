package findings

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compareScores(t *testing.T, label, state, py string) {
	t.Helper()
	code, out := runPy(t, py, state, "linux-hardening", "web1")
	res, err := Scores(state, "linux-hardening", "web1")
	gcode := 0
	if err != nil {
		gcode = 2
	}
	got := FormatScores(res)
	if code != gcode || out != got {
		t.Errorf("%s: python=%d go=%d\npython: %q\ngo:     %q", label, code, gcode, out, got)
	}
}

func TestScoresFixturesMatchPython(t *testing.T) {
	py := pyScript(t, "component_scores.py")
	sums := map[string]string{
		"lynis eski":    `{"hardening_index": 83, "date": "2026-10-25T08:44:34Z"}`,
		"ortak":         `{"tool":"Tiger","date":"2026-10-26T08:44:34Z","score":71}`,
		"ondalık":       `{"tool":"X","score":83.04,"date":"2026-10-27T00:00:00Z"}`,
		"ondalık .5":    `{"tool":"X","score":2.25,"date":"2026-10-28T00:00:00Z"}`,
		"tam float":     `{"tool":"X","score":83.0,"date":"2026-10-29T00:00:00Z"}`,
		"aralık dışı":   `{"tool":"X","score":1001}`,
		"negatif":       `{"tool":"X","score":-1}`,
		"bool":          `{"tool":"X","score":true}`,
		"dize puan":     `{"tool":"X","score":"83"}`,
		"NaN":           `{"tool":"X","score":NaN,"hardening_index":55,"date":"2026-10-30T00:00:00Z"}`,
		"Infinity":      `{"tool":"X","score":Infinity}`,
		"kötü araç":     `{"tool":"bu çok uzun bir araç adı değil mi evet","score":50,"date":"2026-10-31T00:00:00Z"}`,
		"araç sayı":     `{"tool":7,"score":50,"date":"2026-11-01T00:00:00Z"}`,
		"araç yok":      `{"score":50,"date":"2026-11-02T00:00:00Z"}`,
		"araç satır":    `{"tool":"abc\n","score":50,"date":"2026-11-03T00:00:00Z"}`,
		"kötü tarih":    `{"tool":"X","score":50,"date":"2026-13-45T00:00:00Z"}`,
		"tek haneli":    `{"tool":"X","score":50,"date":"2026-1-5T1:2:3Z"}`,
		"şubat 31":      `{"tool":"X","score":50,"date":"2026-02-31T00:00:00Z"}`,
		"saniye 60":     `{"tool":"X","score":50,"date":"2026-02-03T00:00:60Z"}`,
		"tarih sayı":    `{"tool":"X","score":50,"date":5}`,
		"dizi":          `[1,2,3]`,
		"bozuk json":    `{"score": `,
		"yinelenen":     `{"tool":"X","score":10,"score":20,"date":"2026-11-04T00:00:00Z"}`,
		"büyük sayı":    `{"tool":"X","score":1e400}`,
		"1e2":           `{"tool":"X","score":1e2,"date":"2026-11-05T00:00:00Z"}`,
		"yalnız legacy": `{"tool":"Y","hardening_index":12,"date":"2026-11-06T00:00:00Z"}`,
		"dizede NaN":    `{"tool":"NaN","score":9,"date":"2026-11-07T00:00:00Z"}`,
	}
	i := 0
	for _, p := range ordered(sums) {
		name, c := p.K, p.V
		state := t.TempDir()
		put(t, state, "linux-hardening/web1/out/web1/x/summary.json", c, 1_700_000_000+int64(i))
		compareScores(t, name, state, py)
		i++
	}
	// araç başına yalnız son 2, farklı araçlar, mtime yedeği
	state := t.TempDir()
	for k := 0; k < 5; k++ {
		put(t, state, fmt.Sprintf("linux-hardening/web1/out/web1/d%d/summary.json", k), fmt.Sprintf(`{"tool":"Lynis","score":%d,"date":"2026-10-%02dT00:00:00Z"}`, 60+k, 10+k), 1_600_000_000)
		put(t, state, fmt.Sprintf("linux-hardening/web1/out/web1/e%d/summary.json", k), fmt.Sprintf(`{"tool":"Aaa","score":%d}`, 40+k), 1_650_000_000+int64(k)*1000)
	}
	compareScores(t, "çok kayıt", state, py)
	compareScores(t, "yok", t.TempDir(), py)
}

func TestScoresFuzzMatchPython(t *testing.T) {
	py := pyScript(t, "component_scores.py")
	rng := rand.New(rand.NewSource(11))
	nums := []string{"0", "1000", "1000.1", "83", "83.5", "83.04", "2.25", "0.05", "999.95", "-0.0", "7e1", "null", "true", `"5"`, "[1]", "NaN", "Infinity", "-Infinity", "1e999"}
	tools := []string{`"Lynis"`, `"A b"`, `"x/y"`, `""`, `"` + strings.Repeat("a", 21) + `"`, "null", "5", `"ok-1.2"`}
	dates := []string{`"2026-10-25T08:44:34Z"`, `"2026-1-1T0:0:0Z"`, `"bad"`, "null", `"2026-10-25T08:44:34"`, `"0000-01-01T00:00:00Z"`, `"2038-01-19T03:14:08Z"`}
	pick := func(l []string) string { return l[rng.Intn(len(l))] }
	for it := 0; it < 200; it++ {
		state := t.TempDir()
		for k := 0; k < 1+rng.Intn(5); k++ {
			var f []string
			if rng.Intn(5) > 0 {
				f = append(f, `"score":`+pick(nums))
			}
			if rng.Intn(3) == 0 {
				f = append(f, `"hardening_index":`+pick(nums))
			}
			if rng.Intn(4) > 0 {
				f = append(f, `"tool":`+pick(tools))
			}
			if rng.Intn(4) > 0 {
				f = append(f, `"date":`+pick(dates))
			}
			put(t, state, fmt.Sprintf("linux-hardening/web1/o/%d/summary.json", k), "{"+strings.Join(f, ",")+"}", 1_700_000_000+int64(rng.Intn(100000)))
		}
		compareScores(t, fmt.Sprintf("fuzz %d", it), state, py)
	}
}

func TestScoresBadArgs(t *testing.T) {
	if _, err := Scores(t.TempDir(), "a/b", "s"); err != ErrBadArgs {
		t.Error("bozuk ad kabul edildi")
	}
	_ = os.Stdout
	_ = filepath.Join
}
