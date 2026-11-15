package platform

import "strings"

// AppBundlePath: çalışan ikili bir macOS uygulama paketinin (.app) içindeyse paketin yolu ("/Applications/KeyLab.app"), değilse ok=false.
// Örn. "/Volumes/USB/KeyLab.app/Contents/MacOS/KeyLab" → "/Volumes/USB/KeyLab.app".
func AppBundlePath(exe string) (string, bool) {
	const mark = ".app/Contents/MacOS/"
	i := strings.Index(exe, mark)
	if i < 0 {
		return "", false
	}
	app := exe[:i+len(".app")]
	if app == "" || strings.HasSuffix(app, "/.app") && len(app) == len("/.app") {
		return "", false
	}
	return app, true
}

// IsTranslocated: macOS uygulamayı karantina nedeniyle rastgele, salt okunur bir yola taşıyarak (App Translocation) çalıştırıyor mu? O durumda uygulama kendini
// güncelleyemez (gerçek konumu görünmez/yazılamaz): kullanıcı önce uygulamayı Uygulamalar'a ya da USB'ye taşımalıdır.
func IsTranslocated(appPath string) bool { return strings.Contains(appPath, "/AppTranslocation/") }
