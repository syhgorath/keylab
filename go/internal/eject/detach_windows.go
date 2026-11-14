//go:build windows

package eject

import "os/exec"

func detach(cmd *exec.Cmd) {}
