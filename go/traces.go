package main

// Taşınabilir (USB) modda bilgisayarda iz bırakmama: KeyLab'ın kendi yazdığı, USB'ye konamayan ya da konmaması gereken şeyler GEÇİCİ bir klasöre
// gider ve çıkışta silinir; USB'de tutulabilenler (known_hosts) USB'de durur.
//
//   - Python önbelleği (venv): taşınamaz (içindeki yollar bu bilgisayara özgü) ve FAT/exFAT sembolik bağ tutamaz → geçici klasör (her oturumda yeniden kurulur).
//   - Ayrı tarayıcı profili: Chromium'un SingletonLock sembolik bağı FAT/exFAT'ta oluşturulamaz → geçici klasör.
//   - ssh known_hosts: USB'deki durum klasörüne (state/known_hosts): kabul edilen host key'ler bilgisayarın ~/.ssh/known_hosts'una yazılmaz.
//
//   - Gömülü pencerenin WebKit verisi (~/Library/WebKit ve ~/Library/Caches altında): misafir modda pencere kapanınca ve her açılışta silinir.
//     Yalnız pencereli derleme ve yalnız KENDİ klasörü (.app: paket kimliği; paketsiz ikili: dosya adı); bkz. webKitOwnID, sweepWebKitTraces.
//
// Hâlâ bilgisayarda kalanlar: Keychain kayıtları (parola/knock), kullanıcının kendi ssh-agent'ına eklenen anahtarlar (Çıkış USB anahtarlarını siler),
// macOS'un kendi izleri (Spotlight, son kullanılanlar, günlükler).

import (
	"keylab/internal/envx"
	"keylab/internal/platform"
	"os"
	"path/filepath"
	"strings"
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
	writeOwner(tmp) // sahibi (bu süreç): düzgün kapanmazsa sonraki açılışta eski oturum olarak tanınır (bkz. stale.go)
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

// webKitOwnID: BU sürecin gömülü penceresinin WebKit klasör adı. "" → silinecek bir şey yok.
//   - Pencereli derleme değilse (window=false: --chrome, CGO'suz) WebKit'e hiç yazılmaz.
//   - .app içinden çalışıyorsa paket kimliği.
//   - Paketsiz ikilide macOS dosya adını kullanır (yayın ikilisi keylab-X.Y.Z-darwin-arm64, deneme klasöründeki keylab). Yalnız "keylab" ile
//     başlayan adlar kabul edilir: başka ada çevrilmiş bir ikili, adı çakışan yabancı bir uygulamanın verisini silmesin.
//
// Böylece paketsiz bir deneme ikilisi aynı Mac'teki kurulu KeyLab.app'in verisine dokunmaz. USB'deki ve kurulu .app aynı kimliği paylaşır: ayrılamaz.
func webKitOwnID(window bool, exe string) string {
	if !window {
		return ""
	}
	if appBase(exe) != filepath.Dir(exe) { // exe bir .app/Contents/MacOS içinde
		return platform.WebKitBundleID
	}
	if name := filepath.Base(exe); strings.HasPrefix(strings.ToLower(name), "keylab") {
		return name
	}
	return ""
}

// sweepWebKitTraces: misafir (USB) modda gömülü pencerenin bu bilgisayarda bıraktığı WebKit verisini (çerez, yerel depo, önbellek) siler. WebKit süreçleri
// pencere kapanırken de yazdığı için pencere kapandıktan sonra ve her açılışta (Cmd+Q/çökme artıkları) çağrılır. Misafir değilse HİÇBİR ŞEY silinmez
// (kurulu/güvenilir Mac'te bu veri kullanıcının kendi uygulama verisidir). Yalnız id'nin iki yoluna (ev klasörü altında) bakar, sembolik bağları izlemez
// (bağın kendisi silinir, hedefine dokunulmaz).
func sweepWebKitTraces(guest bool, home, id string, remove func(string) error) int {
	if !guest {
		return 0
	}
	n := 0
	for _, d := range platform.WebKitTraceDirs(home, id) {
		if _, err := os.Lstat(d); err != nil {
			continue
		}
		if remove(d) == nil {
			n++
		}
	}
	return n
}

func sweepWebKitNow() {
	home, _ := os.UserHomeDir()
	sweepWebKitTraces(platform.GuestMode(), home, webKitOwnID(windowSupported(), selfExe()), os.RemoveAll)
}
