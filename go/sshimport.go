package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"keylab/internal/servers"
)

// ~/.ssh/config içe aktarma (arayüz). İstemciden YALNIZ takma ad (alias) listesi alınır; ana makine/kullanıcı/port/anahtar
// değerleri istemciden okunmaz, dosya sunucu tarafında yeniden ayrıştırılır ve her kayıt servers.Write'ın doğrulamasından geçer.
// Private key'ler kopyalanmaz; yalnızca IdentityFile yolu KEY olarak yazılır. Var olan sunucunun üzerine yazılmaz.

type sshHostRow struct {
	Alias string `json:"alias"`
	Name  string `json:"name"`
	Host  string `json:"host"`
	User  string `json:"user"`
	Port  string `json:"port"`
	Key   string `json:"key"`
	Added bool   `json:"added"`
}

type sshImportRow struct {
	Alias  string `json:"alias"`
	Name   string `json:"name"`
	Status string `json:"status"` // imported | skipped | error
	Error  string `json:"error,omitempty"`
}

func sshConfigPath() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(h, ".ssh", "config")
}

// sshConfigList: dosya yoksa exists=false (hata değil).
func (a *App) sshConfigList(cfg string) (exists bool, rows []sshHostRow, err error) {
	entries, err := servers.ParseSSHConfig(cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, []sshHostRow{}, nil
		}
		return false, nil, err
	}
	rows = []sshHostRow{}
	for _, e := range entries {
		name := servers.ModuleNameFromAlias(e.Alias)
		_, added := a.findServer(name)
		rows = append(rows, sshHostRow{e.Alias, name, e.Host, e.User, e.Port, e.IdentityFile, added})
	}
	return true, rows, nil
}

func (a *App) sshConfigImport(cfg string, aliases []string) ([]sshImportRow, error) {
	entries, err := servers.ParseSSHConfig(cfg)
	if err != nil {
		return nil, err
	}
	byAlias := map[string]servers.HostEntry{}
	for _, e := range entries {
		if _, dup := byAlias[e.Alias]; !dup {
			byAlias[e.Alias] = e
		}
	}
	a.vmu.Lock()
	defer a.vmu.Unlock()
	out := []sshImportRow{}
	seen := map[string]bool{}
	for _, al := range aliases {
		if seen[al] {
			continue
		}
		seen[al] = true
		e, ok := byAlias[al]
		if !ok {
			out = append(out, sshImportRow{Alias: al, Status: "error", Error: "ssh config içinde bu takma ad yok"})
			continue
		}
		name := servers.ModuleNameFromAlias(al)
		res := servers.ImportHosts(a.serversDir, []servers.HostEntry{e}, "root")
		switch {
		case len(res) == 1 && res[0].Skipped:
			out = append(out, sshImportRow{Alias: al, Name: name, Status: "skipped"})
		case len(res) == 1 && res[0].Err != nil:
			out = append(out, sshImportRow{Alias: al, Name: name, Status: "error", Error: res[0].Err.Error()})
		default:
			out = append(out, sshImportRow{Alias: al, Name: name, Status: "imported"})
		}
	}
	return out, nil
}

func (a *App) sshConfigHostsHandler(w http.ResponseWriter, r *http.Request) {
	exists, rows, err := a.sshConfigList(sshConfigPath())
	if err != nil {
		http.Error(w, "ssh config okunamadı", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"exists": exists, "hosts": rows})
}

func (a *App) sshConfigImportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Aliases []string `json:"aliases"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&req) != nil || len(req.Aliases) == 0 || len(req.Aliases) > 500 {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	rows, err := a.sshConfigImport(sshConfigPath(), req.Aliases)
	if err != nil {
		http.Error(w, "ssh config okunamadı", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "results": rows})
}
