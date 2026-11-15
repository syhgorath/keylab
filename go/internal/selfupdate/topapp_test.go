package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

// exFAT/FAT USB'de macOS açılan her öğenin yanına AppleDouble yan dosyası (._KeyLab.app) yazar: paket yine tek .app olarak tanınmalı.
func TestTopAppIgnoresAppleDoubleSidecars(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "KeyLab.app", "Contents"), 0o755)
	os.WriteFile(filepath.Join(d, "._KeyLab.app"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(d, ".DS_Store"), []byte("x"), 0o644)
	got, err := topApp(d)
	if err != nil || filepath.Base(got) != "KeyLab.app" {
		t.Fatalf("%q %v", got, err)
	}
	os.WriteFile(filepath.Join(d, "baska.txt"), []byte("x"), 0o644) // gerçek yabancı öğe yine reddedilir
	if _, err := topApp(d); err == nil {
		t.Fatal("beklenmeyen öğe kabul edildi")
	}
}
