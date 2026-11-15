//go:build windows

package platform

import "errors"

// Windows'ta kaynak ölçümü henüz yok (Windows desteği gelince eklenecek).
func ProcessTable() ([]Proc, error) {
	return nil, errors.New("bu işletim sisteminde süreç tablosu yok")
}
func DiskUsage(string) (total, free uint64, ok bool) { return 0, 0, false }
func TotalMemoryBytes() uint64                       { return 0 }
func ProcessCommands() (map[int]string, error) {
	return nil, errors.New("bu işletim sisteminde süreç tablosu yok")
}
func Terminate(int) error { return errors.New("bu işletim sisteminde desteklenmiyor") }
