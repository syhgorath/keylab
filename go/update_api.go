package main

import (
	"encoding/json"
	"keylab/internal/cli"
	"keylab/internal/platform"
	"keylab/internal/selfupdate"
	"keylab/internal/trust"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Uygulama içi güncelleme (macOS KeyLab.app). GET /api/update/check: son sürüm + GitHub sürüm notları (yalnız kullanıcı isteyince; KeyLab kendiliğinden internete çıkmaz).
// POST /api/update/apply: paketi indirir, release anahtarıyla imzasını doğrular, açar, doğrular, .app'i BÜTÜNÜYLE değiştirir (eskisi yedeklenir) ve yeniden başlatır.
// GET /api/update/status: apply sürerken adım adım durum. Terminal sürümü için `keylab update` komutu (selfupdate) aynen durur.

func (a *App) newUpdater() *selfupdate.Updater {
	if a.updaterFn != nil {
		return a.updaterFn()
	}
	root := findRoot("")
	return &selfupdate.Updater{SignersFile: signersPath(root, "release_signers"), Version: versionOf(root), Head: cli.HTTPHead}
}

// updateTarget: güncellenecek .app yolu; olmazsa reason (arayüz çevirir): not_bundle | translocated | not_writable.
func (a *App) updateTarget() (app, reason string) {
	if a.updateTargetFn != nil {
		return a.updateTargetFn()
	}
	exe, err := filepath.EvalSymlinks(selfExe())
	if err != nil {
		exe = selfExe()
	}
	app, ok := platform.AppBundlePath(exe)
	if !ok {
		return "", "not_bundle" // komut satırı ikilisi: `keylab update`
	}
	if platform.IsTranslocated(app) {
		return "", "translocated"
	}
	f, err := os.CreateTemp(filepath.Dir(app), ".keylab-wtest-")
	if err != nil {
		return "", "not_writable"
	}
	f.Close()
	os.Remove(f.Name())
	return app, ""
}

type updateCheck struct {
	Current  string `json:"current"`
	Latest   string `json:"latest"`
	Newer    bool   `json:"newer"`
	Notes    string `json:"notes,omitempty"`
	CanApply bool   `json:"canApply"`
	Reason   string `json:"reason,omitempty"`
}

func (a *App) updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET", http.StatusMethodNotAllowed)
		return
	}
	u := a.newUpdater()
	latest, ok := u.Latest()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if !ok {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "offline"})
		return
	}
	info := updateCheck{Current: u.Version, Latest: latest, Newer: trust.VersionCmp(latest, u.Version) > 0}
	if notes, err := u.ReleaseNotes(latest); err == nil {
		info.Notes = notes
	}
	if info.Newer {
		app, reason := a.updateTarget()
		info.CanApply, info.Reason = app != "", reason
	}
	_ = json.NewEncoder(w).Encode(info)
}

type updateStatus struct {
	State string `json:"state"` // "" | running | done | error
	Step  string `json:"step,omitempty"`
	Error string `json:"error,omitempty"`
}

func (a *App) setUpdate(state, step, errText string) {
	a.updMu.Lock()
	a.updState, a.updStep, a.updErr = state, step, errText
	a.updMu.Unlock()
}

func (a *App) updateStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET", http.StatusMethodNotAllowed)
		return
	}
	a.updMu.Lock()
	st := updateStatus{State: a.updState, Step: a.updStep, Error: a.updErr}
	a.updMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(st)
}

func (a *App) updateApplyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Force bool `json:"force"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	reply := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	a.jmu.Lock()
	running := len(a.runningLocked())
	a.jmu.Unlock()
	running += a.runningTasks()
	if running > 0 && !req.Force {
		reply(http.StatusConflict, map[string]any{"error": "running", "jobs": running})
		return
	}
	if !a.updBusy.CompareAndSwap(false, true) {
		reply(http.StatusConflict, map[string]string{"error": "busy"})
		return
	}
	defer a.updBusy.Store(false)
	app, reason := a.updateTarget()
	if app == "" {
		reply(http.StatusConflict, map[string]string{"error": reason})
		return
	}
	u := a.newUpdater()
	latest, ok := u.Latest()
	if !ok {
		reply(http.StatusBadGateway, map[string]string{"error": "offline"})
		return
	}
	if trust.VersionCmp(latest, u.Version) <= 0 {
		reply(http.StatusConflict, map[string]string{"error": "up_to_date"})
		return
	}
	a.setUpdate("running", "", "")
	verify := platform.CodesignVerify
	if a.verifyAppFn != nil {
		verify = a.verifyAppFn
	}
	if _, err := u.ApplyApp(latest, app, verify, func(s string) { a.setUpdate("running", s, "") }); err != nil {
		a.setUpdate("error", "", err.Error())
		reply(http.StatusInternalServerError, map[string]string{"error": "failed", "message": err.Error()})
		return
	}
	a.setUpdate("done", "", "")
	reply(http.StatusOK, map[string]any{"ok": true, "restarting": true, "version": latest})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() { // yanıt tarayıcıya ulaşsın; sonra yeniden başlatma yardımcısı + temiz çıkış (terminaller, özel ajan, geçici klasör)
		time.Sleep(800 * time.Millisecond)
		relaunch := platform.RelaunchApp
		if a.relaunchFn != nil {
			relaunch = a.relaunchFn
		}
		if err := relaunch(app); err != nil {
			a.setUpdate("error", "", "yeniden başlatma yardımcısı başlatılamadı; uygulamayı elle açın: "+err.Error())
		}
		a.killAllTerms()
		a.exitNow()
	}()
}
