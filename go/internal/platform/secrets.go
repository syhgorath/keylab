// Package platform: işletim sistemine bağlı işlerin tek yeri. İş kodu (servers, cli, runner, ana paket) macOS'a özgü araçları (security,
// osascript, diskutil…) doğrudan çağırmaz; bu paketteki arayüzleri kullanır. macOS uygulaması önceki çağrıları BİREBİR korur
// (sözleşme testleri: internal/servers, internal/cli, internal/runner altındaki *_contract_test.go ve ana paketteki platform_contract_test.go);
// Linux/Windows uygulamaları sonradan eklenir.
package platform

import (
	"io"
	"runtime"
)

// KeychainService: gizli değerlerin tutulduğu Keychain servis adı (eski kayıtlar bu ada bağlıdır; DEĞİŞTİRİLMEZ).
const KeychainService = "USBMonitor"

// Secrets: gizli değer deposu (SSH anahtar passphrase'i, knock dizisi). Hesap adları çağıranda üretilir (ör. USBMonitor_<ÖNEK>_Passphrase).
// Değerler hiçbir zaman komut satırında geçmez.
type Secrets interface {
	// Find: hesabın değerini verir.
	Find(account string) (string, bool)
	// Has: kayıt var mı (değeri okumadan).
	Has(account string) bool
	// Delete: kaydı siler; yoksa false.
	Delete(account string) bool
	// Set: değeri yazar/günceller; değer komut satırında GÖRÜNMEDEN (standart girdiden) aracıya verilir.
	Set(account, value string) error
	// SetInteractive: değeri aracın kendi istemiyle (terminalden, ekranda yansımadan) sorup yazar; stdio çağıranın terminalidir.
	SetInteractive(account string, stdin io.Reader, stdout, stderr io.Writer) error
	// Available: bu bilgisayarda depo aracı var mı?
	Available() bool
}

// SecretsSupported: bu işletim sisteminde gizli değer deposu (Keychain) resmen destekleniyor mu? (Şimdilik yalnız macOS; Linux/Windows uygulamaları gelince genişler.)
func SecretsSupported() bool { return runtime.GOOS == "darwin" && !GuestMode() }

// Default: bu işletim sisteminin gizli değer deposu (KeychainService altında). macOS dışında şimdilik aynı `security` çağrılarını dener ve
// araç olmadığı için hepsi başarısız olur (bugünkü davranış); Linux/Windows uygulamaları gelince burada seçilecek.
func Default() Secrets { return For(KeychainService) }

// For: belirli bir servis adı için depo (yalnız SSH_ASKPASS'ın ASKPASS_SERVICE geçersiz kılması içindir).
func For(service string) Secrets {
	if GuestMode() {
		return NoSecrets{} // misafir mod: Keychain kullanılmaz
	}
	if service == "" {
		service = KeychainService
	}
	return SecurityKeychain{Service: service}
}
