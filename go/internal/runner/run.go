package runner

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"sshpocket-go/internal/manifest"
)

// Refused: çalıştırılmadan reddedildi (kapı/değişken/onay/hazırlık).
const Refused = 190

// ExecCtx, bir eylem çalıştırmanın bağlamıdır (kabuktaki COMP_CTX_* değişkenleri).
type ExecCtx struct {
	PkgDir, BinDir, Server, Host, User, Port, Key, Prefix, Profile string
	ValuesFile                                                     string // boş: değer dosyası yok
	BecomeExe                                                      string // boş ya da /usr/bin/sudo.ws
	RunSet                                                         string // katalog seçimi: satır satır ad=değer
	Gates                                                          *Gates
	AddRetry                                                       func(prefix, key string) error
	KnownHosts                                                     string
	Ctx                                                            context.Context
	Stdin                                                          io.Reader // onaylar ve sudo istemi
	Stdout                                                         io.Writer // komut çıktısı
	Stderr                                                         io.Writer // bilgi/hata iletileri
	Now                                                            func() int64
	LogKeep                                                        int

	rd *bufio.Reader
}

func (ec *ExecCtx) values() string {
	if ec.ValuesFile == "" {
		return "/dev/null"
	}
	return ec.ValuesFile
}

func (ec *ExecCtx) say(format string, a ...any) { fmt.Fprintf(ec.Stderr, format, a...) }

// input, girdi okuyucusudur: onaylar ve alt sürecin (ansible/ssh) istemleri AYNI tamponlu okuyucudan okunur
// (kabukta tek dosya tanıtıcısı paylaşılır; tamponlanan veri alt sürece de iletilmeli).
func (ec *ExecCtx) input() *bufio.Reader {
	if ec.rd == nil {
		var in io.Reader = ec.Stdin
		if in == nil {
			in = strings.NewReader("")
		}
		ec.rd = bufio.NewReader(in)
	}
	return ec.rd
}

// ask: istemi Stderr'e yazar, girdiden bir satır okur; girdi yoksa (EOF) yeni satır yazıp ok=false döner.
func (ec *ExecCtx) ask(prompt string) (string, bool) {
	fmt.Fprint(ec.Stderr, prompt)
	line, err := ec.input().ReadString('\n')
	if err != nil {
		fmt.Fprintln(ec.Stderr)
		return "", false
	}
	return strings.TrimSuffix(line, "\n"), true
}

func (ec *ExecCtx) now() int64 {
	if ec.Now != nil {
		return ec.Now()
	}
	return time.Now().Unix()
}

func (ec *ExecCtx) ctx() context.Context {
	if ec.Ctx != nil {
		return ec.Ctx
	}
	return context.Background()
}

// refuse: ileti yazar ve Refused döner.
func (ec *ExecCtx) refuse(msg string) int {
	if msg != "" {
		fmt.Fprintln(ec.Stderr, msg)
	}
	return Refused
}

var reRemoteTmp = regexp.MustCompile(`^/tmp/sshpocket-[A-Za-z0-9]{8}$`)

const remoteScript = `d=$(mktemp -d) || exit 96; trap "rm -rf \"$d\"" EXIT; install -o root -g root -m 0700 -- "$1" "$d/s.sh" || exit 96; h=$(sha256sum "$d/s.sh" | cut -d" " -f1); [ "$h" = "$2" ] || { echo "ÖZET UYUŞMUYOR: betik çalıştırılmadı (sunucudaki kopya imzalı paketteki betikle aynı değil)." >&2; exit 97; }; shift 2; bash "$d/s.sh" "$@"`

