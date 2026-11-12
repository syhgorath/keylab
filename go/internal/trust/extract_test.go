package trust

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// extractTo normalde CheckArchive'dan geçmiş arşivle çağrılır; yine de tek başına da dest dışına yazmamalı (zip-slip).
func TestExtractToRejectsEscapingPaths(t *testing.T) {
	for _, name := range []string{"../kacis.txt", "abc/../../kacis.txt", "..", "abc/../../../etc/x"} {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
		tw.Write([]byte("x"))
		tw.Close()
		gz.Close()
		root := t.TempDir()
		dest := filepath.Join(root, "hedef")
		os.MkdirAll(dest, 0o755)
		arc := filepath.Join(root, "a.tar.gz")
		os.WriteFile(arc, buf.Bytes(), 0o600)
		if err := extractTo(arc, dest); err == nil {
			t.Errorf("%q kabul edildi", name)
		}
		if _, err := os.Stat(filepath.Join(root, "kacis.txt")); err == nil {
			t.Errorf("%q: dest dışına dosya yazıldı", name)
		}
	}
	// normal yol çalışmaya devam eder
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "abc-1.0.0/VERSION", Mode: 0o644, Size: 6, Typeflag: tar.TypeReg})
	tw.Write([]byte("1.0.0\n"))
	tw.Close()
	gz.Close()
	root := t.TempDir()
	arc := filepath.Join(root, "a.tar.gz")
	os.WriteFile(arc, buf.Bytes(), 0o600)
	dest := filepath.Join(root, "hedef")
	os.MkdirAll(dest, 0o755)
	if err := extractTo(arc, dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "abc-1.0.0", "VERSION")); string(b) != "1.0.0\n" {
		t.Errorf("normal arşiv açılmadı: %q", b)
	}
}
