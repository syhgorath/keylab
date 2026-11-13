// Eylem çalıştırma: yetkilendirme, pty altında iş yürütme, parola penceresi, maskeleme.
// (main.go'dan mekanik olarak bölündü; davranış değişmedi.)
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

// classify: arayüzden hangi eylemler çalıştırılabilir?
//
//	run:   salt okunur, sudo istemeyen, onay gerektirmeyen, önkoşulsuz → arayüz doğrudan çalıştırır.
//	term:  aynı ama sudo ister → macOS Terminal'de sshpocket motoru açılır, parolayı kullanıcı orada girer.
//	write: değişiklik yapar (yıkıcı DEĞİL) ve önkoşulu (taze preview) tamam → onaylı akışla (plan, snapshot, ad/e-h onayı) Terminal'de çalışır.
//
// Yıkıcı eylemler de write sayılır ama authorizeRun daha sıkı onay ister (ad birebir + snapshot + "geri alınamayabilir" onayı).
// Eksik zorunlu değişken ve eski/başarısız önkoşul eylemi kapatır.
func classify(a ActionInfo) (run, term, write bool, reason string) {
	switch {
	case !a.Applicable:
		return false, false, false, "bu sunucunun platformuna uygulanmaz"
	case len(a.MissingVars) > 0:
		return false, false, false, "gerekli değişken eksik: " + strings.Join(a.MissingVars, ", ") + " (Değişkenler kartındaki Düzenle… ile ayarlayın)"
	case len(a.Effects) > 0:
		if !a.PrereqsOK {
			return false, false, false, "önkoşul tamam değil: " + strings.Join(a.Requires, ", ") + " son 30 dk içinde başarıyla çalışmış olmalı"
		}
		return false, false, true, "değişiklik yapar: sudo parolasını Terminal'de girersiniz"
	case len(a.Requires) > 0 && !a.PrereqsOK:
		return false, false, false, "önkoşul tamam değil: " + strings.Join(a.Requires, ", ") + " son 30 dk içinde başarıyla çalışmış olmalı"
	case a.Confirm != "" && a.Confirm != "none":
		return false, false, false, "onay ister: henüz bağlı değil (sshpocket menüsünden çalıştırın)"
	case a.RequiresSud:
		return false, true, false, "sudo parolası gerekir: parola arayüzde girilmez, Terminal'de girersiniz"
	}
	return true, false, false, ""
}

// confirmOK: arayüzün topladığı onay, eylemin confirm kuralına uyuyor mu? (sunucu tarafında yeniden denetlenir)
func confirmOK(a ActionInfo, server, got string) bool {
	switch a.Confirm {
	case "type-server-name":
		return got == server
	case "yes-no":
		return got == "evet"
	}
	return got == "" || got == "evet"
}

