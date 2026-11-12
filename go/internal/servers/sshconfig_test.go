package servers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var sshConfigs = []string{
	"Host web1\n  HostName 10.0.0.5\n  User root\n  Port 2222\n  IdentityFile ~/.ssh/id_web\n",
	"Host a b c\n  HostName h\n",
	"Host *\n  User x\nHost real\n  HostName r.example\n",
	"Host web?\n  HostName x\nHost !neg ok\n  HostName y\n",
	"host = lower\n  hostname=eq.example\n  USER = u\n  port=2200\n  identityfile=\"/k y\"\n",
	"Host m\n  HostName a\n  HostName b\n  User one\n  User two\n  IdentityFile k1\n  IdentityFile k2\n",
	"Host x\n  HostName h\nMatch host x\n  User ignored\n  HostName ignored\nHost y\n",
	"# yorum\n\n   # girintili yorum\nHost z\n\tHostName\tz.example\n\tPort\t22\n",
	"Host quoted\n  HostName \"q.example\"\n  User \"qu\"\n",
	"Host noval\n  HostName\n  User\n  HostName later\n",
	"Host crlf\r\n  HostName c.example\r\n  User u\r\n",
	"Host ü ç\n  HostName x\n",
	"Host   spaced   out  \n  HostName x\n",
	"  Include other\nHost i\n  HostName i\n",
	"Host last\n  HostName l",
	"",
	"Host a\nHost b\n  HostName only-b\n",
	"HostName orphan\nUser orphan\nHost p\n",
	"Host 'sq'\n  HostName s\n",
	"Host h1,h2\n  HostName c\n",
}

func TestParseSSHConfigMatchesShell(t *testing.T) {
	r := root(t)
	dir := t.TempDir()
	for i, c := range sshConfigs {
		f := filepath.Join(dir, fmt.Sprintf("c%d", i))
		os.WriteFile(f, []byte(c), 0o600)
		s := shellG(t, r, nil, "ssh_config_hosts", f)
		ents, err := ParseSSHConfig(f)
		if err != nil || s.code != 0 {
			t.Fatalf("durum %d: kabuk=%d %q go=%v", i, s.code, s.err, err)
		}
		var b strings.Builder
		for _, e := range ents {
			b.WriteString(e.Alias + "|" + e.Host + "|" + e.User + "|" + e.Port + "|" + e.IdentityFile + "\n")
		}
		if b.String() != s.out {
			t.Errorf("durum %d (%q):\nkabuk=%q\ngo=   %q", i, c, s.out, b.String())
		}
	}
	if _, err := ParseSSHConfig(filepath.Join(dir, "yok")); err == nil {
		t.Error("olmayan dosya hata vermedi")
	}
}

func TestModuleNameFromAliasMatchesShell(t *testing.T) {
	r := root(t)
	for _, a := range []string{"web1", "Web-1", "1web", "_x", "a.b", "ü", "A B", "", "web_01", "ÇĞ", "x-y.z", "9"} {
		s := shellG(t, r, nil, "ssh_config_module_name", a)
		if got := ModuleNameFromAlias(a); got != s.out {
			t.Errorf("%q: kabuk=%q go=%q", a, s.out, got)
		}
	}
}

func TestImportHostsMatchesShell(t *testing.T) {
	r := root(t)
	t.Setenv("USER", "testkullanici") // sabit kullanıcı: altın dosyada kişisel ad olmasın, CI'da da aynı
	cfg := filepath.Join(t.TempDir(), "config")
	os.WriteFile(cfg, []byte("Host web1\n  HostName 10.0.0.5\n  User root\n  IdentityFile ~/k\nHost mevcut\n  HostName 10.0.0.6\nHost bozuk\n  HostName a b\nHost Yeni-Ad\n  HostName y.example\n  Port 2200\n"), 0o600)
	d1, d2 := t.TempDir(), t.TempDir()
	for _, d := range []string{d1, d2} {
		os.MkdirAll(filepath.Join(d, "mevcut"), 0o700)
	}
	// kabuk betiği etkileşimli seçim sorar: numaralar "1,2,3,4" stdin'den (ok tuşları kapalı: boruda ui_arrows_enabled yanlış)
	shTree := goldDo(t, func() string {
		cmd := execCommand(r, d1, cfg)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("kabuk: %v\n%s", err, out)
		}
		return tree(t, d1)
	})
	ents, _ := ParseSSHConfig(cfg)
	res := ImportHosts(d2, ents, os.Getenv("USER"))
	if len(res) != 4 || !res[1].Skipped || res[2].Err == nil || res[0].Err != nil || res[3].Err != nil {
		t.Fatalf("sonuçlar beklenmedik: %+v", res)
	}
	if a, b := shTree, tree(t, d2); a != b {
		t.Errorf("ağaç farklı\nkabuk: %s\ngo:    %s", a, b)
	}
}
