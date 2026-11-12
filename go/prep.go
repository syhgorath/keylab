package main

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"keylab/internal/servers"
)

// PrepState: Özet sekmesindeki "Bağlantı hazırlığı" adımlarının BU BİLGİSAYARDA bilinen durumları.
// Sunucuya bağlanılmaz; "anahtar sunucuya gönderildi mi" bu yüzden burada yoktur (bağlanmadan bilinemez).
type PrepState struct {
	Key        bool `json:"key"`        // özel anahtar dosyası var
	Agent      bool `json:"agent"`      // anahtar şu an ssh-agent'ta yüklü
	Passphrase bool `json:"passphrase"` // passphrase bu bilgisayarın Keychain'inde kayıtlı
}

func (a *App) prepState(s Server) PrepState {
	st := PrepState{Key: s.HasKey}
	if !s.HasKey {
		return st
	}
	st.Passphrase = servers.Keychain{}.Has(servers.PassphraseAccount(servers.Prefix(s.Name)))
	st.Agent = keyInAgent(s.keyPath)
	return st
}

// keyInAgent: <anahtar>.pub'ın parmak izi `ssh-add -l` çıktısında geçiyor mu (özel anahtar ya da passphrase okunmaz).
func keyInAgent(keyPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh-keygen", "-lf", keyPath+".pub").Output()
	if err != nil {
		return false
	}
	f := strings.Fields(string(out))
	if len(f) < 2 || !strings.HasPrefix(f[1], "SHA256:") {
		return false
	}
	list, _ := exec.CommandContext(ctx, "ssh-add", "-l").Output()
	return strings.Contains(string(list), f[1])
}
