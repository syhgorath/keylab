package main

import (
	"encoding/json"
	"fmt"
	"keylab/internal/platform"
	"net/http"
	"os"
	"path/filepath"
)

// "Bu Mac'e güveniyorum": taşınabilir (USB) kullanımda varsayılan misafir mod (özel ssh-agent, Keychain yok) yerine bu bilgisayarın Keychain'i ve
// kullanıcının kendi ssh-agent'ı kullanılır ("tak diye girer": passphrase Keychain'de durur). İşaret dosyası BU BİLGİSAYARDADIR
// (~/.keylab/keychain-ok), USB'de değil: başka bir bilgisayara takılınca o bilgisayar güvenilir sayılmaz, misafir mod kalır.
// Karar açılışta verilir (özel ajan başlatılsın mı); ayar bir sonraki açılışta geçerli olur. KEYLAB_GUEST ortam değişkeni her zaman önceliklidir.

var trustedMacFile = func() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".keylab", "keychain-ok")
}

func trustedMac() bool {
	p := trustedMacFile()
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func setTrustedMac(on bool) error {
	p := trustedMacFile()
	if p == "" {
		return fmt.Errorf("ev klasörü bulunamadı")
	}
	if !on {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte("bu bilgisayar güvenilir: KeyLab taşınabilir modda Keychain kullanır\n"), 0o600)
}

// wantGuestMode: taşınabilir düzende misafir mod istensin mi? KEYLAB_GUEST açıkça verilmişse o kazanır ("0" kapalı, başka her değer açık);
// verilmemişse bu bilgisayar güvenilir işaretliyse misafir DEĞİL, değilse misafir.
func wantGuestMode(guestEnv string, trusted bool) bool {
	if guestEnv == "0" {
		return false
	}
	if guestEnv != "" {
		return true
	}
	return !trusted
}

type trustedMacStatus struct {
	Portable bool   `json:"portable"` // uygulama taşınabilir düzende mi (yalnız o zaman anlamlı)
	Trusted  bool   `json:"trusted"`  // bu bilgisayar güvenilir işaretli (bir sonraki açılışta geçerli)
	Active   string `json:"active"`   // şu anki çalışan mod: "guest" | "keychain"
}

func (a *App) trustedMacState() trustedMacStatus {
	mode := "keychain"
	if platform.GuestMode() {
		mode = "guest"
	}
	return trustedMacStatus{Portable: a.isPortable(), Trusted: trustedMac(), Active: mode}
}

// GET /api/trusted-mac → durum; POST {enabled} → ayarlar ve durumu döner (yalnız taşınabilir düzende; aksi 409).
func (a *App) trustedMacHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&req) != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		if !a.isPortable() {
			http.Error(w, "bu ayar yalnızca taşınabilir (USB) kullanımda anlamlıdır", http.StatusConflict)
			return
		}
		if err := setTrustedMac(req.Enabled); err != nil {
			http.Error(w, "ayar kaydedilemedi: "+err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "GET ya da POST", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(a.trustedMacState())
}
