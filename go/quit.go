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
	terms := a.killAllTerms()
	vol := a.volumesOS()
	usb := vol.MountRoot(a.serversDir)
	var log bytes.Buffer
	ejected, deferred := false, false
	if usb != "" {
		deferred = vol.WillDefer(usb)
		ejected = vol.Eject(&log, usb) == 0
	}
	cleaned := a.runCleanups() // özel ajan ve geçici klasör (venv, tarayıcı profili): eject'ten SONRA (eject USB anahtarlarını ajandan siler)
	resp := map[string]any{"ok": true, "terms": terms, "usb": usbName(usb), "ejected": ejected && !deferred, "deferred": ejected && deferred, "log": strings.TrimSpace(log.String())}
	if a.isPortable() {
		resp["report"] = quitReport(terms, cleaned, platform.GuestMode(), platform.SecretsSupported())
	}
	_ = json.NewEncoder(w).Encode(resp)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(500 * time.Millisecond) // yanıt tarayıcıya ulaşsın
		a.runCleanups()                    // (yanıtta zaten çalıştı; sinyalle gelen çıkışlar için güvence)
		if a.exitFn != nil {
			a.exitFn()
			return
		}
		os.Exit(0)
	}()
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
