package trust

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Gerçek yayın imzaları: bu depodaki imzalı sshpocket sürüm arşivleri ve gerçek paket indeksi,
// saf Go doğrulayıcıyla DOĞRULANMALI (ve ssh-keygen ile aynı sonucu vermeli).

func TestRealReleaseSignaturesVerify(t *testing.T) {
	root := repoRoot(t)
	signers, err := os.ReadFile(filepath.Join(root, "release_signers"))
	if err != nil {
		t.Skip("release_signers yok")
	}
	archives, _ := filepath.Glob(filepath.Join(root, "dist/sshpocket-*.tar.gz"))
	if len(archives) == 0 {
		t.Skip("dist arşivi yok")
	}
	ok := 0
	for _, a := range archives {
		sig, err := os.ReadFile(a + ".sig")
		if err != nil {
			continue
		}
		data, _ := os.ReadFile(a)
		got := Verify(ParseAllowedSigners(string(signers)), "sshpocket-release", "sshpocket-release", sig, bytes.NewReader(data)) == nil
		if _, err := os.Stat("/usr/bin/ssh-keygen"); err == nil {
			ref := sshVerify(t, string(signers), "sshpocket-release", "sshpocket-release", sig, data)
			if ref != got {
				t.Errorf("%s: ssh-keygen=%v go=%v", filepath.Base(a), ref, got)
			}
		}
		if got {
			ok++
		}
		// bir bayt bozulursa reddedilmeli
		data[len(data)/2] ^= 1
		if Verify(ParseAllowedSigners(string(signers)), "sshpocket-release", "sshpocket-release", sig, bytes.NewReader(data)) == nil {
			t.Errorf("%s: bozulmuş arşiv kabul edildi", filepath.Base(a))
		}
	}
	t.Logf("%d/%d gerçek yayın arşivi saf Go ile doğrulandı", ok, len(archives))
	if ok == 0 {
		t.Error("hiçbir gerçek yayın doğrulanamadı")
	}
}

func TestRealIndexVerifiesAndParses(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "components/.index")
	if _, err := os.Stat(filepath.Join(dir, "index.json")); err != nil {
		t.Skip("gerçek indeks önbelleği yok")
	}
	c := &Client{ComponentsDir: filepath.Join(root, "components"), SignersFile: filepath.Join(root, "index_signers"), SSHPocketVer: ReadVersionFile(filepath.Join(root, "VERSION"))}
	if !c.Enabled() {
		t.Skip("indeks kapalı")
	}
	idx, err := c.LoadCached()
	if err != nil {
		// süre dolmuşsa saati geçmişe alıp imzayı yine de doğrula
		c.Now = func() int64 { return time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC).Unix() }
		idx, err = c.LoadCached()
	}
	if err != nil {
		t.Fatalf("gerçek indeks doğrulanamadı: %v", err)
	}
	lock, err := LoadLock(filepath.Join(root, "components.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, r := range c.Report(idx, lock) {
		kinds = append(kinds, r.ID+"="+r.Kind)
	}
	t.Logf("gerçek indeks seri %d, geçerlilik %s; kararlar: %s", idx.Serial, idx.ValidUntil, strings.Join(kinds, " "))
}
