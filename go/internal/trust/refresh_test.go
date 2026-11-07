package trust

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}

func idxDoc(serial int, issued, valid string, pkgs map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"schema": 1, "serial": serial, "issued_at": issued, "valid_until": valid, "packages": pkgs})
	return b
}

func stateSnap(dir string) map[string]string {
	out := map[string]string{}
	for _, n := range []string{"index.json", "index.json.sig", "state"} {
		b, err := os.ReadFile(filepath.Join(dir, ".index", n))
		if err != nil {
			out[n] = "(yok)"
		} else {
			out[n] = shaHex(b)
		}
	}
	return out
}

func TestRefreshMatchesShell(t *testing.T) {
	root := repoRoot(t)
	needSSHKeygen(t)
	type step struct {
		name string
		prep func(t *testing.T, src string, priv string)
		now  int64
	}
	pk := map[string]any{"utility": map[string]any{"repo": "o/utility", "version": "0.1.2", "asset": "utility-0.1.2.tar.gz", "sha256": sha1}}
	pk2 := map[string]any{"utility": map[string]any{"repo": "o/utility", "version": "0.1.3", "asset": "utility-0.1.3.tar.gz", "sha256": sha1}}
	put := func(t *testing.T, src, priv string, doc []byte, ns string) {
		os.WriteFile(filepath.Join(src, "index.json"), doc, 0o600)
		os.WriteFile(filepath.Join(src, "index.json.sig"), sign(t, priv, ns, "", doc), 0o600)
	}
	const issued, valid = "2026-10-25T00:00:00Z", "2026-12-01T00:00:00Z"
	steps := []step{
		{"serial 2 kabul", func(t *testing.T, s, p string) { put(t, s, p, idxDoc(2, issued, valid, pk), IndexNamespace) }, testNow},
		{"aynı indeks yeniden", func(t *testing.T, s, p string) {}, testNow},
		{"eski seri 1 reddedilir", func(t *testing.T, s, p string) { put(t, s, p, idxDoc(1, issued, valid, pk), IndexNamespace) }, testNow},
		{"aynı seri farklı içerik", func(t *testing.T, s, p string) { put(t, s, p, idxDoc(2, issued, valid, pk2), IndexNamespace) }, testNow},
		{"imza ile içerik uyuşmuyor", func(t *testing.T, s, p string) {
			put(t, s, p, idxDoc(5, issued, valid, pk), IndexNamespace)
			os.WriteFile(filepath.Join(s, "index.json"), idxDoc(6, issued, valid, pk2), 0o600)
		}, testNow},
		{"yanlış ad alanı imzası", func(t *testing.T, s, p string) { put(t, s, p, idxDoc(7, issued, valid, pk), ComponentNamespace) }, testNow},
		{"süresi dolmuş", func(t *testing.T, s, p string) { put(t, s, p, idxDoc(8, issued, valid, pk), IndexNamespace) }, 1796000000},
		{"gelecekte imzalı", func(t *testing.T, s, p string) { put(t, s, p, idxDoc(9, issued, valid, pk), IndexNamespace) }, 1700000000},
		{"imzalı ama şema bozuk", func(t *testing.T, s, p string) { put(t, s, p, []byte(`{"schema":9}`), IndexNamespace) }, testNow},
		{"imzalı ama JSON değil", func(t *testing.T, s, p string) { put(t, s, p, []byte(`çöp`), IndexNamespace) }, testNow},
		{"imza dosyası yok", func(t *testing.T, s, p string) {
			put(t, s, p, idxDoc(10, issued, valid, pk), IndexNamespace)
			os.Remove(filepath.Join(s, "index.json.sig"))
		}, testNow},
		{"indeks dosyası yok", func(t *testing.T, s, p string) { os.Remove(filepath.Join(s, "index.json")) }, testNow},
		{"serial 3 kabul", func(t *testing.T, s, p string) { put(t, s, p, idxDoc(3, issued, valid, pk2), IndexNamespace) }, testNow},
		{"serial 3 aynı kabul", func(t *testing.T, s, p string) {}, testNow},
	}
	type env struct {
		dir, src, comp, signers, priv string
	}
	mk := func(withSigner bool, priv, pub string) env {
		dir := t.TempDir()
		e := env{dir: dir, src: filepath.Join(dir, "src"), comp: filepath.Join(dir, "components"), signers: filepath.Join(dir, "index_signers"), priv: priv}
		os.MkdirAll(e.src, 0o755)
		txt := "# yorum\n"
		if withSigner {
			txt += "sshpocket-index namespaces=\"sshpocket-index\" " + pub + "\n"
		}
		os.WriteFile(e.signers, []byte(txt), 0o600)
		return e
	}
	kd := t.TempDir()
	priv, pub := newKey(t, kd, "idx")

	// kapalı özellik
	{
		e := mk(false, priv, pub)
		sh := runShell(t, root, []string{"SSHPOCKET_INDEX_SIGNERS=" + e.signers, "SSHPOCKET_COMPONENTS_DIR=" + e.comp}, []string{"Components.sh", "ComponentsInstall.sh", "ComponentsIndex.sh"}, "", "comp_index_refresh")
		c := &Client{ComponentsDir: e.comp, SignersFile: e.signers}
		_, err := c.Refresh()
		if sh.code != 2 || !errors.Is(err, ErrDisabled) {
			t.Errorf("kapalı: kabuk=%d go=%v", sh.code, err)
		}
	}

	sh := mk(true, priv, pub)
	gv := mk(true, priv, pub)
	prelude := `comp_index_download() { local n="${1##*/}"; [ -f "$SRC/$n" ] || return 1; cp "$SRC/$n" "$2"; }`
	for _, st := range steps {
		st.prep(t, sh.src, priv)
		st.prep(t, gv.src, priv)
		out := runShell(t, root, []string{"SSHPOCKET_INDEX_SIGNERS=" + sh.signers, "SSHPOCKET_COMPONENTS_DIR=" + sh.comp, "SSHPOCKET_INDEX_NOW=" + fmt.Sprint(st.now), "SRC=" + sh.src}, []string{"Components.sh", "ComponentsInstall.sh", "ComponentsIndex.sh"}, prelude, "comp_index_refresh")
		c := &Client{ComponentsDir: gv.comp, SignersFile: gv.signers, Now: func() int64 { return st.now },
			Download: func(url, dest string, max int64) error {
				return copyFile(filepath.Join(gv.src, filepath.Base(url)), dest)
			}}
		idx, err := c.Refresh()
		gcode := 0
		if err != nil {
			gcode = 1
		}
		if out.code != gcode {
			t.Errorf("[%s] çıkış kabuk=%d (%q) go=%d (%v)", st.name, out.code, strings.TrimSpace(out.stderr), gcode, err)
			continue
		}
		if err != nil && strings.TrimSpace(strings.ReplaceAll(out.stderr, sh.comp, "COMP")) != strings.ReplaceAll(err.Error(), gv.comp, "COMP") {
			t.Errorf("[%s] ileti kabuk=%q go=%q", st.name, strings.TrimSpace(out.stderr), err)
		}
		if a, b := stateSnap(sh.dir), stateSnap(gv.dir); !reflect.DeepEqual(a, b) {
			t.Errorf("[%s] durum farkı kabuk=%v go=%v", st.name, a, b)
		}
		if err == nil && idx == nil {
			t.Errorf("[%s] indeks nil", st.name)
		}
	}
	// önbellekten yükleme: sağlam, sonra imza/durum bozulmaları
	cached := func(label string) {
		out := runShell(t, root, []string{"SSHPOCKET_INDEX_SIGNERS=" + sh.signers, "SSHPOCKET_COMPONENTS_DIR=" + sh.comp, "SSHPOCKET_INDEX_NOW=" + fmt.Sprint(testNow)}, []string{"Components.sh", "ComponentsInstall.sh", "ComponentsIndex.sh"}, "", "comp_index_load_cached")
		c := &Client{ComponentsDir: gv.comp, SignersFile: gv.signers, Now: func() int64 { return testNow }}
		_, err := c.LoadCached()
		if (out.code == 0) != (err == nil) {
			t.Errorf("önbellek [%s] kabuk=%d (%q) go=%v", label, out.code, out.stderr, err)
		}
	}
	cached("sağlam")
	for _, e := range []env{sh, gv} {
		b, _ := os.ReadFile(filepath.Join(e.comp, ".index", "state"))
		os.WriteFile(filepath.Join(e.comp, ".index", "state"), []byte(strings.Replace(string(b), "serial=3", "serial=2", 1)), 0o600)
	}
	cached("durum uyuşmuyor")
	for _, e := range []env{sh, gv} {
		os.WriteFile(filepath.Join(e.comp, ".index", "index.json"), []byte(`{"schema":1}`), 0o600)
	}
	cached("içerik bozuldu")
	for _, e := range []env{sh, gv} {
		os.Remove(filepath.Join(e.comp, ".index", "index.json.sig"))
	}
	cached("imza yok")
}

