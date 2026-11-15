//go:build !windows

package platform

import (
	"os/exec"
	"syscall"
)

func detachProc(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
