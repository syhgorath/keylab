// Package settings: sshpocket.conf (ANAHTAR=değer) — Settings.sh karşılığı.
// Öncelik: ortam değişkeni (SSHPOCKET_<ANAHTAR>) > dosya > varsayılan. Dosya ASLA source edilmez; yalnızca bilinen anahtarlar okunur.
// Terminal menüsü kaldırıldığı için UI anahtarı kullanılmaz ama dosyada varsa korunur ve doğrulanır (geriye uyum).
package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Known, settings_known.
func Known(k string) bool { return k == "STATUS" || k == "STATUS_TIMEOUT" || k == "UI" }

// Default, settings_default.
func Default(k string) (string, bool) {
	switch k {
	case "STATUS":
		return "on", true
	case "STATUS_TIMEOUT":
		return "1", true
	case "UI":
		return "auto", true
	}
	return "", false
}

// Normalize, settings_normalize.
func Normalize(k, v string) (string, bool) {
	switch k {
	case "STATUS":
		switch v {
		case "on", "1", "true", "yes":
			return "on", true
		case "off", "0", "false", "no":
			return "off", true
		}
	case "STATUS_TIMEOUT":
		if len(v) == 1 && v[0] >= '1' && v[0] <= '5' {
			return v, true
		}
	case "UI":
		switch v {
		case "auto", "arrows", "numbers", "fzf":
			return v, true
		}
	}
	return "", false
}

// Store: ayar dosyası yolu (boşsa Get yalnızca ortam ve varsayılana bakar).
type Store struct{ File string }

func (s Store) fileRaw(k string) string {
	if s.File == "" {
		return ""
	}
	b, err := os.ReadFile(s.File)
	if err != nil {
		return ""
	}
	raw := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, k+"=") { // son tanım kazanır
			raw = line[len(k)+1:]
		}
	}
	raw = strings.ReplaceAll(raw, "\r", "")
	return strings.Trim(raw, " \t\n\v\f\r")
}

// Get, settings_get.
func (s Store) Get(k string) (string, bool) {
	if !Known(k) {
		return "", false
	}
	if raw := os.Getenv("SSHPOCKET_" + k); raw != "" {
		if v, ok := Normalize(k, raw); ok {
			return v, true
		}
	}
	if raw := s.fileRaw(k); raw != "" {
		if v, ok := Normalize(k, raw); ok {
			return v, true
		}
	}
	return Default(k)
}

// Source, settings_source: env | file | default.
func (s Store) Source(k string) string {
	if raw := os.Getenv("SSHPOCKET_" + k); raw != "" {
		if _, ok := Normalize(k, raw); ok {
			return "env"
		}
	}
	if raw := s.fileRaw(k); raw != "" {
		if _, ok := Normalize(k, raw); ok {
			return "file"
		}
	}
	return "default"
}

// Set, settings_set: diğer satırlar korunur; geçersiz değer/anahtar reddedilir; dosya izni 644.
func (s Store) Set(k, v string) error {
	if !Known(k) {
		return fmt.Errorf("Bilinmeyen ayar: %s", k)
	}
	val, ok := Normalize(k, v)
	if !ok {
		return fmt.Errorf("Geçersiz değer: %s", v)
	}
	var out strings.Builder
	done := false
	if b, err := os.ReadFile(s.File); err == nil {
		lines := strings.Split(string(b), "\n")
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for _, line := range lines {
			if strings.HasPrefix(line, k+"=") {
				if !done {
					out.WriteString(k + "=" + val + "\n")
					done = true
				}
				continue
			}
			out.WriteString(line + "\n")
		}
	} else {
		out.WriteString("# sshpocket ayarları (gizli bilgi YAZMAYIN)\n")
	}
	if !done {
		out.WriteString(k + "=" + val + "\n")
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.File), filepath.Base(s.File)+".")
	if err != nil {
		return errors.New("Ayar dosyası yazılamıyor: " + s.File)
	}
	name := tmp.Name()
	_, werr := tmp.WriteString(out.String())
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(name)
		return errors.New("Ayar dosyası yazılamıyor: " + s.File)
	}
	os.Chmod(name, 0o644)
	if err := os.Rename(name, s.File); err != nil {
		os.Remove(name)
		return errors.New("Ayar dosyası yazılamıyor: " + s.File)
	}
	return nil
}
