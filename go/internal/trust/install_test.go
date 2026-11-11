package trust

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type tfile struct {
	name string
	body string
	mode int64
	typ  byte
	link string
}

func buildTarGz(t *testing.T, files []tfile) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		h := &tar.Header{Name: f.name, Mode: f.mode, Typeflag: f.typ, Linkname: f.link}
		if h.Typeflag == 0 {
			h.Typeflag = tar.TypeReg
		}
		if h.Mode == 0 {
			h.Mode = 0o644
		}
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(f.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			tw.Write([]byte(f.body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func goodPkg(id, ver string) []tfile {
	p := id + "-" + ver
	return []tfile{
		{name: p + "/", typ: tar.TypeDir, mode: 0o755},
		{name: p + "/component.json", body: fmt.Sprintf(`{"id":%q,"version":%q}`, id, ver)},
		{name: p + "/VERSION", body: ver + "\n"},
		{name: p + "/playbooks/", typ: tar.TypeDir, mode: 0o755},
		{name: p + "/playbooks/a.yml", body: "- hosts: all\n"},
		{name: p + "/scripts/run.sh", body: "#!/bin/sh\n", mode: 0o755},
		{name: p + "/.DS_Store", body: "x"},
	}
}

// signEnv: test anahtarı, kilit dosyası ve bileşen klasörü.
type signEnv struct {
	dir, lockPath, compDir, priv, pub string
}

func newSignEnv(t *testing.T) *signEnv {
	t.Helper()
	needSSHKeygen(t)
	dir := t.TempDir()
	priv, pub := newKey(t, dir, "k")
	return &signEnv{dir: dir, priv: priv, pub: pub, lockPath: filepath.Join(dir, "components.lock"), compDir: filepath.Join(dir, "components")}
}

func (e *signEnv) signer() string {
	return `sshpocket-component namespaces="sshpocket-component" ` + e.pub
}

func shaHex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func (e *signEnv) writeLock(t *testing.T, entries ...LockEntry) {
	var sb strings.Builder
	for _, en := range entries {
		sb.WriteString(block(en.ID, "repo", en.Repo, "version", en.Version, "sha256", en.SHA256, "signers", en.Signers))
	}
	os.WriteFile(e.lockPath, []byte(sb.String()), 0o600)
}

func (e *signEnv) writeArchive(t *testing.T, name string, data []byte, ns string) (arc, sig string) {
	arc = filepath.Join(e.dir, name)
	os.WriteFile(arc, data, 0o600)
	s := sign(t, e.priv, ns, "", data)
	sig = arc + ".sig"
	os.WriteFile(sig, s, 0o600)
	return
}

// snapshot: bileşen klasörünün (geçici dosyalar hariç) dosya→(kip, içerik özeti) haritası.
func snapshot(root string) map[string]string {
	out := map[string]string{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if strings.Contains(rel, ".tmp") || strings.Contains(rel, ".install.lock") {
			return nil
		}
		if d.IsDir() {
			out[rel+"/"] = "dir"
			return nil
		}
		info, _ := d.Info()
		b, _ := os.ReadFile(p)
		out[rel] = fmt.Sprintf("%o %s", info.Mode().Perm(), shaHex(b))
		return nil
	})
	return out
}

type installCase struct {
	name        string
	id, ver     string
	data        []byte
	ns          string // imza ad alanı ("" → sshpocket-component)
	lockVer     string // kilit sürümü ("" → ver)
	lockSHA     string // "" → arşivin özeti
	expected    string // beklenen özet (indeksten)
	tamperSig   bool
	preinstall  []string // önceden kurulacak sürümler (aynı arşiv fabrikası)
	editInstall func(dir string)
}

func TestInstallArchiveMatchesShell(t *testing.T) {
	root := repoRoot(t)
	good := func(id, ver string) []byte { return buildTarGz(t, goodPkg(id, ver)) }
	p := func(id, ver string, extra ...tfile) []byte { return buildTarGz(t, append(goodPkg(id, ver), extra...)) }
	paxFirst := func(id, ver string) []byte { // git archive gibi: ilk kayıt pax genel başlığı
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": "0123456789abcdef0123456789abcdef01234567"}})
		for _, f := range goodPkg(id, ver) {
			h := &tar.Header{Name: f.name, Mode: 0o644, Typeflag: tar.TypeReg, Size: int64(len(f.body))}
			if f.typ == tar.TypeDir {
				h.Typeflag, h.Mode, h.Size = tar.TypeDir, 0o755, 0
			}
			tw.WriteHeader(h)
			if h.Typeflag == tar.TypeReg {
				tw.Write([]byte(f.body))
			}
		}
		tw.Close()
		gz.Close()
		return buf.Bytes()
	}
	cases := []installCase{
		{name: "pax genel başlık (git archive)", id: "abc", ver: "1.0.0", data: paxFirst("abc", "1.0.0")},
		{name: "normal", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0")},
		{name: "yanlış ad alanı imzası", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0"), ns: "sshpocket-index"},
		{name: "bozuk imza", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0"), tamperSig: true},
		{name: "kilit özeti farklı", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0"), lockSHA: sha1},
		{name: "kilit sürümü farklı", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0"), lockVer: "1.0.1"},
		{name: "indeksten, sürüm yüksek", id: "abc", ver: "1.1.0", data: good("abc", "1.1.0"), lockVer: "1.0.0", expected: "auto"},
		{name: "indeksten, sürüm kilit altında", id: "abc", ver: "0.9.0", data: good("abc", "0.9.0"), lockVer: "1.0.0", expected: "auto"},
		{name: "indeksten, kilit sürümü, özet farklı", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0"), lockSHA: sha1, expected: "auto"},
		{name: "indeksten, geçersiz özet", id: "abc", ver: "1.1.0", data: good("abc", "1.1.0"), lockVer: "1.0.0", expected: "xyz"},
		{name: "geri sürüm", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0"), lockVer: "1.0.0", preinstall: []string{"1.0.0", "1.2.0"}},
		{name: "aynı sürüm zaten kurulu", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0"), preinstall: []string{"1.0.0"}},
		{name: "aynı sürüm, dosyalar değişmiş", id: "abc", ver: "1.0.0", data: good("abc", "1.0.0"), preinstall: []string{"1.0.0"},
			editInstall: func(d string) { os.WriteFile(filepath.Join(d, "abc/1.0.0/VERSION"), []byte("hacked\n"), 0o644) }},
		{name: "yükseltme", id: "abc", ver: "1.1.0", data: good("abc", "1.1.0"), lockVer: "1.0.0", expected: "auto", preinstall: []string{"1.0.0"}},
		{name: "yanlış üst dizin", id: "abc", ver: "1.0.0", data: buildTarGz(t, []tfile{{name: "baska/component.json", body: "{}"}})},
		{name: "iki üst dizin", id: "abc", ver: "1.0.0", data: p("abc", "1.0.0", tfile{name: "baska/x", body: "1"})},
		{name: "mutlak yol", id: "abc", ver: "1.0.0", data: p("abc", "1.0.0", tfile{name: "/etc/passwd", body: "1"})},
		{name: "yol .. içeriyor", id: "abc", ver: "1.0.0", data: p("abc", "1.0.0", tfile{name: "abc-1.0.0/../x", body: "1"})},
		{name: "ad içinde .. alt dizeyi", id: "abc", ver: "1.0.0", data: p("abc", "1.0.0", tfile{name: "abc-1.0.0/a..b", body: "1"})},
		{name: "sembolik bağ", id: "abc", ver: "1.0.0", data: p("abc", "1.0.0", tfile{name: "abc-1.0.0/l", typ: tar.TypeSymlink, link: "/etc/passwd"})},
		{name: "sert bağ", id: "abc", ver: "1.0.0", data: p("abc", "1.0.0", tfile{name: "abc-1.0.0/h", typ: tar.TypeLink, link: "abc-1.0.0/VERSION"})},
		{name: "fifo", id: "abc", ver: "1.0.0", data: p("abc", "1.0.0", tfile{name: "abc-1.0.0/f", typ: tar.TypeFifo})},
		{name: "bildirim id farklı", id: "abc", ver: "1.0.0", data: buildTarGz(t, []tfile{{name: "abc-1.0.0/component.json", body: `{"id":"xyz","version":"1.0.0"}`}})},
		{name: "bildirim sürüm farklı", id: "abc", ver: "1.0.0", data: buildTarGz(t, []tfile{{name: "abc-1.0.0/component.json", body: `{"id":"abc","version":"1.0.1"}`}})},
		{name: "bildirim yok", id: "abc", ver: "1.0.0", data: buildTarGz(t, []tfile{{name: "abc-1.0.0/VERSION", body: "1.0.0\n"}})},
		{name: "bildirim bozuk", id: "abc", ver: "1.0.0", data: buildTarGz(t, []tfile{{name: "abc-1.0.0/component.json", body: "{"}})},
		{name: "yalnız kök dizin", id: "abc", ver: "1.0.0", data: buildTarGz(t, []tfile{{name: "abc-1.0.0/", typ: tar.TypeDir}})},
		{name: "boş arşiv", id: "abc", ver: "1.0.0", data: buildTarGz(t, nil)},
		{name: "gzip değil", id: "abc", ver: "1.0.0", data: []byte("düz metin, tar değil")},
		{name: "yazılabilir kip temizlenir", id: "abc", ver: "1.0.0", data: p("abc", "1.0.0", tfile{name: "abc-1.0.0/w.txt", body: "x", mode: 0o666}, tfile{name: "abc-1.0.0/x.sh", body: "x", mode: 0o777})},
		{name: "geçersiz kimlik", id: "ABC", ver: "1.0.0", data: good("abc", "1.0.0")},
		{name: "kilitte yok", id: "yokk", ver: "1.0.0", data: good("yokk", "1.0.0")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var res [2]struct {
				snap map[string]string
				code int
				msg  string
			}
			for impl := 0; impl < 2; impl++ { // 0 kabuk, 1 Go
				e := newSignEnv(t)
				lockID := c.id
				if c.name == "kilitte yok" {
					lockID = "baska"
				}
				if c.name == "geçersiz kimlik" {
					lockID = "abc"
				}
				lv, ls := c.lockVer, c.lockSHA
				if lv == "" {
					lv = c.ver
				}
				ns := c.ns
				if ns == "" {
					ns = ComponentNamespace
				}
				arc, sig := e.writeArchive(t, "pkg.tar.gz", c.data, ns)
				if ls == "" {
					ls = shaHex(c.data)
				}
				if c.tamperSig {
					b, _ := os.ReadFile(sig)
					b = bytes.Replace(b, []byte("A"), []byte("B"), 1)
					os.WriteFile(sig, b, 0o600)
				}
				expected := c.expected
				if expected == "auto" {
					expected = shaHex(c.data)
				}
				lock := func(sha string) {
					e.writeLock(t, LockEntry{ID: lockID, Repo: "o/" + lockID, Version: lv, SHA256: sha, Signers: e.signer()})
				}
				// önceden kurulanlar: kendi arşivleriyle, kilit o sürümlere göre yazılarak
				for _, pv := range c.preinstall {
					pd := buildTarGz(t, goodPkg(c.id, pv))
					parc, psig := e.writeArchive(t, "pre-"+pv+".tar.gz", pd, ComponentNamespace)
					e.writeLock(t, LockEntry{ID: lockID, Repo: "o/" + lockID, Version: pv, SHA256: shaHex(pd), Signers: e.signer()})
					var out shOut
					if impl == 0 {
						out = runShell(t, root, []string{"SSHPOCKET_COMPONENTS_LOCK=" + e.lockPath, "SSHPOCKET_COMPONENTS_DIR=" + e.compDir, "SSHPOCKET_INSTALL_LOCK_TRIES=2"}, []string{"Components.sh", "ComponentsInstall.sh"}, "", "comp_install_archive", c.id, pv, parc, psig)
					} else {
						lk, err := LoadLock(e.lockPath)
						if err != nil {
							t.Fatal(err)
						}
						_, err = InstallArchive(e.compDir, lk, c.id, pv, parc, psig, "")
						if err != nil {
							out.code = 1
						}
					}
					if out.code != 0 {
						t.Fatalf("önkoşul kurulumu başarısız (impl %d): %v", impl, out)
					}
				}
				if c.editInstall != nil {
					c.editInstall(e.compDir)
				}
				lock(ls)
				if impl == 0 {
					args := []string{c.id, c.ver, arc, sig}
					if expected != "" {
						args = append(args, expected)
					}
					out := runShell(t, root, []string{"SSHPOCKET_COMPONENTS_LOCK=" + e.lockPath, "SSHPOCKET_COMPONENTS_DIR=" + e.compDir, "SSHPOCKET_INSTALL_LOCK_TRIES=2"}, []string{"Components.sh", "ComponentsInstall.sh"}, "", "comp_install_archive", args...)
					res[0].code = out.code
					res[0].msg = strings.TrimSpace(out.stdout + out.stderr)
				} else {
					lk, err := LoadLock(e.lockPath)
					if err != nil {
						res[1].code, res[1].msg = 1, err.Error()
					} else {
						msg, err := InstallArchive(e.compDir, lk, c.id, c.ver, arc, sig, expected)
						if err != nil {
							res[1].code, res[1].msg = 1, err.Error()
						} else {
							res[1].msg = msg
						}
					}
				}
				res[impl].snap = snapshot(e.compDir)
			}
			if res[0].code != res[1].code {
				t.Errorf("çıkış kabuk=%d (%q) go=%d (%q)", res[0].code, res[0].msg, res[1].code, res[1].msg)
				return
			}
			if res[0].msg != res[1].msg {
				t.Errorf("ileti\n kabuk=%q\n go   =%q", res[0].msg, res[1].msg)
			}
			if !reflect.DeepEqual(res[0].snap, res[1].snap) {
				t.Errorf("klasör farkı\n kabuk=%v\n go   =%v", sortedKV(res[0].snap), sortedKV(res[1].snap))
			}
		})
	}
}

func sortedKV(m map[string]string) []string {
	var out []string
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
