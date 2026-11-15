package eject

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// deferFx: USB'den çalışan sahte uygulama (uzun yaşayan bir süreç) + sahte diskutil. Yardımcı bu sürecin PID'ini izler.
func deferFx(t *testing.T) (usb, log string, app *exec.Cmd) {
	t.Helper()
	usb = mkUSB(t)
	bin := t.TempDir()
	log = filepath.Join(t.TempDir(), "calls.log")
	os.WriteFile(filepath.Join(bin, "diskutil"), []byte("#!/bin/bash\necho \"diskutil $*\" >> \""+log+"\"\nexit 0\n"), 0o755)
	os.WriteFile(filepath.Join(bin, "ssh-add"), []byte("#!/bin/bash\nexit 1\n"), 0o755)
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	old, oldPID, oldWait := executablePath, parentPID, deferMaxWait
	t.Cleanup(func() { executablePath, parentPID, deferMaxWait = old, oldPID, oldWait })
	executablePath = func() (string, error) { return filepath.Join(usb, "KeyLab.app", "Contents", "MacOS", "KeyLab"), nil }
	app = exec.Command("/bin/sleep", "60")
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Process.Kill(); app.Wait() })
	parentPID = func() int { return app.Process.Pid }
	return
}

func readLog(log string) string { b, _ := os.ReadFile(log); return string(b) }

// KeyLab çıkarılacak USB'nin kendisinden çalışıyorsa çıkarma SENKRON yapılmaz; üstelik uygulama YAŞADIĞI SÜRECE yardımcı da çıkarmaz
// (4.0.8 hatası: yardımcı 2 sn sonra uygulama hâlâ açıkken USB'yi zorla çıkarıyor, süreç %400 CPU'ya fırlayıp bilgisayar kasılıyordu).
func TestDeferredEjectWaitsForTheAppToExit(t *testing.T) {
	usb, log, app := deferFx(t)
	var out bytes.Buffer
	if code := Run(&out, usb); code != 0 {
		t.Fatalf("çıkış %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "uygulama kapandıktan hemen sonra") {
		t.Fatalf("ileti: %q", out.String())
	}
	time.Sleep(4 * time.Second) // eski yardımcı 2 sn'de çıkarırdı; uygulama hâlâ yaşıyor
	if got := readLog(log); got != "" {
		t.Fatalf("uygulama hâlâ çalışırken USB çıkarıldı/unmount edildi: %q", got)
	}
	app.Process.Kill() // kullanıcı "Kapat"a bastı: uygulama bitti
	app.Wait()
	var got string
	for i := 0; i < 100 && !strings.Contains(got, "eject"); i++ {
		time.Sleep(100 * time.Millisecond)
		got = readLog(log)
	}
	if !strings.Contains(got, "diskutil eject "+usb) || strings.Contains(got, "unmount force") {
		t.Fatalf("uygulama bittikten sonra normal çıkarma beklenirdi: %q", got)
	}
}

// Uygulama hiç kapanmazsa yardımcı süre dolunca HİÇBİR ŞEY yapmadan çıkar (zorla unmount asla): USB'yi elle çıkarmak güvenli, zorla çıkarmak değil.
func TestDeferredEjectGivesUpWithoutForcing(t *testing.T) {
	usb, log, _ := deferFx(t)
	deferMaxWait = 1500 * time.Millisecond
	var out bytes.Buffer
	if code := Run(&out, usb); code != 0 {
		t.Fatalf("çıkış %d: %s", code, out.String())
	}
	time.Sleep(5 * time.Second)
	if got := readLog(log); got != "" {
		t.Fatalf("bekleme süresi dolunca yardımcı USB'ye dokunmamalıydı: %q", got)
	}
}

func TestSelfOnVolume(t *testing.T) {
	old := executablePath
	t.Cleanup(func() { executablePath = old })
	usb := t.TempDir()
	cases := map[string]bool{
		filepath.Join(usb, "KeyLab.app", "Contents", "MacOS", "KeyLab"): true,
		filepath.Join(usb, "keylab"):                                    true,
		usb + "x/keylab":                                                false, // önek tuzağı
		"/Applications/KeyLab.app/Contents/MacOS/KeyLab":                false,
	}
	for exe, want := range cases {
		executablePath = func() (string, error) { return exe, nil }
		if got := selfOnVolume(usb); got != want {
			t.Errorf("%q: %v (beklenen %v)", exe, got, want)
		}
	}
}
