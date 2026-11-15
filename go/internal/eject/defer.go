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
	"strconv"
	"strings"
	"time"
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

// deferScript: $1 = USB, $2 = uygulamanın (çağıran sürecin) PID'i, $3 = en çok kaç yarım saniye beklenecek. Süreç BİTENE dek bekler; bitmeden ASLA
// çıkarma ya da zorla unmount yapmaz (süreç hâlâ diskteyken birimi çekmek onu %400+ CPU'luk döngüye sokar). Bekleme süresi dolarsa hiçbir şey yapmadan
// çıkar (USB'yi elle çıkarmak güvenli; zorla çıkarmak değil). Süreç bittikten sonra normal çıkarma, olmazsa (artık süreç yok, birim meşgul) zorla unmount.
const deferScript = `i=0; while kill -0 "$2" 2>/dev/null; do i=$((i+1)); [ "$i" -gt "$3" ] && exit 0; sleep 0.5; done; sleep 1; diskutil eject "$1" >/dev/null 2>&1 || { diskutil unmount force "$1" >/dev/null 2>&1; }`

var (
	// parentPID: yardımcının bitmesini bekleyeceği süreç (uygulamanın kendisi); testte değiştirilir.
	parentPID = os.Getpid
	// deferMaxWait: yardımcının uygulamanın bitmesini en çok ne kadar bekleyeceği (arayüzde "Kapat" gelmese bile uygulama 5 dk sonra kendiliğinden kapanır).
	deferMaxWait = 10 * time.Minute
)

// runDeferred: çıkarmayı ayrı bir yardımcı süreçle KeyLab kapandıktan sonra yapar. Yardımcı çağıran sürecin PID'ini izler: süreç ne zaman biterse
// (arayüzde "Kapat"a basınca, 5 dk sonra ya da CLI komutu bitince) ondan SONRA çıkarır.
func runDeferred(w io.Writer, usb string) int {
	cmd := exec.Command("/bin/sh", "-c", deferScript, "sh", usb, strconv.Itoa(parentPID()), strconv.Itoa(int(deferMaxWait/(500*time.Millisecond))))
	detach(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(w, "⚠️  USB çıkarma yardımcısı başlatılamadı; KeyLab kapandıktan sonra USB'yi elle çıkarın")
		return 1
	}
	_ = cmd.Process.Release()
	fmt.Fprintf(w, "\n📤 KeyLab bu USB'den çalıştığı için çıkarma uygulama kapandıktan hemen sonra yapılacak: %s\n", filepath.Base(usb))
	return 0
}
