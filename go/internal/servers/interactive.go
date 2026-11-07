package servers

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Etkileşimli akışlar (pty altında çalışır): SSHKeyCopy.sh ssh_copy_key ve ModuleManager.sh cmd_remove.
// İstem metinleri kabukla birebir aynıdır (arayüz bu metinleri izleyip yanıt verir).

// Console: etkileşim kanalları. ChildIn alt süreçlere (ssh-copy-id, ssh-keygen) doğrudan verilir (tamponlu okuyucudan geçmez).
type Console struct {
	In      *bufio.Reader
	Out     io.Writer
	Err     io.Writer
	ChildIn *os.File
}

func (c Console) ask(prompt string) (string, bool) {
	fmt.Fprint(c.Err, prompt)
	l, err := c.In.ReadString('\n')
	if err != nil && l == "" {
		fmt.Fprintln(c.Err)
		return "", false
	}
	return strings.TrimSuffix(l, "\n"), true
}

func (c Console) say(format string, a ...any) { fmt.Fprintf(c.Out, format+"\n", a...) }

func (c Console) child(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.ChildIn, c.Out, c.Err
	return cmd
}

func yes(a string) bool { return a == "y" || a == "Y" }

// KeyCopy, ssh_copy_key: public key'i sunucuya ssh-copy-id ile gönderir. Parola yalnızca ssh'ın kendi isteminde girilir.
// PasswordAuthentication'ı kapatmak BİLEREK yapılmaz. Çıkış kodu: 0 tamam/yapılacak bir şey yok, 1 hata/iptal.
func KeyCopy(c Console, user, ip, port, key string) int {
	pub := key + ".pub"
	if st, err := os.Stat(pub); err != nil || !st.Mode().IsRegular() {
		if st, err := os.Stat(key); err != nil || !st.Mode().IsRegular() {
			c.say("⚠️  Key bulunamadı: %s", key)
			return 1
		}
		c.say("ℹ️  .pub yok, private key'den türetiliyor (passphrase istenebilir)...")
		f, err := os.OpenFile(pub, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			c.say("⚠️  Public key türetilemedi")
			return 1
		}
		cmd := exec.Command("ssh-keygen", "-y", "-f", key)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = c.ChildIn, f, c.Err
		err = cmd.Run()
		f.Close()
		if err != nil {
			os.Remove(pub)
			c.say("⚠️  Public key türetilemedi")
			return 1
		}
	}
	fp := ""
	if o, err := exec.Command("ssh-keygen", "-lf", pub).Output(); err == nil {
		if f := strings.Fields(firstLineOf(string(o))); len(f) >= 2 {
			fp = f[1]
		}
	}
	c.say("")
	c.say("Hedef:       %s@%s:%s", user, ip, port)
	c.say("Fingerprint: %s", fp)
	ans, _ := c.ask("Bu public key bu sunucuya eklensin mi? (y/n) ")
	if !yes(ans) {
		c.say("İptal edildi.")
		return 1
	}
	// Bu key sunucuda zaten kabul ediliyor mu? (etkileşimsiz deneme)
	if st, err := os.Stat(key); err == nil && st.Mode().IsRegular() {
		chk := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no",
			"-o", "IdentitiesOnly=yes", "-o", "ConnectTimeout=8", "-i", key, "-p", port, user+"@"+ip, "true")
		if chk.Run() == nil {
			c.say("✅ Bu key sunucuda zaten yetkili, yapılacak bir şey yok.")
			return 0
		}
	}
	boot, _ := c.ask("Girişte başka bir key dosyası kullanılsın mı? (yol girin, Enter = agent/parola) ")
	args := []string{"-i", pub, "-p", port}
	if boot != "" {
		if boot[0] == '~' {
			boot = os.Getenv("HOME") + boot[1:]
		}
		if st, err := os.Stat(boot); err != nil || !st.Mode().IsRegular() {
			c.say("⚠️  Dosya yok: %s", boot)
			return 1
		}
		args = append(args, "-o", "IdentityFile="+boot, "-o", "IdentitiesOnly=no")
	}
	args = append(args, user+"@"+ip)
	if user == "root" {
		c.say("")
		c.say("ℹ️  Kullanıcı 'root': Ubuntu/Debian'da root hesabı genelde kilitlidir ve root'a şifreyle")
		c.say("   girişe izin verilmez. Şifre reddedilirse normal bir kullanıcı deneyin ya da")
		c.say("   başarısızlıktan sonra gösterilecek 'elle ekleme' komutunu kullanın.")
	}
	c.say("")
	c.say("🔑 ssh-copy-id çalışıyor (gerekirse host key ve parola burada sorulur)...")
	if c.child("ssh-copy-id", args...).Run() == nil {
		c.say("")
		c.say("✅ Key gönderildi. Şimdi 'Bağlan' ile key ile girişi DENEYİN.")
		c.say("ℹ️  Sunucuda PasswordAuthentication'ı kapatmak istiyorsanız bunu")
		c.say("    key ile girişin çalıştığını gördükten sonra ELLE yapın.")
		return 0
	}
	c.say("⚠️  ssh-copy-id başarısız oldu.")
	c.say("   Sık nedenler: yanlış şifre, kullanıcının şifresiz/kilitli olması (örn. Ubuntu'da root),")
	c.say("   sunucuda PasswordAuthentication kapalı olması, fail2ban engeli.")
	if h, err := ManualKeyHelp(pub); err == nil {
		fmt.Fprint(c.Out, h)
	}
	return 1
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// RemoveInteractive, cmd_remove: sunucu adı yazılarak onaylanır; klasör silinir; Knock dizisi Keychain'den silinir;
// passphrase kaydı varsa silinip silinmeyeceği sorulur. Çıkış kodu: 0 tamam, 1 iptal/hata.
func RemoveInteractive(c Console, kc Keychain, serversDir, name string) int {
	if !NameValid(name) {
		fmt.Fprintf(c.Err, "⚠️  Geçersiz sunucu adı: '%s'\n", name)
		return 1
	}
	dir := filepath.Join(serversDir, name)
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
		fmt.Fprintf(c.Err, "⚠️  Sunucu yok: %s\n", name)
		return 1
	}
	c.say("Silinecek: %s", dir)
	if ents, err := os.ReadDir(dir); err == nil {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, n := range names {
			c.say("%s", n)
		}
	}
	confirm, _ := c.ask(fmt.Sprintf("Onaylamak için sunucu adını yazın (%s): ", name))
	if confirm != name {
		c.say("İptal edildi.")
		return 1
	}
	if err := Remove(serversDir, name); err != nil {
		fmt.Fprintf(c.Err, "⚠️  %v\n", err)
		return 1
	}
	c.say("✅ Silindi: %s", name)
	if _, err := exec.LookPath("security"); err == nil {
		up := Prefix(name)
		if kc.Delete(KnockAccount(up)) {
			c.say("✅ Knock dizisi Keychain'den silindi")
		}
		if kc.Has(PassphraseAccount(up)) {
			del, _ := c.ask("Keychain'deki passphrase kaydı da silinsin mi? (y/n) ")
			if yes(del) && kc.Delete(PassphraseAccount(up)) {
				c.say("✅ Keychain kaydı silindi")
			}
		}
	}
	return 0
}
