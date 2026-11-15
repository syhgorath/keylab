package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"keylab/internal/vault"
	"net/http"
	"strings"
)

// Kasa uçları (hepsi guard'lı; POST CSRF başlığı ister). Yanıtlar HİÇBİR ZAMAN parola/passphrase içermez.

func vaultJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func vaultErr(w http.ResponseWriter, m *vaultMgr, err error) {
	code, msg := http.StatusBadRequest, err.Error()
	switch {
	case errors.Is(err, errVaultThrottled):
		code = http.StatusTooManyRequests
	case errors.Is(err, vault.ErrWrongPassword):
		code, msg = http.StatusUnauthorized, "ana parola yanlış"
	case errors.Is(err, vault.ErrExists), errors.Is(err, errVaultUnlocked):
		code = http.StatusConflict
	case errors.Is(err, vault.ErrLocked):
		code = http.StatusLocked
	case errors.Is(err, errVaultOff):
		code = http.StatusNotImplemented
	case errors.Is(err, vault.ErrNotFound):
		code, msg = http.StatusNotFound, "kasa yok"
	}
	vaultJSON(w, code, map[string]any{"error": msg, "retryAfter": m.status().RetryAfter})
}

func (a *App) vaultStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET", http.StatusMethodNotAllowed)
		return
	}
	vaultJSON(w, http.StatusOK, a.vaultM().status())
}

// vaultActionHandler: POST /api/vault/{create,unlock,lock,passphrase,passphrase/delete,password}
func (a *App) vaultActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	m := a.vaultM()
	act := strings.TrimPrefix(r.URL.Path, "/api/vault/")
	var req struct {
		Password    string `json:"password"`
		NewPassword string `json:"newPassword"`
		Server      string `json:"server"`
		Passphrase  string `json:"passphrase"`
	}
	if act != "lock" {
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req) != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
	}
	switch act {
	case "create":
		if err := m.create(req.Password); err != nil {
			vaultErr(w, m, err)
			return
		}
		vaultJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "unlock":
		loaded, failed, err := m.unlock(req.Password)
		if err != nil {
			vaultErr(w, m, err)
			return
		}
		vaultJSON(w, http.StatusOK, map[string]any{"ok": true, "loaded": loaded, "failed": failed})
	case "lock":
		m.lock()
		vaultJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "passphrase":
		if err := m.setPassphrase(req.Server, req.Passphrase); err != nil {
			vaultErr(w, m, err)
			return
		}
		vaultJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "passphrase/delete":
		ok, err := m.removePassphrase(req.Server)
		if err != nil {
			vaultErr(w, m, err)
			return
		}
		vaultJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": ok})
	case "password":
		if err := m.changePassword(req.Password, req.NewPassword); err != nil {
			vaultErr(w, m, err)
			return
		}
		vaultJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		http.Error(w, fmt.Sprintf("bilinmeyen kasa işlemi: %s", act), http.StatusNotFound)
	}
}
