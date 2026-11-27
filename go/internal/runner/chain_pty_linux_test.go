//go:build linux

package runner

import (
	"fmt"
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
	var unlock int
	var n uint32
	ioctl(0x40045431, uintptr(unsafe.Pointer(&unlock))) // TIOCSPTLCK
	ioctl(0x80045430, uintptr(unsafe.Pointer(&n)))      // TIOCGPTN
	slave, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	return os.NewFile(uintptr(fd), "/dev/ptmx"), slave
}
