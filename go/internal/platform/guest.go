package platform

import (
	"keylab/internal/envx"
	"os"
)

// Misafir (taşınabilir/USB) mod: bu bilgisayarın Keychain'ine ve kullanıcının kendi ssh-agent'ına DOKUNULMAZ. Anahtarlar yalnız KeyLab'ın özel
// ssh-agent'ına eklenir (passphrase her oturumda ssh-add isteminde sorulur, hiçbir yere kaydedilmez), parola/knock deposu yoktur.
// Mod KEYLAB_GUEST=1 ortam değişkeniyle taşınır: KeyLab bunu kendi sürecinde ayarlar, alt süreçler (eylemler, askpass) de görür.
// KEYLAB_GUEST=0 modu açıkça kapatır (taşınabilirde bile Keychain kullanılsın isteyenler için).

// GuestMode: misafir mod açık mı?
func GuestMode() bool { return envx.Get("GUEST") == "1" }

// SetGuestMode: modu (bu süreç ve alt süreçler için) açar/kapatır.
func SetGuestMode(on bool) {
	v := "0"
	if on {
		v = "1"
	}
	os.Setenv("KEYLAB_GUEST", v)
}
