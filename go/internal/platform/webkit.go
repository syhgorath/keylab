package platform

import "path/filepath"

// WebKitBundleID: KeyLab.app'in paket kimliği (scripts/package-app.sh). Gömülü pencere (WKWebView) kendi verisini bu kimlikle macOS'un kullanıcı
// klasörlerine yazar: çerezler/yerel depo/önbellek. Misafir (USB) modda bu bilgisayarda iz kalmaması için silinir.
const WebKitBundleID = "io.github.syhgorath.keylab"

// WebKitTraceDirs: pencerenin bu bilgisayarda bıraktığı veri klasörleri (yalnız macOS'ta bulunurlar; başka sistemlerde yoklarsa silinecek bir şey de yoktur).
func WebKitTraceDirs(home string) []string {
	if home == "" || !filepath.IsAbs(home) {
		return nil
	}
	return []string{
		filepath.Join(home, "Library", "WebKit", WebKitBundleID),
		filepath.Join(home, "Library", "Caches", WebKitBundleID),
	}
}
