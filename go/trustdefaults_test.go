package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Gömülü kopyalar depo kökündeki dosyalarla AYNI olmalı (senkron: go/scripts/sync-embedded.sh).
func TestEmbeddedMatchesRepoRoot(t *testing.T) {
	for _, name := range []string{"release_signers", "index_signers", "components.lock"} {
		root, err := os.ReadFile(filepath.Join("..", name))
		if err != nil {
			t.Skipf("depo kökü yok (%v)", err)
		}
		emb, ok := embeddedTrust(name)
		if !ok {
			t.Fatalf("%s gömülü değil", name)
		}
		if !bytes.Equal(root, emb) {
			t.Errorf("go/embedded/%s depo kökündeki %s ile farklı; çalıştırın: bash go/scripts/sync-embedded.sh", name, name)
		}
	}
	if _, ok := embeddedTrust("../etc/passwd"); ok {
		t.Error("bilinmeyen ad okunabildi")
	}
}

func TestEmbeddedSignersAreNotEmpty(t *testing.T) {
	for name, ns := range map[string]string{"release_signers": `namespaces="sshpocket-release"`, "index_signers": `namespaces="sshpocket-index"`} {
		b, _ := embeddedTrust(name)
		found := false
		for _, l := range strings.Split(string(b), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") && strings.Contains(l, " ssh-ed25519 ") && strings.Contains(l, ns) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s içinde ad alanı %s olan anahtar satırı yok (boş dosya özelliği kapatır)", name, ns)
		}
	}
	if b, _ := embeddedTrust("components.lock"); !strings.Contains(string(b), "[linux-hardening]") {
		t.Error("gömülü kilitte linux-hardening yok")
	}
}

func TestSignersPathLooseFirstThenEmbedded(t *testing.T) {
	t.Setenv("KEYLAB_CACHE_DIR", t.TempDir())
	matMu.Lock()
	matDone = map[string]string{}
	matMu.Unlock()
	root := t.TempDir()

	// kökte yok → gömülü kopya önbelleğe (0600) yazılır
	p := signersPath(root, "index_signers")
	if !strings.HasPrefix(p, os.Getenv("KEYLAB_CACHE_DIR")) || filepath.Base(p) != "index_signers" {
		t.Fatalf("gömülü yol önbellekte değil: %q", p)
	}
	got, _ := os.ReadFile(p)
	want, _ := embeddedTrust("index_signers")
	if !bytes.Equal(got, want) {
		t.Fatal("yazılan içerik gömülüyle aynı değil")
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Fatalf("izin %v", st.Mode().Perm())
	}
	// eski yer (sistem/) gömülüden önce gelir
	os.MkdirAll(filepath.Join(root, "sistem"), 0o755)
	os.WriteFile(filepath.Join(root, "sistem", "index_signers"), []byte("x"), 0o600)
	if q := signersPath(root, "index_signers"); q != filepath.Join(root, "sistem", "index_signers") {
		t.Fatalf("sistem/: %q", q)
	}
	// kökteki dosya her şeyden önce gelir
	os.WriteFile(filepath.Join(root, "index_signers"), []byte("y"), 0o600)
	if q := signersPath(root, "index_signers"); q != filepath.Join(root, "index_signers") {
		t.Fatalf("kök: %q", q)
	}
	// bilinmeyen ad gömülüye düşmez
	if q := signersPath(t.TempDir(), "baska"); strings.Contains(q, "trust") {
		t.Fatalf("bilinmeyen ad: %q", q)
	}
}

func TestMaterializeRewritesTamperedCopy(t *testing.T) {
	t.Setenv("KEYLAB_CACHE_DIR", t.TempDir())
	matMu.Lock()
	matDone = map[string]string{}
	matMu.Unlock()
	p, ok := materializeSigners("release_signers")
	if !ok {
		t.Fatal("yazılamadı")
	}
	os.WriteFile(p, []byte("sahte anahtar"), 0o600) // süreç dışından kurcalanmış
	matMu.Lock()
	matDone = map[string]string{} // yeni süreç
	matMu.Unlock()
	if _, ok := materializeSigners("release_signers"); !ok {
		t.Fatal("yeniden yazılamadı")
	}
	got, _ := os.ReadFile(p)
	want, _ := embeddedTrust("release_signers")
	if !bytes.Equal(got, want) {
		t.Fatal("kurcalanmış kopya yeni süreçte gömülüyle değiştirilmedi")
	}
	if _, ok := materializeSigners("components.lock"); ok {
		t.Error("kilit imzacı gibi yazılamaz")
	}
}

func TestBootstrapLock(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".keylab")
	if !bootstrapLock(root) {
		t.Fatal("boş kökte kilit oluşmadı")
	}
	got, _ := os.ReadFile(filepath.Join(root, "components.lock"))
	want, _ := embeddedTrust("components.lock")
	if !bytes.Equal(got, want) {
		t.Fatal("kopya gömülüyle aynı değil")
	}
	// var olan kilide dokunulmaz
	os.WriteFile(filepath.Join(root, "components.lock"), []byte("[ozel]\n"), 0o644)
	if bootstrapLock(root) {
		t.Fatal("var olan kilit ezildi")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "components.lock")); string(b) != "[ozel]\n" {
		t.Fatalf("kilit değişti: %q", b)
	}
	// eski sistem/ yerindeki kilit de sayılır
	r2 := t.TempDir()
	os.MkdirAll(filepath.Join(r2, "sistem"), 0o755)
	os.WriteFile(filepath.Join(r2, "sistem", "components.lock"), []byte("x"), 0o644)
	if bootstrapLock(r2) {
		t.Fatal("sistem/components.lock varken yazıldı")
	}
}

func TestEngineVersion(t *testing.T) {
	root := t.TempDir()
	old := buildVersion
	t.Cleanup(func() { buildVersion = old })
	buildVersion = ""
	if v := engineVersion(root); v != "" {
		t.Fatalf("bilinmiyorsa boş: %q", v)
	}
	os.WriteFile(filepath.Join(root, "VERSION"), []byte("3.9.1\n"), 0o644)
	if v := engineVersion(root); v != "3.9.1" {
		t.Fatalf("VERSION: %q", v)
	}
	buildVersion = "4.0.0"
	if v := engineVersion(root); v != "4.0.0" {
		t.Fatalf("gömülü sürüm önce gelir: %q", v)
	}
}
