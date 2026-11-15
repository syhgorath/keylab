//go:build !windows

package main

import (
	"io/fs"
	"os"
	"syscall"
)

// ownedByUs: klasör bu kullanıcıya mı ait (başkasının klasörüne dokunulmaz).
func ownedByUs(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Getuid())
}
