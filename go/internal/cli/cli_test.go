package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"keylab/internal/shellgold"
	"keylab/internal/trust"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	r, _ := filepath.Abs("../../..")
	if os.Getenv("KEYLAB_KAYIT_KABUK") == "1" { // kayıt: kabuk motoru gerekir; oynatmada gerekmez (altın dosya)
		if _, err := os.Stat(filepath.Join(r, "referans/kabuk/sistem/Helpers/ComponentsCLI.sh")); err != nil {
			t.Skip("kabuk motoru yok")
		}
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen yok")
	}
	return r
}

func shaHex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func tarGz(id, ver string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	p := id + "-" + ver
	add := func(name, body string, dir bool) {
		h := &tar.Header{Name: name, Mode: 0o644, Typeflag: tar.TypeReg, Size: int64(len(body))}
		if dir {
			h = &tar.Header{Name: name, Mode: 0o755, Typeflag: tar.TypeDir}
		}
		tw.WriteHeader(h)
		if !dir {
			tw.Write([]byte(body))
		}
	}
	add(p+"/", "", true)
	add(p+"/component.json", fmt.Sprintf(`{"id":%q,"version":%q}`, id, ver), false)
	add(p+"/VERSION", ver+"\n", false)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sshSign(t *testing.T, priv, ns string, data []byte) []byte {
	t.Helper()
	d := t.TempDir()
	f := filepath.Join(d, "d")
	os.WriteFile(f, data, 0o600)
	if o, err := exec.Command("ssh-keygen", "-Y", "sign", "-f", priv, "-n", ns, f).CombinedOutput(); err != nil {
		t.Fatalf("sign: %v %s", err, o)
	}
	b, _ := os.ReadFile(f + ".sig")
	return b
}

func keypair(t *testing.T, dir, name string) (priv, pub string) {
	priv = filepath.Join(dir, name)
	if o, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, o)
	}
	b, _ := os.ReadFile(priv + ".pub")
	f := strings.Fields(string(b))
	return priv, f[0] + " " + f[1]
}

// fixtures: tüm dünya için sabit parçalar (anahtarlar, arşivler, indeks) — her senaryo bunlardan YENİ bir dünya kurar.
type fixtures struct {
	dir             string
	compPub, idxPub string
	cPriv, iPriv    string
	archives        map[string][]byte // "id-ver" → tar.gz
	sigs            map[string][]byte
	netDir          string // sahte ağ: URL → dosya
	gold            *shellgold.Store
}

func netName(url string) string { return strings.NewReplacer("/", "_", ":", "_").Replace(url) }

func newFixtures(t *testing.T) *fixtures {
	f := &fixtures{dir: t.TempDir(), archives: map[string][]byte{}, sigs: map[string][]byte{}, gold: shellgold.Open(t, "cli")}
	f.cPriv, f.compPub = keypair(t, f.dir, "c")
	f.iPriv, f.idxPub = keypair(t, f.dir, "i")
	f.netDir = filepath.Join(f.dir, "net")
	os.MkdirAll(f.netDir, 0o755)
	for _, k := range [][2]string{{"alfa", "1.0.0"}, {"alfa", "1.1.0"}, {"beta", "1.0.0"}, {"gama", "1.0.0"}, {"delta", "2.0.0"}} {
		a := tarGz(k[0], k[1])
		name := k[0] + "-" + k[1]
		f.archives[name] = a
		f.sigs[name] = sshSign(t, f.cPriv, "sshpocket-component", a)
		url := fmt.Sprintf("https://github.com/o/%s/releases/download/v%s/%s.tar.gz", k[0], k[1], name)
		os.WriteFile(filepath.Join(f.netDir, netName(url)), a, 0o600)
		os.WriteFile(filepath.Join(f.netDir, netName(url+".sig")), f.sigs[name], 0o600)
	}
	now := time.Now().UTC()
	doc, _ := json.Marshal(map[string]any{"schema": 1, "serial": 5, "issued_at": now.Add(-24 * time.Hour).Format("2006-01-02T15:04:05Z"), "valid_until": now.Add(30 * 24 * time.Hour).Format("2006-01-02T15:04:05Z"),
		"packages": map[string]any{
			"alfa":  map[string]any{"repo": "o/alfa", "version": "1.1.0", "asset": "alfa-1.1.0.tar.gz", "sha256": shaHex(f.archives["alfa-1.1.0"])},
			"beta":  map[string]any{"repo": "o/beta", "version": "1.0.0", "asset": "beta-1.0.0.tar.gz", "sha256": shaHex(f.archives["beta-1.0.0"])},
			"gama":  map[string]any{"repo": "o/gama", "version": "1.0.0", "asset": "gama-1.0.0.tar.gz", "sha256": shaHex(f.archives["gama-1.0.0"])},
			"delta": map[string]any{"repo": "o/delta", "version": "2.0.0", "asset": "delta-2.0.0.tar.gz", "sha256": shaHex(f.archives["delta-2.0.0"]), "yanked": true},
		}})
	base := "https://idx.example/index"
	os.WriteFile(filepath.Join(f.netDir, netName(base+"/index.json")), doc, 0o600)
	os.WriteFile(filepath.Join(f.netDir, netName(base+"/index.json.sig")), sshSign(t, f.iPriv, "sshpocket-index", doc), 0o600)
	return f
}

