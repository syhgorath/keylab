//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func openSlaveTTY(ttyName string) (*os.File, error) {
	return os.OpenFile(ttyName, os.O_RDWR|syscall.O_NOCTTY, 0)
}

// Çocuk yeni bir oturumun lideri olur ve PTY'yi kontrol terminali edinir (Ctrl-C vb. doğru çalışır).
func setTermSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
}

// termSignalGroup: oturumun süreç grubuna sinyal gönderir (hard=false: SIGHUP, true: SIGKILL).
func termSignalGroup(pid int, hard bool) {
	sig := syscall.SIGHUP
	if hard {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-pid, sig)
}
