package trust

import (
	"fmt"
	"strings"
)

// components.lock YAZMA tarafı (scripts/lock-component.sh karşılığı). Okuma/doğrulama lock.go'dadır.

// Text, girdinin dosyadaki metnidir (sonunda tek satır sonu).
func (e LockEntry) Text() string {
	return fmt.Sprintf("[%s]\nrepo = %s\nversion = %s\nsha256 = %s\nsigners = %s\n", e.ID, e.Repo, e.Version, e.SHA256, e.Signers)
}

// ReplaceLockEntry, eski dosya metninden e.ID girdisini çıkarır (diğer satırlar, yorumlar aynen kalır), yeni girdiyi SONA ekler
// ve art arda gelen boş satırları teke indirir (kabukta: awk … ; printf '\n%s\n' | cat -s).
func ReplaceLockEntry(old string, e LockEntry) string {
	var kept []string
	if old != "" {
		lines := strings.Split(old, "\n")
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		skip := false
		for _, l := range lines {
			if len(l) >= 2 && l[0] == '[' && l[len(l)-1] == ']' {
				skip = l == "["+e.ID+"]"
			}
			if !skip {
				kept = append(kept, l)
			}
		}
	}
	text := strings.Join(kept, "\n")
	if len(kept) > 0 {
		text += "\n"
	}
	text += "\n" + e.Text()

	// cat -s: art arda boş satırlar teke iner
	in := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	var out []string
	prevBlank := false
	for _, l := range in {
		if l == "" {
			if prevBlank {
				continue
			}
			prevBlank = true
		} else {
			prevBlank = false
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n") + "\n"
}

// ExtractArchive, CheckArchive'dan geçmiş arşivi dest'e açar.
func ExtractArchive(archive, dest string) error { return extractTo(archive, dest) }
