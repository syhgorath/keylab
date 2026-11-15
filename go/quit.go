package main

// Çıkış düğmesi (POST /api/quit): açık terminalleri kapatır, USB'den çalışılıyorsa `keylab eject` işini yapar (USB'deki anahtarları
// ssh-agent'tan siler, USB'yi çıkarır), sonra uygulamayı durdurur. Keychain kayıtlarına dokunulmaz (eski kabuktaki "Çıkış" ile aynı değil:
// o, sahibin parolalarını silmez).
//
// Çalışan eylem işi varsa 409 döner; arayüz onay isterse force=true ile yeniden gönderir. USB tespiti: sunucular klasörü
// /Volumes/<ad>/... altındaysa o birim (macOS); değilse çıkarılacak bir şey yok, yalnız uygulama kapanır.
// Not: ikili çıkarılacak USB'nin KENDİSİNDEN çalışıyorsa (KeyLab.app USB'de) disk sürecin altından çekilmez: çıkarma uygulama kapandıktan
// sonra ayrı bir yardımcı süreçle yapılır (yanıtta "deferred": true; bkz. internal/eject/defer.go). Eskiden zorla çıkarılan birimdeki süreç ölmek
// yerine %400+ CPU ile döngüye giriyordu.

import (
	"bytes"
	"encoding/json"
	"keylab/internal/platform"
	"keylab/internal/vault"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// usbName: birim adı ("" ise USB yok).
func usbName(usb string) string {
	if usb == "" {
		return ""
	}
	return filepath.Base(usb)
}

// killAllTerms: tüm gömülü terminal oturumlarını sonlandırır (kilit DIŞINDA); sayı döner.
func (a *App) killAllTerms() int {
	a.tmu.Lock()
	all := make([]*TermSession, 0, len(a.terms))
	for _, ts := range a.terms {
		all = append(all, ts)
	}
	a.tmu.Unlock()
	for _, ts := range all {
		ts.kill()
	}
	return len(all)
}

func (a *App) quitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Force bool `json:"force"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	a.jmu.Lock()
	running := len(a.runningLocked())
	a.jmu.Unlock()
	running += a.runningTasks() // görev terminalleri (xterm) de çalışan iş sayılır
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if running > 0 && !req.Force {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "running", "jobs": running})
		return
	}
	if !a.quitStart() {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "quitting"})
		return
	}
	// Çıkış adım adım yapılır ve her adım kaydedilir (GET /api/quit/status): arayüz canlı gösterir. Süreç burada DURMAZ: kullanıcı "Kapat"a
	// (POST /api/quit/finish) basınca ya da 5 dk sonra kapanır; böylece pencere, yapılanları okuyabilirsiniz diye açık kalır.
	a.quitPause()
	a.qBegin("terms")
	terms := a.killAllTerms()
	if terms > 0 {
		a.qEnd("terms", "done", terms, "")
	} else {
		a.qEnd("terms", "skipped", 0, "")
	}
	a.quitPause()
	if m := a.vaultM(); m.enabled() && vaultExists(m) {
		a.qBegin("vault")
		m.lock()
		a.qEnd("vault", "done", 0, "")
		a.quitPause()
	}
	vol := a.volumesOS()
	usb := vol.MountRoot(a.serversDir)
	var log bytes.Buffer
	ejected, deferred := false, false
	if usb != "" {
		a.qBegin("usb")
		deferred = vol.WillDefer(usb)
		ejected = vol.Eject(&log, usb) == 0
		switch {
		case ejected && deferred:
			a.qEnd("usb", "deferred", 0, usbName(usb))
		case ejected:
			a.qEnd("usb", "done", 0, usbName(usb))
		default:
			a.qEnd("usb", "error", 0, usbName(usb))
		}
		a.quitPause()
	}
	// özel ajan ve geçici klasör (venv, tarayıcı profili): eject'ten SONRA (eject USB anahtarlarını ajandan siler)
	cleaned := a.runCleanupsSteps(func(label string) { a.quitPause(); a.qBegin(label) }, func(label string) { a.qEnd(label, "done", 0, "") })
	a.quitPause()
	a.qBegin("keychain")
	if platform.GuestMode() {
		a.qEnd("keychain", "info", 0, "guest") // bu Mac'in Keychain'i ve ssh-agent'ı hiç kullanılmadı
	} else {
		a.qEnd("keychain", "info", 0, "kept") // Keychain kayıtlarına dokunulmadı
	}
	a.quitDone()
	resp := map[string]any{"ok": true, "terms": terms, "usb": usbName(usb), "ejected": ejected && !deferred, "deferred": ejected && deferred, "log": strings.TrimSpace(log.String()), "steps": a.quitSteps()}
	if a.isPortable() {
		resp["report"] = quitReport(terms, cleaned, platform.GuestMode(), platform.SecretsSupported())
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// quitReport: taşınabilir modda Çıkış sonrası özet: temizlenenler ve bu bilgisayarda KALANLAR. Kodlar arayüzde çevrilir.
//
//	cleaned: terms (açık terminaller kapatıldı), agent (özel ssh-agent sonlandı), tmp (geçici klasör: venv + tarayıcı profili silindi)
//	remains: macos (sistemin kendi izleri), keychain (Keychain kayıtları: yalnız misafir olmayan taşınabilirde), user_agent (kullanıcının ssh-agent'ına eklenenler)
func quitReport(terms int, cleaned []string, guest, secrets bool) map[string]any {
	c := []string{}
	if terms > 0 {
		c = append(c, "terms")
	}
	c = append(c, cleaned...)
	r := []string{"macos"}
	if !guest {
		if secrets {
			r = append(r, "keychain")
		}
		r = append(r, "user_agent")
	}
	return map[string]any{"cleaned": c, "remains": r}
}

// isPortable: uygulama taşınabilir düzende mi (servers/ yanında)? Testte App.portableFn ile değiştirilir.
func (a *App) isPortable() bool {
	if a.portableFn != nil {
		return a.portableFn()
	}
	return isPortableLayout(a.serversDir, selfExe())
}

// ---------- adım kaydı ----------

type quitStep struct {
	ID     string `json:"id"`               // terms | vault | usb | agent | tmp | keychain
	State  string `json:"state"`            // running | done | skipped | deferred | error | info
	N      int    `json:"n,omitempty"`      // terms: kapatılan oturum sayısı
	Detail string `json:"detail,omitempty"` // usb: birim adı; keychain: guest | kept
}

func (a *App) quitStart() bool {
	a.qmu.Lock()
	defer a.qmu.Unlock()
	if a.qstate != "" {
		return false
	}
	a.qstate, a.qsteps = "running", nil
	return true
}

func (a *App) qBegin(id string) {
	a.qmu.Lock()
	a.qsteps = append(a.qsteps, quitStep{ID: id, State: "running"})
	a.qmu.Unlock()
}

func (a *App) qEnd(id, state string, n int, detail string) {
	a.qmu.Lock()
	defer a.qmu.Unlock()
	for i := len(a.qsteps) - 1; i >= 0; i-- {
		if a.qsteps[i].ID == id {
			a.qsteps[i].State, a.qsteps[i].N, a.qsteps[i].Detail = state, n, detail
			return
		}
	}
	a.qsteps = append(a.qsteps, quitStep{ID: id, State: state, N: n, Detail: detail})
}

func (a *App) quitSteps() []quitStep {
	a.qmu.Lock()
	defer a.qmu.Unlock()
	return append([]quitStep(nil), a.qsteps...)
}

// quitDone: adımlar bitti; süreç "Kapat" ile ya da quitAutoExit sonra kapanır (pencere okunabilsin diye hemen kapanmaz).
func (a *App) quitDone() {
	a.qmu.Lock()
	a.qstate = "done"
	a.qmu.Unlock()
	d := quitAutoExit
	if a.quitAutoExitAfter > 0 {
		d = a.quitAutoExitAfter
	}
	time.AfterFunc(d, a.exitNow)
}

const quitAutoExit = 5 * time.Minute

// quitPause: adımlar gözle izlenebilsin diye adımlar arası kısa bekleme (testte a.quitPauseFn ile kapatılır).
func (a *App) quitPause() {
	if a.quitPauseFn != nil {
		a.quitPauseFn()
		return
	}
	time.Sleep(220 * time.Millisecond)
}

func (a *App) exitNow() {
	a.runCleanups() // (adımlarda zaten çalıştı; sinyalle gelen çıkışlar için güvence)
	if a.exitFn != nil {
		a.exitFn()
		return
	}
	os.Exit(0)
}

// GET /api/quit/status → {state: "" | running | done, steps: [...]}
func (a *App) quitStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET", http.StatusMethodNotAllowed)
		return
	}
	a.qmu.Lock()
	st := a.qstate
	a.qmu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"state": st, "steps": a.quitSteps()})
}

// POST /api/quit/finish: adımlar bittiyse yanıtı verip süreci kapatır (pencere de kapanır); bitmediyse 409.
func (a *App) quitFinishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	a.qmu.Lock()
	st := a.qstate
	a.qmu.Unlock()
	if st != "done" {
		http.Error(w, "çıkış adımları henüz bitmedi", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(500 * time.Millisecond) // yanıt tarayıcıya ulaşsın
		a.exitNow()
	}()
}

func vaultExists(m *vaultMgr) bool { return vault.Exists(m.path) }
