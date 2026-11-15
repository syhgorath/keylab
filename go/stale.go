package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Düzgün kapanmayan oturumların temizliği. KeyLab'ı Cmd+Q, Dock > Çık, çökme, Etkinlik Monitörü'nden zorla çıkış ya da güç kesintisiyle kapatırsanız Go'daki
// kapanış kodu (terminalleri kapat, özel ssh-agent'ı durdur, geçici klasörü sil) çalışmaz: özel ssh-agent ÇALIŞMAYA DEVAM eder (içinde yüklü anahtarlarla) ve
// geçici klasör (Python ortamı, ajan soketi) kalır. Bu yüzden her KeyLab açılışında, sahibi artık yaşamayan eski oturumlar aranıp temizlenir.
//
// Güvenlik ilkeleri (yanlış şeyi silmemek için): yalnız geçici klasörün (os.TempDir) DOĞRUDAN altındaki `keylab-<rakamlar>` klasörlerine bakılır; sembolik bağlar
// ve başka kullanıcıya ait klasörler atlanır; süreç tablosu okunamazsa HİÇBİR ŞEY silinmez; yalnız soket yolu o klasörü gösteren `ssh-agent` süreçleri durdurulur.
// Sahip: klasördeki owner.pid (KeyLab yazar). owner.pid olmayan (eski sürümlerden kalma) klasörler yalnız başka hiçbir KeyLab çalışmıyorsa temizlenir (çalışan
// eski bir sürümün klasörü olabilir).

const ownerFile = "owner.pid"

var staleDirRe = regexp.MustCompile(`^keylab-[0-9]+$`)

type staleReport struct {
	Dirs   int `json:"dirs"`   // silinen geçici klasör
	Agents int `json:"agents"` // durdurulan özel ssh-agent
}

type staleEnv struct {
	root  string
	self  int
	procs func() (map[int]string, error)
	kill  func(pid int) error
	owned func(fs.FileInfo) bool
}

// isKeyLabProcess: komut satırı KeyLab'ın kendisi mi (ssh-agent gibi yolunda "keylab" geçen başka süreçler DEĞİL).
func isKeyLabProcess(cmd string) bool {
	if strings.Contains(cmd, "/Contents/MacOS/KeyLab") {
		return true
	}
	tok := cmd
	if i := strings.IndexByte(cmd, ' '); i >= 0 {
		tok = cmd[:i]
	}
	switch filepath.Base(tok) {
	case "keylab", "KeyLab", "sshpocket":
		return true
	}
	return false
}

func writeOwner(dir string) {
	_ = os.WriteFile(filepath.Join(dir, ownerFile), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600)
}

func cleanStaleSessions(env staleEnv) staleReport {
	var rep staleReport
	procs, err := env.procs()
	if err != nil || len(procs) == 0 { // süreç tablosu yoksa canlı/ölü ayırt edilemez: dokunma
		return rep
	}
	ents, err := os.ReadDir(env.root)
	if err != nil {
		return rep
	}
	otherKeyLab := false
	for pid, cmd := range procs {
		if pid != env.self && isKeyLabProcess(cmd) {
			otherKeyLab = true
			break
		}
	}
	for _, e := range ents {
		if !staleDirRe.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(env.root, e.Name())
		fi, err := os.Lstat(dir)
		if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() || !env.owned(fi) {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(dir, ownerFile)); err == nil {
			pid, perr := strconv.Atoi(strings.TrimSpace(string(b)))
			if perr == nil {
				if pid == env.self {
					continue
				}
				if cmd, alive := procs[pid]; alive && isKeyLabProcess(cmd) {
					continue // sahibi yaşıyor: canlı oturum
				}
			}
		} else if otherKeyLab {
			continue // sahibi bilinmiyor ve başka bir KeyLab çalışıyor: onun klasörü olabilir
		}
		sock := filepath.Join(dir, "agent.sock")
		for pid, cmd := range procs {
			if strings.HasPrefix(cmd, "ssh-agent") && strings.Contains(cmd, sock) && env.kill(pid) == nil {
				rep.Agents++
			}
		}
		if os.RemoveAll(dir) == nil {
			rep.Dirs++
		}
	}
	return rep
}
