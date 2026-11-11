package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTerminalCommand(t *testing.T) {
	got := terminalCommand("/Volumes/te st/it's/sshpocket", "/Volumes/te st/it's", "kurulum", "durum", "ubuntutest")
	want := `cd '/Volumes/te st/it'\''s' && '/Volumes/te st/it'\''s/sshpocket' components -root '/Volumes/te st/it'\''s' run kurulum durum ubuntutest`
	if got != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
}

func TestAppleScriptQuote(t *testing.T) {
	if got := appleScriptQuote(`a"b\c`); got != `"a\"b\\c"` {
		t.Fatalf("got %s", got)
	}
}

// AppleScript metni derlenebilir mi (çalıştırılmaz)
func TestAppleScriptCompiles(t *testing.T) {
	if _, err := exec.LookPath("osacompile"); err != nil {
		t.Skip("osacompile yok")
	}
	cmd := terminalCommand("/Volumes/te\"st/it's/sshpocket", "/Volumes/te\"st/it's", "kurulum", "durum", "ubuntutest")
	script := "tell application \"Terminal\" to activate\ntell application \"Terminal\" to do script " + appleScriptQuote(cmd)
	out := t.TempDir() + "/x.scpt"
	if b, err := exec.Command("osacompile", "-o", out, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("derlenemedi: %v %s", err, strings.TrimSpace(string(b)))
	}
}

func TestClassify(t *testing.T) {
	ok := []Prereq{{Action: "preview", OK: true}}
	cases := []struct {
		name          string
		a             ActionInfo
		run, term, wr bool
	}{
		{"salt okunur", ActionInfo{Applicable: true}, true, false, false},
		{"salt okunur sudo", ActionInfo{Applicable: true, RequiresSud: true}, false, true, false},
		{"yazar, önkoşul tamam", ActionInfo{Applicable: true, RequiresSud: true, Effects: []string{"changes-config"}, Requires: []string{"preview"}, Prereqs: ok, PrereqsOK: true}, false, false, true},
		{"yazar, önkoşul eksik", ActionInfo{Applicable: true, RequiresSud: true, Effects: []string{"changes-config"}, Requires: []string{"preview"}}, false, false, false},
		{"yazar, önkoşulsuz", ActionInfo{Applicable: true, Effects: []string{"installs-package"}, PrereqsOK: true}, false, false, true},
		{"yıkıcı, önkoşul tamam", ActionInfo{Applicable: true, RequiresSud: true, Effects: []string{"changes-config", "destructive"}, PrereqsOK: true}, false, false, true},
		{"yıkıcı, önkoşul eksik", ActionInfo{Applicable: true, RequiresSud: true, Effects: []string{"changes-config", "destructive"}}, false, false, false},
		{"yıkıcı, değişken eksik", ActionInfo{Applicable: true, Effects: []string{"destructive"}, PrereqsOK: true, MissingVars: []string{"v"}}, false, false, false},
		{"onay ister (salt okunur)", ActionInfo{Applicable: true, Confirm: "yes-no"}, false, false, false},
		{"salt okunur, önkoşul eski", ActionInfo{Applicable: true, Requires: []string{"x"}}, false, false, false},
		{"platform dışı", ActionInfo{Applicable: false}, false, false, false},
		{"eksik değişken", ActionInfo{Applicable: true, RequiresSud: true, MissingVars: []string{"x"}}, false, false, false},
		{"yazar ama değişken eksik", ActionInfo{Applicable: true, Effects: []string{"changes-config"}, PrereqsOK: true, MissingVars: []string{"v"}}, false, false, false},
	}
	for _, c := range cases {
		run, term, wr, _ := classify(c.a)
		if run != c.run || term != c.term || wr != c.wr {
			t.Errorf("%s: run=%v term=%v write=%v, beklenen %v %v %v", c.name, run, term, wr, c.run, c.term, c.wr)
		}
	}
}

