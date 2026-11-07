package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// SSHHelper.sh ssh_agent_add / ssh_agent_add_retry karşılığı: parolalı anahtarı macOS Keychain'deki parolayla ssh-agent'a ekler.
// Parola diske YAZILMAZ: ssh-add, SSH_ASKPASS olarak bu ikilinin kendisini çağırır (AskpassMain) ve o `security find-generic-password -w`
// ile Keychain'den okuyup yazar. Yanlış parolada ssh-add sonsuz yeniden denediği için askpass yalnızca BİR kez cevap verir.
//
// Bilinçli fark: Keychain'e kaydetme öncesi açıklama notu kabukta stdout'a gider (kapıda /dev/null'a atılıp kullanıcı görmezdi);
// Go bunu Stderr'e yazar, kullanıcı macOS'un "password data for new item:" isteminin ne olduğunu görür.

const (
	KeychainService = "USBMonitor"
	askpassEnv      = "SSHPOCKET_ASKPASS_STATE"
)

// KeychainAccount, ssh_keychain_account karşılığı.
func KeychainAccount(prefix string) string { return "USBMonitor_" + prefix + "_Passphrase" }

// AgentAdder: addRetry davranışının bağımlılıkları.
type AgentAdder struct {
	Ask     func(prompt string) (string, bool) // satır okur; girdi yoksa ok=false
	Err     io.Writer
	Exe     string // askpass olarak çağrılacak ikili (boşsa os.Executable)
	Tmp     string // askpass durum dosyası klasörü (boşsa TMPDIR/os temp)
	Keyring func(args ...string) *exec.Cmd
}

func (a *AgentAdder) errw() io.Writer {
	if a.Err == nil {
		return io.Discard
	}
	return a.Err
}

func (a *AgentAdder) security(args ...string) *exec.Cmd {
	if a.Keyring != nil {
		return a.Keyring(args...)
	}
	return exec.Command("security", args...)
}

func fingerprintOf(key string) string {
	o, _, _ := runCapture("ssh-keygen", []string{"-lf", key}, nil)
	f := strings.Fields(firstLine(string(o)))
	if len(f) < 2 {
		return ""
	}
	return f[1]
}

func agentHas(fp string) bool {
	if fp == "" {
		return false
	}
	o, _, _ := runCapture("ssh-add", []string{"-l"}, nil)
	return strings.Contains(string(o), fp)
}

// agentReachable: ssh-add -l çıkış 2 = ajana ulaşılamıyor (parola suçlanmaz).
func agentReachable() bool {
	cmd := exec.Command("ssh-add", "-l")
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {
		return false
	}
	return true
}

// add: ssh_agent_add (Keychain kaydı denetlenmez; askpass yoksa/boşsa ssh-add başarısız olur).
func (a *AgentAdder) add(prefix, key string) error {
	fp := fingerprintOf(key)
	if agentHas(fp) {
		fmt.Fprintln(a.errw(), "✅ SSH key zaten agent'ta")
		return nil
	}
	fmt.Fprintln(a.errw(), "🔐 SSH key agent'a ekleniyor (Keychain passphrase'i ile)...")
	exe := a.Exe
	if exe == "" {
		exe, _ = os.Executable()
	}
	tmp := a.Tmp
	if tmp == "" {
		tmp = os.TempDir()
	}
	state := ""
	if f, err := os.CreateTemp(tmp, "sshpocket-askpass."); err == nil {
		state = f.Name()
		f.Close()
		os.Remove(state) // yoksa "ilk çağrı"; askpass kendisi oluşturur
		defer os.Remove(state)
	}
	cmd := exec.Command("ssh-add", key)
	cmd.Env = append(os.Environ(),
		"SSH_ASKPASS="+exe,
		askpassEnv+"="+state,
		"SSH_ASKPASS_REQUIRE=force",
		"SSHPOCKET_ASKPASS=1",
		"ASKPASS_ACCOUNT="+KeychainAccount(prefix),
		"ASKPASS_SERVICE="+KeychainService,
	)
	if os.Getenv("DISPLAY") == "" {
		cmd.Env = append(cmd.Env, "DISPLAY=:0")
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	_ = cmd.Run()
	if agentHas(fp) {
		fmt.Fprintln(a.errw(), "✅ SSH key agent'a eklendi")
		return nil
	}
	fmt.Fprintln(a.errw(), "⚠️  Key agent'a eklenemedi, ssh passphrase'i kendisi soracak")
	return fmt.Errorf("ajana eklenemedi")
}

// AddRetry: ssh_agent_add_retry. Başarısızsa (ajana ulaşılıyorsa) "yanlış olabilir, yeniden kaydedeyim mi" diye sorar; en çok 3 deneme.
func (a *AgentAdder) AddRetry(prefix, key string) error {
	if a.add(prefix, key) == nil {
		return nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		if !agentReachable() {
			return fmt.Errorf("ssh-agent'a ulaşılamıyor")
		}
		if a.Ask == nil {
			return fmt.Errorf("girdi yok")
		}
		ans, ok := a.Ask("Kayıtlı passphrase yanlış olabilir. Yeniden girip Keychain'e kaydedeyim mi? [e/H]: ")
		if !ok {
			return fmt.Errorf("girdi yok")
		}
		switch ans {
		case "e", "E", "evet", "Evet":
		default:
			return fmt.Errorf("reddedildi")
		}
		w := a.errw()
		fmt.Fprintln(w, "🔐 macOS şimdi SSH anahtarınızın PASSPHRASE'ini (anahtarı oluştururken verdiğiniz) Keychain'e kaydetmek için soracak.")
		fmt.Fprintln(w, "   İstem 'password data for new item:' diye görünür; yazarken ekranda bir şey görünmez.")
		fmt.Fprintln(w, "   Aynı passphrase'i yazın; macOS doğrulama için bir kez daha sorabilir. Sunucunun sudo parolası DEĞİLDİR.")
		// -w son argüman: parola komut satırında görünmez, güvenli sorulur (security terminalden okur)
		cmd := a.security("add-generic-password", "-U", "-a", KeychainAccount(prefix), "-s", KeychainService, "-w")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		if a.add(prefix, key) == nil {
			return nil
		}
	}
	return fmt.Errorf("ajana eklenemedi")
}

// AskpassMain: ikili SSH_ASKPASS olarak çağrıldığında (SSHPOCKET_ASKPASS=1) main'in en başında çalıştırılır; çalıştıysa true döner
// ve süreç çıkış kodu code olmalıdır. askpass.sh ile aynı: durum dosyası varsa boş satır, yoksa oluşturup Keychain parolasını yazar.
func AskpassMain(stdout io.Writer) (handled bool, code int) {
	if os.Getenv("SSHPOCKET_ASKPASS") != "1" {
		return false, 0
	}
	if st := os.Getenv(askpassEnv); st != "" {
		if _, err := os.Lstat(st); err == nil {
			fmt.Fprintln(stdout)
			return true, 0
		}
		if f, err := os.OpenFile(st, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err == nil {
			f.Close()
		}
	}
	acct := os.Getenv("ASKPASS_ACCOUNT")
	if acct == "" {
		return true, 1
	}
	svc := os.Getenv("ASKPASS_SERVICE")
	if svc == "" {
		svc = KeychainService
	}
	cmd := exec.Command("security", "find-generic-password", "-a", acct, "-s", svc, "-w")
	cmd.Stdout = stdout
	if err := cmd.Run(); err != nil {
		return true, 1
	}
	return true, 0
}