// runTee, komutu çalıştırır; stdout+stderr birleşik olarak hem ec.Stdout'a hem günlüğe (log açıksa) yazılır. Çıkış kodu:
// sinyalle ölürse 128+sinyal, başlatılamazsa 127.
func (ec *ExecCtx) runTee(cmd *exec.Cmd, log io.Writer) int {
	var w io.Writer = ec.Stdout
	if w == nil {
		w = io.Discard
	}
	if log != nil {
		w = io.MultiWriter(w, log)
	}
	// Alt sürecin girdisi: girdi gerçek bir dosya tanıtıcısıysa (terminal) ve tamponda bekleyen veri yoksa DOĞRUDAN verilir.
	// Aksi halde Go, girdiyi aktaran bir goroutine açar ve Wait o goroutine terminalden bir satır okuyana kadar bekler: alt
	// süreç bitmiş olsa da koşu Enter'a basılana kadar takılırdı (gerçek VM denemesinde görüldü). Dosya değilse kendi
	// borumuzu kurarız; alt süreç bitince okuma ucu kapanır ve Wait beklemez.
	in := ec.input()
	var stdinCleanup func()
	if f, ok := ec.Stdin.(*os.File); ok && in.Buffered() == 0 {
		cmd.Stdin = f
	} else if pr, pw, err := os.Pipe(); err == nil {
		cmd.Stdin = pr
		go func() {
			_, _ = io.Copy(pw, in)
			pw.Close()
		}()
		stdinCleanup = func() { pr.Close() }
	} else {
		cmd.Stdin = in
	}
	cmd.Stdout, cmd.Stderr = w, w
	cmd.WaitDelay = 500 * time.Millisecond
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(w, "%v\n", err)
		return 127
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ec.ctx().Done():
			_ = cmd.Process.Signal(syscall.SIGTERM)
		case <-done:
		}
	}()
	_ = cmd.Wait()
	close(done)
	if stdinCleanup != nil {
		stdinCleanup()
	}
	ps := cmd.ProcessState
	if ps == nil {
		return 127
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ps.ExitCode()
}

func fileHash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

