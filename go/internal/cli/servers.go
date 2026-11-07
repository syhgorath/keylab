package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"sshpocket-go/internal/runner"
	"sshpocket-go/internal/servers"
)

// Etkileşimli sunucu işlemleri (kabuktaki ImportSSHConfig.sh, Knock.sh knock_manage, SSHModuleEdit.sh ssh_keychain_manage).
// Terminal menüsü (ok tuşu/fzf) bilerek yoktur: kabuğun düz numaralı akışının aynısı. İstem metinleri kabukla aynıdır.

// Servers: bağımlılıklar.
type Servers struct {
	Dir         string // servers/
	In          *bufio.Reader
	Out, Err    io.Writer
	ChildIn     *os.File // security/ssh-add gibi alt süreçlerin terminali
	Keychain    servers.Keychain
	DefaultUser string // ~/.ssh/config'te User yoksa (kabuk: $USER)
	Askpass     string // ssh-add askpass olarak çağrılacak ikili (boşsa kendisi)
}

func (s *Servers) say(f string, a ...any) { fmt.Fprintf(s.Out, f+"\n", a...) }

func (s *Servers) ask(prompt string) (string, bool) {
	fmt.Fprint(s.Err, prompt)
	l, err := s.In.ReadString('\n')
	if err != nil && l == "" {
		fmt.Fprintln(s.Err)
		return "", false
	}
	return strings.TrimSuffix(l, "\n"), true
}

// ImportSSHConfig, ImportSSHConfig.sh (düz akış): ~/.ssh/config'teki Host kayıtlarından seçilenleri servers/ altına yazar.
// Private key'ler KOPYALANMAZ; IdentityFile yolu KEY olarak yazılır.
func (s *Servers) ImportSSHConfig(config string) int {
	if st, err := os.Stat(config); err != nil || !st.Mode().IsRegular() {
		s.say("⚠️  Config bulunamadı: %s", config)
		return 1
	}
	ents, _ := servers.ParseSSHConfig(config)
	user := s.DefaultUser
	for i := range ents {
		if ents[i].User == "" {
			ents[i].User = user
		}
		if ents[i].Port == "" {
			ents[i].Port = "22"
		}
	}
	if len(ents) == 0 {
		s.say("Joker içermeyen Host kaydı bulunamadı.")
		return 0
	}
	s.say("Bulunan sunucular (%s):", config)
	for i, e := range ents {
		s.say("  %2d) %-20s %s@%s:%s", i+1, e.Alias, e.User, e.Host, e.Port)
	}
	s.say("")
	pick, ok := s.ask("İçe aktarılacaklar (örn: 1,3  |  a = hepsi  |  Enter = iptal): ")
	if !ok || pick == "" {
		s.say("İptal edildi.")
		return 0
	}
	var sel []servers.HostEntry
	if pick == "a" || pick == "A" {
		sel = ents
	} else {
		for _, n := range strings.FieldsFunc(pick, func(r rune) bool { return r == ',' || r == ' ' }) {
			if i, err := strconv.Atoi(n); err == nil && strings.Trim(n, "0123456789") == "" && i >= 1 && i <= len(ents) {
				sel = append(sel, ents[i-1])
			} else {
				s.say("⚠️  Geçersiz seçim atlandı: %s", n)
			}
		}
	}
	imported := 0
	for _, e := range sel {
		r := servers.ImportHosts(s.Dir, []servers.HostEntry{e}, s.DefaultUser)[0]
		switch {
		case r.Skipped:
			s.say("⏭️  %s zaten var, atlandı", r.Name)
		case r.Err != nil:
			s.say("⚠️  %s içe aktarılamadı (geçersiz değer)", r.Name)
		default:
			k := ""
			if e.IdentityFile != "" {
				k = "  key: " + e.IdentityFile
			}
			s.say("✅ %s (%s)%s", r.Name, servers.Prefix(r.Name), k)
			imported++
		}
	}
	s.say("")
	s.say("%d sunucu eklendi. Passphrase'i Keychain'e eklemek için (varsa):", imported)
	s.say("  security add-generic-password -U -a USBMonitor_<ÖNEK>_Passphrase -s USBMonitor -w")
	return 0
}

