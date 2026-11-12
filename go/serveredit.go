package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"keylab/internal/servers"
)

// updateServerAddr: server.env'deki IP, USER ve PORT'u günceller. Diğer satırlar (yorumlar, KNOCK*, KEY, OS…), anahtar
// dosyaları ve Keychain kayıtları (sunucu adına bağlıdır, IP'ye değil) olduğu gibi kalır. Dönüş: HTTP kodu ve hata.
func (a *App) updateServerAddr(name, host, user, port string) (int, error) {
	if !reName.MatchString(name) {
		return http.StatusBadRequest, fmt.Errorf("geçersiz sunucu adı")
	}
	dir := filepath.Join(a.serversDir, name)
	if st, err := os.Stat(filepath.Join(dir, "server.env")); err != nil || !st.Mode().IsRegular() {
		return http.StatusNotFound, fmt.Errorf("sunucu yok")
	}
	if port == "" {
		port = "22"
	}
	if err := servers.ValidateTarget(host, user, port); err != nil {
		return http.StatusBadRequest, err
	}
	a.vmu.Lock()
	defer a.vmu.Unlock()
	for _, kv := range [][2]string{{"IP", host}, {"USER", user}, {"PORT", port}} {
		if err := servers.Set(dir, kv[0], kv[1]); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("server.env yazılamadı")
		}
	}
	a.mu.Lock()
	delete(a.conn, name)
	delete(a.up, name) // eski adrese göre ölçülmüş durum geçersiz: yeniden ölçülsün
	a.mu.Unlock()
	return 0, nil
}

// /api/server-edit  POST {name, host, user, port}
func (a *App) serverEditHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Name, Host, User, Port string }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req) != nil {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	if code, err := a.updateServerAddr(req.Name, strings.TrimSpace(req.Host), strings.TrimSpace(req.User), strings.TrimSpace(req.Port)); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
