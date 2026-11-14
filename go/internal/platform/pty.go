package platform

import (
	"runtime"
	"strings"
)

// PTY: komutu sahte terminal (pty) altında çalıştırma ve gömülü terminal desteği.
type PTY interface {
	// Wrap: komutu pty altında çalıştıran sarmalayıcı komut (`script`).
	Wrap(argv []string) []string
	// TerminalSupported: gömülü terminal (ssh -tt) bu işletim sisteminde var mı?
	TerminalSupported() bool
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// BSDScriptPTY: macOS/BSD `script -q /dev/null komut…`.
type BSDScriptPTY struct{}

func (BSDScriptPTY) Wrap(argv []string) []string {
	return append([]string{"script", "-q", "/dev/null"}, argv...)
}
func (BSDScriptPTY) TerminalSupported() bool { return true }

// UtilLinuxScriptPTY: Linux (util-linux) `script -qfec 'komut…' /dev/null`.
type UtilLinuxScriptPTY struct{}

func (UtilLinuxScriptPTY) Wrap(argv []string) []string {
	q := make([]string, len(argv))
	for i, x := range argv {
		q[i] = shQuote(x)
	}
	return []string{"script", "-qfec", strings.Join(q, " "), "/dev/null"}
}
func (UtilLinuxScriptPTY) TerminalSupported() bool { return true }

// WindowsPTY: Windows'ta pty yok (ConPTY henüz yazılmadı): sarmalayıcı yok, gömülü terminal desteklenmez.
type WindowsPTY struct{}

func (WindowsPTY) Wrap(argv []string) []string { return argv }
func (WindowsPTY) TerminalSupported() bool     { return false }

// DefaultPTY: bu işletim sisteminin pty uygulaması.
func DefaultPTY() PTY {
	switch runtime.GOOS {
	case "darwin":
		return BSDScriptPTY{}
	case "windows":
		return WindowsPTY{}
	}
	return UtilLinuxScriptPTY{}
}
