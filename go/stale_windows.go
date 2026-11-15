//go:build windows

package main

import "io/fs"

func ownedByUs(fs.FileInfo) bool { return true }
