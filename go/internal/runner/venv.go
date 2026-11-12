package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"keylab/internal/envx"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Python yorumlayıcı seçimi ve hash'li venv önbelleği (ComponentsVenv.sh).
// Venv bir ÖNBELLEKTİR: kontrol makinesinde üretilir, taşınmaz, silinebilir, yeniden üretilir.
// Konum: ${SSHPOCKET_CACHE_DIR:-~/.cache/sshpocket}/venv/<paket>-<ortam-kimliği>
// Kurulum YALNIZCA venv'in içine: pip --require-hashes --only-binary :all:; global pip, sudo, brew yok.

const defaultPyCandidates = "python3.14 python3.13 python3.12 python3.11 python3.10 python3"

// CacheDir: önbellek kökü.
func CacheDir() string {
	if d := envx.Get("CACHE_DIR"); d != "" {
		return d
	}
	c := filepath.Join(os.Getenv("HOME"), ".cache")
	return envx.PickDir(filepath.Join(c, "keylab"), filepath.Join(c, "sshpocket"))
}

func mm(v string) (major, minor int) {
	parts := strings.SplitN(v, ".", 3)
	major, _ = strconv.Atoi(parts[0])
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return
}

func mmCmp(a, b string) int {
	am, an := mm(a)
	bm, bn := mm(b)
	switch {
	case am != bm:
		if am > bm {
			return 1
		}
		return -1
	case an != bn:
		if an > bn {
			return 1
		}
		return -1
	}
	return 0
}

var (
	rePyVer   = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	rePyRange = regexp.MustCompile(`^>=([0-9]+\.[0-9]+)(,<([0-9]+\.[0-9]+))?$`)
)

// PythonInRange: sürüm "X.Y.Z", aralık ">=3.12" ya da ">=3.12,<4.0" (yalnızca major.minor karşılaştırılır).
func PythonInRange(ver, rng string) bool {
	if !rePyVer.MatchString(ver) {
		return false
	}
	m := rePyRange.FindStringSubmatch(rng)
	if m == nil {
		return false
	}
	if mmCmp(ver, m[1]) == -1 {
		return false
	}
	if m[3] != "" && mmCmp(ver, m[3]) != -1 {
		return false
	}
	return true
}

