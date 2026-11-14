package main

// /api/about gövdesi: sürüm, klasörler, platform (Ayarlar > Yollar ve yapılandırma).

import (
	"path/filepath"
	"runtime"
)

// aboutInfo: sürüm İKİLİNİN gömülü sürümüdür (buildVersion); yoksa (geliştirme derlemesi) kök klasördeki VERSION dosyası.
// Kurulu modda (~/.keylab) kökte VERSION dosyası bulunmaz; eskiden sürüm bu yüzden boş ("—") görünüyordu.
func (a *App) aboutInfo() map[string]any {
	root := filepath.Dir(filepath.Clean(a.compDir))
	ver := versionOf(root)
	if ver == "unknown" {
		ver = ""
	}
	conf := map[string]string{}
	for k, v := range parseEnv(filepath.Join(root, "sshpocket.conf")) {
		if len(k) <= 40 && len(v) <= 80 {
			conf[k] = v
		}
	}
	return map[string]any{"version": ver, "root": root, "serversDir": a.serversDir, "componentsDir": a.compDir, "stateDir": a.stateDir,
		"conf": conf, "platform": runtime.GOOS + "/" + runtime.GOARCH, "go": runtime.Version(), "askpass": askpassAvailable(), "usb": usbName(usbRoot(a.serversDir))}
}
