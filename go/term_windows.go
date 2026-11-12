//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
)

func openSlaveTTY(ttyName string) (*os.File, error) {
	return nil, errors.New("Windows'ta gömülü terminal yok")
}

func setTermSysProcAttr(cmd *exec.Cmd) {}

func termSignalGroup(pid int, hard bool) {
	if p, err := os.FindProcess(pid); err == nil {
		_ = p.Kill()
	}
}
