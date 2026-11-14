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
	"io"
	"keylab/internal/eject"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// usbRoot: yol /Volumes/<ad>/... ise "/Volumes/<ad>", değilse "".
func usbRoot(dir string) string {
	p := filepath.ToSlash(filepath.Clean(dir))
	const pre = "/Volumes/"
	if !strings.HasPrefix(p, pre) {
		return ""
	}
	name := strings.SplitN(strings.TrimPrefix(p, pre), "/", 2)[0]
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return pre + name
}

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
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if running > 0 && !req.Force {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "running", "jobs": running})
		return
	}
	terms := a.killAllTerms()
	usb := usbRoot(a.serversDir)
	var log bytes.Buffer
	ejected, deferred := false, false
	if usb != "" {
		deferred = ejectDeferred(usb)
		eject := a.ejectFn
		if eject == nil {
			eject = ejectUSB
		}
		ejected = eject(&log, usb) == 0
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "terms": terms, "usb": usbName(usb), "ejected": ejected && !deferred, "deferred": ejected && deferred, "log": strings.TrimSpace(log.String())})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(500 * time.Millisecond) // yanıt tarayıcıya ulaşsın
		if a.exitFn != nil {
			a.exitFn()
			return
		}
		os.Exit(0)
	}()
}

// ejectUSB: gerçek çıkarma (testte App.ejectFn ile değiştirilir).
func ejectUSB(w io.Writer, usb string) int { return eject.Run(w, usb) }

// ejectDeferred: KeyLab o USB'nin kendisinden mi çalışıyor? Öyleyse çıkarma uygulama kapandıktan sonra yapılır (bkz. internal/eject/defer.go);
// testte değiştirilir.
var ejectDeferred = eject.WillDefer
