package main

// Taşınabilir (USB) modda bilgisayarda iz bırakmama: KeyLab'ın kendi yazdığı, USB'ye konamayan ya da konmaması gereken şeyler GEÇİCİ bir klasöre
// gider ve çıkışta silinir; USB'de tutulabilenler (known_hosts) USB'de durur.
//
//   - Python önbelleği (venv): taşınamaz (içindeki yollar bu bilgisayara özgü) ve FAT/exFAT sembolik bağ tutamaz → geçici klasör (her oturumda yeniden kurulur).
//   - Ayrı tarayıcı profili: Chromium'un SingletonLock sembolik bağı FAT/exFAT'ta oluşturulamaz → geçici klasör.
//   - ssh known_hosts: USB'deki durum klasörüne (state/known_hosts): kabul edilen host key'ler bilgisayarın ~/.ssh/known_hosts'una yazılmaz.
//
// Hâlâ bilgisayarda kalanlar: Keychain kayıtları (parola/knock), kullanıcının kendi ssh-agent'ına eklenen anahtarlar (Çıkış USB anahtarlarını siler),
// macOS'un kendi izleri (Spotlight, son kullanılanlar, günlükler).

import (
	"keylab/internal/envx"
	"os"
	"path/filepath"
	"sync"
)

// isPortableLayout: sunucular klasörü uygulamanın/ikilinin yanındaki servers/ mi (taşınabilir mod)?
func isPortableLayout(servers, exe string) bool {
	return filepath.Clean(servers) == filepath.Join(appBase(exe), "servers")
}

// setupPortableTraces: geçici klasörü kurar ve ortam değişkenlerini ayarlar (alt süreçler — eylemler, askpass — de görür). profBase: tarayıcı profilinin
// konacağı klasör. Kullanıcı bu değişkenleri kendisi vermişse ona dokunulmaz. cleanup bir kez çalışır.
func setupPortableTraces(stateDir string) (profBase string, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "keylab-")
	if err != nil {
		return "", nil, err
	}
	_ = os.Chmod(tmp, 0o700)
	if envx.Get("CACHE_DIR") == "" {
		os.Setenv("KEYLAB_CACHE_DIR", filepath.Join(tmp, "cache"))
	}
	if envx.Get("KNOWN_HOSTS") == "" && stateDir != "" {
		if err := os.MkdirAll(stateDir, 0o700); err == nil {
			os.Setenv("KEYLAB_KNOWN_HOSTS", filepath.Join(stateDir, "known_hosts"))
		}
	}
	var once sync.Once
	return tmp, func() { once.Do(func() { _ = os.RemoveAll(tmp) }) }, nil
}

// cleanupItem: çıkışta çalışacak bir temizlik ve kullanıcıya gösterilecek kodu (etiket: "agent", "tmp").
type cleanupItem struct {
	label string
	f     func()
}

// addCleanup: çıkışta (Çıkış düğmesi, sinyal) çalışacak temizlik ekler. f nil ise yok sayılır.
func (a *App) addCleanup(label string, f func()) {
	if f == nil {
		return
	}
	a.cleanMu.Lock()
	a.cleanups = append(a.cleanups, cleanupItem{label, f})
	a.cleanMu.Unlock()
}

// runCleanups: eklenen temizlikleri sırayla (bir kez) çalıştırır; çalıştırılanların etiketlerini döner.
func (a *App) runCleanups() []string { return a.runCleanupsSteps(nil, nil) }

// runCleanupsSteps: runCleanups + her temizlikten önce begin(etiket), sonra end(etiket) bildirimi (çıkış adımları için).
func (a *App) runCleanupsSteps(begin, end func(label string)) []string {
	a.cleanMu.Lock()
	items := a.cleanups
	a.cleanups = nil
	a.cleanMu.Unlock()
	done := make([]string, 0, len(items))
	for _, it := range items {
		if begin != nil {
			begin(it.label)
		}
		it.f()
		if end != nil {
			end(it.label)
		}
		done = append(done, it.label)
	}
	return done
}
