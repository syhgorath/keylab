package platform

import (
	"errors"
	"io"
)

// ErrNoSecrets: gizli değer deposu kullanılamıyor (misafir mod).
var ErrNoSecrets = errors.New("gizli değer deposu kullanılamıyor (taşınabilir mod: bu bilgisayarın Keychain'i kullanılmaz)")

// NoSecrets: hiçbir şey saklamayan/bulmayan depo (misafir mod). Hiçbir araç çağrılmaz.
type NoSecrets struct{}

func (NoSecrets) Find(string) (string, bool) { return "", false }
func (NoSecrets) Has(string) bool            { return false }
func (NoSecrets) Delete(string) bool         { return false }
func (NoSecrets) Set(string, string) error   { return ErrNoSecrets }
func (NoSecrets) SetInteractive(string, io.Reader, io.Writer, io.Writer) error {
	return ErrNoSecrets
}
func (NoSecrets) Available() bool { return false }
