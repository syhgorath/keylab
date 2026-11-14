package platform

import "runtime"

// Agent: SSH anahtarını ssh-agent'a ekleme komutu (işletim sistemine göre ek bayraklar).
type Agent interface {
	// AddKeyArgv: anahtarı ajana ekleyen TAM komut (ilk öğe "ssh-add").
	AddKeyArgv(key string) []string
}

// KeychainAgent: macOS; parolayı Keychain'e de kaydeder (`ssh-add --apple-use-keychain`).
type KeychainAgent struct{}

func (KeychainAgent) AddKeyArgv(key string) []string {
	return []string{"ssh-add", "--apple-use-keychain", key}
}

// OpenSSHAgent: düz OpenSSH ssh-add (macOS dışı).
type OpenSSHAgent struct{}

func (OpenSSHAgent) AddKeyArgv(key string) []string { return []string{"ssh-add", key} }

// DefaultAgent: bu işletim sisteminin ajan komutu.
func DefaultAgent() Agent {
	if GuestMode() {
		return OpenSSHAgent{} // misafir mod: Keychain'e kaydetme yok; passphrase her oturumda sorulur
	}
	if runtime.GOOS == "darwin" {
		return KeychainAgent{}
	}
	return OpenSSHAgent{}
}