func TestConfirmOK(t *testing.T) {
	typ := ActionInfo{Confirm: "type-server-name"}
	yn := ActionInfo{Confirm: "yes-no"}
	if !confirmOK(typ, "vm1", "vm1") || confirmOK(typ, "vm1", "VM1") || confirmOK(typ, "vm1", "") || confirmOK(typ, "vm1", "evet") {
		t.Error("type-server-name kuralı bozuk")
	}
	if !confirmOK(yn, "vm1", "evet") || confirmOK(yn, "vm1", "vm1") || confirmOK(yn, "vm1", "") {
		t.Error("yes-no kuralı bozuk")
	}
}

// ---- pty çalıştırıcı testleri (script gerekir; yoksa atlanır)

func testApp() *App { return &App{jobs: map[string]*Job{}, up: map[string]bool{}} }

func waitFor(t *testing.T, a *App, what string, d time.Duration, cond func() bool) {
	t.Helper()
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		a.jmu.Lock()
		ok := cond()
		a.jmu.Unlock()
		if ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("zaman aşımı: %s", what)
}

func TestPtyPasswordPromptAndEchoOff(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script yok")
	}
	a := testApp()
	job, err := a.newJob("s", "p", "a")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.spawn(job, t.TempDir(), []string{"bash", "-c", "read -s -p 'password: ' x; echo; echo got=${#x}"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, a, "istem", 5*time.Second, func() bool { return strings.Contains(string(job.buf), "password:") })
	// istem "\n" ile bitmiyor: satır satır okuma bunu kaçırırdı
	if strings.HasSuffix(string(job.buf), "\n") {
		t.Error("istem satır sonu olmadan gelmeli")
	}
	if _, err := job.stdin.Write([]byte("abc\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, a, "bitiş", 5*time.Second, func() bool { return job.Done })
	out := string(job.buf)
	if !strings.Contains(out, "got=3") {
		t.Errorf("parola ulaşmadı: %q", out)
	}
	if strings.Contains(out, "abc") {
		t.Errorf("parola çıktıda yankılandı: %q", out)
	}
	if job.Exit != 0 {
		t.Errorf("çıkış %d", job.Exit)
	}
}

func TestPtyCancel(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script yok")
	}
	a := testApp()
	job, _ := a.newJob("s", "p", "a")
	if err := a.spawn(job, t.TempDir(), []string{"bash", "-c", "echo basladi; sleep 60"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, a, "başlangıç", 5*time.Second, func() bool { return strings.Contains(string(job.buf), "basladi") })
	job.cancel()
	waitFor(t, a, "iptal sonrası bitiş", 8*time.Second, func() bool { return job.Done })
}

func TestSingleJobAtATime(t *testing.T) {
	a := testApp()
	if _, err := a.newJob("s", "p", "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.newJob("s", "p", "b"); err == nil {
		t.Error("ikinci eşzamanlı iş reddedilmeli")
	}
}

func TestValidUTF8Prefix(t *testing.T) {
	b := []byte("aş")[:2] // "ş" iki bayt, ikincisi kesildi
	if n := validUTF8Prefix(b); n != 1 {
		t.Errorf("n=%d, beklenen 1", n)
	}
	if n := validUTF8Prefix([]byte("abc")); n != 3 {
		t.Errorf("n=%d", n)
	}
}

func TestMaskSecrets(t *testing.T) {
	for _, in := range []string{"BECOME password: hunter2", "REDIS_PASSWORD=abc123", "token: xyz", "SECRET_KEY=k"} {
		if got := maskLine2(in); strings.Contains(got, "hunter2") || strings.Contains(got, "abc123") || strings.Contains(got, "xyz") || strings.HasSuffix(got, "=k") || !strings.Contains(got, "****") {
			t.Errorf("maskelenmedi: %q → %q", in, got)
		}
	}
	if got := maskLine2("TASK [Gathering Facts]"); got != "TASK [Gathering Facts]" {
		t.Errorf("ilgisiz satır bozuldu: %q", got)
	}
}

func TestSanitizePrompt(t *testing.T) {
	if got := sanitizePrompt("BECOME password:\x00\x1b[31m "); strings.ContainsAny(got, "\x00\x1b") {
		t.Errorf("kontrol karakteri kaldı: %q", got)
	}
	if got := sanitizePrompt(strings.Repeat("a", 500)); len([]rune(got)) != 160 {
		t.Errorf("uzunluk %d", len([]rune(got)))
	}
	if got := sanitizePrompt("  "); got != "Parola" {
		t.Errorf("boş istem: %q", got)
	}
}

// parola penceresi betiği derlenir mi (çalıştırılmaz: ekranda pencere açmamak için)
func TestAskpassScriptCompiles(t *testing.T) {
	if _, err := exec.LookPath("osacompile"); err != nil {
		t.Skip("osacompile yok")
	}
	out := t.TempDir() + "/a.scpt"
	for _, prompt := range []string{"BECOME password:", `kötü "tırnak" \ ve 'tek'`, "İştah çalışma ğüşöç"} {
		if b, err := exec.Command("osacompile", "-o", out, "-e", askpassScript(prompt)).CombinedOutput(); err != nil {
			t.Fatalf("derlenmedi (%q): %v %s", prompt, err, strings.TrimSpace(string(b)))
		}
	}
}

// ---- değişken düzenleme (motorun gerçek kütüphanesiyle; /Volumes/test yoksa atlanır)

func TestParseDescribe(t *testing.T) {
	out := "kurulum_yazilim\tenum\tno\t\"docker\"\t\t\t\tKurulacak yazılım.\tSeçim\t[\"docker\",\"nginx\"]\n" +
		"hardening_ssh_allow_users\tlist[string]\tno\t[]\t\t\t\tSSH kısıtı\tSSH\t\n" +
		"bozuk satır\n"
	l := parseDescribe(out)
	if len(l) != 2 {
		t.Fatalf("beklenen 2 değişken, gelen %d", len(l))
	}
	if l[0].Default != "docker" || len(l[0].Values) != 2 || l[0].Group != "Seçim" {
		t.Errorf("ilk değişken yanlış: %+v", l[0])
	}
	if l[1].Default != "" || l[1].Type != "list[string]" {
		t.Errorf("ikinci değişken yanlış: %+v", l[1])
	}
}

func TestJSONScalarText(t *testing.T) {
	for in, want := range map[string]string{`"x"`: "x", `[]`: "", `["a","b"]`: "a,b", `true`: "true", `300`: "300", `null`: "", ``: ""} {
		if got := jsonScalarText(in); got != want {
			t.Errorf("%q → %q, beklenen %q", in, got, want)
		}
	}
}

func TestVarsEndToEnd(t *testing.T) {
	if _, err := os.Stat("/Volumes/test/sistem/Helpers/component_run.py"); err != nil {
		t.Skip("USB motoru yok")
	}
	dir := t.TempDir()
	srvDir := dir + "/servers"
	if err := os.MkdirAll(srvDir+"/vm1", 0o755); err != nil {
		t.Fatal(err)
	}
	vf := srvDir + "/vm1/utility.vars"
	if err := os.WriteFile(vf, []byte("# elle yazılmış yorum\nutility_timezone=Europe/Istanbul\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{serversDir: srvDir, compDir: "/Volumes/test/components", jobs: map[string]*Job{}, up: map[string]bool{}}

	// geçerli yazma: yorum korunur, izin 0600
	if _, code, err := a.saveVar("vm1", "utility", "utility_locale", "en_US.UTF-8", "set", false); err != nil {
		t.Fatalf("geçerli yazma reddedildi (%d): %v", code, err)
	}
	b, _ := os.ReadFile(vf)
	if !strings.Contains(string(b), "# elle yazılmış yorum") || !strings.Contains(string(b), "utility_locale=en_US.UTF-8") || !strings.Contains(string(b), "utility_timezone=Europe/Istanbul") {
		t.Errorf("dosya yanlış: %q", b)
	}
	if st, _ := os.Stat(vf); st.Mode().Perm() != 0o600 {
		t.Errorf("izin %v, beklenen 0600", st.Mode().Perm())
	}
	// değiştirme tek satır bırakır
	if _, _, err := a.saveVar("vm1", "utility", "utility_timezone", "UTC", "set", false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(vf)
	if strings.Count(string(b), "utility_timezone=") != 1 || !strings.Contains(string(b), "utility_timezone=UTC") {
		t.Errorf("değiştirme bozuk: %q", b)
	}
	// geçersiz: yazılmaz
	before, _ := os.ReadFile(vf)
	if _, code, err := a.saveVar("vm1", "kurulum", "kurulum_yazilim", "oracle", "set", false); err == nil || code != 400 {
		t.Errorf("geçersiz değer reddedilmeli (kod %d, hata %v)", code, err)
	}
	// bilinmeyen değişken / kötü girdiler
	for _, c := range []struct{ name, val string }{{"yok_boyle_bir_sey", "x"}, {"utility_locale", "a\nb=c"}, {"UTILITY", "x"}, {"utility_locale", strings.Repeat("x", 1100)}} {
		if _, code, err := a.saveVar("vm1", "utility", c.name, c.val, "set", false); err == nil || code != 400 {
			t.Errorf("%q reddedilmeli (kod %d)", c.name, code)
		}
	}
	after, _ := os.ReadFile(vf)
	if string(before) != string(after) {
		t.Error("reddedilen istekler dosyayı değiştirdi")
	}
	// sunucu yok
	if _, code, err := a.saveVar("yok", "utility", "utility_locale", "x", "set", false); err == nil || code != 404 {
		t.Errorf("olmayan sunucu: kod %d", code)
	}
	// sıfırlama
	if _, _, err := a.saveVar("vm1", "utility", "utility_locale", "", "unset", false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(vf)
	if strings.Contains(string(b), "utility_locale") || !strings.Contains(string(b), "# elle yazılmış yorum") {
		t.Errorf("sıfırlama bozuk: %q", b)
	}
	// çalışan iş varken düzenleme yok
	a.running = &Job{}
	if _, code, err := a.saveVar("vm1", "utility", "utility_locale", "en_US.UTF-8", "set", false); err == nil || code != 409 {
		t.Errorf("çalışan iş varken reddedilmeli (kod %d)", code)
	}
}

func TestVarsWarnNeedsAck(t *testing.T) {
	if _, err := os.Stat("/Volumes/test/sistem/Helpers/component_run.py"); err != nil {
		t.Skip("USB motoru yok")
	}
	dir := t.TempDir()
	_ = os.MkdirAll(dir+"/servers/vm1", 0o755)
	a := &App{serversDir: dir + "/servers", compDir: "/Volumes/test/components", jobs: map[string]*Job{}, up: map[string]bool{}}
	// uyarı eşiğindeki değer: onay olmadan yazılmaz, onayla yazılır
	if w, code, err := a.saveVar("vm1", "linux-hardening", "hardening_ssh_disable_password", "false", "set", false); err == nil || code != 409 || w == "" {
		t.Skipf("bu değişken uyarı üretmiyor olabilir (kod %d, uyarı %q, hata %v)", code, w, err)
	}
	if _, err := os.Stat(dir + "/servers/vm1/linux-hardening.vars"); err == nil {
		t.Error("onaysız uyarılı değer yazıldı")
	}
	if _, _, err := a.saveVar("vm1", "linux-hardening", "hardening_ssh_disable_password", "false", "set", true); err != nil {
		t.Fatalf("onayla yazılmalıydı: %v", err)
	}
}

// Gerçek imzalı indeks (USB önbelleği) geçici bir motor ağacında Go güven katmanıyla doğrulanır; USB'ye yazılmaz.
func TestCheckUpdatesWithRealIndex(t *testing.T) {
	src := "/Volumes/test"
	if _, err := os.Stat(src + "/components/.index/index.json"); err != nil {
		t.Skip("USB indeksi yok")
	}
	tmp := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	signersSrc := src + "/index_signers" // USB eski düzende olabilir: sistem/index_signers
	if _, err := os.Stat(signersSrc); err != nil {
		signersSrc = src + "/sistem/index_signers"
	}
	for _, f := range [][2]string{{signersSrc, tmp + "/index_signers"}, {src + "/components.lock", tmp + "/components.lock"}, {src + "/VERSION", tmp + "/VERSION"}} {
		b, err := os.ReadFile(f[0])
		must(err)
		must(os.WriteFile(f[1], b, 0o600))
	}
	for id, ver := range map[string]string{"utility": "0.1.0", "kurulum": "0.1.0"} {
		must(os.MkdirAll(tmp+"/components/"+id, 0o755))
		must(os.WriteFile(tmp+"/components/"+id+"/current", []byte(ver+"\n"), 0o644))
	}
	a := &App{compDir: tmp + "/components", jobs: map[string]*Job{}}
	a.idxDL = func(url, dest string, max int64) error {
		b, err := os.ReadFile(src + "/components/.index/" + filepath.Base(url))
		if err != nil {
			return err
		}
		return os.WriteFile(dest, b, 0o600)
	}
	rep, err := a.checkUpdates()
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK || rep.Serial == "" || len(rep.Rows) == 0 {
		t.Fatalf("rapor boş: %+v", rep)
	}
	byID := map[string]UpdateRow{}
	for _, r := range rep.Rows {
		byID[r.ID] = r
	}
	if r, ok := byID["kurulum"]; !ok || r.Verdict != "current" {
		t.Errorf("kurulum: %+v", r)
	}
	if r, ok := byID["linux-hardening"]; !ok || r.Installed != "-" {
		t.Errorf("kurulu olmayan paket '-' olmalı: %+v", r)
	}
	// aynı indeks yeniden: kabul; USB'deki durum dosyası değişmedi
	if _, err := a.checkUpdates(); err != nil {
		t.Fatal(err)
	}
	// imza bozulursa reddedilir ve durum ilerlemez
	before, _ := os.ReadFile(tmp + "/components/.index/state")
	a.idxDL = func(url, dest string, max int64) error {
		b, err := os.ReadFile(src + "/components/.index/" + filepath.Base(url))
		if err != nil {
			return err
		}
		if strings.HasSuffix(url, "index.json") {
			b = bytes.Replace(b, []byte("0.1.2"), []byte("9.9.9"), 1)
		}
		return os.WriteFile(dest, b, 0o600)
	}
	bad, _ := a.checkUpdates()
	after, _ := os.ReadFile(tmp + "/components/.index/state")
	if bad.OK || bad.Exit == 0 || !strings.Contains(bad.Raw, "imza") || string(before) != string(after) {
		t.Errorf("bozuk indeks reddedilmeliydi: %+v", bad)
	}
}

// ---- sunucu ekleme (motorun gerçek işlevleri; USB yoksa atlanır)

func serverTestApp(t *testing.T) *App {
	if _, err := os.Stat("/Volumes/test/sistem/Helpers/ModuleFiles.sh"); err != nil {
		t.Skip("USB motoru yok")
	}
	return &App{serversDir: t.TempDir() + "/servers", compDir: "/Volumes/test/components", jobs: map[string]*Job{}, up: map[string]bool{}}
}

func TestCreateServer(t *testing.T) {
	a := serverTestApp(t)
	if code, err := a.createServer("yeni_vm", "192.0.2.10", "admin", "2222", "ubuntu-26.04"); err != nil {
		t.Fatalf("oluşturulamadı (%d): %v", code, err)
	}
	b, _ := os.ReadFile(a.serversDir + "/yeni_vm/server.env")
	for _, want := range []string{"IP=192.0.2.10", "USER=admin", "PORT=2222", "OS=ubuntu-26.04"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("server.env'de %q yok: %q", want, b)
		}
	}
	if st, _ := os.Stat(a.serversDir + "/yeni_vm"); st.Mode().Perm() != 0o700 {
		t.Errorf("dizin izni %v", st.Mode().Perm())
	}
	if st, _ := os.Stat(a.serversDir + "/yeni_vm/server.env"); st.Mode().Perm() != 0o600 {
		t.Errorf("dosya izni %v", st.Mode().Perm())
	}
	// var olanın üzerine yazılmaz
	if code, err := a.createServer("yeni_vm", "192.0.2.99", "x", "22", ""); err == nil || code != 409 {
		t.Errorf("yinelenen ad reddedilmeli (kod %d)", code)
	}
	if b2, _ := os.ReadFile(a.serversDir + "/yeni_vm/server.env"); string(b2) != string(b) {
		t.Error("var olan sunucu dosyası değişti")
	}
	// kötü girdiler
	for _, c := range [][5]string{{"Büyük", "a", "u", "22", ""}, {"ok1", "a b", "u", "22", ""}, {"ok2", "a", "-x", "22", ""}, {"ok3", "a", "u", "70000", ""}, {"ok4", "a", "u", "22", "windows"}, {"../x", "a", "u", "22", ""}} {
		if code, err := a.createServer(c[0], c[1], c[2], c[3], c[4]); err == nil || code != 400 {
			t.Errorf("%v reddedilmeli (kod %d)", c, code)
		}
	}
	if ents, _ := os.ReadDir(a.serversDir); len(ents) != 1 {
		t.Errorf("reddedilen istekler dizin bıraktı: %d girdi", len(ents))
	}
}

// gerçek ssh-keygen: pty'de parola istemleri (iki kez) boş yanıtlanır; anahtar oluşur, 0600; ikinci deneme üzerine yazmaz
func TestKeygenTask(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen yok")
	}
	a := serverTestApp(t)
	if _, err := a.createServer("kvm", "192.0.2.10", "admin", "22", ""); err != nil {
		t.Fatal(err)
	}
	job, code, err := a.startServerTask("keygen", "kvm")
	if err != nil {
		t.Fatalf("başlamadı (%d): %v", code, err)
	}
	for i := 0; i < 2; i++ { // "Enter passphrase" ve "Enter same passphrase again"
		waitFor(t, a, "parola istemi", 8*time.Second, func() bool {
			return strings.Count(strings.ToLower(string(job.buf)), "passphrase") > i && !strings.HasSuffix(string(job.buf), "\n")
		})
		a.jmu.Lock()
		n := len(job.buf)
		a.jmu.Unlock()
		if _, err := job.stdin.Write([]byte("\n")); err != nil {
			t.Fatal(err)
		}
		waitFor(t, a, "yanıt işlendi", 8*time.Second, func() bool { return len(job.buf) > n || job.Done })
	}
	waitFor(t, a, "bitiş", 15*time.Second, func() bool { return job.Done })
	if job.Exit != 0 {
		t.Fatalf("ssh-keygen çıkış %d: %s", job.Exit, job.buf)
	}
	key := a.serversDir + "/kvm/id_ed25519_kvm"
	if st, err := os.Stat(key); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("özel anahtar yok ya da izin yanlış: %v", err)
	}
	if _, err := os.Stat(key + ".pub"); err != nil {
		t.Error("genel anahtar yok")
	}
	before, _ := os.ReadFile(key)
	a.running = nil // ilk iş bitti
	if _, code, err := a.startServerTask("keygen", "kvm"); err == nil || code != 409 {
		t.Errorf("var olan anahtarın üzerine yazılmamalı (kod %d)", code)
	}
	if after, _ := os.ReadFile(key); string(before) != string(after) {
		t.Error("anahtar değişti")
	}
	if _, code, _ := a.startServerTask("bilinmeyen", "kvm"); code != 400 {
		t.Errorf("bilinmeyen adım: kod %d", code)
	}
	if _, code, _ := a.startServerTask("keycopy", "yok_boyle"); code != 404 {
		t.Errorf("olmayan sunucu: kod %d", code)
	}
}

// ---- silme ve yeniden adlandırma (motorun gerçek ModuleManager.sh betiği)

func TestRenameAndRemoveServer(t *testing.T) {
	a := serverTestApp(t)
	a.stateDir = t.TempDir()
	if _, err := os.Stat("/Volumes/test/sistem/ModuleManager.sh"); err != nil {
		t.Skip("ModuleManager.sh yok")
	}
	if _, err := a.createServer("eski_ad", "192.0.2.10", "admin", "22", ""); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(a.serversDir+"/eski_ad/id_ed25519_eski_ad", []byte("GIZLI"), 0o600)
	_ = os.WriteFile(a.serversDir+"/eski_ad/id_ed25519_eski_ad.pub", []byte("PUB"), 0o644)
	_ = os.MkdirAll(a.stateDir+"/utility/eski_ad", 0o700)
	_ = os.WriteFile(a.stateDir+"/utility/eski_ad/results", []byte("1|0.1.2|durum|ok\n"), 0o600)
	if _, err := a.createServer("baska", "192.0.2.11", "admin", "22", ""); err != nil {
		t.Fatal(err)
	}

	// kötü istekler
	for _, c := range [][2]string{{"eski_ad", "eski_ad"}, {"eski_ad", "baska"}, {"eski_ad", "Büyük"}, {"eski_ad", "../x"}, {"yok_boyle", "yeni_ad"}, {"../x", "yeni_ad"}} {
		if code, err := a.renameServer(c[0], c[1]); err == nil || code < 400 {
			t.Errorf("%v reddedilmeli (kod %d)", c, code)
		}
	}
	if _, err := os.Stat(a.serversDir + "/eski_ad/server.env"); err != nil {
		t.Fatal("reddedilen istekler sunucuyu bozdu")
	}
	// başarılı yeniden adlandırma
	if code, err := a.renameServer("eski_ad", "yeni_ad"); err != nil {
		t.Fatalf("yeniden adlandırılamadı (%d): %v", code, err)
	}
	for _, f := range []string{"server.env", "id_ed25519_yeni_ad", "id_ed25519_yeni_ad.pub"} {
		if _, err := os.Stat(a.serversDir + "/yeni_ad/" + f); err != nil {
			t.Errorf("%s yeni klasörde yok", f)
		}
	}
	if _, err := os.Stat(a.serversDir + "/eski_ad"); err == nil {
		t.Error("eski klasör duruyor")
	}
	if _, err := os.Stat(a.stateDir + "/utility/yeni_ad/results"); err != nil {
		t.Error("durum/günlük klasörü taşınmadı")
	}
	if _, err := os.Stat(a.stateDir + "/utility/eski_ad"); err == nil {
		t.Error("eski durum klasörü duruyor")
	}
	// çalışan iş varken yeniden adlandırma ve silme yok
	a.running = &Job{}
	if code, err := a.renameServer("yeni_ad", "x_ad"); err == nil || code != 409 {
		t.Errorf("çalışan iş varken reddedilmeli (kod %d)", code)
	}
	if _, code, err := a.removeServer("yeni_ad", "yeni_ad", true); err == nil || code != 409 {
		t.Errorf("çalışan iş varken silme reddedilmeli (kod %d)", code)
	}
	a.running = nil

	// silme: onay şartları
	files, state, err := a.serverFiles("yeni_ad")
	if err != nil || len(files) != 3 || len(state) != 1 {
		t.Fatalf("dosya listesi yanlış: %v %v %v", files, state, err)
	}
	if _, code, err := a.removeServer("yeni_ad", "yanlis", true); err == nil || code != 400 {
		t.Errorf("yanlış ad reddedilmeli (kod %d)", code)
	}
	if _, code, err := a.removeServer("yeni_ad", "yeni_ad", false); err == nil || code != 400 {
		t.Errorf("anahtar onayı olmadan reddedilmeli (kod %d)", code)
	}
	if _, err := os.Stat(a.serversDir + "/yeni_ad"); err != nil {
		t.Fatal("reddedilen silme isteği sunucuyu sildi")
	}
	// başarılı silme (motorun ad sorusu otomatik yanıtlanır)
	job, code, err := a.removeServer("yeni_ad", "yeni_ad", true)
	if err != nil {
		t.Fatalf("silme başlamadı (%d): %v", code, err)
	}
	waitFor(t, a, "silme bitişi", 20*time.Second, func() bool { return job.Done })
	if job.Exit != 0 {
		t.Fatalf("silme çıkışı %d: %s", job.Exit, job.buf)
	}
	if _, err := os.Stat(a.serversDir + "/yeni_ad"); err == nil {
		t.Error("sunucu klasörü silinmedi")
	}
	if _, err := os.Stat(a.serversDir + "/baska/server.env"); err != nil {
		t.Error("başka sunucu etkilendi")
	}
	// günlükler kalır; purge yalnızca sunucu yokken çalışır
	if _, err := os.Stat(a.stateDir + "/utility/yeni_ad/results"); err != nil {
		t.Error("günlükler kendiliğinden silindi")
	}
	if code, err := a.purgeServerState("baska"); err == nil || code != 409 {
		t.Errorf("var olan sunucunun günlüğü silinmemeli (kod %d)", code)
	}
	if _, err := a.purgeServerState("yeni_ad"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.stateDir + "/utility/yeni_ad"); err == nil {
		t.Error("günlükler silinmedi")
	}
}

// yıkıcı eylem: sunucu adı birebir + geri alınamama onayı + snapshot; üçü de sunucu tarafında zorunlu
func TestAuthorizeDestructive(t *testing.T) {
	if _, err := os.Stat("/Volumes/test/sistem/Helpers/component_run.py"); err != nil {
		t.Skip("USB motoru yok")
	}
	// kurulum yukselt: yıkıcı + type-server-name; önkoşul (preview) taze olmalı → test dizinine taze sonuç yaz
	dir := t.TempDir()
	srvDir := dir + "/servers"
	st := dir + "/state"
	_ = os.MkdirAll(srvDir+"/vm1", 0o755)
	_ = os.WriteFile(srvDir+"/vm1/server.env", []byte("IP=192.0.2.10\nUSER=admin\nPORT=22\n"), 0o600)
	_ = os.WriteFile(srvDir+"/vm1/kurulum.vars", []byte("kurulum_yazilim=redis\nkurulum_mod=docker\n"), 0o600)
	_ = os.MkdirAll(st+"/kurulum/vm1", 0o700)
	_ = os.WriteFile(st+"/kurulum/vm1/results", []byte(fmt.Sprintf("%d|0.1.0|preview|ok\n", time.Now().Unix())), 0o600)
	a := &App{serversDir: srvDir, compDir: "/Volumes/test/components", stateDir: st, jobs: map[string]*Job{}, up: map[string]bool{}}

	ai, err := a.findAction("vm1", "kurulum", "yukselt")
	if err != nil || !ai.Destructive || !ai.Write {
		t.Fatalf("yukselt yıkıcı yazan eylem olmalı: %+v %v", ai, err)
	}
	cases := []struct {
		name     string
		snap     bool
		confirm  string
		ack      bool
		wantCode int
	}{
		{"snapshot yok", false, "vm1", true, 400},
		{"yanlış ad", true, "VM1", true, 400},
		{"ad boş", true, "", true, 400},
		{"geri alınamama onayı yok", true, "vm1", false, 400},
		{"evet yetmez", true, "evet", true, 400},
		{"hepsi tamam", true, "vm1", true, 0},
	}
	for _, c := range cases {
		_, code, err := a.authorizeRun("vm1", "kurulum", "yukselt", c.snap, c.confirm, c.ack)
		if c.wantCode == 0 && err != nil {
			t.Errorf("%s: kabul edilmeliydi: %v", c.name, err)
		}
		if c.wantCode != 0 && (err == nil || code != c.wantCode) {
			t.Errorf("%s: reddedilmeliydi (kod %d, hata %v)", c.name, code, err)
		}
	}
	// önkoşul bayatlayınca yıkıcı eylem de kapanır
	_ = os.WriteFile(st+"/kurulum/vm1/results", []byte(fmt.Sprintf("%d|0.1.0|preview|ok\n", time.Now().Add(-2*time.Hour).Unix())), 0o600)
	if _, code, err := a.authorizeRun("vm1", "kurulum", "yukselt", true, "vm1", true); err == nil || code != 409 {
		t.Errorf("bayat önkoşulla yıkıcı eylem reddedilmeli (kod %d, hata %v)", code, err)
	}
}

// Alt komutlar (keycopy, server-remove) test ikilisinin kendisiyle pty altında çalışır.
func TestMain(m *testing.M) {
	if code, ok := subcommand(os.Args[1:]); ok {
		os.Exit(code)
	}
	os.Exit(m.Run())
}
