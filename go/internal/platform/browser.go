package platform

// Browser: arayüzü kullanıcıya gösterme yolları (web arayüzü yerel adreste çalışır).
type Browser interface {
	// ChromiumCandidates: Chromium tabanlı tarayıcı ikilisi olabilecek yollar, öncelik sırasıyla (KeyLab bunlardan biriyle ayrı profil ve
	// eklentisiz uygulama penceresi açar). home: kullanıcı ev klasörü.
	ChromiumCandidates(home string) []string
	// OpenURL: adresi sistemin varsayılan tarayıcısında açar (başlatır, beklemez).
	OpenURL(url string) error
	// Supported: KeyLab açılışta arayüzü kendisi açabilir mi (macOS: evet; diğer sistemlerde adres terminale yazılır, kullanıcı açar).
	Supported() bool
}

// DefaultBrowser: bu işletim sisteminin tarayıcı açıcısı. macOS dışında şimdilik aynı yollar (adaylar yok/`open` yok); Linux/Windows sonra.
func DefaultBrowser() Browser { return MacBrowser{} }