const idxBase = "https://idx.example/index"

type world struct {
	root, comp, lock, signers, servers string
	f                                  *fixtures
	goldTree                           *string // kabuk dünyası için: kayıttan gelen dosya ağacı (oynatmada gerçek kabuk koşmaz)
}

func (f *fixtures) world(t *testing.T, lockPkgs []string, install map[string]string, indexOn bool) *world {
	t.Helper()
	root := t.TempDir()
	w := &world{root: root, comp: filepath.Join(root, "components"), lock: filepath.Join(root, "components.lock"), signers: filepath.Join(root, "index_signers"), servers: filepath.Join(root, "servers"), f: f}
	os.MkdirAll(w.comp, 0o755)
	os.MkdirAll(w.servers, 0o700)
	var lk strings.Builder
	for _, id := range lockPkgs {
		ver := map[string]string{"alfa": "1.0.0", "beta": "1.0.0", "gama": "1.0.0", "delta": "2.0.0"}[id]
		fmt.Fprintf(&lk, "[%s]\nrepo = o/%s\nversion = %s\nsha256 = %s\nsigners = sshpocket-component namespaces=\"sshpocket-component\" %s\n", id, id, ver, shaHex(f.archives[id+"-"+ver]), f.compPub)
	}
	os.WriteFile(w.lock, []byte(lk.String()), 0o600)
	if indexOn {
		os.WriteFile(w.signers, []byte("sshpocket-index namespaces=\"sshpocket-index\" "+f.idxPub+"\n"), 0o600)
	} else {
		os.WriteFile(w.signers, []byte("# boş\n"), 0o600)
	}
	lock, err := trust.LoadLock(w.lock)
	if err != nil {
		t.Fatal(err)
	}
	for id, ver := range install {
		name := id + "-" + ver
		arc := filepath.Join(root, name+".tar.gz")
		os.WriteFile(arc, f.archives[name], 0o600)
		os.WriteFile(arc+".sig", f.sigs[name], 0o600)
		expected := ""
		if e, _ := lock.Get(id); e.Version != ver {
			expected = shaHex(f.archives[name])
		}
		if _, err := trust.InstallArchive(w.comp, lock, id, ver, arc, arc+".sig", expected); err != nil {
			t.Fatalf("kurulum %s: %v", name, err)
		}
		os.Remove(arc)
		os.Remove(arc + ".sig")
	}
	return w
}

func (w *world) goCLI(stdin string) (*Components, *bytes.Buffer, *bytes.Buffer) {
	var so, se bytes.Buffer
	c := &Components{Dir: w.comp, LockPath: w.lock, ServersDir: w.servers, SignersFile: w.signers, IndexBase: idxBase,
		SSHPocketVer: "9.9.9", In: strings.NewReader(stdin), Out: &so, Err: &se,
		Download: func(url, dest string, max int64) error {
			b, err := os.ReadFile(filepath.Join(w.f.netDir, netName(url)))
			if err != nil {
				return err
			}
			return os.WriteFile(dest, b, 0o600)
		},
	}
	return c, &so, &se
}

var shLibs = []string{"Validate.sh", "ModuleFiles.sh", "SSHHelper.sh", "SSHModule.sh", "SSHModuleEdit.sh", "Servers.sh", "Components.sh", "ComponentsInstall.sh", "ComponentsManifest.sh", "ComponentsVenv.sh", "ComponentsRun.sh", "ComponentsVars.sh", "ComponentsGates.sh", "ComponentsOnboard.sh", "ComponentsReport.sh", "ComponentsIndex.sh", "ComponentsCLI.sh"}

