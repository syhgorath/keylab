package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// StartPrivateAgent: KeyLab'a ait özel bir ssh-agent başlatır (soket dir/agent.sock, yalnız bu kullanıcı). Dönen soket yolu SSH_AUTH_SOCK olarak
// ayarlanırsa KeyLab ve alt süreçleri kullanıcının kendi ajanını görmez/değiştirmez. stop: ajanı sonlandırır ve soketi siler (birden çok çağrı güvenli).
// Not: KeyLab SIGKILL ile öldürülürse ajan süreci kalabilir (anahtarlar yalnız bellekte; soket dosyası geçici klasörle birlikte silinir).
func StartPrivateAgent(dir string) (sock string, stop func(), err error) {
	if _, err := exec.LookPath("ssh-agent"); err != nil {
		return "", nil, fmt.Errorf("ssh-agent bulunamadı")
	}
	sock = filepath.Join(dir, "agent.sock")
	if len(sock) > 100 { // unix soket yolu sınırı (macOS 104)
		short, derr := os.MkdirTemp("/tmp", "kl-")
		if derr != nil {
			return "", nil, fmt.Errorf("soket yolu çok uzun: %s", sock)
		}
		sock = filepath.Join(short, "a")
	}
	cmd := exec.Command("ssh-agent", "-D", "-a", sock)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	stop = func() {
		_ = cmd.Process.Kill()
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
		}
		_ = os.Remove(sock)
		if filepath.Dir(sock) != dir {
			_ = os.Remove(filepath.Dir(sock))
		}
	}
	for i := 0; i < 50; i++ { // soket hazır olana kadar bekle (en çok ~3 sn)
		if c, derr := net.Dial("unix", sock); derr == nil {
			c.Close()
			return sock, stop, nil
		}
		select {
		case <-exited:
			return "", nil, fmt.Errorf("ssh-agent hemen sonlandı")
		case <-time.After(60 * time.Millisecond):
		}
	}
	stop()
	return "", nil, fmt.Errorf("ssh-agent soketi açılmadı")
}
