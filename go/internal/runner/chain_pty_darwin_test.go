//go:build darwin

package runner

import (
	"bytes"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

func testPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	ioctl := func(req, arg uintptr) {
		if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, arg); e != 0 {
			t.Fatal(e)
		}
	}
	var name [128]byte
	ioctl(0x20007454, 0) // TIOCPTYGRANT
	ioctl(0x20007452, 0) // TIOCPTYUNLK
	ioctl(0x40807453, uintptr(unsafe.Pointer(&name[0])))
	slave, err = os.OpenFile(string(name[:bytes.IndexByte(name[:], 0)]), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	return os.NewFile(uintptr(fd), "/dev/ptmx"), slave
}
