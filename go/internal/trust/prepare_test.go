package trust

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareMatchesShell(t *testing.T) {
	root := repoRoot(t)
	e := newSignEnv(t)
	data := buildTarGz(t, goodPkg("abc", "1.0.0"))
	arc, sig := e.writeArchive(t, "p.tar.gz", data, ComponentNamespace)
	e.writeLock(t, LockEntry{ID: "abc", Repo: "o/abc", Version: "1.0.0", SHA256: shaHex(data), Signers: e.signer()})
	lock, _ := LoadLock(e.lockPath)
	shEnv := []string{"SSHPOCKET_COMPONENTS_LOCK=" + e.lockPath, "SSHPOCKET_COMPONENTS_DIR=" + e.compDir}
	libs := []string{"Components.sh", "ComponentsInstall.sh", "ComponentsManifest.sh"}
	// çıkış kodu + (ileti) karşılaştırması; bildirim çıktısı yalnızca başarıda kıyaslanmaz (manifest paketi ayrı sınanır)
	check := func(label, id string) {
		t.Helper()
		sh := runShell(t, root, shEnv, libs, `comp_manifest_info() { return 0; }`, "comp_prepare", id)
		_, mf, err := Prepare(e.compDir, lock, id)
		if (sh.code == 0) != (err == nil) {
			t.Errorf("%s: kabuk=%d (%q) go=%v", label, sh.code, sh.stderr, err)
			return
		}
		if err != nil && strings.TrimSpace(sh.stderr) != err.Error() {
			t.Errorf("%s: ileti kabuk=%q go=%q", label, strings.TrimSpace(sh.stderr), err)
		}
		if err == nil && !strings.HasSuffix(mf, "abc/1.0.0/component.json") {
			t.Errorf("%s: yol %q", label, mf)
		}
	}
	check("kurulu değil", "abc")
	if _, err := InstallArchive(e.compDir, lock, "abc", "1.0.0", arc, sig, ""); err != nil {
		t.Fatal(err)
	}
	check("kurulu", "abc")
	check("kilitte yok", "baska")
	check("geçersiz", "ABC")
	os.WriteFile(filepath.Join(e.compDir, "abc/1.0.0/VERSION"), []byte("hacked"), 0o644)
	check("dosya değişmiş", "abc")
	// kilit tabanı kurulu sürümün üstüne çıkınca
	os.WriteFile(filepath.Join(e.compDir, "abc/1.0.0/VERSION"), []byte("1.0.0\n"), 0o644)
	e.writeLock(t, LockEntry{ID: "abc", Repo: "o/abc", Version: "1.5.0", SHA256: shaHex(data), Signers: e.signer()})
	lock, _ = LoadLock(e.lockPath)
	check("taban üstünde", "abc")
}
