//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	tiocSptlck = 0x40045431
	tiocGptn   = 0x80045430
)

func openPTY() (ptmx *os.File, ttyName string, err error) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open /dev/ptmx: %w", err)
	}
	var unlock int
	if err := ptyIoctl(fd, tiocSptlck, uintptr(unsafe.Pointer(&unlock))); err != nil {
		syscall.Close(fd)
		return nil, "", fmt.Errorf("TIOCSPTLCK: %w", err)
	}
	var ptn uint32
	if err := ptyIoctl(fd, tiocGptn, uintptr(unsafe.Pointer(&ptn))); err != nil {
		syscall.Close(fd)
		return nil, "", fmt.Errorf("TIOCGPTN: %w", err)
	}
	return os.NewFile(uintptr(fd), "/dev/ptmx"), fmt.Sprintf("/dev/pts/%d", ptn), nil
}

func ptyIoctl(fd int, req, arg uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, arg)
	if e != 0 {
		return e
	}
	return nil
}

func setWinsize(f *os.File, rows, cols uint16) error {
	ws := struct{ Row, Col, X, Y uint16 }{rows, cols, 0, 0}
	return ptyIoctl(int(f.Fd()), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&ws)))
}
