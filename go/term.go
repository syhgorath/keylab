package main

// Gömülü terminal: kayıtlı bir sunucuya `ssh -tt` ile PTY üzerinden bağlanır; çıktı SSE ile akar, girdi POST ile gelir.
// Güvenlik: tüm uç noktalar app.guard'ladır (jeton çerezi, Host denetimi, POST için CSRF başlığı). Yerel kabuk AÇILMAZ; yalnız kayıtlı ve
// doğrulanmış bir sunucuya ssh başlatılır (argüman listesiyle, kabuk yok; knock süreç içinde gönderilir). Terminal açma/kapama
// <durum klasörü>/terminal-audit.log dosyasına yazılır.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"keylab/internal/cli"
	"keylab/internal/servers"
)

const (
	maxTerms   = 4
	maxSubs    = 8 // oturum başına eşzamanlı akış sayısı
	termIdle   = 30 * time.Minute
	termDone   = 10 * time.Minute // bitmiş oturum haritada bu süreden sonra silinir
	ringMax    = 256 << 10        // 256 KB
	maxInBytes = 8192
)

type termEvent struct {
	Kind string // "data", "exit"
	Data string
}

type TermSession struct {
	ID      string
	Server  string
	Started time.Time

	mu         sync.Mutex // lastActive, ring, subs, done, exit
	lastActive time.Time
	ring       []byte
	subs       map[chan termEvent]struct{}
	done       bool
	exit       int

	wmu    sync.Mutex // PTY yazımı (mu'dan ayrı: yazma bloklanırsa çıktı akışı durmasın)
	ptmx   *os.File
	cmd    *exec.Cmd
	exited chan struct{} // süreç bitince kapanır
}

func appendRing(ring, chunk []byte, max int) []byte {
	if len(chunk) >= max {
		return append([]byte(nil), chunk[len(chunk)-max:]...)
	}
	res := append(ring, chunk...)
	if len(res) > max {
		res = res[len(res)-max:]
	}
	return res
}

// broadcast: mu tutulurken çağrılır.
func (s *TermSession) broadcast(ev termEvent) {
	for ch := range s.subs {
		select {
		case ch <- ev:
		default: // yavaş istemcide takılmamak için atla
		}
	}
}

func (s *TermSession) readLoop() {
	buf := make([]byte, 32<<10)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			s.lastActive = time.Now()
			s.ring = appendRing(s.ring, chunk, ringMax)
			s.broadcast(termEvent{Kind: "data", Data: base64.StdEncoding.EncodeToString(chunk)})
			s.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
}

func (s *TermSession) waitLoop(a *App) {
	werr := s.cmd.Wait()
	exitCode := 0
	if werr != nil {
		var ee *exec.ExitError
		if errors.As(werr, &ee) {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}
	s.mu.Lock()
	s.done = true
	s.exit = exitCode
	s.broadcast(termEvent{Kind: "exit", Data: fmt.Sprintf("%d", exitCode)})
	s.mu.Unlock()
	close(s.exited)
	_ = s.ptmx.Close()
	if a != nil {
		a.auditTerm("exit", s.Server, s.ID)
	}
}

// startTermProcess: argv'yi yeni bir PTY'nin ucunda başlatır. Ssh'tan bağımsızdır (testlenebilir).
func startTermProcess(argv []string, dir string, env []string, rows, cols uint16) (*TermSession, error) {
	ptmx, ttyName, err := openPTY()
	if err != nil {
		return nil, fmt.Errorf("PTY açılamadı: %w", err)
	}
	slave, err := openSlaveTTY(ttyName)
	if err != nil {
		_ = ptmx.Close()
		return nil, fmt.Errorf("slave PTY açılamadı: %w", err)
	}
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	_ = setWinsize(ptmx, rows, cols)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.Env = env
	setTermSysProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		_ = slave.Close()
		_ = ptmx.Close()
		return nil, fmt.Errorf("başlatılamadı: %w", err)
	}
	_ = slave.Close() // çocuk devraldı, ebeveynde kapat

	b := make([]byte, 12)
	_, _ = rand.Read(b)
	ts := &TermSession{
		ID: hex.EncodeToString(b), Started: time.Now().UTC(), lastActive: time.Now(),
		ptmx: ptmx, cmd: cmd, subs: map[chan termEvent]struct{}{}, exited: make(chan struct{}),
	}
	return ts, nil
}

