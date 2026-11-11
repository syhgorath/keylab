//go:build !windows

package doctor

import "syscall"

// statfsFreeKB: klasörün bulunduğu diskte kullanılabilir alan (KB); ölçülemezse (0, false).
func statfsFreeKB(dir string) (int64, bool) {
	var s syscall.Statfs_t
	if syscall.Statfs(dir, &s) != nil {
		return 0, false
	}
	return int64(s.Bavail) * int64(s.Bsize) / 1024, true
}
