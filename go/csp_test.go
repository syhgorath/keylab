package main

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func TestCSPForbidsInlineStyleAttributesAndInlineScript(t *testing.T) {
	for _, want := range []string{"default-src 'none'", "script-src 'self'", "style-src-attr 'none'", "connect-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(contentSecurityPolicy, want) {
			t.Errorf("CSP %q içermeli: %s", want, contentSecurityPolicy)
		}
	}
	for _, d := range strings.Split(contentSecurityPolicy, ";") {
		d = strings.TrimSpace(d)
		if strings.HasPrefix(d, "script-src") && strings.Contains(d, "unsafe") {
			t.Errorf("script-src güvensiz kaynak içermemeli: %s", d)
		}
	}
}

// Gömülü web dosyalarında satır içi stil özniteliği kalmamalı (CSP style-src-attr 'none' onları zaten engeller; burada kaynakta yakalanır).
// CSSOM (el.style.x = …) serbesttir; setAttribute("style") ve cssText engellidir.
var reInlineStyle = regexp.MustCompile(`(?i)\bstyle\s*=\s*["'\\]|setAttribute\(\s*["']style["']|\.style\.cssText`)

func TestWebAssetsHaveNoInlineStyleAttributes(t *testing.T) {
	n := 0
	err := fs.WalkDir(webFS, "web", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasPrefix(path, "web/vendor/") || !(strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".js")) {
			return nil
		}
		b, err := webFS.ReadFile(path)
		if err != nil {
			return err
		}
		n++
		for i, line := range strings.Split(string(b), "\n") {
			if reInlineStyle.MatchString(line) {
				t.Errorf("%s:%d satır içi stil: %s", path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 8 {
		t.Errorf("taranan web dosyası sayısı beklenenden az: %d", n)
	}
}
