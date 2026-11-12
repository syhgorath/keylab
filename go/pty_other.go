//go:build !darwin && !linux

package main

import (
	"errors"
	"os"
)

// Windows (ConPTY henüz yok) ve diğer sistemler: gömülü terminal desteklenmez; openTerm Windows'ta daha önce 501 döner.
func openPTY() (ptmx *os.File, ttyName string, err error) {
	return nil, "", errors.New("bu sistemde gömülü terminal (PTY) desteklenmiyor")
}

func setWinsize(f *os.File, rows, cols uint16) error { return nil }