// sanitizePrompt: pencerede gösterilecek istem metni (kontrol karakterleri atılır, en fazla 160 karakter)
func sanitizePrompt(p string) string {
	var b strings.Builder
	for _, r := range p {
		if r >= 32 && r != 127 {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if rs := []rune(out); len(rs) > 160 {
		out = string(rs[:160])
	}
	if out == "" {
		out = "Parola"
	}
	return out
}

// askpassScript: gizli yanıtlı macOS parola penceresi (System Events ile öne gelir; İptal → osascript hata kodu verir)
func askpassScript(prompt string) string {
	return "tell application \"System Events\"\n" +
		"activate\n" +
		"set r to text returned of (display dialog " + appleScriptQuote(sanitizePrompt(prompt)) + " with title \"sshpocket\" default answer \"\" with hidden answer buttons {\"İptal\", \"Tamam\"} default button \"Tamam\" cancel button \"İptal\" with icon caution)\n" +
		"end tell\n" +
		"return r"
}

func askpassAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("osascript")
	return err == nil
}

// askPassNative: parolayı işletim sisteminin kendi penceresinden alır; tarayıcıya hiç uğramaz. Parola argümanla değil osascript'in çıktısıyla gelir.
func askPassNative(prompt string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e", askpassScript(prompt)).Output()
	if err != nil {
		return nil, fmt.Errorf("parola penceresi iptal edildi")
	}
	return []byte(strings.TrimSuffix(string(out), "\n")), nil
}

// terminalCommand: macOS Terminal'de çalışacak komut (Go ikilisi). Tüm parçalar doğrulanmış adlardır ve tek tırnakla alıntılanır.
func terminalCommand(exe, root, pkg, action, server string) string {
	return "cd " + shQuote(root) + " && " + shQuote(exe) + " components -root " + shQuote(root) + " run " + pkg + " " + action + " " + server
}

// appleScriptQuote: AppleScript metin değişmezi için kaçışlar (\ ve ")
func appleScriptQuote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

// Job: arka planda, sahte terminal (pty) altında çalışan bir motor komutu. Çıktı ham bayt olarak biriktirilir
// (istem satırları "\n" ile bitmediği için satır satır okunamaz); girdi stdin borusuyla pty'ye gider.
type Job struct {
	ID      string `json:"id"`
	Server  string `json:"server"`
	Package string `json:"package"`
	Action  string `json:"action"`
	Started string `json:"started"`
	Done    bool   `json:"done"`
	Exit    int    `json:"exit"`
	// exclusive: genel durumu/dizini değiştiren iş (paket kurulumu, sunucu silme): çalışırken başka iş olamaz, başkaları çalışırken başlamaz
	exclusive bool
	buf       []byte
	env       []string // spawn öncesi eklenecek ortam değişkenleri
	stdin     io.WriteCloser
	cancel    context.CancelFunc
}

const (
	maxJobBytes = 4 << 20
	jobTimeout  = 45 * time.Minute // hardening apply tek başına ~16 dk sürebilir
)

var reSecretLine = regexp.MustCompile(`(?i)((become|sudo|passw[a-z]*|parola|token|secret|api[_-]?key)[A-Za-z_ ]{0,20}[:=]\s*)\S.*`)

func maskLine(l string) string { return reSecretLine.ReplaceAllString(l, "${1}****") }

// maskLine2: çok satırlı metinde her satıra uygular
func maskLine2(t string) string { return reSecretLine.ReplaceAllString(t, "${1}****") }

// validUTF8Prefix: parça sınırında kesilmiş çok baytlı karakteri sona bırakır
func validUTF8Prefix(b []byte) int {
	for i := len(b); i > 0 && i > len(b)-4; i-- {
		if utf8.Valid(b[:i]) {
			return i
		}
	}
	return len(b)
}

// findAction: sunucu ve paket için eylem üst verisini bulur (istemciye güvenilmez)
func (a *App) findAction(server, pkg, action string) (*ActionInfo, error) {
	ai, _, err := a.findActionItem(server, pkg, action, "")
	return ai, err
}

// findActionItem: item doluysa eylem o katalog öğesi için aranır: öğe paketin `items` listesinde olmalı ve (liste doluysa) eylem öğenin
// `actions` listesinde olmalıdır; önkoşul/son sonuç durumu O ÖĞENİN kapsamında hesaplanır. Dönen ikinci değer paketin `item_var` adıdır.
func (a *App) findActionItem(server, pkg, action, item string) (*ActionInfo, string, error) {
	s, ok := a.findServerQuiet(server)
	if !ok || !s.Valid {
		return nil, "", fmt.Errorf("sunucu yok ya da yapılandırması geçersiz")
	}
	for _, p := range a.packageInfosFor(s, item) {
		if p.ID != pkg {
			continue
		}
		if item != "" {
			var it *ItemInfo
			for i := range p.Items {
				if p.Items[i].ID == item {
					it = &p.Items[i]
				}
			}
			if it == nil {
				return nil, "", fmt.Errorf("katalogda olmayan öğe: %s", item)
			}
			if len(it.Actions) > 0 && !strings.Contains(","+strings.Join(it.Actions, ",")+",", ","+action+",") {
				return nil, "", fmt.Errorf("%s için '%s' eylemi tanımlı değil", item, action)
			}
		}
		for i := range p.Actions {
			if p.Actions[i].ID == action {
				return &p.Actions[i], p.ItemVar, nil
			}
		}
	}
	return nil, "", fmt.Errorf("paket ya da eylem bulunamadı")
}

// authorizeRun: eylem arayüzden çalıştırılabilir mi, gerekli onaylar tamam mı? (yazan eylemlerde onaylar burada yeniden denetlenir)
func (a *App) authorizeRun(server, pkg, action string, snapshot bool, confirm string, ackDestructive bool) (*ActionInfo, int, error) {
	ai, _, code, err := a.authorizeRunItem(server, pkg, action, "", snapshot, confirm, ackDestructive)
	return ai, code, err
}

// authorizeRunItem: authorizeRun'ın öğe kapsamlı hali (item boşsa aynısı). Dönen string paketin item_var adıdır (item doluysa).
func (a *App) authorizeRunItem(server, pkg, action, item string, snapshot bool, confirm string, ackDestructive bool) (*ActionInfo, string, int, error) {
	ai, itemVar, err := a.findActionItem(server, pkg, action, item)
	if err != nil {
		return nil, "", http.StatusConflict, err
	}
	switch {
	case ai.Runnable || ai.Terminal:
	case ai.Write:
		if !snapshot {
			return nil, "", http.StatusBadRequest, fmt.Errorf("snapshot onayı gerekli")
		}
		if !confirmOK(*ai, server, confirm) {
			return nil, "", http.StatusBadRequest, fmt.Errorf("onay eşleşmedi")
		}
		if ai.Destructive {
			// yıkıcı eylem: motorun kuralı ne olursa olsun sunucu adı birebir yazılmalı ve geri alınamama onayı verilmeli
			if confirm != server {
				return nil, "", http.StatusBadRequest, fmt.Errorf("yıkıcı eylem: onay için sunucu adı birebir yazılmalı")
			}
			if !ackDestructive {
				return nil, "", http.StatusBadRequest, fmt.Errorf("yıkıcı eylem: geri alınamayabileceğini onaylamalısınız")
			}
		}
	default:
		return nil, "", http.StatusConflict, fmt.Errorf("bu eylem arayüzden çalıştırılamaz: %s", ai.Reason)
	}
	return ai, itemVar, 0, nil
}

// ptyArgv: komutu sahte terminal altında çalıştıran sarmalayıcı (macOS/BSD ve Linux script)
func ptyArgv(argv []string) []string {
	if runtime.GOOS == "darwin" {
		return append([]string{"script", "-q", "/dev/null"}, argv...)
	}
	q := make([]string, len(argv))
	for i, x := range argv {
		q[i] = shQuote(x)
	}
	return []string{"script", "-qfec", strings.Join(q, " "), "/dev/null"}
}

// spawn: argv'yi pty altında başlatır ve çıktıyı job.buf'a akıtır. Çağıran a.jmu'yu TUTMAMALI.
func (a *App) spawn(job *Job, dir string, argv []string) error {
	if _, err := exec.LookPath("script"); err != nil {
		return fmt.Errorf("'script' aracı bulunamadı (sahte terminal için gerekli)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	pa := ptyArgv(argv)
	cmd := exec.CommandContext(ctx, pa[0], pa[1:]...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "TERM=dumb", "ANSIBLE_NOCOLOR=1", "NO_COLOR=1"), job.env...)
	setJobSysProcAttr(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		cancel()
		return err
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	// iptal: önce Ctrl-C (pty satır disiplini SIGINT'e çevirir), kısa süre sonra süreç grubuna SIGTERM
	cmd.Cancel = func() error {
		_, _ = stdin.Write([]byte{0x03})
		go func() { time.Sleep(1500 * time.Millisecond); killJobGroup(cmd) }()
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		cancel()
		pr.Close()
		pw.Close()
		return err
	}
	pw.Close()
	job.stdin, job.cancel = stdin, cancel
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := pr.Read(chunk)
			if n > 0 {
				a.jmu.Lock()
				if len(job.buf)+n <= maxJobBytes {
					job.buf = append(job.buf, chunk[:n]...)
				}
				a.jmu.Unlock()
			}
			if err != nil {
				break
			}
		}
		pr.Close()
		werr := cmd.Wait()
		cancel()
		a.jmu.Lock()
		job.Done = true
		if werr != nil {
			if ee, ok := werr.(*exec.ExitError); ok {
				job.Exit = ee.ExitCode()
			} else {
				job.Exit = 1
			}
			if ctx.Err() == context.DeadlineExceeded {
				job.buf = append(job.buf, []byte("\n⏱ zaman aşımı: eylem durduruldu\n")...)
			}
		}
		a.jmu.Unlock()
	}()
	return nil
}

// Eşzamanlılık kuralı: FARKLI sunucularda eylemler paralel çalışır (en çok maxParallelJobs); aynı sunucuda aynı anda tek iş.
// Genel işler (paket kurulumu: sunucu yok; sunucu silme) yalnız başka hiçbir iş yokken başlar ve çalışırken başkası başlayamaz.
// (Üst sınır Ayarlar'dan değişir: bkz. uisettings.go; burası varsayılan.)
const maxParallelJobs = defaultMaxJobs

// runningLocked: çalışan (bitmemiş) işler. a.jmu tutulurken çağrılır.
func (a *App) runningLocked() []*Job {
	var out []*Job
	for _, j := range a.jobs {
		if !j.Done {
			out = append(out, j)
		}
	}
	return out
}

// anyRunning: herhangi bir iş çalışıyor mu? (genel işlemlerin ve yeniden adlandırmanın kapısı)
func (a *App) anyRunning() bool {
	a.jmu.Lock()
	defer a.jmu.Unlock()
	return len(a.runningLocked()) > 0
}

// serverBusy: bu sunucuda bir iş ya da genel bir iş çalışıyor mu? (değişken düzenleme kapısı)
func (a *App) serverBusy(server string) bool {
	a.jmu.Lock()
	defer a.jmu.Unlock()
	for _, j := range a.runningLocked() {
		if j.exclusive || j.Server == server {
			return true
		}
	}
	return false
}

// newJob: işi kaydeder; eşzamanlılık kuralını uygular
func (a *App) newJob(server, pkg, action string) (*Job, error) {
	return a.newJobX(false, server, pkg, action)
}

// newJobX: batch=true yalnız toplu işlem yürütücüsünden çağrılır; toplu iş sürerken diğer tüm başlatıcılar reddedilir.
func (a *App) newJobX(batch bool, server, pkg, action string) (*Job, error) {
	a.jmu.Lock()
	defer a.jmu.Unlock()
	if !batch && a.batchActive() {
		return nil, fmt.Errorf("toplu işlem çalışıyor; bitmesini bekleyin ya da iptal edin")
	}
	excl := server == "" || action == "upgrade" || action == "remove"
	run := a.runningLocked()
	if excl && len(run) > 0 {
		return nil, fmt.Errorf("başka eylemler çalışırken bu işlem başlatılamaz; bitmelerini bekleyin")
	}
	for _, j := range run {
		if j.exclusive {
			return nil, fmt.Errorf("genel bir işlem (paket kurulumu/silme) çalışıyor; bitmesini bekleyin")
		}
		if j.Server == server {
			return nil, fmt.Errorf("bu sunucuda başka bir eylem çalışıyor")
		}
	}
	if lim := a.maxJobs(); len(run) >= lim {
		return nil, fmt.Errorf("en çok %d eylem aynı anda çalışabilir (Ayarlar > Eylemler); birinin bitmesini bekleyin", lim)
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	job := &Job{ID: hex.EncodeToString(b), Server: server, Package: pkg, Action: action, Started: time.Now().UTC().Format(time.RFC3339), exclusive: excl}
	a.jobs[job.ID] = job
	if len(a.jobs) > 20 { // eski bitmiş işleri at
		for id, j := range a.jobs {
			if j.Done && id != job.ID {
				delete(a.jobs, id)
				break
			}
		}
	}
	return job, nil
}

// jobArgv: testte sahte komutla değiştirilir (üretimde runArgv).
var jobArgv = runArgv

// runArgv: `keylab run` komut satırı. Öğe seçimi koşuya özel `-set <item_var>=<öğe>` ile verilir (değer dosyasına yazılmaz; motor
// kapsamı ve kapıları aynı seçimle yeniden hesaplar).
func runArgv(exe, servers, comps, state, pkg, action, server, itemVar, item string) []string {
	argv := []string{exe, "run", "-servers", servers, "-components", comps, "-state", state}
	if item != "" && itemVar != "" {
		argv = append(argv, "-set", itemVar+"="+item)
	}
	return append(argv, pkg, action, server)
}

// startJob: eylemi pty altında Go yürütücüsüyle (`keylab run`) çalıştırır. Kapılar, günlük ve sonuç kaydı yürütücünün işidir.
func (a *App) startJob(server, pkg, action, item string, snapshot bool, confirm string, ackDestructive bool) (*Job, int, error) {
	return a.startJobX(false, server, pkg, action, item, snapshot, confirm, ackDestructive)
}

func (a *App) startJobX(batch bool, server, pkg, action, item string, snapshot bool, confirm string, ackDestructive bool) (*Job, int, error) {
	_, itemVar, code, err0 := a.authorizeRunItem(server, pkg, action, item, snapshot, confirm, ackDestructive)
	if err0 != nil {
		return nil, code, err0
	}
	root := filepath.Dir(filepath.Clean(a.compDir))
	exe, err := os.Executable() // eylemi Go yürütücüsü (internal/runner) çalıştırır: kendi ikilimiz, `run` alt komutu
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("sshpocket ikilisinin yolu bulunamadı: %w", err)
	}
	argv := jobArgv(exe, a.serversDir, a.compDir, a.stateDir, pkg, action, server, itemVar, item)
	job, err := a.newJobX(batch, server, pkg, action)
	if err != nil {
		return nil, http.StatusConflict, err
	}
	if err := a.spawn(job, root, argv); err != nil {
		a.jmu.Lock()
		job.Done, job.Exit = true, 1
		a.jmu.Unlock()
		return nil, http.StatusInternalServerError, err
	}
	return job, 0, nil
}
