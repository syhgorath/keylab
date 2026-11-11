//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// Arka plan işi: kendi süreç grubunda başlar; iptalde grubun tamamına SIGTERM gider.
func setJobSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killJobGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}