func TestEvalMatchesShell(t *testing.T) {
	root := repoRoot(t)
	needSSHKeygen(t)
	kd := t.TempDir()
	priv, pub := newKey(t, kd, "idx")
	type sc struct {
		name                 string
		idxVer, idxRepo      string
		idxSHA               string
		requires             string
		yanked               bool
		lockVer, lockRepo    string
		lockSHA              string
		installed            string
		me                   string
		inLock, inIdxPackage bool
	}
	base := sc{idxVer: "0.2.0", idxRepo: "o/p", idxSHA: sha1, requires: "3.7.0", lockVer: "0.1.0", lockRepo: "o/p", lockSHA: strings.Repeat("b", 64), installed: "0.1.0", me: "3.9.0", inLock: true, inIdxPackage: true}
	mut := func(name string, f func(*sc)) sc { s := base; s.name = name; f(&s); return s }
	cases := []sc{
		mut("yükseltilebilir", func(s *sc) {}),
		mut("güncel", func(s *sc) { s.installed = "0.2.0" }),
		mut("kurulu değil", func(s *sc) { s.installed = "" }),
		mut("yanked", func(s *sc) { s.yanked = true }),
		mut("sshpocket eski", func(s *sc) { s.requires = "4.0.0"; s.me = "3.9.0" }),
		mut("me boş", func(s *sc) { s.requires = "4.0.0"; s.me = "" }),
		mut("me geçersiz", func(s *sc) { s.requires = "4.0.0"; s.me = "dev" }),
		mut("başka depo", func(s *sc) { s.idxRepo = "x/y" }),
		mut("kilit tabanı altında", func(s *sc) { s.idxVer = "0.0.9" }),
		mut("aynı sürüm farklı özet", func(s *sc) { s.idxVer = "0.1.0" }),
		mut("aynı sürüm aynı özet", func(s *sc) { s.idxVer = "0.1.0"; s.lockSHA = sha1; s.installed = "0.1.0" }),
		mut("kilitte yok", func(s *sc) { s.inLock = false }),
		mut("indekste yok", func(s *sc) { s.inIdxPackage = false }),
		mut("kurulu daha yeni", func(s *sc) { s.installed = "0.3.0" }),
		mut("yanked ve eski depo", func(s *sc) { s.yanked = true; s.idxRepo = "x/y" }),
		mut("10 > 9 sayısal", func(s *sc) { s.idxVer = "0.10.0"; s.installed = "0.9.0"; s.lockVer = "0.9.0" }),
	}
	prelude := `th() { comp_index_refresh >/dev/null 2>&1 || { echo "REFRESH-FAIL"; return 1; }; comp_index_eval p; }; comp_index_download() { cp "$SRC/${1##*/}" "$2"; }`
	for _, c := range cases {
		dir := t.TempDir()
		src := filepath.Join(dir, "src")
		os.MkdirAll(src, 0o755)
		comp := filepath.Join(dir, "components")
		signers := filepath.Join(dir, "index_signers")
		os.WriteFile(signers, []byte("sshpocket-index namespaces=\"sshpocket-index\" "+pub+"\n"), 0o600)
		pkgs := map[string]any{}
		if c.inIdxPackage {
			e := map[string]any{"repo": c.idxRepo, "version": c.idxVer, "asset": "p-" + c.idxVer + ".tar.gz", "sha256": c.idxSHA, "requires_sshpocket": c.requires}
			if c.yanked {
				e["yanked"] = true
			}
			pkgs["p"] = e
		} else {
			pkgs["q"] = map[string]any{"repo": "o/q", "version": "1.0.0", "asset": "q-1.0.0.tar.gz", "sha256": sha1}
		}
		doc := idxDoc(2, "2026-10-25T00:00:00Z", "2026-12-01T00:00:00Z", pkgs)
		os.WriteFile(filepath.Join(src, "index.json"), doc, 0o600)
		os.WriteFile(filepath.Join(src, "index.json.sig"), sign(t, priv, IndexNamespace, "", doc), 0o600)
		lockPath := filepath.Join(dir, "components.lock")
		var entries []LockEntry
		if c.inLock {
			entries = append(entries, LockEntry{ID: "p", Repo: c.lockRepo, Version: c.lockVer, SHA256: c.lockSHA, Signers: `sshpocket-component namespaces="sshpocket-component" ` + pub})
		}
		(&signEnv{lockPath: lockPath}).writeLock(t, entries...)
		if c.installed != "" {
			os.MkdirAll(filepath.Join(comp, "p"), 0o755)
			os.WriteFile(filepath.Join(comp, "p", "current"), []byte(c.installed+"\n"), 0o644)
		}
		out := runShell(t, root, []string{"SSHPOCKET_INDEX_SIGNERS=" + signers, "SSHPOCKET_COMPONENTS_DIR=" + comp, "SSHPOCKET_COMPONENTS_LOCK=" + lockPath,
			"SSHPOCKET_INDEX_NOW=" + fmt.Sprint(testNow), "SRC=" + src, "SCRIPT_VERSION=" + c.me}, []string{"Components.sh", "ComponentsInstall.sh", "ComponentsIndex.sh"}, prelude, "th")
		me := c.me
		if me == "" { // kabuk SCRIPT_VERSION boşken repodaki VERSION dosyasını okur
			me = ReadVersionFile(filepath.Join(root, "VERSION"))
		}
		cl := &Client{ComponentsDir: comp, SignersFile: signers, SSHPocketVer: me, Now: func() int64 { return testNow },
			Download: func(url, dest string, max int64) error { return copyFile(filepath.Join(src, filepath.Base(url)), dest) }}
		idx, err := cl.Refresh()
		if err != nil {
			t.Fatalf("%s: yenileme: %v", c.name, err)
		}
		lock, err := LoadLock(lockPath)
		if err != nil {
			t.Fatalf("%s: kilit: %v", c.name, err)
		}
		v := cl.Eval(idx, lock, "p")
		got := v.Kind + "|" + v.Detail
		if want := strings.TrimSpace(out.stdout); want != got {
			t.Errorf("%s: kabuk=%q go=%q", c.name, want, got)
		}
	}
}
