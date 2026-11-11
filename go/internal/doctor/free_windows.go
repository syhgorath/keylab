//go:build windows

package doctor

// statfsFreeKB: Windows'ta disk alanı henüz ölçülmüyor; "bilinmiyor" döner (uydurma değer yok).
func statfsFreeKB(dir string) (int64, bool) {
	return 0, false
}