type res struct {
	code     int
	out, err string
}

// gres: kabuk çalışmasının kaydedilen sonucu (çıktılar ve dosya ağacı, geçici yollar/damgalar normalleştirilmiş).
type gres struct {
	Code           int
	Out, Err, Tree string
}

// shell: kabuk çalışmasının sonucu. Oynatmada altın dosyadan gelir (anahtar: test adı + çağrı sırası); w.tree() kayıtlı ağacı döndürür.
func (w *world) shell(t *testing.T, r, stdin string, args ...string) res {
	t.Helper()
	g := shellgold.Do(t, w.f.gold, w.f.gold.Next(t), func() gres {
		x := w.shellLive(t, r, stdin, args...)
		return gres{x.code, w.norm(x.out), w.norm(x.err), w.treeLive(t)}
	})
	w.goldTree = &g.Tree
	return res{g.Code, g.Out, g.Err}
}

func (w *world) shellLive(t *testing.T, r, stdin string, args ...string) res {
	t.Helper()
	prelude := `comp_download() { cp "$NETDIR/$(printf '%s' "$1" | tr '/:' '__')" "$2"; }
comp_index_download() { cp "$NETDIR/$(printf '%s' "$1" | tr '/:' '__')" "$2"; }
comp_http_head() { case "$1" in *o/alfa*) echo https://github.com/o/alfa/releases/tag/v1.2.0 ;; *o/beta*) echo https://github.com/o/beta/releases/tag/v1.0.0 ;; *) : ;; esac; }
comp_http_json() { printf '[\n  {\n    "tag_name": "v3.0.0"\n  }\n]\n'; }`
	script := `export SISTEM_PATH="$1"; shift; for l in ` + strings.Join(shLibs, " ") + `; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; eval "$PRELUDE"; components_cli "$@"`
	cmd := exec.Command("bash", append([]string{"-c", script, "_", filepath.Join(r, "referans/kabuk/sistem")}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8", "PRELUDE="+prelude, "NETDIR="+w.f.netDir, "SSHPOCKET_COMPONENTS_DIR="+w.comp, "SSHPOCKET_COMPONENTS_LOCK="+w.lock,
		"SSHPOCKET_INDEX_SIGNERS="+w.signers, "SSHPOCKET_INDEX_URL="+idxBase, "SSHPOCKET_SERVERS_DIR="+w.servers, "SCRIPT_VERSION=9.9.9")
	cmd.Stdin = strings.NewReader(stdin)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	return res{code, so.String(), se.String()}
}

var rePrompt = regexp.MustCompile(`[^\n]*\[y/N\] `)
var reValid = regexp.MustCompile(`geçerlilik: \d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ`)
var reStamp = regexp.MustCompile(`bozuk-\d{8}-\d{6}`)

func (w *world) tree(t *testing.T) string {
	if w.goldTree != nil {
		return *w.goldTree
	}
	return w.treeLive(t)
}

func (w *world) treeLive(t *testing.T) string {
	var lines []string
	filepath.Walk(w.comp, func(p string, info os.FileInfo, err error) error {
		if err != nil || p == w.comp {
			return nil
		}
		rel, _ := filepath.Rel(w.comp, p)
		rel = reStamp.ReplaceAllString(rel, "bozuk-STAMP")
		if strings.HasPrefix(rel, ".index/") || rel == ".index" {
			if info.IsDir() {
				return nil
			}
			lines = append(lines, rel+" -") // içerik (süre, rastgele anahtar imzası) koşuya bağlı: yalnız varlığı karşılaştırılır
			return nil
		}
		if info.IsDir() {
			lines = append(lines, rel+"/")
		} else {
			b, _ := os.ReadFile(p)
			lines = append(lines, fmt.Sprintf("%s %v %s", rel, info.Mode().Perm(), shaHex(b)[:12]))
		}
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (w *world) norm(s string) string {
	s = strings.ReplaceAll(s, "./start.sh --components", "keylab components") // kasıtlı fark: komut adı (kabukta start.sh)
	s = strings.ReplaceAll(s, w.root, "<W>")
	if r, err := filepath.Abs("../../.."); err == nil {
		s = strings.ReplaceAll(s, r, "<R>") // depo kökü (kişisel yol altın dosyaya girmesin)
	}
	s = reStamp.ReplaceAllString(s, "bozuk-STAMP")
	s = reValid.ReplaceAllString(s, "geçerlilik: <T>") // indeks süresi "şimdi + 30 gün": koşuya bağlı
	return s
}

func compare(t *testing.T, label string, sh, g res, w1, w2 *world, t1, t2 string) {
	t.Helper()
	gerr := rePrompt.ReplaceAllString(g.err, "")
	if sh.code != g.code || w1.norm(sh.out) != w2.norm(g.out) || w1.norm(sh.err) != w2.norm(gerr) {
		t.Errorf("%s: kabuk=%d go=%d\nstdout kabuk: %q\nstdout go:    %q\nstderr kabuk: %q\nstderr go:    %q", label, sh.code, g.code, w1.norm(sh.out), w2.norm(g.out), w1.norm(sh.err), w2.norm(gerr))
	}
	if t1 != t2 {
		t.Errorf("%s: dosya ağacı farklı\nkabuk:\n%s\ngo:\n%s", label, t1, t2)
	}
}

func run(t *testing.T, w *world, stdin string, fn func(c *Components) int) res {
	c, so, se := w.goCLI(stdin)
	code := fn(c)
	return res{code, so.String(), se.String()}
}

type scenario struct {
	name    string
	lock    []string
	install map[string]string
	index   bool
}

var scenarios = []scenario{
	{"hepsi kurulu, indeks kapalı", []string{"alfa", "beta"}, map[string]string{"alfa": "1.0.0", "beta": "1.0.0"}, false},
	{"karışık durum, indeks açık", []string{"alfa", "beta", "gama"}, map[string]string{"alfa": "1.0.0", "beta": "1.0.0"}, true},
	{"kilit yok paket", []string{"gama"}, map[string]string{}, true},
	{"kilitten yeni kurulu", []string{"alfa"}, map[string]string{"alfa": "1.1.0"}, true},
	{"boş kilit", []string{}, map[string]string{}, false},
}

func TestListMatchesShell(t *testing.T) {
	r := repoRoot(t)
	f := newFixtures(t)
	for _, s := range scenarios {
		w1, w2 := f.world(t, s.lock, s.install, s.index), f.world(t, s.lock, s.install, s.index)
		sh := w1.shell(t, r, "", "list")
		g := run(t, w2, "", func(c *Components) int { return c.List() })
		compare(t, "list/"+s.name, sh, g, w1, w2, w1.tree(t), w2.tree(t))
	}
	// bozuk kurulum: dosya değişmiş
	w1, w2 := f.world(t, []string{"alfa"}, map[string]string{"alfa": "1.0.0"}, false), f.world(t, []string{"alfa"}, map[string]string{"alfa": "1.0.0"}, false)
	for _, w := range []*world{w1, w2} {
		os.WriteFile(filepath.Join(w.comp, "alfa", "1.0.0", "VERSION"), []byte("oynandı\n"), 0o644)
	}
	compare(t, "list/bozuk", w1.shell(t, r, "", "list"), run(t, w2, "", func(c *Components) int { return c.List() }), w1, w2, w1.tree(t), w2.tree(t))
	// eksik kurulum: current var, klasör yok
	// kilit bozuk
	w1, w2 = f.world(t, []string{"alfa"}, nil, false), f.world(t, []string{"alfa"}, nil, false)
	for _, w := range []*world{w1, w2} {
		os.WriteFile(w.lock, []byte("[alfa]\nrepo = o/alfa\n"), 0o600)
	}
	compare(t, "list/bozuk kilit", w1.shell(t, r, "", "list"), run(t, w2, "", func(c *Components) int { return c.List() }), w1, w2, w1.tree(t), w2.tree(t))
}

func TestSyncMatchesShell(t *testing.T) {
	r := repoRoot(t)
	f := newFixtures(t)
	cases := []struct {
		name, stdin string
		args        []string
		s           scenario
		tamper      bool
	}{
		{"eksik --yes", "", []string{"--yes"}, scenarios[2], false},
		{"eksik onay y", "y\n", nil, scenarios[2], false},
		{"eksik onay n", "n\n", nil, scenarios[2], false},
		{"eksik onay e", "e\n", nil, scenarios[2], false},
		{"hepsi tamam", "", []string{"--yes"}, scenarios[0], false},
		{"bozuk --yes", "", []string{"--yes"}, scenarios[0], true},
		{"bozuk onay n", "n\n", nil, scenarios[0], true},
		{"kilitten yeni", "", []string{"--yes"}, scenarios[3], false},
		{"karışık", "y\ny\n", nil, scenarios[1], false},
	}
	for _, c := range cases {
		w1, w2 := f.world(t, c.s.lock, c.s.install, c.s.index), f.world(t, c.s.lock, c.s.install, c.s.index)
		if c.tamper {
			for _, w := range []*world{w1, w2} {
				os.WriteFile(filepath.Join(w.comp, "alfa", "1.0.0", "VERSION"), []byte("oynandı\n"), 0o644)
			}
		}
		sh := w1.shell(t, r, c.stdin, append([]string{"sync"}, c.args...)...)
		g := run(t, w2, c.stdin, func(cc *Components) int { return cc.Sync(len(c.args) > 0 && c.args[0] == "--yes") })
		compare(t, "sync/"+c.name, sh, g, w1, w2, w1.tree(t), w2.tree(t))
	}
}

func TestUpdateUpgradeMatchShell(t *testing.T) {
	r := repoRoot(t)
	f := newFixtures(t)
	type tc struct {
		name, stdin string
		cmd         []string
		s           scenario
	}
	mixed := scenarios[1]
	cases := []tc{
		{"update", "", []string{"update"}, mixed},
		{"update indeks kapalı", "", []string{"update"}, scenarios[0]},
		{"upgrade --yes", "", []string{"upgrade", "--yes"}, mixed},
		{"upgrade onay y", "y\n", []string{"upgrade"}, mixed},
		{"upgrade onay n", "n\n", []string{"upgrade"}, mixed},
		{"upgrade adı verilen kurulu olmayan", "", []string{"upgrade", "--yes", "gama"}, mixed},
		{"upgrade alfa", "", []string{"upgrade", "--yes", "alfa"}, mixed},
		{"upgrade bilinmeyen ad", "", []string{"upgrade", "--yes", "yok"}, mixed},
		{"upgrade geçersiz ad", "", []string{"upgrade", "--yes", "BÜYÜK"}, mixed},
		{"upgrade bilinmeyen bayrak", "", []string{"upgrade", "--x"}, mixed},
		{"upgrade yükseltilecek yok", "", []string{"upgrade", "--yes"}, scenarios[3]},
		{"upgrade kapalı", "", []string{"upgrade", "--yes"}, scenarios[0]},
	}
	for _, c := range cases {
		w1, w2 := f.world(t, c.s.lock, c.s.install, c.s.index), f.world(t, c.s.lock, c.s.install, c.s.index)
		sh := w1.shell(t, r, c.stdin, c.cmd...)
		g := run(t, w2, c.stdin, func(cc *Components) int {
			if c.cmd[0] == "update" {
				return cc.Update()
			}
			return cc.Upgrade(c.cmd[1:])
		})
		compare(t, c.name, sh, g, w1, w2, w1.tree(t), w2.tree(t))
	}
}

func TestCheckNewMatchesShell(t *testing.T) {
	r := repoRoot(t)
	f := newFixtures(t)
	s := scenarios[1]
	w1, w2 := f.world(t, s.lock, s.install, s.index), f.world(t, s.lock, s.install, s.index)
	sh := w1.shell(t, r, "", "check-new")
	c, so, se := w2.goCLI("")
	c.Head = func(url string) string {
		switch {
		case strings.Contains(url, "o/alfa"):
			return "https://github.com/o/alfa/releases/tag/v1.2.0"
		case strings.Contains(url, "o/beta"):
			return "https://github.com/o/beta/releases/tag/v1.0.0"
		}
		return ""
	}
	c.JSON = func(string) string { return "[\n  {\n    \"tag_name\": \"v3.0.0\"\n  }\n]\n" }
	code := c.CheckNew()
	compare(t, "check-new", sh, res{code, so.String(), se.String()}, w1, w2, w1.tree(t), w2.tree(t))
}

func TestPruneMatchesShell(t *testing.T) {
	r := repoRoot(t)
	f := newFixtures(t)
	build := func() *world {
		w := f.world(t, []string{"alfa", "beta"}, map[string]string{"alfa": "1.0.0", "beta": "1.0.0"}, false)
		// alfa: 1.1.0 etkin; eski sürüm klasörleri yan yana
		lock, _ := trust.LoadLock(w.lock)
		arc := filepath.Join(w.root, "a.tar.gz")
		os.WriteFile(arc, f.archives["alfa-1.1.0"], 0o600)
		os.WriteFile(arc+".sig", f.sigs["alfa-1.1.0"], 0o600)
		trust.InstallArchive(w.comp, lock, "alfa", "1.1.0", arc, arc+".sig", shaHex(f.archives["alfa-1.1.0"]))
		lk, _ := os.ReadFile(w.lock)
		os.WriteFile(w.lock, []byte(strings.Replace(string(lk), "version = 1.0.0", "version = 0.5.0", 1)), 0o600)
		for _, v := range []string{"0.5.0", "0.9.0", "3.0.0"} {
			os.MkdirAll(filepath.Join(w.comp, "alfa", v), 0o755)
			os.WriteFile(filepath.Join(w.comp, "alfa", v, "component.json"), []byte("{}"), 0o644)
			os.WriteFile(filepath.Join(w.comp, "alfa", v+".files"), []byte("x"), 0o644)
		}
		os.MkdirAll(filepath.Join(w.comp, "alfa", "1.0.0.bozuk-20260101-000000"), 0o755)
		os.MkdirAll(filepath.Join(w.comp, "yeni-paket", "1.0.0"), 0o755) // current yok
		os.MkdirAll(filepath.Join(w.comp, "BÜYÜK"), 0o755)
		return w
	}
	for _, c := range []struct {
		name, stdin string
		args        []string
	}{
		{"dry-run", "", []string{"--dry-run"}},
		{"--yes", "", []string{"--yes"}},
		{"onay y", "y\n", nil},
		{"onay n", "n\n", nil},
		{"bilinmeyen bayrak", "", []string{"--x"}},
	} {
		w1, w2 := build(), build()
		sh := w1.shell(t, r, c.stdin, append([]string{"prune"}, c.args...)...)
		g := run(t, w2, c.stdin, func(cc *Components) int { return cc.Prune(c.args) })
		compare(t, "prune/"+c.name, sh, g, w1, w2, w1.tree(t), w2.tree(t))
	}
	// kurulu paket yok
	w1, w2 := f.world(t, nil, nil, false), f.world(t, nil, nil, false)
	compare(t, "prune/boş", w1.shell(t, r, "", "prune"), run(t, w2, "", func(c *Components) int { return c.Prune(nil) }), w1, w2, w1.tree(t), w2.tree(t))
}

func TestValidateMatchesShell(t *testing.T) {
	r := repoRoot(t)
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 yok")
	}
	f := newFixtures(t)
	pkg := filepath.Join(r, "components/utility/0.1.0")
	if _, err := os.Stat(pkg); err != nil {
		t.Skip("örnek paket yok")
	}
	w1, w2 := f.world(t, nil, nil, false), f.world(t, nil, nil, false)
	// kurulu paket kimliği için components'e bildirimi olan gerçek paketi kopyala
	for _, w := range []*world{w1, w2} {
		dst := filepath.Join(w.comp, "utility", "0.1.0")
		os.MkdirAll(filepath.Dir(dst), 0o755)
		exec.Command("cp", "-R", pkg, dst).Run()
		os.WriteFile(filepath.Join(w.comp, "utility", "current"), []byte("0.1.0\n"), 0o644)
	}
	for _, args := range [][]string{{pkg}, {pkg, "--strict"}, {pkg, "--unsigned"}, {"utility"}, {"utility", "--strict"}, {"yok"}, {}, {"--strict"}, {filepath.Join(r, "nonexistent")}} {
		sh := w1.shell(t, r, "", append([]string{"validate"}, args...)...)
		g := run(t, w2, "", func(c *Components) int { return c.Validate(args) })
		// çıktılardaki dünya yolları farklı: norm değişir
		sh.out, g.out = strings.ReplaceAll(sh.out, w1.root, "<W>"), strings.ReplaceAll(g.out, w2.root, "<W>")
		compare(t, fmt.Sprintf("validate %v", args), sh, g, w1, w2, "", "")
	}
}
