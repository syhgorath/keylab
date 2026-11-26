package main

// Görev terminali: paket eylemi (`keylab run …`) gömülü terminalde (xterm) bir PTY oturumu olarak çalışır. Sudo/ansible parolası ve motorun
// onay soruları DOĞRUDAN terminalde yazılır (ayrı parola kutusu ya da macOS Terminal.app gerekmez). Kapılar (önizleme, önkoşul, onay) sunucuda
// `authorizeRunItem` ile aynıdır; günlük ve sonuç kaydı yürütücünün işidir. Eylem arayüzde "ssh" oturumuyla aynı terminal penceresinin
// sekmesi olarak görünür; bitince sonuç kartı arayüzde yenilenir.

import (
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
func (a *App) openTask(server, pkg, action, item string, snapshot bool, confirm string, ackDestructive bool, rows, cols uint16) (*TermSession, int, error) {
	return a.openTaskX(false, server, pkg, action, item, snapshot, confirm, ackDestructive, rows, cols)
}

// openTaskX: chain=true tek tık zinciridir (önizleme → eylem → kontrol, `keylab run -chain`); kapıları authorizeChain denetler, onayları
// (yıkıcı eylemde sunucu adı) ve sudo parolasını yürütücü terminalde sorar.
func (a *App) openTaskX(chain bool, server, pkg, action, item string, snapshot bool, confirm string, ackDestructive bool, rows, cols uint16) (*TermSession, int, error) {
	if !platform.DefaultPTY().TerminalSupported() {
		return nil, http.StatusNotImplemented, fmt.Errorf("Windows'ta gömülü terminal henüz yok (WSL 2 kullanabilirsiniz)")
	}
	var itemVar string
	var code int
	var err error
	if chain {
		code, err = a.authorizeChain(server, pkg, action, item)
	} else {
		_, itemVar, code, err = a.authorizeRunItem(server, pkg, action, item, snapshot, confirm, ackDestructive)
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

	ts, err := startTermProcess(taskProgram(exe), argv[1:], root, taskEnv(), rows, cols) // komut adı kullanıcı girdisinden gelmez: kendi ikilimiz
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	ts.Server, ts.Kind, ts.Pkg, ts.Action = server, "task", pkg, action
	ts.Title = pkg + " · " + action
	a.tmu.Lock()
	a.terms[ts.ID] = ts
	a.tmu.Unlock()
	kind := "task:"
	if chain {
		kind = "chain:"
	}
	a.auditTerm(kind+pkg+"."+action, server, ts.ID)
	go ts.readLoop()
	go ts.waitLoop(a)
	return ts, 0, nil
}

// POST /api/term/task {server, package, action, item?, snapshot?, confirm?, ackDestructive?, rows?, cols?} → {id, server, kind, title, pkg, action}
func (a *App) termTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Server         string `json:"server"`
		Package        string `json:"package"`
		Action         string `json:"action"`
		Item           string `json:"item"`
		Snapshot       bool   `json:"snapshot"`
		Confirm        string `json:"confirm"`
		AckDestructive bool   `json:"ackDestructive"`
		Chain          bool   `json:"chain"`
		Rows           uint16 `json:"rows"`
		Cols           uint16 `json:"cols"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil ||
		!reName.MatchString(req.Server) || !reName.MatchString(req.Package) || !reAction.MatchString(req.Action) {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	if req.Rows < 4 || req.Rows > 300 {
		req.Rows = 24
	}
	if req.Cols < 10 || req.Cols > 600 {
		req.Cols = 80
	}
	ts, code, err := a.openTaskX(req.Chain, req.Server, req.Package, req.Action, req.Item, req.Snapshot, req.Confirm, req.AckDestructive, req.Rows, req.Cols)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": ts.ID, "server": ts.Server, "kind": ts.Kind, "title": ts.Title, "pkg": ts.Pkg, "action": ts.Action,
		"started": ts.Started.Format(time.RFC3339)})
}
