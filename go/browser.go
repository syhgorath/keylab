package main

// Tarayıcı başlatma. Amaç: arayüzün (parola/terminal girdisi dahil) kullanıcının günlük tarayıcı profilindeki EKLENTİLERDEN ve çerezlerinden
// ayrı çalışması. macOS'ta Chromium tabanlı bir tarayıcı (Chrome, Edge, Brave, Chromium) bulunursa KeyLab'a ait ayrı profille (durum
// klasörü/browser-profile), eklentiler kapalı ve uygulama penceresi (--app) kipinde açılır; bulunamazsa sistem tarayıcısı kullanılır
// (Safari gibi tarayıcılar komut satırından profil/eklenti denetimi sunmaz). Bu, ağ izolasyonu değildir: sunucu hâlâ 127.0.0.1'dedir.
//
// Açılış adresi TEK KULLANIMLIK jeton taşır (?b=…): tarayıcı süreci komut satırını (adresi) oturum boyunca saklar ve `ps` ile başka yerel
// kullanıcılarca görülebilir; gerçek oturum jetonu bu yüzden komut satırına yazılmaz (bkz. App.useBootToken, main.go root).

import (
	"os"
	"os/exec"
	"path/filepath"
)

// chromiumApps: macOS uygulama paket adları (ikili adı paket adıyla aynıdır), öncelik sırasıyla.
var chromiumApps = []string{"Google Chrome", "Microsoft Edge", "Brave Browser", "Chromium"}

// chromiumCandidates: aranan ikili yolları (önce /Applications, sonra ~/Applications; uygulama sırasına göre).
func chromiumCandidates(home string) []string {
	var out []string
	for _, app := range chromiumApps {
		for _, base := range []string{"/Applications", filepath.Join(home, "Applications")} {
			out = append(out, filepath.Join(base, app+".app", "Contents", "MacOS", app))
		}
	}
	return out
}

// findChromium: adaylardan var olan ilk ikili ("" = yok). exists enjekte edilir (test).
func findChromium(home string, exists func(string) bool) string {
	for _, c := range chromiumCandidates(home) {
		if exists(c) {
			return c
		}
	}
	return ""
}

// isolatedBrowserArgs: ayrı profil + uygulama penceresi + eklentisiz.
func isolatedBrowserArgs(url, profileDir string) []string {
	return []string{"--user-data-dir=" + profileDir, "--app=" + url, "--disable-extensions", "--no-first-run", "--no-default-browser-check"}
}

// normalizeBrowserMode: "" ya da "auto" → auto; "default" → default; başka değer → ok=false.
func normalizeBrowserMode(m string) (string, bool) {
	switch m {
	case "", "auto":
		return "auto", true
	case "default":
		return "default", true
	}
	return "", false
}

func isExecutableFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// launchBrowser: url'yi açar. isolated=true → ayrı profilli Chromium; false → sistem tarayıcısı (`open`).
func launchBrowser(url, stateDir, mode string) (isolated bool) {
	if mode == "auto" {
		home, _ := os.UserHomeDir()
		if bin := findChromium(home, isExecutableFile); bin != "" {
			profile := filepath.Join(stateDir, "browser-profile")
			if err := os.MkdirAll(profile, 0o700); err == nil {
				_ = os.Chmod(profile, 0o700)
				cmd := exec.Command(bin, isolatedBrowserArgs(url, profile)...)
				if cmd.Start() == nil {
					go func() { _ = cmd.Wait() }() // zombi bırakma
					return true
				}
			}
		}
	}
	_ = exec.Command("open", url).Start()
	return false
}