func (s *Servers) security(args ...string) error {
	cmd := exec.Command("security", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = s.ChildIn, s.Out, s.Err
	return cmd.Run()
}

func (s *Servers) keyNote() {
	s.say("🔐 macOS şimdi SSH anahtarınızın PASSPHRASE'ini (anahtarı oluştururken verdiğiniz) Keychain'e kaydetmek için soracak.")
	s.say("   İstem 'password data for new item:' diye görünür; yazarken ekranda bir şey görünmez.")
	s.say("   Aynı passphrase'i yazın; macOS doğrulama için bir kez daha sorabilir. Sunucunun sudo parolası DEĞİLDİR.")
}

func (s *Servers) dirOf(name string) (string, bool) {
	if !servers.NameValid(name) {
		fmt.Fprintf(s.Err, "⚠️  Geçersiz sunucu adı: %s\n", name)
		return "", false
	}
	d := filepath.Join(s.Dir, name)
	if st, err := os.Stat(filepath.Join(d, "server.env")); err != nil || !st.Mode().IsRegular() {
		fmt.Fprintf(s.Err, "⚠️  Sunucu yok: %s\n", name)
		return "", false
	}
	return d, true
}

func protoOK(v string) bool { return v == "tcp" || v == "udp" }
func delayOK(v string) bool {
	n, err := strconv.Atoi(v)
	return err == nil && strings.Trim(v, "0123456789") == "" && n <= 5000
}

// knockAsk, _knock_ask: geçersizse nedenini söyleyip yeniden sorar (en çok 3 deneme); yanıt küçük harfe çevrilir.
func (s *Servers) knockAsk(prompt, def string, ok func(string) bool, why string) (string, bool) {
	for tries := 0; tries < 3; tries++ {
		ans, got := s.ask(prompt)
		if !got {
			return "", false
		}
		if ans == "" {
			ans = def
		}
		ans = strings.ToLower(ans)
		if ok(ans) {
			return ans, true
		}
		s.say("   ⚠️  '%s' geçerli değil: %s", ans, why)
	}
	s.say("⚠️  3 geçersiz deneme; ayar yapılmadı.")
	return "", false
}

// KnockManage, knock_manage (düz akış): protokol, bekleme ve dizi ayarı; dizi Keychain'de.
func (s *Servers) KnockManage(name string) int {
	dir, ok := s.dirOf(name)
	if !ok {
		return 1
	}
	prefix := servers.Prefix(name)
	account := servers.KnockAccount(prefix)
	s.say("")
	fmt.Fprintf(s.Out, "%s\n\n", "Port knocking: sunucunun SSH portu kapalıyken, önceden kararlaştırılmış bir port\nsırasına 'vurarak' açtırma yöntemidir. Sunucu yöneticisi bu sırayı sunucuda tanımlamış olmalı.")
	if servers.KnockEnabled(dir) && s.Keychain.Has(account) {
		p, _ := servers.Get(dir, "KNOCK_PROTO")
		d, _ := servers.Get(dir, "KNOCK_DELAY_MS")
		s.say("Durum: ✅ etkin (%s, gecikme %s ms; dizi Keychain'de)", p, d)
	} else {
		s.say("Durum: ❌ kapalı")
	}
	s.say("  1) Diziyi kaydet / güncelle")
	s.say("  2) Kapat (Keychain kaydını sil)")
	s.say("  0) Geri (q)")
	choice, got := s.ask("Seçim [0]: ")
	if !got {
		return 0
	}
	switch choice {
	case "1":
		s.say("")
		s.say("1/3 Protokol: knock paketlerinin türü. Çoğu kurulumda tcp; sunucu yöneticiniz udp dediyse udp.")
		proto, ok := s.knockAsk("    Protokol (tcp/udp) [Enter = tcp]: ", "tcp", protoOK, "yalnızca tcp veya udp yazın (bilmiyorsanız Enter = tcp)")
		if !ok {
			return 1
		}
		s.say("2/3 Bekleme: her vuruş arasında beklenecek süre (milisaniye). Bilmiyorsanız 300 uygundur.")
		delay, ok := s.knockAsk("    Bekleme ms (0-5000) [Enter = 300]: ", "300", delayOK, "0 ile 5000 arasında bir sayı yazın")
		if !ok {
			return 1
		}
		s.say("3/3 Dizi: sunucu yöneticisinin verdiği port sırası, virgülle ayrılmış (örnek: 7000,8000,9000).")
		s.say("    Sıradaki istem ekranda görünmez ve macOS aynı değeri İKİ KEZ isteyebilir (ikincisinde aynısını yazın).")
		if err := s.security("add-generic-password", "-U", "-a", account, "-s", servers.KeychainService, "-w"); err != nil {
			s.say("⚠️  Kaydedilemedi")
			return 1
		}
		seq, _ := s.Keychain.Find(account)
		if !servers.KnockPortsValid(seq) {
			s.Keychain.Delete(account)
			s.say("⚠️  Dizi geçersiz ve silindi. Biçim: 1-10 port, her biri 1-65535, virgülle (örnek: 7000,8000,9000)")
			return 1
		}
		servers.Set(dir, "KNOCK", "keychain")
		servers.Set(dir, "KNOCK_PROTO", proto)
		servers.Set(dir, "KNOCK_DELAY_MS", delay)
		s.say("✅ Knock etkin (%s, %s ms): bağlanmadan önce otomatik gönderilir", proto, delay)
	case "2":
		s.Keychain.Delete(account)
		servers.Set(dir, "KNOCK", "")
		s.say("✅ Knock kapatıldı")
	}
	return 0
}

// KeychainManage, ssh_keychain_manage (düz akış): passphrase Keychain kaydını kaydet/güncelle/sil; kayıttan sonra anahtarı açarak doğrular.
func (s *Servers) KeychainManage(name string) int {
	dir, ok := s.dirOf(name)
	if !ok {
		return 1
	}
	prefix := servers.Prefix(name)
	account := servers.PassphraseAccount(prefix)
	key := servers.KeyPath(s.Dir, name)
	s.say("")
	if s.Keychain.Has(account) {
		s.say("Keychain kaydı: ✅ var (%s)", account)
	} else {
		s.say("Keychain kaydı: ❌ yok")
	}
	s.say("  1) Kaydet / güncelle")
	s.say("  2) Sil")
	s.say("  0) Geri (q)")
	choice, got := s.ask("Seçim [0]: ")
	if !got {
		return 0
	}
	switch choice {
	case "1":
		s.keyNote()
		if err := s.security("add-generic-password", "-U", "-a", account, "-s", servers.KeychainService, "-w"); err != nil {
			s.say("⚠️  Kaydedilemedi")
			return 0
		}
		s.say("✅ Keychain'e kaydedildi")
		if st, err := os.Stat(key); err == nil && st.Mode().IsRegular() {
			s.say("🔎 Doğrulanıyor (key agent'tan çıkarılıp passphrase ile yeniden ekleniyor)...")
			exec.Command("ssh-add", "-d", key).Run()
			ad := &runner.AgentAdder{Err: s.Err, Exe: s.Askpass}
			if ad.AddOnce(prefix, key) == nil {
				s.say("✅ Key açıldı, passphrase doğru.")
			} else {
				s.say("⚠️  Doğrulanamadı: passphrase yanlış olabilir. Seçenek 1 ile tekrar deneyin.")
			}
		}
	case "2":
		if s.Keychain.Delete(account) {
			s.say("✅ Keychain kaydı silindi")
		} else {
			s.say("ℹ️  Silinecek kayıt yok")
		}
	}
	_ = dir
	return 0
}

// SendKnock: bağlanmadan önce knock gönderir (etkin değilse hiçbir şey yapmaz).
func SendKnock(ctx context.Context, serversDir, name string) error {
	if !servers.NameValid(name) {
		return fmt.Errorf("geçersiz sunucu adı")
	}
	dir := filepath.Join(serversDir, name)
	ip, _ := servers.Get(dir, "IP")
	return servers.PreConnect(ctx, servers.Keychain{}, dir, servers.Prefix(name), ip)
}
