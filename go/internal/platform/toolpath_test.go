package platform

import (
	"os"
	"strings"
	"testing"
)

func TestAugmentPATH(t *testing.T) {
	all := func(string) bool { return true }
	none := func(string) bool { return false }
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	// Dock'tan açılan çıplak PATH: eksikler sona eklenir, sistem sırası korunur
	got := AugmentPATH("/usr/bin:/bin:/usr/sbin:/sbin", dirs, all)
	if got != "/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin" {
		t.Fatalf("PATH: %q", got)
	}
	// zaten varsa (sondaki / olsa da) yinelenmez, kullanıcının sırası değişmez
	got = AugmentPATH("/usr/local/bin/:/opt/homebrew/bin:/usr/bin", dirs, all)
	if got != "/usr/local/bin/:/opt/homebrew/bin:/usr/bin" {
		t.Fatalf("yinelendi: %q", got)
	}
	// olmayan klasör eklenmez
	if got = AugmentPATH("/usr/bin", dirs, none); got != "/usr/bin" {
		t.Fatalf("olmayan klasör eklendi: %q", got)
	}
	// boş PATH
	if got = AugmentPATH("", dirs[:1], all); got != "/opt/homebrew/bin" {
		t.Fatalf("boş PATH: %q", got)
	}
	// yineleme tekrar çağrıda da eklenmez (idempotent)
	once := AugmentPATH("/usr/bin", dirs, all)
	if twice := AugmentPATH(once, dirs, all); twice != once || strings.Count(twice, "/opt/homebrew/bin") != 1 {
		t.Fatalf("idempotent değil: %q", twice)
	}
}

func TestEnsureToolPathKeepsExisting(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	EnsureToolPath()
	if p := getenvPath(); !strings.HasPrefix(p, "/usr/bin:/bin") {
		t.Fatalf("mevcut PATH bozuldu: %q", p)
	}
}

func getenvPath() string { return os.Getenv("PATH") }