// ExecBody, kapılar → değişkenler → onay → çalıştırma (comp_exec_body). Dönüş: çıkış kodu (Refused: hiçbir şey çalışmadı).
// sdir: durum klasörü (last-run.log burada).
func (ec *ExecCtx) execBody(info []string, id, ver, action string) (rc int, sdir string) {
	runner := manifest.InfoGet(info, "runner")
	if st, err := os.Stat(filepath.Join(ec.PkgDir, "component.json")); err != nil || !st.Mode().IsRegular() {
		return ec.refuse("Paket dizini bulunamadı."), ""
	}
	switch runner {
	case "ansible":
		if st, err := os.Stat(ec.BinDir); ec.BinDir == "" || err != nil || !st.IsDir() {
			return ec.refuse("Python ortamı (venv) bulunamadı."), ""
		}
	case "script":
	default:
		return ec.refuse("Desteklenmeyen runner: " + runner), ""
	}

	// ---- kapılar ----
	if err := NotRoot(info, action, ec.User); err != nil {
		return ec.refuse(err.Error()), ""
	}
	if err := KnownHost(ec.Host, ec.Port, ec.KnownHosts); err != nil {
		return ec.refuse(err.Error()), ""
	}
	if err := Agent(ec.Prefix, ec.Key, ec.AddRetry); err != nil {
		return ec.refuse(err.Error()), ""
	}
	if err := Profile(info, ec.Profile); err != nil {
		return ec.refuse(err.Error()), ""
	}
	if err := ec.Gates.Requires(info, id, ec.Server, ver, action); err != nil {
		return ec.refuse(err.Error()), ""
	}
	if err := RequiresVars(info, action, ec.values()); err != nil {
		return ec.refuse(err.Error()), ""
	}

	sdir, err := ec.Gates.StateEnsure(id, ec.Server)
	if err != nil {
		return Refused, ""
	}
	work, err := os.MkdirTemp("", "sshpocket-run.")
	if err != nil {
		return ec.refuse("Geçici klasör oluşturulamadı."), sdir
	}
	os.Chmod(work, 0o700)
	defer os.RemoveAll(work)
	vf := filepath.Join(work, "vars.json")

	// ---- değişkenler: dir_var HER ZAMAN kullanıcı klasörüne yönlendirilir ----
	forced := map[string]string{}
	if dirVar := manifest.ActionGet(info, action, "outputs.dir_var"); dirVar != "" {
		outdir := filepath.Join(sdir, "out")
		if os.MkdirAll(outdir, 0o700) != nil || os.Chmod(outdir, 0o700) != nil {
			return ec.refuse("Çıktı klasörü oluşturulamadı."), sdir
		}
		forced[dirVar] = outdir
	}
	setKV := func(kv string) {
		k, v, _ := strings.Cut(kv, "=")
		forced[k] = v
	}
	for _, kv := range manifest.InfoAll(info, "action."+action+".set") {
		if kv != "" {
			setKV(kv)
		}
	}
	applied := ApplyVars(info, action, ec.values())
	for _, kv := range applied {
		setKV(kv)
	}
	var runsets []string
	if ec.RunSet != "" {
		rs, err := RunSets(info, action, ec.RunSet)
		if err != nil {
			return ec.refuse(err.Error()), sdir
		}
		runsets = rs
		for _, kv := range runsets {
			setKV(kv)
		}
		if len(runsets) > 0 {
			ec.say("ℹ️  Bu koşuya özel (değişken dosyanıza YAZILMAZ):\n")
			for _, kv := range runsets {
				ec.say("    %s\n", kv)
			}
		}
	}
	opt := manifest.VarsOptions{Forced: forced, OutPath: vf}
	if !RequiredApplies(info, action) {
		opt.NoReq = true
	}
	warns, errs, code := manifest.BuildVars(filepath.Join(ec.PkgDir, "component.json"), ec.values(), opt)
	if code != 0 {
		for _, e := range errs {
			fmt.Fprintln(ec.Stderr, e)
		}
		return ec.refuse("Değişkenler geçersiz; çalıştırılmadı."), sdir
	}
	if WarnApplies(info, action) {
		if err := WarnVars(warns, ec.ask); err != nil {
			return ec.refuse(err.Error()), sdir
		}
	}

	// ---- onay ----
	ApplyVarsNotice(info, ec.values(), applied, ec.Stderr)
	if err := Confirm(info, action, ec.Server, ec.ask, ec.Stderr); err != nil {
		return ec.refuse(err.Error()), sdir
	}

	if runner == "script" {
		return ec.execScript(info, action, vf, work, sdir), sdir
	}

	// ---- girdiler ----
	group := manifest.InfoGet(info, "inventory_group")
	if group == "" {
		group = "sshpocket"
	}
	inv := filepath.Join(work, "inventory")
	if err := manifest.WriteInventory(inv, group, ec.Server, ec.Host, ec.User, ec.Port, ec.Key, ec.BecomeExe); err != nil {
		return ec.refuse(err.Error()), sdir
	}
	argv, _, err := manifest.BuildArgv(info, action, inv, ec.Server, vf)
	if err != nil {
		return ec.refuse(err.Error()), sdir
	}
	prog := argv[0]
	if strings.Contains(prog, "/") {
		return ec.refuse("Çalıştırılacak program bulunamadı: " + prog), sdir
	}
	if st, err := os.Stat(filepath.Join(ec.BinDir, prog)); err != nil || st.Mode()&0o111 == 0 || st.IsDir() {
		return ec.refuse("Çalıştırılacak program bulunamadı: " + prog), sdir
	}
	argv[0] = filepath.Join(ec.BinDir, prog)
	for _, w := range argv {
		if w == "--ask-become-pass" || w == "-K" {
			ec.say("🔑 Sudo parolası istenecek: aşağıdaki 'BECOME password:' sunucudaki sudo parolasıdır.\n")
			ec.say("   Sudo parola istemiyorsa (NOPASSWD) yalnızca Enter'a basın.\n")
			ec.say("   Parola yalnızca Ansible'ın kendi istemine girilir; sshpocket'e, günlüğe ya da komut satırına yazılmaz.\n")
			break
		}
	}

	// ---- çalıştır (günlük kullanıcı klasöründe, izin 600) ----
	logPath := filepath.Join(sdir, "last-run.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		return Refused, sdir
	}
	os.Chmod(logPath, 0o600)
	lf, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return Refused, sdir
	}
	defer lf.Close()
	var q strings.Builder
	for _, w := range argv {
		q.WriteString(shQuoteQ(w) + " ")
	}
	fmt.Fprintf(lf, "komut: %s\n", q.String())
	for _, kv := range append(append([]string(nil), applied...), runsets...) {
		fmt.Fprintf(lf, "bu koşuya özel: %s\n", kv)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = ec.PkgDir
	timeout := os.Getenv("ANSIBLE_TIMEOUT")
	if timeout == "" {
		timeout = "30"
	}
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1", "ANSIBLE_CONFIG="+filepath.Join(ec.PkgDir, "ansible.cfg"),
		"ANSIBLE_HOST_KEY_CHECKING=True", "ANSIBLE_TIMEOUT="+timeout, "PATH="+ec.BinDir+":"+os.Getenv("PATH"))
	rc = ec.runTee(cmd, lf)
	logText, _ := os.ReadFile(logPath)
	if rc != 0 && strings.Contains(string(logText), "waiting for privilege escalation prompt") {
		ec.say("  ↳ Sudo istemi beklenirken zaman aşımı: sudo parolası yanlış olabilir ya da sunucudaki sudo Ansible ile uyumsuz olabilir\n")
		ec.say("    (Ubuntu 26.04'te varsayılan sudo-rs parolalı kullanımda takılır). Parolayı kontrol edin; sudo'yu elle deneyin (ssh ile: sudo -v).\n")
	}
	if rc != 0 && (strings.Contains(string(logText), "Incorrect sudo password") || strings.Contains(string(logText), "Missing sudo password")) {
		ec.say("  ↳ Sudo parolası reddedildi. Aynı komutu yeniden çalıştırıp 'BECOME password:' istemine doğru parolayı yazın.\n")
	}
	return rc, sdir
}

