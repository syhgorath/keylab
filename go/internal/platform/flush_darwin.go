//go:build darwin

package platform

import (
	"os"
	"syscall"
	"unsafe"
)

// FlushInput: terminalde bekleyen, henüz okunmamış girdiyi atar. f terminal değilse false döner.
func FlushInput(f *os.File) bool {
	read := int32(1) // FREAD: yalnız girdi kuyruğu
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCFLUSH), uintptr(unsafe.Pointer(&read)))
	return errno == 0
}
