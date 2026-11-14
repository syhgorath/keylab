package platform

import (
	"bytes"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func guestOff(t *testing.T) {
	t.Setenv("KEYLAB_GUEST", "")
	t.Setenv("SSHPOCKET_GUEST", "")
}

func TestGuestModeEnv(t *testing.T) {
	guestOff(t)
	if GuestMode() {
		t.Fatal("varsayılan açık olmamalı")
	}
	SetGuestMode(true)
	if !GuestMode() || os.Getenv("KEYLAB_GUEST") != "1" {
		t.Fatal("açılmadı / alt süreçler için ortam değişkeni yok")
	}
	SetGuestMode(false)
	if GuestMode() || os.Getenv("KEYLAB_GUEST") != "0" {
		t.Fatal("kapanmadı")
	}
}

func TestGuestModeSwitchesBackends(t *testing.T) {
	guestOff(t)
	t.Setenv("PATH", t.TempDir()) // hiçbir araç yok: misafirde ZATEN hiçbir araç çağrılmamalı
	if _, ok := Default().(NoSecrets); ok {
		t.Fatal("misafir değilken NoSecrets")
	}
	SetGuestMode(true)
	if _, ok := Default().(NoSecrets); !ok {
		t.Fatalf("misafirde Default: %T", Default())
	}
	if _, ok := For("Baska").(NoSecrets); !ok {
		t.Fatalf("misafirde For: %T", For("Baska"))
	}
	if got := strings.Join(DefaultAgent().AddKeyArgv("/k"), " "); got != "ssh-add /k" {
		t.Fatalf("misafirde ssh-add: %q (Keychain bayrağı olmamalı)", got)
	}
	if SecretsSupported() {
		t.Fatal("misafirde Keychain desteklenmiyor sayılmalı")
	}
}

func TestNoSecretsBehaviour(t *testing.T) {
	var s Secrets = NoSecrets{}
	if _, ok := s.Find("a"); ok || s.Has("a") || s.Delete("a") || s.Available() {
		t.Error("NoSecrets bir şey buldu/sildi")
	}
	if err := s.Set("a", "b"); !errors.Is(err, ErrNoSecrets) {
		t.Errorf("Set: %v", err)
	}
	if err := s.SetInteractive("a", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); !errors.Is(err, ErrNoSecrets) {
		t.Errorf("SetInteractive: %v", err)
	}
}

// Gerçek ssh-agent: özel ajan başlar, anahtar eklenir, başka ajandan AYRIDIR, stop soketi ve süreci temizler.
func TestPrivateAgentLifecycle(t *testing.T) {
	for _, tool := range []string{"ssh-agent", "ssh-add", "ssh-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " yok")
		}
	}
	dir := t.TempDir()
	sock, stop, err := StartPrivateAgent(dir)
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			stop()
		}
	}()
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("soket bağlanamadı: %v", err)
	}
	c.Close()
	if st, err := os.Stat(sock); err != nil || st.Mode()&os.ModeSocket == 0 {
		t.Fatalf("soket dosyası: %v %v", st, err)
	}
	key := filepath.Join(t.TempDir(), "k")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, out)
	}
	run := func(sk string, args ...string) (string, error) {
		cmd := exec.Command("ssh-add", args...)
		cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+sk)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run(sock, key); err != nil {
		t.Fatalf("ssh-add: %v %s", err, out)
	}
	if out, err := run(sock, "-l"); err != nil || !strings.Contains(out, "ED25519") {
		t.Fatalf("özel ajanda anahtar yok: %v %s", err, out)
	}
	// ikinci bir özel ajan birincinin anahtarını GÖRMEZ (kullanıcının ajanı da görmez)
	sock2, stop2, err := StartPrivateAgent(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer stop2()
	if out, _ := run(sock2, "-l"); strings.Contains(out, "ED25519") {
		t.Fatalf("başka ajan anahtarı gördü: %s", out)
	}
	alive := func() bool { // ajan SÜRECİ (yalnız soket dosyası değil) yaşıyor mu?
		pg, err := exec.LookPath("pgrep")
		if err != nil {
			return false
		}
		return exec.Command(pg, "-f", "ssh-agent -D -a "+sock).Run() == nil
	}
	if _, err := exec.LookPath("pgrep"); err == nil && !alive() {
		t.Fatal("özel ajan süreci çalışmıyor görünüyor")
	}
	stop()
	stopped = true
	stop() // ikinci çağrı güvenli
	for i := 0; i < 30 && alive(); i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if alive() {
		t.Fatal("özel ajan süreci stop sonrası hâlâ yaşıyor")
	}
	if _, err := os.Stat(sock); err == nil {
		t.Fatal("soket silinmedi")
	}
	for i := 0; i < 30; i++ {
		if c, err := net.DialTimeout("unix", sock, 50*time.Millisecond); err != nil {
			return
		} else {
			c.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("ajan hâlâ bağlantı kabul ediyor")
}

func TestPrivateAgentMissingTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, _, err := StartPrivateAgent(t.TempDir()); err == nil {
		t.Fatal("ssh-agent yokken hata dönmedi")
	}
}
