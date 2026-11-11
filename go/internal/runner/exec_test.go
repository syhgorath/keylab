package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProgressParseMatchesShell(t *testing.T) {
	root := repoRoot(t)
	logs := map[string]string{
		"yok":                     "",
		"ilerleme yok":            "ansible çıktısı\nTASK [x]\n",
		"temel":                   "SSHPOCKET_PROGRESS alan=base sira=1/3 durum=basladi\nSSHPOCKET_PROGRESS alan=base sira=1/3 durum=bitti degisen=2 atlanan=1 kesin=yes uyari=0\n",
		"yalnız başladı":          "SSHPOCKET_PROGRESS alan=ssh sira=2/3 durum=basladi bekle_tahmin=300\n",
		"üç alan":                 "SSHPOCKET_PROGRESS alan=a sira=1/3 durum=bitti degisen=1\nSSHPOCKET_PROGRESS alan=b sira=2/3 durum=basladi\nSSHPOCKET_PROGRESS alan=c sira=3/3 durum=basladi bekle_tahmin=12\n",
		"bittiden sonra başladı":  "SSHPOCKET_PROGRESS alan=a sira=1/2 durum=bitti degisen=5\nSSHPOCKET_PROGRESS alan=a sira=1/2 durum=basladi\n",
		"iki kez bitti":           "SSHPOCKET_PROGRESS alan=a sira=1/2 durum=bitti degisen=5\nSSHPOCKET_PROGRESS alan=a sira=1/2 durum=bitti\n",
		"ansible msg":             "    \"msg\": \"SSHPOCKET_PROGRESS alan=base sira=1/3 durum=bitti degisen=2 atlanan=0\"\n}\n",
		"tırnak ve kaçış":         "ok: [h] => {\"msg\": \"SSHPOCKET_PROGRESS alan=base sira=1/3 durum=basladi\\\"}\"}\n",
		"crlf":                    "SSHPOCKET_PROGRESS alan=base sira=1/3 durum=bitti degisen=2\r\n",
		"bozuk alan":              "SSHPOCKET_PROGRESS alan=BASE sira=1/3 durum=bitti\nSSHPOCKET_PROGRESS alan=1x sira=1/3 durum=bitti\n",
		"bozuk durum":             "SSHPOCKET_PROGRESS alan=a sira=1/3 durum=bitmedi\n",
		"bozuk sıra":              "SSHPOCKET_PROGRESS alan=a sira=1-3 durum=bitti\nSSHPOCKET_PROGRESS alan=a sira=x/3 durum=bitti\n",
		"sıra eksik":              "SSHPOCKET_PROGRESS alan=a durum=bitti\n",
		"bilinmeyen anahtar":      "SSHPOCKET_PROGRESS alan=a sira=1/3 durum=bitti ekstra=1 degisen=7\n",
		"bekle bozuk":             "SSHPOCKET_PROGRESS alan=a sira=1/3 durum=basladi bekle_tahmin=abc\n",
		"yinelenen anahtar":       "SSHPOCKET_PROGRESS alan=a alan=b sira=1/3 durum=bitti\n",
		"sekme":                   "SSHPOCKET_PROGRESS\talan=a\tsira=1/3\tdurum=bitti\tdegisen=3\n",
		"satır başında boşluk":    "   SSHPOCKET_PROGRESS alan=a sira=1/3 durum=bitti degisen=1\n",
		"iki işaret aynı satırda": "SSHPOCKET_PROGRESS alan=a sira=1/3 durum=bitti SSHPOCKET_PROGRESS alan=b sira=2/3 durum=bitti\n",
		"boş değer":               "SSHPOCKET_PROGRESS alan=a sira=1/3 durum=bitti degisen= atlanan=2\n",
		"büyük sıra":              "SSHPOCKET_PROGRESS alan=a sira=10/12 durum=bitti degisen=0 uyari=3 kesin=no\n",
	}
	dir := t.TempDir()
	for name, content := range logs {
		f := filepath.Join(dir, "log")
		os.WriteFile(f, []byte(content), 0o600)
		sh := runShell(t, root, nil, "", "", "comp_progress_parse", f)
		rows, ok := ProgressParse(f)
		if (sh.code == 0) != ok {
			t.Errorf("[%s] başarı kabuk=%d go=%v", name, sh.code, ok)
			continue
		}
		var got []string
		for _, r := range rows {
			got = append(got, r.TSV())
		}
		if want := splitNL(sh.stdout); !reflect.DeepEqual(want, got) {
			t.Errorf("[%s]\n kabuk=%q\n go   =%q", name, want, got)
		}
	}
	if _, ok := ProgressParse(filepath.Join(dir, "olmayan")); ok {
		t.Error("olmayan dosya ok")
	}
}

