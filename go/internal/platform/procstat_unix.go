//go:build !windows

package platform

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// ProcessTable: tüm süreçlerin anlık tablosu (macOS ve Linux'ta `ps`).
func ProcessTable() ([]Proc, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,pcpu=,rss=,etime=,comm=").Output()
	if err != nil {
		return nil, err
	}
	return ParsePS(string(out)), nil
}

// ProcessCommands: pid → tam komut satırı (macOS ve Linux'ta `ps`). Eski oturum temizliği, özel ssh-agent'ın soket yolundan hangi klasöre ait olduğunu bulur.
func ProcessCommands() (map[int]string, error) {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil, err
	}
	return ParsePSCommands(string(out)), nil
}

// Terminate: sürece SIGTERM gönderir (yoksa/izin yoksa hata).
func Terminate(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }

// DiskUsage: path'in bulunduğu birimin toplam ve boş bayt sayısı.
func DiskUsage(path string) (total, free uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	return uint64(st.Blocks) * uint64(st.Bsize), uint64(st.Bavail) * uint64(st.Bsize), true
}

// TotalMemoryBytes: bilgisayarın toplam fiziksel belleği (bilinmiyorsa 0).
func TotalMemoryBytes() uint64 {
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			if n, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return n
			}
		}
	case "linux":
		if b, err := os.ReadFile("/proc/meminfo"); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(l, "MemTotal:") {
					if f := strings.Fields(l); len(f) >= 2 {
						if kb, err := strconv.ParseUint(f[1], 10, 64); err == nil {
							return kb * 1024
						}
					}
				}
			}
		}
	}
	return 0
}