// termEnv: ssh çocuğuna YALNIZ gereken ortam geçer (SSHPOCKET_*/KEYLAB_*/ANSIBLE_* ve diğer gizli değişkenler geçmez).
func termEnv() []string {
	env := []string{"TERM=xterm-256color", "COLORTERM=truecolor", "LANG=en_US.UTF-8"}
	for _, k := range []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "SSH_AUTH_SOCK"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func (a *App) auditTerm(event, server, id string) {
	if a.stateDir == "" {
		return
	}
	_ = os.MkdirAll(a.stateDir, 0o700)
	f, err := os.OpenFile(filepath.Join(a.stateDir, "terminal-audit.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\t%s\t%s\t%s\n", time.Now().UTC().Format(time.RFC3339), event, server, id)
}

func (a *App) openTerm(server string, rows, cols uint16) (*TermSession, int, error) {
	if runtime.GOOS == "windows" {
		return nil, http.StatusNotImplemented, fmt.Errorf("Windows'ta gömülü terminal henüz yok (WSL 2 kullanabilirsiniz)")
	}
	if !reName.MatchString(server) {
		return nil, http.StatusBadRequest, fmt.Errorf("geçersiz sunucu adı")
	}
	s, ok := a.findServer(server)
	if !ok || !s.Valid {
		return nil, http.StatusNotFound, fmt.Errorf("sunucu yok ya da yapılandırması geçersiz")
	}
	if err := servers.ValidateTarget(s.Host, s.User, s.Port); err != nil { // veri katmanının gevşek düzenli ifadesine ek, sıkı doğrulama
		return nil, http.StatusBadRequest, fmt.Errorf("sunucu bilgisi geçersiz: %v", err)
	}

	// açılışlar sıraya girer: sayım ile ekleme arasında yarış olmaz
	a.topen.Lock()
	defer a.topen.Unlock()
	a.tmu.Lock()
	if a.terms == nil {
		a.terms = map[string]*TermSession{}
	}
	if len(a.terms) >= maxTerms {
		for id, ts := range a.terms { // bitmişleri temizle
			ts.mu.Lock()
			done := ts.done
			ts.mu.Unlock()
			if done {
				delete(a.terms, id)
			}
		}
	}
	full := len(a.terms) >= maxTerms
	a.tmu.Unlock()
	if full {
		return nil, http.StatusConflict, fmt.Errorf("en fazla %d terminal oturumu açılabilir", maxTerms)
	}

	// Knock (etkin değilse hiçbir şey yapmaz): süreç içinde gönderilir, kabuk kullanılmaz
	kctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := cli.SendKnock(kctx, a.serversDir, server); err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("knock gönderilemedi: %v", err)
	}

	key := s.keyPath
	if env := parseEnv(filepath.Join(a.serversDir, server, "server.env")); env["KEY"] != "" {
		k := env["KEY"]
		if strings.HasPrefix(k, "~") {
			h, _ := os.UserHomeDir()
			k = h + k[1:]
		}
		if filepath.IsAbs(k) && !strings.ContainsAny(k, "\n\r") {
			key = k
		}
	}
	argv := []string{"ssh", "-tt", "-o", "ServerAliveInterval=30"}
	if st, err := os.Stat(key); err == nil && st.Mode().IsRegular() {
		argv = append(argv, "-i", key)
	}
	argv = append(argv, "-p", s.Port, s.User+"@"+s.Host) // user@host her zaman harfle başlar: seçenek sayılamaz

	ts, err := startTermProcess(argv, a.serversDir, termEnv(), rows, cols)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	ts.Server = server
	a.tmu.Lock()
	a.terms[ts.ID] = ts
	a.tmu.Unlock()
	a.auditTerm("open", server, ts.ID)
	go ts.readLoop()
	go ts.waitLoop(a)
	return ts, 0, nil
}

func (a *App) getTerm(id string) *TermSession {
	a.tmu.Lock()
	defer a.tmu.Unlock()
	return a.terms[id] // nil harita okuması güvenli
}

// kill: önce SIGHUP, 2 sn içinde çıkmazsa SIGKILL (yalnız süreç hâlâ bu oturuma aitse: pid yeniden kullanımına karşı).
func (s *TermSession) kill() {
	select {
	case <-s.exited:
		return
	default:
	}
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	pid := s.cmd.Process.Pid
	termSignalGroup(pid, false)
	go func() {
		select {
		case <-s.exited:
		case <-time.After(2 * time.Second):
			termSignalGroup(pid, true)
		}
	}()
}

func (a *App) closeTerm(id string) bool {
	ts := a.getTerm(id)
	if ts == nil {
		return false
	}
	ts.kill() // a.tmu tutulmadan
	return true
}

// reapOnce: boşta ya da bitmiş oturumları haritadan çıkarır ve (kilit DIŞINDA) sonlandırır.
func (a *App) reapOnce(now time.Time, idleAfter, doneAfter time.Duration) {
	var victims []*TermSession
	a.tmu.Lock()
	for id, ts := range a.terms {
		ts.mu.Lock()
		idle := now.Sub(ts.lastActive) > idleAfter && len(ts.subs) == 0
		gone := ts.done && now.Sub(ts.Started) > doneAfter
		ts.mu.Unlock()
		if idle || gone {
			victims = append(victims, ts)
			delete(a.terms, id)
		}
	}
	a.tmu.Unlock()
	for _, ts := range victims {
		ts.kill()
	}
}

func (a *App) reapTerms() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		a.reapOnce(time.Now(), termIdle, termDone)
	}
}

