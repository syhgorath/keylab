//go:build !darwin && !linux

package platform

import "os"

// FlushInput: bu işletim sisteminde gömülü terminal yok; bir şey yapılmaz.
func FlushInput(*os.File) bool { return false }
