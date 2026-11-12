//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	tiocPtyGrant = 0x20007454
	tiocPtyUnlk  = 0x20007452
	tiocPtyGname = 0x40807453 // 128 baytlık slave tty adı döner
)

func openPTY() (ptmx *os.File, ttyName string, err error) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open /dev/ptmx: %w", err)
	}
	if err := ptyIoctl(fd, tiocPtyGrant, 0); err != nil {
		syscall.Close(fd)
		return nil, "", fmt.Errorf("TIOCPTYGRANT: %w", err)
	}
	if err := ptyIoctl(fd, tiocPtyUnlk, 0); err != nil {
		syscall.Close(fd)
		return nil, "", fmt.Errorf("TIOCPTYUNLK: %w", err)
	}
	var name [128]byte
	if err := ptyIoctl(fd, tiocPtyGname, uintptr(unsafe.Pointer(&name[0]))); err != nil {
		syscall.Close(fd)
		return nil, "", fmt.Errorf("TIOCPTYGNAME: %w", err)
	}
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		n = len(name)
	}
	return os.NewFile(uintptr(fd), "/dev/ptmx"), string(name[:n]), nil
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
