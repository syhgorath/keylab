package platform

import "path/filepath"

// WebKitBundleID: KeyLab.app'in paket kimliği (scripts/package-app.sh). Gömülü pencere (WKWebView) kendi verisini (çerezler/yerel depo/önbellek)
// macOS'un kullanıcı klasörlerine yazar: .app içinden bu kimlikle, paketsiz ikiliden ikilinin dosya adıyla. Misafir (USB) modda bu bilgisayarda iz
// kalmaması için silinir.
const WebKitBundleID = "io.github.syhgorath.keylab"

// WebKitTraceDirs: id adlı pencerenin bu bilgisayarda bıraktığı veri klasörleri (yalnız macOS'ta bulunurlar; başka sistemlerde yoklarsa silinecek bir
// şey de yoktur). id tek bir klasör adı olmalı; boşsa ya da yol içeriyorsa nil.
func WebKitTraceDirs(home, id string) []string {
	if home == "" || !filepath.IsAbs(home) {
		return nil
	}
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id {
		return nil
	}
	return []string{
		filepath.Join(home, "Library", "WebKit", id),
		filepath.Join(home, "Library", "Caches", id),
	}
}