func splitNL(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func TestParseResultMatchesShell(t *testing.T) {
	root := repoRoot(t)
	infos := realInfos(t, root)
	info := infos["kurulum/0.1.0"]
	// outputs.format olan eylem ara
	var act, keys string
	for rel, in := range infos {
		for _, a := range splitFields(strings.Join(allValues(in, "action"), " ")) {
			if k := actionGet(in, a, "outputs.keys"); k != "" && actionGet(in, a, "outputs.format") == "keyvalue-line" {
				info, act, keys = in, a, k
				_ = rel
				goto found
			}
		}
	}
found:
	if act == "" {
		t.Skip("outputs.format'lı eylem yok")
	}
	id := infoGet(info, "id")
	k1 := strings.Split(keys, ",")[0]
	pfx := "SSHPOCKET_RESULT " + id + "." + act
	logs := map[string]string{
		"yok":                "",
		"satır yok":          "bir şey\n",
		"geçerli":            pfx + " " + k1 + "=ok\n",
		"boş değerli":        pfx + " " + k1 + "=\n",
		"önek yok":           "SSHPOCKET_RESULT baska.eylem " + k1 + "=ok\n",
		"sadece işaret":      "SSHPOCKET_RESULT\n",
		"bilinmeyen anahtar": pfx + " zzz=1\n",
		"yinelenen":          pfx + " " + k1 + "=a " + k1 + "=b\n",
		"= yok":              pfx + " " + k1 + "\n",
		"kötü değer":         pfx + " " + k1 + "=a$b\n",
		"uzun değer":         pfx + " " + k1 + "=" + strings.Repeat("a", 65) + "\n",
		"64 değer":           pfx + " " + k1 + "=" + strings.Repeat("a", 64) + "\n",
		"son satır kazanır":  pfx + " " + k1 + "=ilk\n" + pfx + " " + k1 + "=son\n",
		"girintili":          "    \"msg\": \"" + pfx + " " + k1 + "=ok\"\n",
		"kaçış \\n":          "msg: " + pfx + " " + k1 + "=ok\\nsonra=1\n",
		"tek tırnak":         "msg: '" + pfx + " " + k1 + "=ok' ve devam\n",
		"crlf":               pfx + " " + k1 + "=ok\r\n",
		"önünde metin":       "komut çıktısı " + pfx + " " + k1 + "=ok\n",
		"iki işaret":         pfx + " " + k1 + "=ok SSHPOCKET_RESULT x.y\n",
		"sekme ayırıcı":      pfx + "\t" + k1 + "=ok\n",
		"çok boşluk":         pfx + "    " + k1 + "=ok\n",
		"+ . : -":            pfx + " " + k1 + "=a+b.c:d-e_f\n",
	}
	dir := t.TempDir()
	prel := `thr() { comp_exec_parse_result "$@"; local rc=$?; printf '%s' "$COMP_EXEC_RESULT" >&3; return $rc; }`
	_ = prel
	for name, content := range logs {
		f := filepath.Join(dir, "log")
		os.WriteFile(f, []byte(content), 0o600)
		sh := runShell(t, root, nil, "", `thr() { comp_exec_parse_result "$@"; local rc=$?; printf '%s' "$COMP_EXEC_RESULT"; return $rc; }`, "thr", strings.Join(info, "\n"), act, f)
		got, err := ParseResult(info, act, f)
		if (sh.code == 0) != (err == nil) {
			t.Errorf("[%s] başarı kabuk=%d (%q) go=%v", name, sh.code, sh.stderr, err)
			continue
		}
		if err != nil {
			if strings.TrimSpace(sh.stderr) != err.Error() {
				t.Errorf("[%s] ileti kabuk=%q go=%q", name, strings.TrimSpace(sh.stderr), err)
			}
			continue
		}
		if want := splitNL(sh.stdout); !reflect.DeepEqual(want, got) {
			t.Errorf("[%s] kabuk=%q go=%q", name, want, got)
		}
	}
	// outputs.format olmayan eylem
	for _, a := range splitFields(strings.Join(allValues(info, "action"), " ")) {
		if actionGet(info, a, "outputs.format") == "" {
			got, err := ParseResult(info, a, filepath.Join(dir, "log"))
			if got != nil || err != nil {
				t.Errorf("%s: format yok ama sonuç/hata: %v %v", a, got, err)
			}
			break
		}
	}
}

func TestSaveLastResultMatchShell(t *testing.T) {
	root := repoRoot(t)
	shDir, goDir := t.TempDir(), t.TempDir()
	for _, c := range []struct {
		act string
		res []string
	}{{"preview", []string{"a=1", "b=2"}}, {"apply", nil}, {"BAD", []string{"a=1"}}, {"x_y", []string{"k=v"}}} {
		sh := runShell(t, root, []string{"RES=" + strings.Join(c.res, "\n")}, "", `ths() { COMP_EXEC_RESULT="$RES"; comp_exec_save_result "$@"; }`, "ths", shDir, c.act)
		err := SaveResult(goDir, c.act, c.res, time.Now().Unix())
		if (sh.code == 0) != (err == nil) {
			t.Errorf("%v: kabuk=%d go=%v", c, sh.code, err)
		}
	}
	norm := func(d string) map[string]string {
		out := map[string]string{}
		ents, _ := os.ReadDir(d)
		for _, e := range ents {
			b, _ := os.ReadFile(filepath.Join(d, e.Name()))
			st, _ := os.Stat(filepath.Join(d, e.Name()))
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			lines[0] = "T"
			out[e.Name()] = fmt.Sprintf("%o %v", st.Mode().Perm(), lines)
		}
		return out
	}
	if a, b := norm(shDir), norm(goDir); !reflect.DeepEqual(a, b) {
		t.Errorf("sonuç dosyaları\n kabuk=%v\n go   =%v", a, b)
	}
	// LastResult: uzun dosya, geçersiz adlar
	state := t.TempDir()
	os.MkdirAll(filepath.Join(state, "pkg", "srv"), 0o700)
	var big []string
	for i := 0; i < 60; i++ {
		big = append(big, fmt.Sprintf("k%d=v", i))
	}
	os.WriteFile(filepath.Join(state, "pkg", "srv", "result.apply"), []byte("123\n"+strings.Join(big, "\n")+"\n"), 0o600)
	for _, c := range [][3]string{{"pkg", "srv", "apply"}, {"pkg", "srv", "yok"}, {"PKG", "srv", "apply"}, {"pkg", "-x", "apply"}, {"pkg", "srv", "A"}} {
		sh := runShell(t, root, []string{"SSHPOCKET_STATE_DIR=" + state}, "", "", "comp_exec_last_result", c[0], c[1], c[2])
		got, ok := LastResult(state, c[0], c[1], c[2])
		if (sh.code == 0) != ok || (ok && !reflect.DeepEqual(splitNL(sh.stdout), got)) {
			t.Errorf("%v: kabuk=%d %d satır, go=%v %d satır", c, sh.code, len(splitNL(sh.stdout)), ok, len(got))
		}
	}
}

func TestMaskMatchesShell(t *testing.T) {
	root := repoRoot(t)
	cases := map[string]string{
		"düz":                   "TASK [x]\nok: [h]\n",
		"become parola":         "BECOME password: hunter2\n",
		"küçük harf":            "sudo password: abc\nPassword=abc def\n",
		"anahtar sözcük":        "api_key = AKIA123\napi-key: x\napikey=y\n",
		"token":                 "token: abc123\nTOKEN=xyz\nmytoken: s3\n",
		"secret":                "client_secret: zzz\n",
		"parola":                "parola: gizli\n",
		"passwd":                "passwd=1234\n",
		"değersiz":              "password:\npassword: \n",
		"20 harf sınırı":        "password" + strings.Repeat("a", 20) + ": v\npassword" + strings.Repeat("a", 21) + ": v\n",
		"boşluklu":              "become method pass: v\n",
		"github":                "ghp_abcdefgh12345678 ve github_pat_ABCDEFGH1234 glpat-12345678 xoxb-12345678 xoxp-abcdefgh\n",
		"github kısa":           "ghp_abc\n",
		"github g":              "a ghp_12345678 b ghp_87654321 c\n",
		"özel anahtar":          "önce\n-----BEGIN OPENSSH PRIVATE KEY-----\nAAAA\nBBBB\n-----END OPENSSH PRIVATE KEY-----\nsonra\n",
		"rsa anahtar":           "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----\n",
		"iki anahtar":           "a\n-----BEGIN PRIVATE KEY-----\nx\n-----END PRIVATE KEY-----\nb\n-----BEGIN EC PRIVATE KEY-----\ny\n-----END EC PRIVATE KEY-----\nc\n",
		"anahtar içinde parola": "-----BEGIN PRIVATE KEY-----\npassword: x\n-----END PRIVATE KEY-----\n",
		"NUL":                   "pass\x00word: abc\nx\x00\n",
		"birden çok eşleşme":    "password: a token: b\n",
		"unicode":               "parola: şifre ğüş\ncomment: ünicode\n",
		"boşluk önce":           "   password: x\n",
		"iki nokta içinde":      "url: http://a:b@h/path\n",
		"sudo satırı":           "sudo: a password is required\n",
		"=:":                    "token=:x\n",
		"boş":                   "",
		"tek boş satır":         "\n",
		"boş satırlar":          "a\n\nb\n",
	}
	for name, in := range cases {
		sh := runShell(t, root, nil, in, "", "_comp_exec_mask")
		got := Mask(in)
		if sh.stdout != got {
			t.Errorf("[%s]\n kabuk=%q\n go   =%q", name, sh.stdout, got)
		}
	}
	// kapanmamış anahtar bloğu: bilinçli fark (Go sonuna kadar gizler ve not düşer)
	open := "a\n-----BEGIN PRIVATE KEY-----\nsecretdata\nmore\n"
	if g := Mask(open); strings.Contains(g, "secretdata") || !strings.Contains(g, "gizlendi") {
		t.Errorf("kapanmamış blok sızdı: %q", g)
	}
}

func TestArchiveLogMatchesShell(t *testing.T) {
	root := repoRoot(t)
	t0 := time.Now().Unix() - 7
	body := "komut: ansible-playbook -e password=hunter2 x.yml\nbirinci satır\nTASK [x]\nBECOME password: sifre\nSSHPOCKET_PROGRESS alan=a sira=1/1 durum=bitti\n"
	cases := []struct {
		name string
		last string
		rc   int
		keep int
	}{
		{"başarı", body, 0, 50}, {"başarısız", body, 2, 50}, {"INT", body, 130, 50}, {"TERM", body, 143, 50}, {"HUP", body, 129, 50},
		{"komut yok", "satır\n", 0, 50}, {"yalnız komut", "komut: x y\n", 1, 50}, {"boş", "", 0, 50},
	}
	for _, c := range cases {
		shS, goS := t.TempDir(), t.TempDir()
		for _, d := range []string{shS, goS} {
			os.WriteFile(filepath.Join(d, "last-run.log"), []byte(c.last), 0o600)
		}
		sh := runShell(t, root, []string{fmt.Sprintf("COMP_LOG_KEEP=%d", c.keep)}, "", `tha() { comp_exec_archive_log "$@"; echo "$COMP_EXEC_LOGFILE"; }`, "tha", shS, "abc", "1.2.3", "apply", "srv", fmt.Sprint(c.rc), fmt.Sprint(t0))
		p, err := ArchiveLog(goS, "abc", "1.2.3", "apply", "srv", c.rc, t0, time.Now().Unix(), c.keep)
		if (sh.code == 0) != (err == nil) {
			t.Errorf("[%s] başarı kabuk=%d go=%v", c.name, sh.code, err)
			continue
		}
		shPath := strings.TrimSpace(sh.stdout)
		if (shPath == "") != (p == "") {
			t.Errorf("[%s] yol kabuk=%q go=%q", c.name, shPath, p)
			continue
		}
		if p == "" {
			continue
		}
		a, _ := os.ReadFile(shPath)
		b, _ := os.ReadFile(p)
		na, nb := normDur(string(a)), normDur(string(b))
		if na != nb {
			t.Errorf("[%s] günlük farkı\n--kabuk--\n%s\n--go--\n%s", c.name, na, nb)
		}
		for _, f := range []string{shPath, p} {
			if st, _ := os.Stat(f); st.Mode().Perm() != 0o600 {
				t.Errorf("[%s] izin %o", c.name, st.Mode().Perm())
			}
			if st, _ := os.Stat(filepath.Dir(f)); st.Mode().Perm() != 0o700 {
				t.Errorf("[%s] dizin izni %o", c.name, st.Mode().Perm())
			}
		}
		// last-fail-log
		_, e1 := os.Stat(filepath.Join(shS, "last-fail-log"))
		_, e2 := os.Stat(filepath.Join(goS, "last-fail-log"))
		if (e1 == nil) != (e2 == nil) {
			t.Errorf("[%s] last-fail-log var mı: kabuk=%v go=%v", c.name, e1 == nil, e2 == nil)
		}
	}
	// ad çakışması (-2, -3) ve temizlik (keep)
	shS, goS := t.TempDir(), t.TempDir()
	for _, d := range []string{shS, goS} {
		os.WriteFile(filepath.Join(d, "last-run.log"), []byte("x\n"), 0o600)
	}
	for i := 0; i < 5; i++ {
		runShell(t, root, []string{"COMP_LOG_KEEP=3"}, "", "", "comp_exec_archive_log", shS, "abc", "1.2.3", "apply", "srv", "0", fmt.Sprint(t0))
		if _, err := ArchiveLog(goS, "abc", "1.2.3", "apply", "srv", 0, t0, time.Now().Unix(), 3); err != nil {
			t.Fatal(err)
		}
	}
	ls := func(d string) []string {
		ents, _ := os.ReadDir(filepath.Join(d, "logs"))
		var n []string
		for _, e := range ents {
			n = append(n, e.Name())
		}
		return n
	}
	if a, b := ls(shS), ls(goS); !reflect.DeepEqual(a, b) {
		t.Errorf("temizlik/çakışma: kabuk=%v go=%v", a, b)
	}
}

func normDur(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "# süre: ") {
			lines[i] = "# süre: N"
		}
	}
	return strings.Join(lines, "\n")
}

// --- küçük Info yardımcıları (testte manifest paketine bağımlılığı azaltmak için)

func infoGet(info []string, key string) string {
	for _, l := range info {
		if strings.HasPrefix(l, key+"=") {
			return l[len(key)+1:]
		}
	}
	return ""
}
func allValues(info []string, key string) []string {
	var out []string
	for _, l := range info {
		if strings.HasPrefix(l, key+"=") {
			out = append(out, l[len(key)+1:])
		}
	}
	return out
}
func actionGet(info []string, a, f string) string { return infoGet(info, "action."+a+"."+f) }
func splitFields(s string) []string               { return strings.Fields(s) }
