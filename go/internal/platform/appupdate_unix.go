//go:build !windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// CodesignVerify: uygulama paketinin imzası (ad-hoc dahil) bozuk mu? `codesign --verify --deep --strict`. Güncelleme paketi açıldıktan sonra, yerine konmadan önce denetlenir
// (zip'in açılması paketin imza mührünü bozmuş olabilir).
func CodesignVerify(appPath string) error {
	out, err := exec.Command("codesign", "--verify", "--deep", "--strict", appPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign doğrulaması başarısız: %v: %s", err, trimOut(out))
	}
	return nil
}

func trimOut(b []byte) string {
	s := string(b)
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

// relaunchScript: $1 = uygulama paketi, $2 = ana süreç PID'i. Süreç bitene dek bekler (en çok 2 dk), sonra yeni sürümü yeni örnek olarak açar.
const relaunchScript = `i=0; while kill -0 "$2" 2>/dev/null; do i=$((i+1)); [ "$i" -gt 240 ] && exit 0; sleep 0.5; done; sleep 1; open -n "$1"`

// RelaunchApp: bu süreç bittikten SONRA güncellenmiş uygulamayı yeniden açan, bağımsız (ayrı oturumlu) bir yardımcı başlatır.
func RelaunchApp(appPath string) error { return relaunchAfter(appPath, os.Getpid()) }

func relaunchAfter(appPath string, pid int) error {
	cmd := exec.Command("/bin/sh", "-c", relaunchScript, "sh", appPath, strconv.Itoa(pid))
	detachProc(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
