//go:build linux

package platform

import (
	"os"
	"syscall"
)

// FlushInput: terminalde bekleyen, henüz okunmamış girdiyi atar. f terminal değilse false döner.
func FlushInput(f *os.File) bool {
	const tcflsh, tciflush = 0x540B, 0 // asm-generic/ioctls.h (amd64 ve arm64)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), tcflsh, tciflush)
	return errno == 0
}
