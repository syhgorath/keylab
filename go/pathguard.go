package main

import (
	"os"
	"path/filepath"
	"strings"
)

// pathWithin: base altındaki bir yolu (base/<parça>…) kurar ve sonucun base'in İÇİNDE kaldığını doğrular; taşıyorsa ya da base'in kendisiyse ""
// döner. Ad doğrulamaları (reName, reNewName) asıl savunmadır; bu, dosya işlemine girmeden hemen önceki ikinci, YAPISAL kapıdır
// ("..", mutlak yol vb. burada elenir). Mutlak yollar karşılaştırılır; base göreli verilmiş olabilir.
func pathWithin(base string, parts ...string) string {
	root, err := filepath.Abs(base)
	if err != nil {
		return ""
	}
	p, err := filepath.Abs(filepath.Join(append([]string{root}, parts...)...))
	if err != nil || !strings.HasPrefix(p, root+string(os.PathSeparator)) {
		return ""
	}
	return p
}

// serverPath: <servers>/<sunucu>[/<parça>…]. Sunucu klasörünün kendisi ya da altı döner; serversDir'in dışına taşan ya da serversDir'in
// kendisine çıkan ad ("..", ".") için "".
func (a *App) serverPath(parts ...string) string { return pathWithin(a.serversDir, parts...) }
