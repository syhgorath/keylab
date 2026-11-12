package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"keylab/internal/manifest"
	"keylab/internal/trust"
)

// CLI, `--components run <paket> <eylem> <sunucu>` akışının bağımlılıklarıdır (ComponentsRunCLI.sh comp_cli_run).
type CLI struct {
	ComponentsDir string // kurulu paketler
	LockPath      string // components.lock
	ServersDir    string // servers/
	StateDir      string // ~/.local/share/sshpocket
	KnownHosts    string // boşsa varsayılan
	AddRetry      func(prefix, key string) error
	Ctx           context.Context
	Stdin         io.Reader
	Stdout        io.Writer // komut çıktısı
	Stderr        io.Writer // bilgi/hata iletileri
	Now           func() time.Time
	LogKeep       int
	RunSet        string // katalog seçimi (koşuya özel): satır satır "ad=değer"; değer dosyasına YAZILMAZ (bkz. RunSets)
}

var (
	reServerName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	reTargetIP   = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)
	reTargetUser = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
	reTargetPort = regexp.MustCompile(`^[0-9]+$`)
)

// EnvGet, `_ssh_env_get` karşılığıdır: server.env benzeri dosyadan güvenle (eval/source olmadan) tek anahtar okur.
func EnvGet(file, want string) (string, bool) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	text := string(b)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t\n\v\f\r")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		key = strings.Trim(key, " \t\n\v\f\r")
		value = strings.Trim(value, " \t\n\v\f\r")
		switch {
		case len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"':
			value = value[1 : len(value)-1]
		case len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'':
			value = value[1 : len(value)-1]
		default:
			// tırnaksız değerde satır sonu yorumunu at: ilk "boşluk + #"
			for i := 0; i+1 < len(value); i++ {
				if (value[i] == ' ' || value[i] == '\t') && value[i+1] == '#' {
					value = value[:i]
					break
				}
			}
			value = strings.TrimRight(value, " \t\n\v\f\r")
		}
		if key == want {
			return value, true
		}
	}
	return "", false
}

// serverGet, srv_get: yeni biçimde (server.env) anahtar adı düz, eski biçimde (.env) <ÖNEK>_<ANAHTAR>.
func serverGet(dir, prefix, key string) (string, bool) {
	envf := filepath.Join(dir, "server.env")
	name := key
	if st, err := os.Stat(envf); err != nil || !st.Mode().IsRegular() {
		envf = filepath.Join(dir, ".env")
		name = prefix + "_" + key
	}
	return EnvGet(envf, name)
}

func validTarget(ip, user, port string) bool {
	if !reTargetIP.MatchString(ip) || !reTargetUser.MatchString(user) || !reTargetPort.MatchString(port) {
		return false
	}
	n, _ := strconv.Atoi(port)
	return n >= 1 && n <= 65535
}

