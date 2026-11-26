package main

// Görev terminali: paket eylemi (`keylab run …`) gömülü terminalde (xterm) bir PTY oturumu olarak çalışır. Sudo/ansible parolası ve motorun
// onay soruları DOĞRUDAN terminalde yazılır (ayrı parola kutusu ya da macOS Terminal.app gerekmez). Kapılar (önizleme, önkoşul, onay) sunucuda
// `authorizeRunItem` ile aynıdır; günlük ve sonuç kaydı yürütücünün işidir. Eylem arayüzde "ssh" oturumuyla aynı terminal penceresinin
// sekmesi olarak görünür; bitince sonuç kartı arayüzde yenilenir.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"keylab/internal/platform"
)

// taskEnv: eylem sürecinin ortamı: terminal ortamı (PATH, HOME, SSH_AUTH_SOCK…) + KeyLab'ın kendi ayarları (KEYLAB_*/SSHPOCKET_*: önbellek,
// known_hosts, misafir mod). Renkli çıktı terminalde istenir (TERM xterm-256color).
func taskEnv() []string {
	env := termEnv()
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "KEYLAB_") || strings.HasPrefix(kv, "SSHPOCKET_") {
			env = append(env, kv)
		}
	}
	return env
}

// runningTasks: çalışmakta olan görev terminallerinin sayısı (Çıkış onayı için).
func (a *App) runningTasks() int {
	a.tmu.Lock()
	defer a.tmu.Unlock()
	n := 0
	for _, ts := range a.terms {
		ts.mu.Lock()
		if ts.Kind == "task" && !ts.done {
			n++
		}
		ts.mu.Unlock()
	}
	return n
}

// taskProgram: çalıştırılacak komut adı (üretimde kendi ikilimiz; testte sahte komutla değiştirilir). Kullanıcı değerleri yalnız ayrı argümanlardır, kabuk yok.
var taskProgram = func(exe string) string { return exe }

// openTask: eylemi görev terminali olarak başlatır. Aynı sunucuda aynı anda tek görev; toplam oturum sınırı ssh oturumlarıyla ortaktır.
func (a *App) openTask(server, pkg, action, item string, rows, cols uint16) (*TermSession, int, error) {
	return a.openTaskX(false, server, pkg, action, item, rows, cols)
}

// openTaskX: chain=true tek tık zinciridir (önizleme → eylem → kontrol, `keylab run -chain`); kapıları authorizeChain denetler, onayları
// (yıkıcı eylemde sunucu adı) ve sudo parolasını yürütücü terminalde sorar.
func (a *App) openTaskX(chain bool, server, pkg, action, item string, rows, cols uint16) (*TermSession, int, error) {
	if !platform.DefaultPTY().TerminalSupported() {
		return nil, http.StatusNotImplemented, fmt.Errorf("Windows'ta gömülü terminal henüz yok (WSL 2 kullanabilirsiniz)")
	}
	var itemVar string
	var code int
	var err error
	if chain {
		code, err = a.authorizeChain(server, pkg, action, item)
	} else {
		itemVar, code, err = a.authorizeTerm(server, pkg, action, item)
	}
	if err != nil {
		return nil, code, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("ikilinin yolu bulunamadı: %w", err)
	}
	argv := jobArgv(exe, a.serversDir, a.compDir, a.stateDir, pkg, action, server, itemVar, item) // testte değiştirilebilir (bkz. jobArgv)
	if chain {
		argv = append([]string{argv[0], argv[1], "-chain"}, argv[2:]...)
	}
	root := filepath.Dir(filepath.Clean(a.compDir))

	kind := "task:"
	if chain {
		kind = "chain:"
	}
	// komut adı kullanıcı girdisinden gelmez: kendi ikilimiz
	return a.startTask(server, pkg, action, pkg+" · "+action, kind+pkg+"."+action, taskProgram(exe), argv[1:], root, rows, cols)
}

// startTask: görev terminali başlatır. Aynı sunucuda aynı anda tek görev; toplam oturum sınırı ssh oturumlarıyla ortaktır.
// program sabit bir ad ya da kendi ikilimizdir; istekten türeyen değerler yalnız args içindedir (kabuk yok).
func (a *App) startTask(server, pkg, action, title, audit, program string, args []string, dir string, rows, cols uint16) (*TermSession, int, error) {
	a.topen.Lock()
	defer a.topen.Unlock()
	if err := a.termSlotCheck(); err != nil {
		return nil, http.StatusConflict, err
	}
	a.tmu.Lock()
	for _, ts := range a.terms {
		ts.mu.Lock()
		busy := ts.Kind == "task" && !ts.done && ts.Server == server
		ts.mu.Unlock()
		if busy {
			a.tmu.Unlock()
			return nil, http.StatusConflict, fmt.Errorf("bu sunucuda zaten çalışan bir eylem var")
		}
	}
	a.tmu.Unlock()

	ts, err := startTermProcess(program, args, dir, taskEnv(), rows, cols)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	ts.Server, ts.Kind, ts.Pkg, ts.Action = server, "task", pkg, action
	ts.Title = title
	a.tmu.Lock()
	a.terms[ts.ID] = ts
	a.tmu.Unlock()
	a.auditTerm(audit, server, ts.ID)
	go ts.readLoop()
	go ts.waitLoop(a)
	return ts, 0, nil
}

