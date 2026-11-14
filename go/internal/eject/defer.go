package eject

// Çalışan ikili çıkarılacak USB'nin KENDİSİNDEN çalışıyorsa (KeyLab.app USB'de), diski sürecin altından çekmek tehlikelidir: zorla çıkarılan
// birimdeki ikili sayfaları okunamaz hale gelir ve Go süreci ölmek yerine çekirdekleri kemiren bir döngüye girebilir (gerçek bir USB'de
// görüldü: 54 dakika, %478 CPU). Bu yüzden böyle durumda çıkarma, süreç bittikten SONRA ayrı bir yardımcı süreçle yapılır.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var executablePath = os.Executable // testte değiştirilir

// selfOnVolume: çalışan ikili usb klasörünün altında mı? (Hem ham hem sembolik bağları çözülmüş yollar denenir: /var → /private/var gibi.)
func selfOnVolume(usb string) bool {
	exe, err := executablePath()
	if err != nil {
		return false
	}
	under := func(e, u string) bool { return strings.HasPrefix(e, filepath.Clean(u)+string(filepath.Separator)) }
	if under(exe, usb) {
		return true
	}
	re, err1 := filepath.EvalSymlinks(exe)
	ru, err2 := filepath.EvalSymlinks(usb)
	return err1 == nil && err2 == nil && under(re, ru)
}

// WillDefer: Run bu USB için çıkarmayı ertelenecek mi (çalışan ikili o USB'den mi)? Arayüz iletisi için.
func WillDefer(usb string) bool { return selfOnVolume(usb) }

// deferScript: süreç bitsin diye bekler, normal çıkarmayı dener; olmazsa (artık meşgul değil) zorla unmount.
const deferScript = `sleep 2; diskutil eject "$1" >/dev/null 2>&1 || { diskutil unmount force "$1" >/dev/null 2>&1; }`

// runDeferred: çıkarmayı ayrı bir yardımcı süreçle KeyLab kapandıktan sonra yapar. Çağıran sürecin kısa süre içinde bitmesi gerekir
// (arayüzde Çıkış zaten 0,5 sn sonra, CLI komutu hemen biter).
func runDeferred(w io.Writer, usb string) int {
	cmd := exec.Command("/bin/sh", "-c", deferScript, "sh", usb)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(w, "⚠️  USB çıkarma yardımcısı başlatılamadı; KeyLab kapandıktan sonra USB'yi elle çıkarın")
		return 1
	}
	_ = cmd.Process.Release()
	fmt.Fprintf(w, "\n📤 KeyLab bu USB'den çalıştığı için çıkarma uygulama kapandıktan hemen sonra yapılacak: %s\n", filepath.Base(usb))
	return 0
}
