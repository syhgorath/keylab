package platform

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// MacBrowser: macOS (uygulama paketleri /Applications ve ~/Applications; sistem tarayıcısı `open` ile).
type MacBrowser struct{}

// chromiumApps: macOS uygulama paket adları (ikili adı paket adıyla aynıdır), öncelik sırasıyla.
var chromiumApps = []string{"Google Chrome", "Microsoft Edge", "Brave Browser", "Chromium"}

// ChromiumCandidates: önce /Applications, sonra ~/Applications; uygulama sırasına göre.
func (MacBrowser) ChromiumCandidates(home string) []string {
	var out []string
	for _, app := range chromiumApps {
		for _, base := range []string{"/Applications", filepath.Join(home, "Applications")} {
			out = append(out, filepath.Join(base, app+".app", "Contents", "MacOS", app))
		}
	}
	return out
}

// Supported: yalnız macOS'ta.
func (MacBrowser) Supported() bool { return runtime.GOOS == "darwin" }

// OpenURL: `open <adres>`.
func (MacBrowser) OpenURL(url string) error { return exec.Command("open", url).Start() }
