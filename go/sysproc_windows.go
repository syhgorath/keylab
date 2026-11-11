//go:build windows

package main

import "os/exec"

// Windows'ta süreç grubu (Setpgid) yoktur; iş doğrudan sonlandırılır.
func setJobSysProcAttr(cmd *exec.Cmd) {}

func killJobGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
