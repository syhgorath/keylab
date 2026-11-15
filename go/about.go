package main

// /api/about gövdesi: sürüm, klasörler, platform (Ayarlar > Yollar ve yapılandırma).

import (
	"keylab/internal/platform"
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
	return map[string]any{"version": ver, "root": root, "serversDir": a.serversDir, "componentsDir": a.compDir, "stateDir": a.stateDir, "staleCleaned": a.stale,
		"conf": conf, "platform": runtime.GOOS + "/" + runtime.GOARCH, "go": runtime.Version(), "askpass": a.promptOS().Available(), "guest": platform.GuestMode(), "usb": usbName(a.volumesOS().MountRoot(a.serversDir))}
}

// promptOS: sistem parola penceresi (platform katmanı); testte App.prompt ile değiştirilir.
func (a *App) promptOS() platform.Prompt {
	if a.prompt != nil {
		return a.prompt
	}
	return platform.DefaultPrompt()
}

// volumesOS: çıkarılabilir birimler (platform katmanı); testte App.volumes ile değiştirilir.
func (a *App) volumesOS() platform.Volumes {
	if a.volumes != nil {
		return a.volumes
	}
	return platform.DefaultVolumes()
}
