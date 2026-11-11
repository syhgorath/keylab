// Package eject: USB'yi güvenli çıkarma (Eject.sh + SSHHelper.sh ssh_agent_remove_usb_keys).
// Çıkarmadan ÖNCE USB'deki servers/*/*.pub'a karşılık gelen anahtarlar ssh-agent'tan silinir (ajandaki başka anahtarlara dokunulmaz).
// Bilinçli fark: kabuğun Log "INFO"/"WARN" satırları (usb_logs/) yazılmaz; eylem günlüğü Go yürütücüsünde ayrıdır.
package eject

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// RemoveUSBKeys, ssh_agent_remove_usb_keys: silinen anahtar sayısı.
func RemoveUSBKeys(usb string) int {
	if usb == "" {
		return 0
	}
	if st, err := os.Stat(filepath.Join(usb, "servers")); err != nil || !st.IsDir() {
		return 0
	}
	if _, err := exec.LookPath("ssh-add"); err != nil {
		return 0
	}
	pubs, _ := filepath.Glob(filepath.Join(usb, "servers", "*", "*.pub"))
	n := 0
	for _, p := range pubs {
		// bash'te `*` gizli (nokta ile başlayan) ad eşlemez
		if strings.HasPrefix(filepath.Base(p), ".") || strings.HasPrefix(filepath.Base(filepath.Dir(p)), ".") {
			continue
		}
		if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
			continue
		}
		if exec.Command("ssh-add", "-d", p).Run() == nil {
			n++
		}
	}
	return n
}

var reDev = regexp.MustCompile(`(?m)^\s*Device Identifier: *(.*)$`)

// Run, EjectUSB. Çıkış kodu: 0 çıkarıldı/yapılacak yok, 1 çıkarılamadı.
func Run(w io.Writer, usb string) int {
	if st, err := os.Stat(usb); usb == "" || err != nil || !st.IsDir() {
		return 0
	}
	if _, err := exec.LookPath("diskutil"); err != nil {
		fmt.Fprintln(w, "⚠️  diskutil yok, USB'yi elle çıkarın")
		return 1
	}
	if n := RemoveUSBKeys(usb); n > 0 {
		fmt.Fprintf(w, "🔑 %d anahtar ssh-agent'tan çıkarıldı\n", n)
	}
	fmt.Fprintf(w, "\n📤 USB çıkarılıyor: %s\n", filepath.Base(usb))
	os.Chdir(os.Getenv("HOME")) // çalışma dizini USB üzerinde olmasın
	disk := ""
	if o, err := exec.Command("diskutil", "info", usb).Output(); err == nil {
		if m := reDev.FindStringSubmatch(string(o)); m != nil {
			disk = strings.TrimSpace(m[1])
		}
	}
	if disk != "" && exec.Command("diskutil", "eject", disk).Run() == nil {
		fmt.Fprintln(w, "✅ USB çıkarıldı")
		return 0
	}
	if exec.Command("diskutil", "unmount", "force", usb).Run() == nil {
		fmt.Fprintln(w, "✅ USB unmount edildi (force)")
		if disk != "" {
			exec.Command("diskutil", "eject", disk).Run()
		}
		return 0
	}
	fmt.Fprintln(w, "⚠️  USB çıkarılamadı (kullanımda olabilir), elle çıkarabilirsiniz")
	return 1
}
