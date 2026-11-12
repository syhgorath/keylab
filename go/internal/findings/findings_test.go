package findings

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"keylab/internal/shellgold"
)

// pyScript: altın dosyalı testler için betik yolu; oynatmada python ve betik gerekmez, yalnız kayıt modunda gerekir.
func pyScript(t *testing.T, name string) string {
	t.Helper()
	if os.Getenv("KEYLAB_KAYIT_KABUK") != "1" {
		p, _ := filepath.Abs("../../../referans/kabuk/sistem/Helpers/" + name)
		return p
	}
	return pyScriptLive(t, name)
}

var (
	goldOnce  sync.Once
	goldStore *shellgold.Store
	reTmp     = regexp.MustCompile(`(?:/private)?(?:/var/folders/[^/\s]+/[^/\s]+/T|/tmp)/[^\s"']+`) // geçici yollar altın dosyaya girmesin
)

func goldDo[T any](t *testing.T, f func() T) T {
	t.Helper()
	goldOnce.Do(func() { goldStore = shellgold.Open(t, "python") })
	return shellgold.Do(t, goldStore, goldStore.Next(t), f)
}

type pyGold struct {
	Code int
	Out  string
}

// runPy: python referansının altın dosyalı hali (anahtar: test adı + sıra). state yolu çıktıda geçerse yer tutucuya çevrilir.
func runPy(t *testing.T, script string, args ...string) (int, string) {
	t.Helper()
	g := goldDo(t, func() pyGold {
		c, o := runPyLive(t, script, args...)
		return pyGold{c, reTmp.ReplaceAllString(o, "<T>")}
	})
	return g.Code, g.Out
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

func pyScriptLive(t *testing.T, name string) string {
	t.Helper()
	p, _ := filepath.Abs("../../../referans/kabuk/sistem/Helpers/" + name)
	if _, err := os.Stat(p); err != nil {
		t.Skip("Python betiği yok")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 yok")
	}
	return p
}

func runPyLive(t *testing.T, script string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command("python3", append([]string{"-I", script}, args...)...)
	var so bytes.Buffer
	cmd.Stdout = &so
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return code, so.String()
}

const hdr = "kimlik\trisk\ttur\taciklama\tayrinti\toneri\talan\tdeger\tkapanir_varsayilan\tplatform\n"

func row(id, risk, tur, text, alan, deger, kapanir, platform string) string {
	return strings.Join([]string{id, risk, tur, text, "ayr", "öneri", alan, deger, kapanir, platform}, "\t") + "\n"
}

func put(t *testing.T, state, rel, content string, mtime int64) string {
	p := filepath.Join(state, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(content), 0o644)
	os.Chtimes(p, time.Unix(mtime, 0), time.Unix(mtime, 0))
	return p
}

func compareFindings(t *testing.T, label, state, py string) {
	t.Helper()
	code, out := runPy(t, py, state, "linux-hardening", "web1", "bulgular.tsv")
	res, err := Load(state, "linux-hardening", "web1", "bulgular.tsv")
	gcode := 0
	if err == ErrBadHeader {
		gcode = 3
	} else if err != nil {
		gcode = 2
	}
	got := reTmp.ReplaceAllString(res.Format(), "<T>")
	if code != gcode || out != got {
		t.Errorf("%s: python=%d go=%d\npython: %q\ngo:     %q", label, code, gcode, trunc(out), trunc(got))
	}
}

func trunc(s string) string {
	if len(s) > 600 {
		return s[:600] + "…"
	}
	return s
}

func TestFindingsFixturesMatchPython(t *testing.T) {
	py := pyScript(t, "component_findings.py")
	good := row("SSH-7408", "orta", "uyari", "Zayıf", "ssh", "PermitRootLogin=no", "evet", "ubuntu")
	cases := map[string]string{
		"basit":             hdr + good,
		"risk sırası":       hdr + row("a1", "bilgi", "oneri", "x", "", "", "", "hepsi") + row("a2", "yuksek", "uyari", "y", "", "", "", "hepsi") + row("a3", "orta", "uyari", "z", "", "", "", "hepsi") + row("a4", "yuksek", "oneri", "w", "", "", "", "ubuntu"),
		"bozuk satırlar":    hdr + good + "az\tsutun\n" + row("-bad", "orta", "uyari", "x", "", "", "", "ubuntu") + row("ok2", "kritik", "uyari", "x", "", "", "", "ubuntu") + row("ok3", "orta", "x", "x", "", "", "", "ubuntu") + row("ok4", "orta", "uyari", "x", "", "", "", "windows") + row("ok5", "orta", "uyari", "x", "Alan", "", "", "ubuntu") + row("ok6", "orta", "uyari", "x", "", "x=1\t2", "", "ubuntu") + row("ok7", "orta", "uyari", "x", "", "", "belki", "ubuntu"),
		"denetim karakteri": hdr + row("c1", "orta", "uyari", "a\x01b\x7fc  \x1bd", "", "", "", "ubuntu"),
		"uzun alan":         hdr + row("l1", "orta", "uyari", strings.Repeat("ç", 400), "", "", "", "ubuntu"),
		"bozuk utf8":        hdr + row("u1", "orta", "uyari", "a\xe2\x82b\xffc\xc3", "", "", "", "ubuntu"),
		"bitişik bozuk":     hdr + row("u2", "orta", "uyari", "a\xff\xffb\xed\xa0\x80c\xf0\x90\x80d\xf4\x90\x80\x80e\xc0\xaf q\xed\x9fz \xe0\xa0 \xf4\x8fz", "", "", "", "ubuntu"),
		"unicode ayırıcı":   hdr + row("s1", "orta", "uyari", "x y\u0085z\x0bq", "", "", "", "ubuntu") + good,
		"CR ve CRLF":        strings.ReplaceAll(hdr+good+row("r2", "dusuk", "oneri", "x", "", "", "", "hepsi"), "\n", "\r\n"),
		"yalnız CR":         strings.ReplaceAll(hdr+good, "\n", "\r"),
		"fazla sütun":       strings.TrimRight(hdr, "\n") + "\tekstra\n" + strings.TrimRight(good, "\n") + "\tekstra\n",
		"başlık boşluklu":   " kimlik \trisk\ttur\taciklama\tayrinti\toneri\talan\tdeger\tkapanir_varsayilan\tplatform \n" + good,
		"başlık bozuk":      "kimlik\trisk\n" + good,
		"başlık yer":        "risk\tkimlik\ttur\taciklama\tayrinti\toneri\talan\tdeger\tkapanir_varsayilan\tplatform\n" + good,
		"boş dosya":         "",
		"yalnız başlık":     hdr,
		"boş satırlar":      hdr + "\n  \n" + good + "\n",
		"sondaki \\n yok":   strings.TrimRight(hdr+good, "\n"),
		"eşlemeli":          hdr + good + row("e2", "orta", "uyari", "x", "", "", "", "ubuntu") + row("e3", "orta", "uyari", "x", "ufw", "", "", "ubuntu"),
		"tekrar":            hdr + good + good,
	}
	for _, p := range ordered(cases) {
		name, c := p.K, p.V
		state := t.TempDir()
		put(t, state, "linux-hardening/web1/out/web1/2026/bulgular.tsv", c, 1_700_000_000)
		compareFindings(t, name, state, py)
	}
	// çok satır → kesilme
	{
		state := t.TempDir()
		var b strings.Builder
		b.WriteString(hdr)
		for i := 0; i < 2105; i++ {
			b.WriteString(row(fmt.Sprintf("id%d", i), Risks[i%4], "uyari", "x", "", "", "", "ubuntu"))
		}
		put(t, state, "linux-hardening/web1/out/web1/2026/bulgular.tsv", b.String(), 1_700_000_000)
		compareFindings(t, "2105 satır", state, py)
	}
	// büyük dosya (> 4 MiB)
	{
		state := t.TempDir()
		pad := row("big", "orta", "uyari", strings.Repeat("a", 250), "", "", "", "ubuntu")
		put(t, state, "linux-hardening/web1/out/web1/2026/bulgular.tsv", hdr+strings.Repeat(pad, 4*1024*1024/len(pad)+30), 1_700_000_000)
		compareFindings(t, "4MiB+", state, py)
	}
}

func TestFindingsNewestSymlinkDepthMatchPython(t *testing.T) {
	py := pyScript(t, "component_findings.py")
	good := hdr + row("n1", "orta", "uyari", "x", "", "", "", "ubuntu")
	state := t.TempDir()
	put(t, state, "linux-hardening/web1/out/web1/a/bulgular.tsv", hdr+row("eski", "orta", "uyari", "x", "", "", "", "ubuntu"), 1_600_000_000)
	put(t, state, "linux-hardening/web1/out/web1/b/bulgular.tsv", good, 1_700_000_000)
	put(t, state, "linux-hardening/web1/out/web1/c/bulgular.tsv", hdr+row("eski2", "orta", "uyari", "x", "", "", "", "ubuntu"), 1_650_000_000)
	compareFindings(t, "en yeni", state, py)

	// bağlantılı dosya ve klasör YOK sayılır
	state = t.TempDir()
	real := put(t, state, "dis/bulgular.tsv", good, 1_800_000_000)
	os.MkdirAll(filepath.Join(state, "linux-hardening/web1/out"), 0o755)
	os.Symlink(real, filepath.Join(state, "linux-hardening/web1/out/bulgular.tsv"))
	os.Symlink(filepath.Join(state, "dis"), filepath.Join(state, "linux-hardening/web1/out/dirlink"))
	put(t, state, "linux-hardening/web1/out/ok/bulgular.tsv", hdr+row("gercek", "orta", "uyari", "x", "", "", "", "ubuntu"), 1_500_000_000)
	compareFindings(t, "symlink", state, py)

	// derinlik sınırı: 5 klasör aşağı inilir, 6'ncıda bulunmaz
	for depth := 3; depth <= 8; depth++ {
		state = t.TempDir()
		rel := "linux-hardening/web1" + strings.Repeat("/d", depth) + "/bulgular.tsv"
		put(t, state, rel, good, 1_700_000_000)
		compareFindings(t, fmt.Sprintf("derinlik %d", depth), state, py)
	}
	// dosya yok / klasör yok
	compareFindings(t, "yok", t.TempDir(), py)
}

func TestFindingsFuzzMatchPython(t *testing.T) {
	py := pyScript(t, "component_findings.py")
	rng := rand.New(rand.NewSource(7))
	pick := func(l []string) string { return l[rng.Intn(len(l))] }
	ids := []string{"A1", "ssh-1", "-x", "", "a b", strings.Repeat("a", 41), "ok_1.2"}
	risks := append(append([]string{}, Risks...), "Yuksek", "")
	turs := []string{"uyari", "oneri", "Uyari", ""}
	texts := []string{"düz", "tab\x01", "uzun" + strings.Repeat("ş", 301), "\xff\xfe", "x y", " pad ", ""}
	alans := []string{"", "ssh", "Ssh", "a-b", strings.Repeat("a", 41)}
	degers := []string{"", "A=1", "a b=1", "A=" + strings.Repeat("v", 201), "9A=1", "A=ç"}
	kaps := []string{"", "evet", "hayir", "x"}
	plats := []string{"ubuntu", "proxmox", "hepsi", "debian", ""}
	n := 0
	for it := 0; it < 150; it++ {
		var b strings.Builder
		b.WriteString(hdr)
		for i := 0; i < 1+rng.Intn(12); i++ {
			cols := []string{pick(ids), pick(risks), pick(turs), pick(texts), pick(texts), pick(texts), pick(alans), pick(degers), pick(kaps), pick(plats)}
			if rng.Intn(15) == 0 {
				cols = cols[:rng.Intn(10)]
			}
			b.WriteString(strings.Join(cols, "\t") + "\n")
		}
		state := t.TempDir()
		put(t, state, "linux-hardening/web1/out/web1/x/bulgular.tsv", b.String(), 1_700_000_000+int64(it))
		compareFindings(t, fmt.Sprintf("fuzz %d", it), state, py)
		n++
	}
	t.Logf("%d rastgele dosya", n)
}

func TestFindingsBadArgs(t *testing.T) {
	if _, err := Load(t.TempDir(), "a/b", "web1", "bulgular.tsv"); err != ErrBadArgs {
		t.Error("bozuk paket adı kabul edildi")
	}
	if _, err := Load(t.TempDir(), "p", "s", "../x.tsv"); err != ErrBadArgs {
		t.Error("bozuk dosya adı kabul edildi")
	}
	if _, err := Load(t.TempDir(), "p", "s", "x.txt"); err != ErrBadArgs {
		t.Error("tsv olmayan dosya kabul edildi")
	}
}