// openServerTask: sunucu hazırlık görevi (anahtar üret/gönder, ajana ekle, Keychain, knock) görev terminalinde; soruları orada yanıtlanır.
func (a *App) openServerTask(task, server string, rows, cols uint16) (*TermSession, int, error) {
	if !platform.DefaultPTY().TerminalSupported() {
		return nil, http.StatusNotImplemented, fmt.Errorf("Windows'ta gömülü terminal henüz yok (WSL 2 kullanabilirsiniz)")
	}
	program, args, code, err := a.serverTaskCmd(task, server)
	if err != nil {
		return nil, code, err
	}
	return a.startTask(server, "", task, task, "server:"+task, program, args, a.engineRoot(), rows, cols)
}

// openRemove: sunucu silme görev terminalinde. Motor "sunucu adını yazın" diye sorar: arayüzde birebir yazılan ve burada yeniden denetlenen
// ad o istem belirince iletilir; sonraki sorular (Keychain kaydı) terminalde yanıtlanır.
func (a *App) openRemove(name, confirm string, ack bool, rows, cols uint16) (*TermSession, int, error) {
	if !platform.DefaultPTY().TerminalSupported() {
		return nil, http.StatusNotImplemented, fmt.Errorf("Windows'ta gömülü terminal henüz yok (WSL 2 kullanabilirsiniz)")
	}
	if code, err := a.authorizeRemove(name, confirm, ack); err != nil {
		return nil, code, err
	}
	ts, code, err := a.startTask(name, "", "remove", "remove", "server:remove", taskProgram(selfExe()), []string{"server-remove", a.serversDir, name}, a.engineRoot(), rows, cols)
	if err != nil {
		return nil, code, err
	}
	go answerWhenAsked(ts, "sunucu adını yazın", name+"\n", 20*time.Second)
	return ts, 0, nil
}

// answerWhenAsked: çıktıda istem belirince yanıtı terminale yazar (oturum biterse ya da süre dolarsa vazgeçer).
func answerWhenAsked(ts *TermSession, prompt, answer string, limit time.Duration) {
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		ts.mu.Lock()
		has, done := bytes.Contains(ts.ring, []byte(prompt)), ts.done
		ts.mu.Unlock()
		if has {
			_, _ = ts.write([]byte(answer))
			return
		}
		if done {
			return
		}
	}
}

// POST /api/term/task {server, package, action, item?, chain?, rows?, cols?} (paket eylemi)
// ya da {server, task, confirm?, ack?, rows?, cols?} (sunucu görevi; task="remove": sunucu silme) → {id, server, kind, title, pkg, action}
func (a *App) termTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Server  string `json:"server"`
		Package string `json:"package"`
		Action  string `json:"action"`
		Item    string `json:"item"`
		Chain   bool   `json:"chain"`
		Task    string `json:"task"`
		Confirm string `json:"confirm"` // yalnız silme: birebir yazılan sunucu adı
		Ack     bool   `json:"ack"`     // yalnız silme: özel anahtarın silineceği onayı
		Rows    uint16 `json:"rows"`
		Cols    uint16 `json:"cols"`
	}
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)
	packaged := reName.MatchString(req.Package) && reAction.MatchString(req.Action)
	server := reAction.MatchString(req.Task) && req.Package == "" && req.Action == "" && !req.Chain
	if err != nil || !reName.MatchString(req.Server) || (req.Task == "" && !packaged) || (req.Task != "" && !server) {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	if req.Rows < 4 || req.Rows > 300 {
		req.Rows = 24
	}
	if req.Cols < 10 || req.Cols > 600 {
		req.Cols = 80
	}
	var ts *TermSession
	var code int
	switch req.Task {
	case "":
		ts, code, err = a.openTaskX(req.Chain, req.Server, req.Package, req.Action, req.Item, req.Rows, req.Cols)
	case "remove":
		ts, code, err = a.openRemove(req.Server, req.Confirm, req.Ack, req.Rows, req.Cols)
	default:
		ts, code, err = a.openServerTask(req.Task, req.Server, req.Rows, req.Cols)
	}
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": ts.ID, "server": ts.Server, "kind": ts.Kind, "title": ts.Title, "pkg": ts.Pkg, "action": ts.Action,
		"started": ts.Started.Format(time.RFC3339)})
}