// Run: kapılar → yürütme → sonuç. Çıkış kodu: 0 başarı, eylemin kodu, 2 kullanım/geçersiz girdi, 190 çalıştırılmadı.
func (c *CLI) Run(id, action, server string) int {
	errw := c.Stderr
	say := func(format string, a ...any) { fmt.Fprintf(errw, format, a...) }
	refuse := func(msg string) int {
		if msg != "" {
			fmt.Fprintln(errw, msg)
		}
		return Refused
	}
	if !trust.IDValid(id) {
		say("Geçersiz paket kimliği.\n")
		return 2
	}
	if !reActionName.MatchString(action) {
		say("Geçersiz eylem adı.\n")
		return 2
	}
	if !reServerName.MatchString(server) {
		say("Geçersiz sunucu adı.\n")
		return 2
	}

	// comp_prepare
	lock, err := trust.LoadLock(c.LockPath)
	if err != nil {
		return refuse(err.Error())
	}
	ver, mf, err := trust.Prepare(c.ComponentsDir, lock, id)
	if err != nil {
		return refuse(err.Error())
	}
	info, merrs, mcode := manifest.ValidateFile(mf, id, ver)
	if mcode != 0 {
		for _, e := range merrs {
			fmt.Fprintln(errw, e)
		}
		return Refused
	}
	known := false
	for _, a := range manifest.InfoAll(info, "action") {
		if a == action {
			known = true
		}
	}
	if !known {
		say("'%s' paketinde '%s' eylemi yok.\n", id, action)
		return 2
	}
	pkg := filepath.Join(c.ComponentsDir, id, ver)

	dir := filepath.Join(c.ServersDir, server)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		say("Sunucu bulunamadı: %s\n", server)
		return 2
	}
	plat, _, pcode := manifest.Platform(filepath.Join(pkg, "component.json"), filepath.Join(dir, id+".vars"))
	if pcode != 0 {
		plat = ""
	}
	if !manifest.ActionOnPlatform(info, action, plat) {
		say("'%s' eylemi bu sunucunun platformunda (%s) kullanılamaz; yalnızca: %s.\n", action, plat,
			strings.ReplaceAll(manifest.ActionGet(info, action, "platforms"), ",", ", "))
		return 2
	}
	prefix := strings.ToUpper(server)
	ip, _ := serverGet(dir, prefix, "IP")
	user, _ := serverGet(dir, prefix, "USER")
	port, _ := serverGet(dir, prefix, "PORT")
	if port == "" {
		port = "22"
	}
	key, _ := serverGet(dir, prefix, "KEY")
	if key != "" {
		if key[0] == '~' {
			key = os.Getenv("HOME") + key[1:]
		}
	} else {
		key = filepath.Join(dir, "id_ed25519_"+server)
	}
	if !validTarget(ip, user, port) {
		return refuse("Sunucu bilgisi eksik/geçersiz (IP, kullanıcı, port).")
	}
	if st, err := os.Stat(key); err != nil || !st.Mode().IsRegular() {
		return refuse("SSH key bulunamadı: " + key)
	}

	rd := bufio.NewReader(orEmpty(c.Stdin))
	readLine := func(prompt string) (string, bool) {
		fmt.Fprint(errw, prompt)
		l, err := rd.ReadString('\n')
		if err != nil {
			fmt.Fprintln(errw)
			return "", false
		}
		return strings.TrimSuffix(l, "\n"), true
	}

	addRetry := c.AddRetry
	if addRetry == nil {
		addRetry = (&AgentAdder{Ask: readLine, Err: errw}).AddRetry
	}

	// Python ortamı (yalnızca ansible runner): yoksa AÇIKÇA onay istenir (ağdan indirir)
	venv := ""
	if manifest.InfoGet(info, "runner") == "ansible" {
		rng := manifest.InfoGet(info, "requires.python")
		if rng == "" {
			rng = ">=3.12"
		}
		v, ok := VenvLookup(id, pkg, rng)
		if !ok {
			say("Python ortamı kurulu değil. İlk kullanımda kurulacak: PyPI'dan yalnızca hash'li wheel paketleri (yaklaşık 10 MB indirme,\n")
			say("diskte yaklaşık 90 MB; ~/.cache/sshpocket altında, sistem Python'una dokunulmaz).\n")
			ans, got := readLine("Kurulsun mu? [e/H]: ")
			if !got {
				say("Onay alınamadı.\n")
				return Refused
			}
			if !yes(ans) {
				say("İptal edildi.\n")
				return Refused
			}
			v, err = VenvEnsure(id, pkg, rng, "")
			if err != nil {
				fmt.Fprintln(errw, err.Error())
				return Refused
			}
		}
		venv = v
	}

	// İşletim sistemi profili: anahtar ajanda olmalı, host key tanınmalı; sonra salt okunur algılama
	if err := KnownHost(ip, port, c.KnownHosts); err != nil {
		return refuse(err.Error())
	}
	if err := Agent(prefix, key, addRetry); err != nil {
		return refuse(err.Error())
	}
	profile, sshErr, derr := DetectProfile(ip, port, user, key)
	if derr != nil {
		say("Sunucunun işletim sistemi algılanamadı (ssh).\n")
		if hint := SSHHint([]byte(sshErr)); hint != "" {
			say("  ↳ %s\n", hint)
		}
		return Refused
	}
	say("Sunucu: %s (%s@%s:%s)  Profil: %s  Paket: %s %s\n", server, user, ip, port, profile, id, ver)
	var nowFn func() int64
	if c.Now != nil {
		nowFn = func() int64 { return c.Now().Unix() }
	}
	WarnClockSkew(ip, port, user, key, nowFn, errw)
	become := ""
	if strings.HasPrefix(profile, "ubuntu-") && DetectSudoWS(ip, port, user, key) {
		become = "/usr/bin/sudo.ws"
		say("  ℹ️  Ubuntu: Ansible sudo-rs yerine klasik sudo (/usr/bin/sudo.ws) ile yetki yükseltir.\n")
	}

	values := filepath.Join(dir, id+".vars")
	if st, err := os.Stat(values); err != nil || !st.Mode().IsRegular() {
		values = ""
	}
	binDir := ""
	if venv != "" {
		binDir = filepath.Join(venv, "bin")
	}
	ec := &ExecCtx{PkgDir: pkg, BinDir: binDir, Server: server, Host: ip, User: user, Port: port, Key: key, Prefix: prefix, Profile: profile,
		ValuesFile: values, BecomeExe: become, RunSet: c.RunSet, Gates: &Gates{StateDir: c.StateDir}, AddRetry: addRetry, KnownHosts: c.KnownHosts,
		Ctx: c.Ctx, Stdin: c.Stdin, Stdout: c.Stdout, Stderr: errw, LogKeep: c.LogKeep, rd: rd}
	if c.Now != nil {
		ec.Now = func() int64 { return c.Now().Unix() }
	}
	res := ExecAction(ec, info, id, ver, action)
	if res.RC == Refused {
		say("Çalıştırılmadı.\n")
		return res.RC
	}
	// Kayıt (gizli bilgi yok): tarih|paket|sürüm|eylem|sonuç
	outcome := "fail"
	if res.RC == 0 {
		outcome = "ok"
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	if f, err := os.OpenFile(filepath.Join(dir, "components.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600); err == nil {
		fmt.Fprintf(f, "%s|%s|%s|%s|%s\n", now.UTC().Format("2006-01-02T15:04:05Z"), id, ver, action, outcome)
		f.Close()
	}
	if len(res.Result) > 0 {
		say("Sonuç:\n")
		for _, l := range res.Result {
			say("  %s\n", l)
		}
	}
	if res.LogFile != "" {
		if res.RC != 0 {
			say("❌ Eylem başarısız. Tam günlük: %s\n", res.LogFile)
		} else {
			say("Günlük: %s\n", res.LogFile)
		}
	}
	return res.RC
}

func orEmpty(r io.Reader) io.Reader {
	if r == nil {
		return strings.NewReader("")
	}
	return r
}
