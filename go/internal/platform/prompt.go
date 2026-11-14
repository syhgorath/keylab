package platform

// Prompt: işletim sisteminin KENDİ parola penceresi (gizli giriş). Parola tarayıcıya/arayüze hiç uğramaz; pencere kapatılırsa hata döner.
type Prompt interface {
	// Available: bu bilgisayarda sistem parola penceresi var mı?
	Available() bool
	// Secret: istemi (kontrol karakterleri atılmış, en fazla 160 karakter) pencerede gösterir ve girilen parolayı verir. İptal → hata.
	// Parola komut satırında geçmez; çağıran işi bitince baytları sıfırlamalıdır.
	Secret(prompt string) ([]byte, error)
}

// DefaultPrompt: bu işletim sisteminin parola penceresi. macOS dışında şimdilik yok (Available false); Linux/Windows uygulamaları sonra.
func DefaultPrompt() Prompt { return OSAScriptPrompt{} }