// subscribe: akışa abone olur; sınır aşılırsa ok=false. replay: o ana dek çıktı.
func (s *TermSession) subscribe() (ch chan termEvent, replay []byte, done bool, exit int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.subs) >= maxSubs {
		return nil, nil, false, 0, false
	}
	ch = make(chan termEvent, 256)
	s.subs[ch] = struct{}{}
	s.lastActive = time.Now()
	return ch, append([]byte(nil), s.ring...), s.done, s.exit, true
}

func (s *TermSession) unsubscribe(ch chan termEvent) {
	s.mu.Lock()
	delete(s.subs, ch)
	s.mu.Unlock()
}

// write: PTY'ye girdi yazar; kapalıysa false.
func (s *TermSession) write(data []byte) (bool, error) {
	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		return false, nil
	}
	s.lastActive = time.Now()
	s.mu.Unlock()
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_, err := s.ptmx.Write(data)
	return true, err
}

// HTTP uç noktaları

func (a *App) termOpenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Server string `json:"server"`
		Rows   uint16 `json:"rows"`
		Cols   uint16 `json:"cols"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req); err != nil {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	if req.Rows < 4 || req.Rows > 300 {
		req.Rows = 24
	}
	if req.Cols < 10 || req.Cols > 600 {
		req.Cols = 80
	}
	ts, code, err := a.openTerm(req.Server, req.Rows, req.Cols)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": ts.ID, "server": ts.Server, "started": ts.Started.Format(time.RFC3339)})
}

func (a *App) termStreamHandler(w http.ResponseWriter, r *http.Request) {
	// akış kısa ömürlü, tek kullanımlık bilet ister (bkz. session.go); EventSource özel başlık gönderemez
	if !a.redeemTermTicket(r.URL.Query().Get("ticket"), r.URL.Query().Get("id")) {
		http.Error(w, "akış bileti geçersiz ya da süresi dolmuş", http.StatusForbidden)
		return
	}
	ts := a.getTerm(r.URL.Query().Get("id"))
	if ts == nil {
		http.Error(w, "oturum yok", http.StatusNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "akış desteklenmiyor", http.StatusInternalServerError)
		return
	}
	ch, replay, isDone, exitCode, ok := ts.subscribe()
	if !ok {
		http.Error(w, "bu oturumun akış sınırına ulaşıldı", http.StatusTooManyRequests)
		return
	}
	defer ts.unsubscribe(ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	if len(replay) > 0 { // geçmiş çıktıyı ilk olay olarak yolla
		fmt.Fprintf(w, "event: data\ndata: %s\n\n", base64.StdEncoding.EncodeToString(replay))
		flusher.Flush()
	}
	if isDone {
		fmt.Fprintf(w, "event: exit\ndata: %d\n\n", exitCode)
		flusher.Flush()
		return
	}
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case ev := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, ev.Data)
			flusher.Flush()
			if ev.Kind == "exit" {
				return
			}
		}
	}
}

func (a *App) termInputHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID   string `json:"id"`
		Data string `json:"data"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxInBytes+1024)).Decode(&req); err != nil {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	if len(req.Data) > maxInBytes {
		http.Error(w, "veri çok büyük", http.StatusBadRequest)
		return
	}
	ts := a.getTerm(req.ID)
	if ts == nil {
		http.Error(w, "oturum yok", http.StatusNotFound)
		return
	}
	open, err := ts.write([]byte(req.Data))
	if !open {
		http.Error(w, "oturum kapalı", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "gönderilemedi", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *App) termResizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID   string `json:"id"`
		Rows uint16 `json:"rows"`
		Cols uint16 `json:"cols"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, "geçersiz", http.StatusBadRequest)
		return
	}
	if req.Rows < 4 || req.Rows > 300 || req.Cols < 10 || req.Cols > 600 {
		http.Error(w, "geçersiz boyut", http.StatusBadRequest)
		return
	}
	ts := a.getTerm(req.ID)
	if ts == nil {
		http.Error(w, "oturum yok", http.StatusNotFound)
		return
	}
	ts.wmu.Lock()
	err := setWinsize(ts.ptmx, req.Rows, req.Cols)
	ts.wmu.Unlock()
	if err != nil {
		http.Error(w, "boyut ayarlanamadı", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *App) termCloseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, "geçersiz", http.StatusBadRequest)
		return
	}
	if !a.closeTerm(req.ID) {
		http.Error(w, "oturum yok", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *App) termListHandler(w http.ResponseWriter, r *http.Request) {
	a.tmu.Lock()
	res := make([]map[string]any, 0, len(a.terms))
	for _, ts := range a.terms {
		ts.mu.Lock()
		res = append(res, map[string]any{"id": ts.ID, "server": ts.Server, "started": ts.Started.Format(time.RFC3339), "done": ts.done})
		ts.mu.Unlock()
	}
	a.tmu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
