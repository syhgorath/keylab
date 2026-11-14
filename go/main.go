// sshpocket Go denemesi: ana sayfa (pano, sunucular, paketler, Lynis, günlükler) yerel web arayüzü olarak.
// Yalnızca 127.0.0.1'e bağlanır, oturum çerezi ister. Salt okunur: sunucuda komut ÇALIŞTIRMAZ
// (tek istisna: kullanıcı isterse macOS Terminal'de ssh penceresi açar). Gizli değer okumaz/göstermez.
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"keylab/internal/envx"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"keylab/internal/cli"
	"keylab/internal/runner"
)

//go:embed web
var webFS embed.FS

func (a *App) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// DNS rebinding koruması: Host başlığı bizim adresimiz olmalı
		if r.Host != a.host {
			http.Error(w, "yasak", http.StatusForbidden)
			return
		}
		c, err := r.Cookie(cookieName)
		if err != nil || !a.sessionValid(c.Value) {
			http.Error(w, "oturum yok ya da süresi doldu", http.StatusUnauthorized)
			return
		}
		// durum değiştiren istekler: özel başlık + aynı kaynak (SameSite=Strict çerezle birlikte CSRF savunması)
		if r.Method == http.MethodPost {
			if r.Header.Get("X-Requested-With") != "sshpocket" {
				http.Error(w, "yasak", http.StatusForbidden)
				return
			}
			if o := r.Header.Get("Origin"); o != "" && o != "http://"+a.host {
				http.Error(w, "yasak", http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}

// root: başlatma adresindeki ?t=<jeton> geçerliyse oturum çerezi verir ve adresi temizler; yenilemede çerez devam eder.
// contentSecurityPolicy: parola kutusu bu sayfada çıkabildiği için yalnızca kendi kaynağımızdan betik, dışarıya bağlantı yok, çerçeveye gömülemez.
// style-src 'unsafe-inline' KALIR: gömülü terminalin xterm.js'i çalışırken iki <style> ELEMENTİ ekler (nonce desteği yok). Buna karşılık
// style-src-attr 'none' satır içi `style="…"` ÖZNİTELİKLERİNİ yasaklar (CSS ile veri sızdırma/arayüz taklidi yüzeyi); arayüz stilleri sınıflardır,
// dinamik değerler CSSOM (el.style.x = …) ile verilir. Web dosyalarında `style="` kalmadığı csp_test.go'da denetlenir.
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; style-src-attr 'none'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

func (a *App) root(static http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != a.host {
			http.Error(w, "yasak", http.StatusForbidden)
			return
		}
		// tek kullanımlık açılış jetonu (?b=): tarayıcı başlatıcısı bunu kullanır; geçerliyse çerez verir ve jeton yanar
		if b := r.URL.Query().Get("b"); b != "" {
			if a.useBootToken(b) {
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: a.newSession(), Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if t := r.URL.Query().Get("t"); t != "" {
			if subtle.ConstantTimeCompare([]byte(t), []byte(a.token)) == 1 {
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: a.newSession(), Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		// parola kutusu bu sayfada çıkabildiği için: yalnızca kendi kaynağımızdan betik, dışarıya bağlantı yok, çerçeveye gömülemez
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		static.ServeHTTP(w, r)
	})
}

// useBootToken: açılış jetonu doğruysa VE daha önce kullanılmadıysa true (bir kez); sonraki her deneme false.
func (a *App) useBootToken(b string) bool {
	a.bootMu.Lock()
	defer a.bootMu.Unlock()
	if a.bootUsed || a.bootTok == "" || subtle.ConstantTimeCompare([]byte(b), []byte(a.bootTok)) != 1 {
		return false
	}
	a.bootUsed = true
	return true
}

func (a *App) findServer(name string) (Server, bool) {
	for _, s := range a.servers(true, name) {
		if s.Name == name {
			return s, true
		}
	}
	return Server{}, false
}

// findServerQuiet: findServer'ın ağa dokunmayan hali (erişilebilirlik ölçülmez; Up/Conn alanları güvenilmez). Yetki/kapı kararları ve
// toplu işlem için: kapalı bir sunucuyu aramak her seferinde bağlantı zaman aşımı beklememeli.
func (a *App) findServerQuiet(name string) (Server, bool) {
	for _, s := range a.servers(false, name) {
		if s.Name == name {
			return s, true
		}
	}
	return Server{}, false
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func (s Server) sshCmd() string {
	k := ""
	if s.HasKey {
		k = "-i " + shQuote(s.keyPath) + " "
	}
	return fmt.Sprintf("ssh %s-p %s %s@%s", k, s.Port, s.User, s.Host)
}

// copyCmd: panoya kopyalanan komut. Knock etkinse önce kendi vuruşunu gönderir (zincirleme), böylece yapıştırıldığı anda geçerli olur.
func (a *App) copyCmd(s Server) string {
	if !s.Knock {
		return s.sshCmd()
	}
	return fmt.Sprintf("%s servers -servers %s knock-send %s && %s", shQuote(selfExe()), shQuote(a.serversDir), shQuote(s.Name), s.sshCmd())
}

func defaultStateDir() string {
	if d := envx.Get("STATE_DIR"); d != "" {
		return d
	}
	base := ""
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		base = x
	} else {
		h, _ := os.UserHomeDir()
		base = filepath.Join(h, ".local", "share")
	}
	// yeni ad keylab; eski sshpocket klasörü varsa (ve yenisi yoksa) veri taşınmadan onda devam edilir
	return envx.PickDir(filepath.Join(base, "keylab"), filepath.Join(base, "sshpocket"))
}

func main() {
	// ssh-add bizi SSH_ASKPASS olarak çağırdıysa: Keychain parolasını yaz ve çık (bkz. runner.AskpassMain)
	if handled, code := runner.AskpassMain(os.Stdout); handled {
		os.Exit(code)
	}
	if code, ok := subcommand(os.Args[1:]); ok {
		os.Exit(code)
	}
	servers := flag.String("servers", "", "sunucular klasörü (varsayılan: $KEYLAB_SERVERS, ikilinin yanındaki servers/ [taşınabilir], ~/.keylab/servers)")
	comps := flag.String("components", "", "components klasörü (varsayılan: sunucular klasörünün yanında)")
	state := flag.String("state", "", "durum/günlük klasörü (varsayılan: ~/.local/share/keylab; eski sshpocket klasörü varsa o)")
	noOpen := flag.Bool("no-open", false, "tarayıcıyı açma")
	browserMode := flag.String("browser", "auto", "tarayıcı: auto = Chrome/Edge/Brave/Chromium varsa KeyLab'a ait ayrı, eklentisiz profille uygulama penceresinde (yoksa sistem tarayıcısı); default = her zaman sistem tarayıcısı")
	flag.Parse()
	bm, okMode := normalizeBrowserMode(*browserMode)
	if !okMode {
		fmt.Fprintln(os.Stderr, "-browser auto ya da default olmalı")
		os.Exit(2)
	}
	cwd, _ := os.Getwd()
	sd, createDirs := resolveServersDir(*servers, envServers(), selfExe(), cwd)
	if sd == "" {
		fmt.Fprintln(os.Stderr, "sunucular klasörü bulunamadı ve ev klasörü bilinmiyor (-servers verin)")
		os.Exit(1)
	}
	cd := *comps
	if cd == "" {
		cd = filepath.Join(filepath.Dir(filepath.Clean(sd)), "components")
	}
	if createDirs { // kurulu mod, ilk açılış: ~/.keylab/{servers,components}
		if err := ensureHomeDirs(sd, cd); err != nil {
			fmt.Fprintln(os.Stderr, "veri klasörü oluşturulamadı:", err)
			os.Exit(1)
		}
		fmt.Println("ilk açılış: veri klasörü oluşturuldu:", filepath.Dir(sd))
	}
	st := *state
	if st == "" {
		st = defaultStateDir()
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tb := make([]byte, 24)
	_, _ = rand.Read(tb)
	bb := make([]byte, 24)
	_, _ = rand.Read(bb)
	app := &App{serversDir: sd, compDir: cd, stateDir: st, token: hex.EncodeToString(tb), bootTok: hex.EncodeToString(bb), host: ln.Addr().String(), up: map[string]bool{}, conn: map[string]string{}, jobs: map[string]*Job{}}
	app.loadUISettings()

	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", app.root(http.FileServer(http.FS(sub))))
	mux.HandleFunc("/api/home", app.guard(func(w http.ResponseWriter, r *http.Request) {
		pk, serial := app.packages()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"servers": app.servers(false), "packages": pk, "indexSerial": serial})
	}))
	// gömülü terminal (bkz. term.go): hepsi guard'lı; POST uç noktaları CSRF başlığı ister
	mux.HandleFunc("/api/term/open", app.guard(app.termOpenHandler))
	mux.HandleFunc("/api/term/ticket", app.guard(app.termTicketHandler))
	mux.HandleFunc("/api/term/stream", app.guard(app.termStreamHandler))
	mux.HandleFunc("/api/term/input", app.guard(app.termInputHandler))
	mux.HandleFunc("/api/term/resize", app.guard(app.termResizeHandler))
	mux.HandleFunc("/api/term/close", app.guard(app.termCloseHandler))
	mux.HandleFunc("/api/term/list", app.guard(app.termListHandler))
	mux.HandleFunc("/api/quit", app.guard(app.quitHandler))
	mux.HandleFunc("/api/status", app.guard(func(w http.ResponseWriter, r *http.Request) {
		detail := r.URL.Query().Get("detail") == "1" // ayrıntı: {ad: {up, conn}}; varsayılan eski biçim {ad: up}
		res := map[string]bool{}
		det := map[string]map[string]any{}
		for _, sv := range app.servers(true) {
			if sv.Checked {
				res[sv.Name] = sv.Up
				det[sv.Name] = map[string]any{"up": sv.Up, "conn": sv.Conn}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if detail {
			_ = json.NewEncoder(w).Encode(det)
			return
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
	mux.HandleFunc("/api/server", app.guard(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if !reName.MatchString(name) {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		s, ok := app.findServer(name)
		if !ok {
			http.Error(w, "sunucu yok", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		ki, _ := (&cli.Servers{Dir: app.serversDir}).KnockStatus(name) // dizinin değeri yok; yalnızca "bu Mac'te kayıtlı mı"
		_ = json.NewEncoder(w).Encode(map[string]any{"server": s, "packages": app.packageInfos(s), "knock": ki, "prep": app.prepState(s)})
	}))
	mux.HandleFunc("/api/lynis", app.guard(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("server")
		if !reName.MatchString(name) {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		report, list := app.findings(name)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"server": name, "report": report, "lynis": app.lynis(name), "findings": list})
	}))
	mux.HandleFunc("/api/packages", app.guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"packages": app.packageOverview(), "servers": app.servers(false)})
	}))
	mux.HandleFunc("/api/run", app.guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var req struct {
				Server, Package, Action, Item, Confirm string
				Snapshot                               bool
				AckDestructive                         bool
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil ||
				!reName.MatchString(req.Server) || !reName.MatchString(req.Package) || !reAction.MatchString(req.Action) ||
				(req.Item != "" && !reName.MatchString(req.Item)) {
				http.Error(w, "geçersiz istek", http.StatusBadRequest)
				return
			}
			j, code, err := app.startJob(req.Server, req.Package, req.Action, req.Item, req.Snapshot, req.Confirm, req.AckDestructive)
			if err != nil {
				http.Error(w, err.Error(), code)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": j.ID})
			return
		}
		id := r.URL.Query().Get("id")
		from := 0
		fmt.Sscanf(r.URL.Query().Get("from"), "%d", &from)
		app.jmu.Lock()
		defer app.jmu.Unlock()
		j, ok := app.jobs[id]
		if !ok {
			http.Error(w, "iş yok", http.StatusNotFound)
			return
		}
		if from < 0 || from > len(j.buf) {
			from = len(j.buf)
		}
		chunk := j.buf[from:]
		n := len(chunk)
		if !j.Done {
			n = validUTF8Prefix(chunk)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": j.ID, "server": j.Server, "package": j.Package, "action": j.Action, "started": j.Started,
			"text": maskLine2(string(chunk[:n])), "next": from + n, "done": j.Done, "exit": j.Exit})
	}))
	// iş bir istem bekliyorsa (parola, e/h, ad onayı) kullanıcının yazdığı satırı pty'ye gönderir. İçerik ASLA günlüğe yazılmaz.
	// iş parola bekliyorsa: sistem penceresinden alınan parola pty'ye yazılır ve bellekte sıfırlanır. Sayfa parolayı HİÇ görmez.
	mux.HandleFunc("/api/server-files", app.guard(func(w http.ResponseWriter, r *http.Request) {
		files, state, err := app.serverFiles(r.URL.Query().Get("name"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"files": files, "state": state})
	}))
	mux.HandleFunc("/api/server-rename", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Old, New string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		if code, err := app.renameServer(strings.TrimSpace(req.Old), strings.TrimSpace(req.New)); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	mux.HandleFunc("/api/server-remove", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name, Confirm string
			Ack           bool
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		j, code, err := app.removeServer(req.Name, req.Confirm, req.Ack)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": j.ID})
	}))
	mux.HandleFunc("/api/server-purge", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Name string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 512)).Decode(&req) != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		if code, err := app.purgeServerState(req.Name); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	mux.HandleFunc("/api/server-create", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Name, Host, User, Port, OS string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		if code, err := app.createServer(strings.TrimSpace(req.Name), strings.TrimSpace(req.Host), strings.TrimSpace(req.User), strings.TrimSpace(req.Port), strings.TrimSpace(req.OS)); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	mux.HandleFunc("/api/jobs", app.guard(app.jobsHandler))
	mux.HandleFunc("/api/ui-settings", app.guard(app.uiSettingsHandler))
	mux.HandleFunc("/api/batch", app.guard(app.batchHandler))
	mux.HandleFunc("/api/batch/options", app.guard(app.batchOptionsHandler))
	mux.HandleFunc("/api/batch/cancel", app.guard(app.batchCancelHandler))
	mux.HandleFunc("/api/backup-export", app.guard(app.backupExportHandler))
	mux.HandleFunc("/api/backup-import", app.guard(app.backupImportHandler))
	mux.HandleFunc("/api/ssh-config-hosts", app.guard(app.sshConfigHostsHandler))
	mux.HandleFunc("/api/ssh-config-import", app.guard(app.sshConfigImportHandler))
	mux.HandleFunc("/api/server-task", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Server, Task string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		j, code, err := app.startServerTask(req.Task, req.Server)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": j.ID})
	}))
	mux.HandleFunc("/api/updates", app.guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rep, err := app.checkUpdates()
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(rep)
	}))
	mux.HandleFunc("/api/upgrade", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Package, Version string
			Ack              bool
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil || !req.Ack {
			http.Error(w, "onay gerekli", http.StatusBadRequest)
			return
		}
		j, code, err := app.startUpgrade(req.Package, req.Version)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": j.ID})
	}))
	mux.HandleFunc("/api/vars", app.guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var req struct {
				Server, Package, Name, Value, Action string
				Ack                                  bool
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil ||
				!reName.MatchString(req.Server) || !reName.MatchString(req.Package) || (req.Action != "set" && req.Action != "unset") {
				http.Error(w, "geçersiz istek", http.StatusBadRequest)
				return
			}
			warn, code, err := app.saveVar(req.Server, req.Package, req.Name, req.Value, req.Action, req.Ack)
			if err != nil {
				w.WriteHeader(code)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "needAck": warn != "" && !req.Ack})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "warn": warn != ""})
			return
		}
		server, pkg := r.URL.Query().Get("server"), r.URL.Query().Get("package")
		if !reName.MatchString(server) || !reName.MatchString(pkg) {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		list, err := app.varSchemas(server, pkg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if list == nil {
			list = []VarSchema{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"server": server, "package": pkg, "vars": list})
	}))
	mux.HandleFunc("/api/run/askpass", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ ID, Prompt string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req) != nil {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		if !askpassAvailable() {
			http.Error(w, "sistem parola penceresi bu platformda yok", http.StatusNotImplemented)
			return
		}
		app.jmu.Lock()
		j, ok := app.jobs[req.ID]
		var in io.WriteCloser
		if ok && !j.Done {
			in = j.stdin
		}
		app.jmu.Unlock()
		if in == nil {
			http.Error(w, "çalışan iş yok", http.StatusNotFound)
			return
		}
		pw, err := askPassNative(req.Prompt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		_, werr := in.Write(append(pw, '\n'))
		for i := range pw { // bellekte sıfırla
			pw[i] = 0
		}
		if werr != nil {
			http.Error(w, "gönderilemedi", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	mux.HandleFunc("/api/run/input", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ ID, Text string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil || len(req.Text) > 256 || strings.ContainsAny(req.Text, "\r\n\x00") {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		app.jmu.Lock()
		j, ok := app.jobs[req.ID]
		var in io.WriteCloser
		if ok && !j.Done {
			in = j.stdin
		}
		app.jmu.Unlock()
		if in == nil {
			http.Error(w, "çalışan iş yok", http.StatusNotFound)
			return
		}
		if _, err := in.Write([]byte(req.Text + "\n")); err != nil {
			http.Error(w, "gönderilemedi", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	mux.HandleFunc("/api/run-terminal", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Server, Package, Action, Item, Confirm string
			Snapshot                               bool
			AckDestructive                         bool
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil ||
			!reName.MatchString(req.Server) || !reName.MatchString(req.Package) || !reAction.MatchString(req.Action) {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		if req.Item != "" { // macOS Terminal komutu öğe seçimi taşımaz: öğe eylemleri yalnız uygulama içinde (kapılı iş olarak) çalışır
			http.Error(w, "öğe eylemleri macOS Terminal'de açılamaz; uygulama içinde çalıştırın", http.StatusBadRequest)
			return
		}
		if _, code, err := app.authorizeRun(req.Server, req.Package, req.Action, req.Snapshot, req.Confirm, req.AckDestructive); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		if runtime.GOOS != "darwin" {
			http.Error(w, "yalnızca macOS", http.StatusNotImplemented)
			return
		}
		root := filepath.Dir(filepath.Clean(app.compDir))
		cmdline := terminalCommand(selfExe(), root, req.Package, req.Action, req.Server)
		script := "tell application \"Terminal\" to activate\ntell application \"Terminal\" to do script " + appleScriptQuote(cmdline)
		if err := exec.Command("osascript", "-e", script).Run(); err != nil {
			http.Error(w, "Terminal açılamadı", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	mux.HandleFunc("/api/run/cancel", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ ID string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		app.jmu.Lock()
		j, ok := app.jobs[req.ID]
		app.jmu.Unlock()
		if !ok || j.Done {
			http.Error(w, "çalışan iş yok", http.StatusNotFound)
			return
		}
		if j.Action == "upgrade" {
			http.Error(w, "paket kurulumu atomiktir; iptal edilemez", http.StatusConflict)
			return
		}
		j.cancel()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	mux.HandleFunc("/api/about", app.guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(app.aboutInfo())
	}))
	mux.HandleFunc("/api/profiles", app.guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(app.profiles())
	}))
	mux.HandleFunc("/api/server-draft", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Name, Host, User, Port, OS string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(app.draft(strings.TrimSpace(req.Name), strings.TrimSpace(req.Host), strings.TrimSpace(req.User), strings.TrimSpace(req.Port), strings.TrimSpace(req.OS)))
	}))
	mux.HandleFunc("/api/logs", app.guard(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(app.logs())
	}))
	mux.HandleFunc("/api/log", app.guard(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		pkg, srv, file := q.Get("package"), q.Get("server"), q.Get("file")
		if !reName.MatchString(pkg) || !reName.MatchString(srv) || !reFile.MatchString(file) {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		p := filepath.Join(app.stateDir, pkg, srv, filepath.FromSlash(file))
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() {
			http.Error(w, "yok", http.StatusNotFound)
			return
		}
		f, err := os.Open(p)
		if err != nil {
			http.Error(w, "açılamadı", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		buf := make([]byte, maxLogBytes)
		n, _ := f.Read(buf)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Log-Mtime", info.ModTime().UTC().Format(time.RFC3339Nano))
		_, _ = w.Write(buf[:n])
		if info.Size() > int64(n) {
			_, _ = w.Write([]byte("\n… (kısaltıldı: ilk 1 MB gösteriliyor)\n"))
		}
	}))
	mux.HandleFunc("/api/action", app.guard(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Name, Action string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			http.Error(w, "geçersiz", http.StatusBadRequest)
			return
		}
		s, ok := app.findServer(req.Name)
		if !ok || !s.Valid {
			http.Error(w, "sunucu yok", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Action {
		case "cmd":
			_ = json.NewEncoder(w).Encode(map[string]string{"cmd": app.copyCmd(s)})
		case "terminal":
			if runtime.GOOS != "darwin" {
				http.Error(w, "yalnızca macOS", http.StatusNotImplemented)
				return
			}
			// knock etkin ama dizi bu bilgisayarda yoksa Terminal'i hiç açma: nedeni arayüzde göster
			if ki, _ := (&cli.Servers{Dir: app.serversDir}).KnockStatus(s.Name); ki.Enabled && !ki.HasSeq {
				http.Error(w, "Knock açık ama dizi bu bilgisayarın Keychain'inde yok; Özet → Port knocking'den girin", http.StatusConflict)
				return
			}
			// Terminal'e knock'lı komut verilir (knock-send && ssh): "🚪 Knock gönderildi" Terminal'de de görünür
			esc := strings.ReplaceAll(strings.ReplaceAll(app.copyCmd(s), `\`, `\\`), `"`, `\"`)
			script := `tell application "Terminal" to activate` + "\n" + `tell application "Terminal" to do script "` + esc + `"`
			if err := exec.Command("osascript", "-e", script).Run(); err != nil {
				http.Error(w, "açılamadı", http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": "açıldı", "knock": s.Knock})
		default:
			http.Error(w, "bilinmeyen eylem", http.StatusBadRequest)
		}
	}))
	mux.HandleFunc("/api/knock", app.guard(app.knockHandler))
	mux.HandleFunc("/api/server-edit", app.guard(app.serverEditHandler))
	url := fmt.Sprintf("http://%s/?t=%s", app.host, app.token)
	fmt.Println("sshpocket (Go deneme):", url)
	fmt.Println("sunucular:", sd, "| components:", cd, "| durum:", st, "| durdurmak için Ctrl-C")
	if !*noOpen && runtime.GOOS == "darwin" {
		// komut satırına GERÇEK jeton değil, tek kullanımlık açılış jetonu yazılır (ps ile görünür); yazdırılan adres elle açmak içindir
		bootURL := fmt.Sprintf("http://%s/?b=%s", app.host, app.bootTok)
		if launchBrowser(bootURL, st, bm) {
			fmt.Println("tarayıcı: KeyLab'a ait ayrı profil, eklentiler kapalı (uygulama penceresi); profil:", filepath.Join(st, "browser-profile"))
		} else if bm == "auto" {
			fmt.Println("not: Chrome/Edge/Brave/Chromium bulunamadı; sistem tarayıcısı açıldı (eklentileriniz etkin olabilir). Ayrı profil için bunlardan birini kurun.")
		}
	}
	go app.reapTerms() // boşta/bitmiş terminal oturumlarını temizler
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.Serve(ln); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
