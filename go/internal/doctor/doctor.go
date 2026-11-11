// Package doctor: ön koşul denetimi (Doctor.sh, `./start.sh --doctor`). Yalnızca okur; hiçbir şey kurmaz.
// Politika: eksik olanı söyler ve ÇÖZÜM KOMUTUNU gösterir; Homebrew/pip/sudo ÇALIŞTIRMAZ.
// Çıkış kodu: 0 hepsi tamam (uyarılar olabilir), 1 zorunlu bir şey eksik.
//
// Bilinçli farklar (Go ikilisi bu araçlara ihtiyaç duymaz): curl, tar, sha256 aracı, nc ve fzf denetimleri kaldırıldı
// (indirme, arşiv, özet ve port yoklaması Go'dadır; terminal menüsü yoktur).
package doctor

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"sshpocket-go/internal/runner"
)

// Options: Python aralığı ve ağ denetiminin kapatılması (SSHPOCKET_DOCTOR_OFFLINE=1 de kapatır).
type Options struct {
	PythonRange string
	Offline     bool
}

type state struct {
	w     io.Writer
	fails int
}

func (s *state) ok(m string)   { fmt.Fprintf(s.w, "  ✅ %s\n", m) }
func (s *state) warn(m string) { fmt.Fprintf(s.w, "  ⚠️  %s\n", m) }
func (s *state) fail(m string) { fmt.Fprintf(s.w, "  ❌ %s\n", m); s.fails++ }
func (s *state) hint(m string) { fmt.Fprintf(s.w, "     ↳ %s\n", m) }

func have(cmd string) bool { _, err := exec.LookPath(cmd); return err == nil }

func freeKB(dir string) (int64, bool) {
	for dir != "/" && dir != "." {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir { // kök (Windows'ta "C:\\" gibi): sonsuz döngü olmasın
			break
		}
		dir = parent
	}
	return statfsFreeKB(dir)
}

// Run, doctor_run.
func Run(w io.Writer, o Options) int {
	if o.PythonRange == "" {
		o.PythonRange = ">=3.12"
	}
	if os.Getenv("SSHPOCKET_DOCTOR_OFFLINE") == "1" {
		o.Offline = true
	}
	s := &state{w: w}
	fmt.Fprintln(w, "sshpocket --doctor")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Temel araçlar")
	for _, c := range []string{"ssh", "ssh-keygen", "ssh-add"} {
		if have(c) {
			s.ok(c)
		} else {
			s.fail(c + " bulunamadı")
		}
	}
	if have("ssh-copy-id") {
		s.ok("ssh-copy-id")
	} else {
		s.warn("ssh-copy-id yok (key gönderme elle kurulum komutuyla yapılır)")
	}
	if runtime.GOOS == "darwin" {
		if have("security") {
			s.ok("security (Keychain)")
		} else {
			s.fail("security (Keychain) bulunamadı")
		}
	} else {
		s.warn("macOS değil: Keychain kullanılamaz (resmî olarak desteklenmiyor)")
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Python (paketler için; aralık: %s)\n", o.PythonRange)
	if py, found := runner.PickPython(o.PythonRange); found {
		s.ok(fmt.Sprintf("python %s (%s)", runner.PythonVersion(py), py))
		if exec.Command(py, "-c", "import venv, ensurepip").Run() == nil {
			s.ok("venv + ensurepip")
		} else {
			s.fail("venv/ensurepip eksik")
			s.hint("Homebrew Python'unda genelde yerleşiktir; sistem Python'u için: Command Line Tools kurun (xcode-select --install)")
		}
	} else {
		oldFound, stubSeen, last := "", false, ""
		for _, cand := range runner.PythonCandidates() {
			path, err := exec.LookPath(cand)
			if err != nil {
				continue
			}
			last = path
			if runner.PythonIsStub(path) {
				stubSeen = true
				continue
			}
			if v := runner.PythonVersion(path); v != "" {
				oldFound += " " + v
			}
		}
		if oldFound != "" {
			s.fail("uygun Python yok (bulunanlar:" + oldFound + ")")
		} else {
			s.fail("Python bulunamadı")
		}
		if stubSeen {
			s.hint(last + " yalnızca Command Line Tools kurulunca gerçek Python olur; atlandı (açılır pencereyi tetiklemesin diye çalıştırılmadı)")
		}
		s.hint("Homebrew varsa:  brew install python@3.13   (sshpocket bunu kendisi ÇALIŞTIRMAZ)")
		s.hint("Homebrew yoksa: https://www.python.org/downloads/ (macOS yükleyicisi)")
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Disk ve ağ")
	cache := runner.CacheDir()
	if kb, ok := freeKB(cache); ok && kb >= 524288 {
		s.ok(fmt.Sprintf("boş alan: %d MB (önbellek: %s)", kb/1024, cache))
	} else {
		s.warn("boş alan az ya da okunamadı (önbellek için ~500 MB önerilir): " + cache)
	}
	if o.Offline {
		s.warn("ağ denetimi atlandı (SSHPOCKET_DOCTOR_OFFLINE=1)")
	} else {
		cl := &http.Client{Timeout: 5 * time.Second}
		if resp, err := cl.Head("https://github.com"); err == nil {
			resp.Body.Close()
			s.ok("github.com erişilebilir (HTTPS)")
		} else {
			s.warn("github.com'a erişilemedi: güncelleme/paket indirme çalışmaz (USB'de wheelhouse varsa kurulum yine de olabilir)")
		}
	}

	fmt.Fprintln(w)
	if s.fails == 0 {
		fmt.Fprintln(w, "Sonuç: hazır ✅")
		return 0
	}
	fmt.Fprintf(w, "Sonuç: %d zorunlu eksik ❌ (yukarıdaki ↳ önerilere bakın)\n", s.fails)
	return 1
}

var _ = strings.TrimSpace
