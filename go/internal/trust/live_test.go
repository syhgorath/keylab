package trust

import (
	"os"
	"path/filepath"
	"testing"
)

// Canlı deneme (yalnızca SSHPOCKET_LIVE=1): GERÇEK imzalı indeksi GitHub'dan HTTPS ile alır ve geçici klasöre yazar.
func TestLiveRefresh(t *testing.T) {
	if os.Getenv("SSHPOCKET_LIVE") != "1" {
		t.Skip("SSHPOCKET_LIVE=1 değil")
	}
	root := repoRoot(t)
	c := &Client{ComponentsDir: t.TempDir(), SignersFile: filepath.Join(root, "index_signers"), SSHPocketVer: ReadVersionFile(filepath.Join(root, "VERSION"))}
	idx, err := c.Refresh()
	if err != nil {
		t.Fatalf("canlı yenileme: %v", err)
	}
	lock, _ := LoadLock(filepath.Join(root, "components.lock"))
	for _, r := range c.Report(idx, lock) {
		t.Logf("%s kurulu=%q karar=%s %s", r.ID, r.Installed, r.Kind, r.Detail)
	}
	t.Logf("seri %d geçerlilik %s", idx.Serial, idx.ValidUntil)
}
