package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// toolDirs: kullanıcının kurduğu araçların (Homebrew Python, ansible, ssh-copy-id…) bulunduğu bilinen klasörler. macOS'ta Dock/Finder'dan
// açılan uygulamaya yalnızca /usr/bin:/bin:/usr/sbin:/sbin verilir; terminalden açılınca ise kabuğun PATH'i gelir. Bu fark yüzünden
// Homebrew'daki Python 3.13 yalnızca terminalden açılınca bulunuyordu.
func toolDirs() []string {
	if runtime.GOOS == "darwin" {
		return []string{"/opt/homebrew/bin", "/opt/homebrew/sbin", "/usr/local/bin", "/usr/local/sbin"}
	}
	return nil
}

// AugmentPATH: path'te olmayan ve var olan (exists) klasörleri SONA ekler. Kullanıcının kendi sırası ve öncelikleri bozulmaz; yinelenen eklenmez.
func AugmentPATH(path string, dirs []string, exists func(string) bool) string {
	have := map[string]bool{}
	for _, p := range filepath.SplitList(path) {
		have[filepath.Clean(p)] = true
	}
	out := path
	for _, d := range dirs {
		if have[d] || !exists(d) {
			continue
		}
		have[d] = true
		if out == "" {
			out = d
		} else {
			out += string(os.PathListSeparator) + d
		}
	}
	return out
}

// EnsureToolPath: süreç PATH'ine bilinen araç klasörlerini ekler (açılışta bir kez; alt süreçler — terminal, eylemler, Ansible — devralır).
func EnsureToolPath() {
	exists := func(d string) bool {
		st, err := os.Stat(d)
		return err == nil && st.IsDir()
	}
	cur := os.Getenv("PATH")
	if np := AugmentPATH(cur, toolDirs(), exists); np != cur && strings.TrimSpace(np) != "" {
		_ = os.Setenv("PATH", np)
	}
}
