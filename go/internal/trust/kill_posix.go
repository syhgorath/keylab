//go:build !windows

package trust

import "syscall"

// isProcessDead: sinyal 0 gönderilemiyorsa süreç yok demektir.
func isProcessDead(pid int) bool {
	return syscall.Kill(pid, 0) != nil
}
