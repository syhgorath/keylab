package main

// Sunucu tarafı arayüz ayarları (şimdilik: aynı anda çalışabilecek eylem sayısı). <state>/keylab-ui.json, 0600.
// Dosya adı sabittir (kullanıcı girdisi yol kurmaz); değer sınırlar içinde doğrulanır, bozuk dosya varsayılana düşer.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

const (
	defaultMaxJobs = 4 // jobs.go'daki maxParallelJobs ile aynı varsayılan
	minMaxJobs     = 1
	maxMaxJobs     = 16
	uiSettingsFile = "keylab-ui.json"
)

type uiSettings struct {
	MaxJobs int `json:"maxJobs"`
}

func clampMaxJobs(n int) int {
	if n < minMaxJobs || n > maxMaxJobs {
		return defaultMaxJobs
	}
	return n
}

// maxJobs: aynı anda çalışabilecek eylem sayısı (ayar yoksa varsayılan)
func (a *App) maxJobs() int {
	n := int(a.maxJobsN.Load())
	if n < minMaxJobs || n > maxMaxJobs {
		return defaultMaxJobs
	}
	return n
}

func (a *App) uiSettingsPath() string { return filepath.Join(a.stateDir, uiSettingsFile) }

// loadUISettings: açılışta bir kez okunur; yok ya da bozuksa varsayılan kalır
func (a *App) loadUISettings() {
	b, err := os.ReadFile(a.uiSettingsPath())
	if err != nil {
		return
	}
	var s uiSettings
	if json.Unmarshal(b, &s) != nil {
		return
	}
	a.maxJobsN.Store(int32(clampMaxJobs(s.MaxJobs)))
}

// saveUISettings: geçici dosyaya yazıp yeniden adlandırır (yarım dosya kalmaz)
func (a *App) saveUISettings(maxJobs int) error {
	if err := os.MkdirAll(a.stateDir, 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(uiSettings{MaxJobs: maxJobs})
	tmp := a.uiSettingsPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, a.uiSettingsPath()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	a.maxJobsN.Store(int32(maxJobs))
	return nil
}

// GET /api/ui-settings → {maxJobs, min, max, default}; POST {maxJobs} → aynı yanıt. Sınır dışı değer 400.
func (a *App) uiSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct{ MaxJobs int }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&req) != nil || req.MaxJobs < minMaxJobs || req.MaxJobs > maxMaxJobs {
			http.Error(w, "aynı anda en çok eylem 1 ile 16 arasında olmalı", http.StatusBadRequest)
			return
		}
		if err := a.saveUISettings(req.MaxJobs); err != nil {
			http.Error(w, "ayar kaydedilemedi", http.StatusInternalServerError)
			return
		}
	} else if r.Method != http.MethodGet {
		http.Error(w, "GET/POST", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"maxJobs": a.maxJobs(), "min": minMaxJobs, "max": maxMaxJobs, "default": defaultMaxJobs})
}
