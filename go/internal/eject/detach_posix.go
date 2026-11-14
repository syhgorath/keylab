//go:build !windows

package eject

import (
	"os/exec"
	"syscall"
)

// detach: yardımcı süreç, başlatan süreç bitince de yaşasın (kendi oturumunda).
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