// PythonVersion: yorumlayıcının "X.Y.Z" sürümü; çalışmazsa boş.
func PythonVersion(interp string) string {
	out, _, err := runCapture(interp, []string{"-c", `import sys; print("%d.%d.%d" % sys.version_info[:3])`}, nil)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// PythonCandidates: aday yorumlayıcı adları (SSHPOCKET_PYTHON_CANDIDATES ile değiştirilebilir).
func PythonCandidates() []string {
	s := envx.Get("PYTHON_CANDIDATES")
	if s == "" {
		s = defaultPyCandidates
	}
	return strings.Fields(s)
}

// PythonIsStub: macOS'ta /usr/bin/python3, Command Line Tools kurulu DEĞİLSE "geliştirici araçlarını kurun" penceresini
// açan bir saplamadır; çalıştırmak o pencereyi tetikler, bu yüzden atlanır.
func PythonIsStub(path string) bool {
	stub := envx.Get("PY_STUB_PATH")
	if stub == "" {
		stub = "/usr/bin/python3"
	}
	if path != stub {
		return false
	}
	xs, err := exec.LookPath("xcode-select")
	if err != nil {
		return false
	}
	return exec.Command(xs, "-p").Run() != nil
}

// PickPython: aralığa uyan ilk yorumlayıcının tam yolu.
func PickPython(rng string) (string, bool) {
	for _, cand := range PythonCandidates() {
		p, err := exec.LookPath(cand)
		if err != nil {
			continue
		}
		if PythonIsStub(p) {
			continue
		}
		v := PythonVersion(p)
		if v == "" {
			continue
		}
		if PythonInRange(v, rng) {
			return p, true
		}
	}
	return "", false
}

var (
	reReqLine  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(\[[A-Za-z0-9,._-]+\])?==[A-Za-z0-9._+!-]+( *;.*)?( *\\)?$`)
	reHashLine = regexp.MustCompile(`^--hash=sha256:[0-9a-f]{64}( *\\)?$`)
)

// RequirementsHashed: yalnızca "paket==sürüm \" + "--hash=sha256:<64 hex>" devam satırları kabul edilir; her gereksinimin en
// az bir hash'i olmalı; -r/-e/-c/--index-url gibi başka satırlar REDDEDİLİR.
func RequirementsHashed(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return errors.New("requirements.txt yok")
	}
	haveHash, n, inReq := false, 0, false
	text := string(b)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimLeft(line, " \t\n\v\f\r")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		switch {
		case reHashLine.MatchString(trimmed):
			if !inReq {
				return errors.New("başıboş hash satırı")
			}
			haveHash = true
		case reReqLine.MatchString(trimmed):
			if inReq && !haveHash {
				return errors.New("hash'siz gereksinim")
			}
			inReq, haveHash = true, false
			n++
		default:
			return fmt.Errorf("kabul edilmeyen satır: %s", trimmed)
		}
	}
	if n == 0 {
		return errors.New("gereksinim yok")
	}
	if !haveHash {
		return errors.New("hash'siz gereksinim")
	}
	return nil
}

func unameField(flag string) string {
	out, err := exec.Command("uname", flag).Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// EnvID: ortam kimliği (16 hane): requirements özeti + python tam sürümü + platform.
func EnvID(reqFile, pyver string) (string, error) {
	rh, err := os.ReadFile(reqFile)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(rh)
	s := hex.EncodeToString(sum[:]) + "|" + pyver + "|" + unameField("-s") + "-" + unameField("-m")
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:16], nil
}

// VenvDir: <önbellek>/venv/<paket>-<ortam-kimliği>.
func VenvDir(id, envid string) string {
	return filepath.Join(CacheDir(), "venv", id+"-"+envid)
}

// VenvHealthy: işaret dosyası + çalışan python.
func VenvHealthy(dir, envid string) bool {
	b, err := os.ReadFile(filepath.Join(dir, ".sshpocket-env"))
	if err != nil || strings.TrimRight(string(b), "\n") != envid {
		return false
	}
	py := filepath.Join(dir, "bin", "python")
	st, err := os.Stat(py)
	if err != nil || st.Mode()&0o111 == 0 {
		return false
	}
	return exec.Command(py, "-c", "import sys").Run() == nil
}

func pipInstall(vpy, req, wheelhouse string) bool {
	base := []string{"-m", "pip", "install", "--no-input", "--disable-pip-version-check", "--require-hashes", "--only-binary", ":all:", "-r", req}
	if wheelhouse != "" {
		if st, err := os.Stat(wheelhouse); err == nil && st.IsDir() {
			args := append(append([]string(nil), base...), "--no-index", "--find-links", wheelhouse)
			if exec.Command(vpy, args...).Run() == nil {
				return true
			}
		}
	}
	if envx.Get("OFFLINE") == "1" {
		return false
	}
	return exec.Command(vpy, base...).Run() == nil
}

// VenvLookup: sağlıklı venv varsa dizinini verir; yoksa ok=false. HİÇBİR ŞEY kurmaz/indirmez.
func VenvLookup(id, pkgDir, rng string) (string, bool) {
	py, ok := PickPython(rng)
	if !ok {
		return "", false
	}
	envid, err := EnvID(filepath.Join(pkgDir, "requirements.txt"), PythonVersion(py))
	if err != nil {
		return "", false
	}
	dir := VenvDir(id, envid)
	if !VenvHealthy(dir, envid) {
		return "", false
	}
	return dir, true
}

// VenvEnsure: aynı ortam kimliğinde sağlıklı venv varsa HİÇBİR ŞEY kurulmaz. Venv nihai konumunda kurulur; işaret
// dosyası EN SON yazılır (yarım kalan kurulum "sağlıksız" sayılır, silinip yeniden kurulur).
func VenvEnsure(id, pkgDir, rng, wheelhouse string) (string, error) {
	req := filepath.Join(pkgDir, "requirements.txt")
	if err := RequirementsHashed(req); err != nil {
		return "", err
	}
	py, ok := PickPython(rng)
	if !ok {
		return "", fmt.Errorf("uygun Python bulunamadı (%s); ./start.sh --doctor", rng)
	}
	envid, err := EnvID(req, PythonVersion(py))
	if err != nil {
		return "", err
	}
	dir := VenvDir(id, envid)
	if VenvHealthy(dir, envid) {
		return dir, nil
	}
	if _, err := os.Lstat(dir); err == nil {
		os.RemoveAll(dir)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	if exec.Command(py, "-m", "venv", dir).Run() != nil {
		os.RemoveAll(dir)
		return "", errors.New("venv oluşturulamadı (python venv/ensurepip eksik olabilir)")
	}
	if !pipInstall(filepath.Join(dir, "bin", "python"), req, wheelhouse) {
		os.RemoveAll(dir)
		return "", errors.New("bağımlılıklar kurulamadı (hash/ağ)")
	}
	if err := os.WriteFile(filepath.Join(dir, ".sshpocket-env"), []byte(envid+"\n"), 0o644); err != nil {
		return "", err
	}
	return dir, nil
}
