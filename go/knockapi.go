package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"keylab/internal/cli"
	"keylab/internal/platform"
)

// /api/knock: port knocking formu (Özet sekmesi).
//
//	GET  ?name=<ad>                     → {enabled, hasSeq, proto, delay}   (dizinin DEĞERİ asla dönmez)
//	POST {name, action:"save",    proto, delay, seq}   diziyi Keychain'e yazar, server.env'i günceller
//	POST {name, action:"disable"}                       Keychain kaydını siler, knock'u kapatır
//	POST {name, action:"send"}                          kayıtlı diziyi şimdi gönderir
//
// Dizi yalnızca istek gövdesinde gelir; günlüğe yazılmaz, komut satırına girmez (security -i standart girdisi).
func (a *App) knockHandler(w http.ResponseWriter, r *http.Request) {
	srv := &cli.Servers{Dir: a.serversDir, Keychain: a.knockSecrets()} // misafir modda dizi şifreli kasada (bkz. vaultsecrets.go)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		info, err := srv.KnockStatus(r.URL.Query().Get("name"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(info)
	case http.MethodPost:
		var req struct{ Name, Action, Proto, Delay, Seq string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req) != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		var err error
		switch req.Action {
		case "save":
			if platform.GuestMode() && !a.knockSecrets().Available() { // kasa yok ya da kilitli (kilitliyken guard zaten 423 verir)
				http.Error(w, "misafir (USB) modda knock dizisi şifreli kasada tutulur: önce Ayarlar > Şifreli passphrase kasası'ndan kasayı oluşturun ve açın", http.StatusConflict)
				return
			}
			err = srv.KnockSave(req.Name, req.Proto, req.Delay, req.Seq)
		case "disable":
			err = srv.KnockDisable(req.Name)
		case "send":
			if _, serr := srv.KnockStatus(req.Name); serr != nil {
				err = serr
			} else if cerr := cli.SendKnockWith(r.Context(), a.knockSecrets(), a.serversDir, req.Name); cerr != nil {
				http.Error(w, a.knockErr(cerr).Error(), http.StatusConflict)
				return
			}
		default:
			http.Error(w, "bilinmeyen eylem", http.StatusBadRequest)
			return
		}
		if err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, cli.ErrKnockStore) || errors.Is(err, cli.ErrKnockVerify) {
				code = http.StatusInternalServerError
			}
			http.Error(w, err.Error(), code)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	default:
		http.Error(w, "GET ya da POST", http.StatusMethodNotAllowed)
	}
}
