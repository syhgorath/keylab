package main

// Gömülü güven noktaları: release_signers (sürüm imzası), index_signers (paket indeksi imzası) ve components.lock (paket kilidi).
// İkili bunları KENDİ İÇİNDE taşır; böylece indirilen tek .app/ikili, yanında hiçbir dosya olmadan ilk açılışta paketleri imzalı biçimde
// kurabilir. Kaynak: depo kökündeki dosyaların kopyaları (go/embedded/); `go test` kopyaların kökle AYNI olduğunu denetler, build-release.sh
// de sürümden önce denetler (senkron için: go/scripts/sync-embedded.sh).
//
// Güven sırası (signersPath): kök klasördeki dosya (ya da sistem/ altındaki eski yer) VARSA o kullanılır (bugünkü davranış; anahtar
// değişimi ve özel indeksler için); yoksa gömülü kopya kullanılır. Gömülü imzacılar diske kalıcı yazılmaz: ssh-keygen dosya yolu istediği için
// her süreç başlangıcında önbellek klasörüne (0600) yeniden yazılır; böylece kullanıcı klasöründe kalıcı, değiştirilebilir bir kopya oluşmaz.
// components.lock ise kullanıcı verisidir (upgrade yazar): kök klasörde yoksa gömülü kilit BİR KEZ oraya kopyalanır (bootstrapLock).
// Gömülü kilit yalnız ilk kurulumun başlangıcıdır; sonraki paket sürümleri imzalı indeksten gelir.

import (
	"embed"
	"keylab/internal/runner"
	"os"
	"path/filepath"
	"sync"
)

//go:embed embedded/release_signers embedded/index_signers embedded/components.lock
var embeddedFS embed.FS

var embeddedNames = map[string]bool{"release_signers": true, "index_signers": true, "components.lock": true}

// embeddedTrust: gömülü dosyanın içeriği (yalnız bilinen adlar).
func embeddedTrust(name string) ([]byte, bool) {
	if !embeddedNames[name] {
		return nil, false
	}
	b, err := embeddedFS.ReadFile("embedded/" + name)
	return b, err == nil
}

var (
	matMu   sync.Mutex
	matDone = map[string]string{}
)

// materializeSigners: gömülü imzacı dosyasını önbellek klasörüne yazar (süreç başına bir kez) ve yolunu verir.
func materializeSigners(name string) (string, bool) {
	if name != "release_signers" && name != "index_signers" {
		return "", false
	}
	matMu.Lock()
	defer matMu.Unlock()
	cache := runner.CacheDir()
	key := cache + "|" + name
	if p, ok := matDone[key]; ok {
		return p, true
	}
	b, ok := embeddedTrust(name)
	if !ok {
		return "", false
	}
	dir := filepath.Join(cache, "trust")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false
	}
	p := filepath.Join(dir, name)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return "", false
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return "", false
	}
	matDone[key] = p
	return p, true
}

// bootstrapLock: kök klasörde components.lock (ya da eski sistem/ yeri) yoksa gömülü kilidi kopyalar; kopyalandıysa true.
// Var olan kilide ASLA dokunmaz.
func bootstrapLock(root string) bool {
	for _, p := range []string{filepath.Join(root, "components.lock"), filepath.Join(root, "sistem", "components.lock")} {
		if _, err := os.Lstat(p); err == nil {
			return false
		}
	}
	b, ok := embeddedTrust("components.lock")
	if !ok {
		return false
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return false
	}
	return os.WriteFile(filepath.Join(root, "components.lock"), b, 0o644) == nil
}

// engineVersion: indeks denetimine verilecek sshpocket sürümü: ikilinin gömülü sürümü, yoksa kökteki VERSION; bilinmiyorsa "" (eski davranış).
func engineVersion(root string) string {
	if v := versionOf(root); v != "unknown" {
		return v
	}
	return ""
}
