package main

// Platform kapısı: işletim sistemine bağlı kod YALNIZ internal/platform (ve macOS uygulaması internal/eject) içinde durur. İş kodunda yeni bir
// `runtime.GOOS` ya da macOS araç çağrısı (`security`, `osascript`, `diskutil`, `open`) eklenirse bu test düşer. Geçici istisnalar aşağıdaki
// listede gerekçesiyle durur; kalktıkça listeden silinir.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// istisnalar: göreli yol → neden
var platformGateAllow = map[string]string{
	"about.go":                          "yalnız GOOS/GOARCH bilgisini gösterir (Ayarlar)",
	"internal/selfupdate/selfupdate.go": "yayın dosyası adı <ad>-<sürüm>-<os>-<mimari>",
	"main.go":                           "macOS Terminal.app'te aç alternatifi (eylemler gömülü terminale taşınınca kalkacak)",
}

var (
	reGOOS = regexp.MustCompile(`runtime\.GOOS`)
	reTool = regexp.MustCompile(`exec\.Command(Context)?\([^"]*"(security|osascript|diskutil|open)"`)
)

// scanPlatformGate: kökteki üretim (test olmayan) Go dosyalarında kapıyı ihlal edenleri döner.
func scanPlatformGate(root string) []string {
	var bad []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch rel {
			case "webtest", "web", "internal/platform", "internal/eject", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		if _, ok := platformGateAllow[rel]; ok {
			return nil
		}
		b, _ := os.ReadFile(p)
		src := string(b)
		if reGOOS.MatchString(src) {
			bad = append(bad, rel+": runtime.GOOS")
		}
		if reTool.MatchString(src) {
			bad = append(bad, rel+": macOS aracı doğrudan çağrılıyor")
		}
		return nil
	})
	return bad
}

func TestPlatformGate(t *testing.T) {
	if bad := scanPlatformGate("."); len(bad) != 0 {
		t.Fatalf("işletim sistemine bağlı kod internal/platform dışında:\n  %s\n(gerekçeli geçici istisna için platform_gate_test.go'daki listeyi güncelleyin)", strings.Join(bad, "\n  "))
	}
}

func TestPlatformGateDetectsViolations(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755)
		os.WriteFile(filepath.Join(root, rel), []byte(src), 0o644)
	}
	write("ok.go", "package x\nfunc f() {}\n")
	write("bad_goos.go", "package x\nimport \"runtime\"\nvar _ = runtime.GOOS\n")
	write("sub/bad_tool.go", "package x\nimport \"os/exec\"\nvar _ = exec.Command(\"osascript\", \"-e\", \"x\")\n")
	write("sub/bad_ctx.go", "package x\nimport (\"context\";\"os/exec\")\nvar _ = exec.CommandContext(context.Background(), \"diskutil\", \"list\")\n")
	write("internal/platform/izinli.go", "package platform\nimport \"runtime\"\nvar _ = runtime.GOOS\n")
	write("bad_test.go", "package x\nimport \"runtime\"\nvar _ = runtime.GOOS\n")
	got := strings.Join(scanPlatformGate(root), "|")
	for _, want := range []string{"bad_goos.go: runtime.GOOS", "sub/bad_tool.go: macOS aracı", "sub/bad_ctx.go: macOS aracı"} {
		if !strings.Contains(got, want) {
			t.Errorf("ihlal yakalanmadı: %q (bulunanlar: %s)", want, got)
		}
	}
	for _, not := range []string{"ok.go", "internal/platform", "bad_test.go"} {
		if strings.Contains(got, not) {
			t.Errorf("yanlış alarm: %q (%s)", not, got)
		}
	}
}