func sshBase(ec *ExecCtx) []string {
	return append(SSHOptions(), "-p", ec.Port, "-i", ec.Key)
}

func (ec *ExecCtx) remoteRm(rpath string) {
	if !reRemoteTmp.MatchString(rpath) {
		return
	}
	args := append(sshBase(ec), ec.User+"@"+ec.Host, "rm -f "+rpath)
	cmd := exec.Command("ssh", args...)
	_ = cmd.Run()
}

// execScript: betik SUNUCUDA çalışır (Mac'te yabancı kod çalışmaz).
func (ec *ExecCtx) execScript(info []string, action, vf, work, sdir string) int {
	words := strings.Fields(manifest.ActionGet(info, action, "cmd"))
	if len(words) == 0 {
		return ec.refuse("Betik bulunamadı, sembolik bağ ya da çok büyük (en çok 256 KB): ")
	}
	script := filepath.Join(ec.PkgDir, words[0])
	st, err := os.Lstat(script)
	sst, serr := os.Stat(script)
	if err != nil || serr != nil || !sst.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 || sst.Size() > 262144 {
		return ec.refuse("Betik bulunamadı, sembolik bağ ya da çok büyük (en çok 256 KB): " + words[0])
	}
	envLines, errs, code := manifest.Env(vf, ec.Server, action, ec.Profile)
	if code != 0 {
		for _, e := range errs {
			fmt.Fprintln(ec.Stderr, e)
		}
		return ec.refuse("Betik hazırlanamadı (değişkenler).")
	}
	body, _ := os.ReadFile(script)
	payload := filepath.Join(work, "payload.sh")
	if err := os.WriteFile(payload, []byte(strings.Join(envLines, "\n")+"\n"+string(body)), 0o600); err != nil {
		return ec.refuse("Betik hazırlanamadı (değişkenler).")
	}
	rargs := ""
	for _, w := range words[1:] {
		rargs += " " + shQuoteQ(w)
	}
	logPath := filepath.Join(sdir, "last-run.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		return Refused
	}
	os.Chmod(logPath, 0o600)
	lf, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return Refused
	}
	defer lf.Close()
	fmt.Fprintf(lf, "komut: ssh -p %s %s@%s bash -s --%s  (betik: %s)\n", ec.Port, ec.User, ec.Host, rargs, words[0])

	pf, err := os.Open(payload)
	if err != nil {
		return Refused
	}
	defer pf.Close()
	if manifest.ActionGet(info, action, "sudo") == "yes" {
		args := append(sshBase(ec), ec.User+"@"+ec.Host, `umask 077; f=$(mktemp /tmp/sshpocket-XXXXXXXX) && cat > "$f" && printf %s "$f"`)
		cmd := exec.Command("ssh", args...)
		cmd.Stdin = pf
		out, err := cmd.Output()
		if err != nil {
			return ec.refuse("Betik sunucuya yazılamadı.")
		}
		rpath := strings.TrimRight(string(out), "\n")
		if !reRemoteTmp.MatchString(rpath) {
			return ec.refuse("Sunucudaki geçici betik yolu beklenmedik: çalıştırılmadı.")
		}
		defer ec.remoteRm(rpath)
		sudoProg := "sudo"
		if ec.BecomeExe != "" {
			sudoProg = ec.BecomeExe
		}
		hash, herr := fileHash(payload)
		if herr != nil || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hash) {
			return ec.refuse("Betiğin özeti hesaplanamadı (sha256sum/shasum yok).")
		}
		ec.say("🔑 Sudo parolası istenecek: aşağıdaki istem sunucudaki sudo parolasıdır (sudo parola istemiyorsa yalnızca Enter).\n")
		ec.say("   Parola yalnızca sunucunun sudo isteminde girilir; sshpocket'e, günlüğe ya da komut satırına yazılmaz.\n")
		rcmd := fmt.Sprintf("%s bash -c '%s' bash %s %s%s", sudoProg, remoteScript, rpath, hash, rargs)
		args = append(sshBase(ec), "-t", ec.User+"@"+ec.Host, rcmd)
		rc := ec.runTee(exec.Command("ssh", args...), lf)
		if rc == 97 {
			ec.say("⛔ Sunucudaki betik özeti yerelle uyuşmadı: betik ÇALIŞTIRILMADI. Sunucuda /tmp'ye başka bir süreç yazmış olabilir; sunucuyu inceleyin.\n")
		}
		return rc
	}
	args := append(sshBase(ec), ec.User+"@"+ec.Host, "bash -s --"+rargs)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = pf
	var w io.Writer = ec.Stdout
	if w == nil {
		w = io.Discard
	}
	w = io.MultiWriter(w, lf)
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Start(); err != nil {
		return 127
	}
	err = cmd.Wait()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 127
}

