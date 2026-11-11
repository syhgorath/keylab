//go:build windows

package trust

import "os"

// isProcessDead: Windows'ta os.FindProcess, süreç yoksa hata döndürür.
func isProcessDead(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return true
	}
	_ = p.Release()
	return false
}