// ExecResult, bir eylem koşusunun sonucudur.
type ExecResult struct {
	RC      int
	Result  []string // SSHPOCKET_RESULT satırından
	LogFile string   // kalıcı arşiv günlüğü
}

// ExecAction, comp_exec_action: kapılar, çalıştırma, sonuç ayrıştırma, günlük arşivi ve kayıt.
// Refused dönerse hiçbir şey çalışmamıştır ve kayıt tutulmaz.
func ExecAction(ec *ExecCtx, info []string, id, ver, action string) ExecResult {
	t0 := ec.now()
	rc, _ := ec.execBody(info, id, ver, action)
	if rc == Refused {
		return ExecResult{RC: rc}
	}
	res := ExecResult{RC: rc}
	sdir, err := ec.Gates.StateEnsure(id, ec.Server)
	if err != nil {
		return res
	}
	if r, err := ParseResult(info, action, filepath.Join(sdir, "last-run.log")); err != nil {
		fmt.Fprintln(ec.Stderr, err.Error())
	} else {
		res.Result = r
	}
	if p, err := ArchiveLog(sdir, id, ver, action, ec.Server, rc, t0, ec.now(), ec.LogKeep); err == nil {
		res.LogFile = p
	}
	_ = SaveResult(sdir, action, res.Result, ec.now())
	outcome := "fail"
	if rc == 0 {
		outcome = "ok"
	}
	_ = ec.Gates.Record(id, ec.Server, ver, action, outcome)
	return res
}

var _ = errors.New
