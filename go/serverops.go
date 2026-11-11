// Sunucu ekleme, anahtar adımları, yeniden adlandırma ve silme.
// (main.go'dan mekanik olarak bölündü; davranış değişmedi.)
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"sshpocket-go/internal/servers"
)

/* ---------- sunucu ekleme: kayıt oluşturma ve anahtar adımları, sshpocket'in kendi kütüphaneleriyle ---------- */

// engineBash: sistem/Helpers altındaki kütüphaneleri source edip bir işlevi çağırır; argümanlar ayrı geçer (kabuk dizesine gömülmez)
func (a *App) engineBash(libs []string, fn string, args ...string) (string, int, error) {
	hd := filepath.Join(a.engineRoot(), "sistem", "Helpers")
	var src strings.Builder
	for _, l := range libs {
		src.WriteString(`. "$HD/` + l + `" && `)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", append([]string{"-c", src.String() + `"$@"`, "_", fn}, args...)...)
	cmd.Env = append(os.Environ(), "HD="+hd, "SISTEM_PATH="+filepath.Join(a.engineRoot(), "sistem"))
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	if err := cmd.Run(); err != nil {
		code := 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		msg := strings.TrimSpace(e.String())
		if msg == "" {
			msg = err.Error()
		}
		return o.String(), code, fmt.Errorf("%s", msg)
	}
	return o.String(), 0, nil
}

// createServer: doğrular (draft ile aynı kurallar) ve servers/<ad>/server.env dosyasını motorun server_write_files işleviyle yazar (dizin 0700, dosya 0600; var olanın üzerine yazmaz)
func (a *App) createServer(name, host, user, port, osID string) (int, error) {
	if port == "" {
		port = "22"
	}
	if reNewName.MatchString(name) {
		if _, err := os.Stat(filepath.Join(a.serversDir, name)); err == nil {
			return http.StatusConflict, fmt.Errorf("bu adda bir sunucu zaten var (üzerine yazılmaz)")
		}
	}
	if d := a.draft(name, host, user, port, osID); !d.OK {
		for _, k := range []string{"name", "host", "user", "port", "os"} {
			if m, ok := d.Errors[k]; ok {
				return http.StatusBadRequest, fmt.Errorf("%s", m)
			}
		}
		return http.StatusBadRequest, fmt.Errorf("geçersiz girdi")
	}
	a.vmu.Lock()
	defer a.vmu.Unlock()
	if err := servers.Write(a.serversDir, name, host, user, port, "", osID); err != nil {
		if errors.Is(err, servers.ErrExists) {
			return http.StatusConflict, fmt.Errorf("bu adda bir sunucu zaten var (üzerine yazılmaz)")
		}
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// startServerTask: yeni sunucu için anahtar adımları pty altında (parola istemleri arayüzdeki parola penceresine düşer)
//
//	keygen  → ssh-keygen (var olan anahtarın üzerine YAZMAZ)
//	keycopy → motorun ssh_copy_key işlevi (parmak izini gösterir, onay ister, ssh-copy-id çalıştırır)
//	sshadd  → ssh-add (macOS'ta Keychain'e kaydederek)
//	keychain → `servers keychain` (USBMonitor_<ÖNEK>_Passphrase kaydı: kaydet/güncelle/sil + doğrulama)
//	knock   → `servers knock` (protokol, bekleme ve diziyi ayarlar; dizi Keychain'de)
func (a *App) startServerTask(kind, server string) (*Job, int, error) {
	if !reName.MatchString(server) {
		return nil, http.StatusBadRequest, fmt.Errorf("geçersiz sunucu adı")
	}
	list := a.servers(false, server)
	if len(list) != 1 || !list[0].Valid {
		return nil, http.StatusNotFound, fmt.Errorf("sunucu yok ya da yapılandırması geçersiz")
	}
	sv := list[0]
	key := sv.keyPath
	if env := parseEnv(filepath.Join(a.serversDir, server, "server.env")); env["KEY"] != "" {
		k := env["KEY"]
		if strings.HasPrefix(k, "~") {
			h, _ := os.UserHomeDir()
			k = h + k[1:]
		}
		if filepath.IsAbs(k) && !strings.ContainsAny(k, "\n\r") {
			key = k
		}
	}
	var argv []string
	switch kind {
	case "keygen":
		if _, err := os.Stat(key); err == nil {
			return nil, http.StatusConflict, fmt.Errorf("bu sunucu için anahtar zaten var (üzerine yazılmaz)")
		}
		argv = []string{"ssh-keygen", "-t", "ed25519", "-a", "100", "-C", server, "-f", key}
	case "keycopy":
		if st, err := os.Stat(key); err != nil || !st.Mode().IsRegular() {
			return nil, http.StatusConflict, fmt.Errorf("önce anahtar oluşturun ya da servers/%s/ içine koyun", server)
		}
		argv = []string{selfExe(), "keycopy", sv.User, sv.Host, sv.Port, key}
	case "sshadd":
		if st, err := os.Stat(key); err != nil || !st.Mode().IsRegular() {
			return nil, http.StatusConflict, fmt.Errorf("anahtar bulunamadı")
		}
		if runtime.GOOS == "darwin" {
			argv = []string{"ssh-add", "--apple-use-keychain", key}
		} else {
			argv = []string{"ssh-add", key}
		}
	case "keychain":
		argv = []string{selfExe(), "servers", "-servers", a.serversDir, "keychain", server}
	case "knock":
		argv = []string{selfExe(), "servers", "-servers", a.serversDir, "knock", server}
	default:
		return nil, http.StatusBadRequest, fmt.Errorf("bilinmeyen adım")
	}
	job, err := a.newJob(server, "", kind)
	if err != nil {
		return nil, http.StatusConflict, err
	}
	if err := a.spawn(job, a.engineRoot(), argv); err != nil {
		a.jmu.Lock()
		job.Done, job.Exit = true, 1
		a.jmu.Unlock()
		return nil, http.StatusInternalServerError, err
	}
	return job, 0, nil
}

/* ---------- sunucu silme ve yeniden adlandırma: sshpocket'in kendi ModuleManager.sh betiğiyle ---------- */

func (a *App) busy() bool {
	a.jmu.Lock()
	defer a.jmu.Unlock()
	return a.running != nil && !a.running.Done
}

// renameServer: Go (servers.Rename; klasörü ve anahtar dosyalarını taşır, hata olursa geri alır);
// ek olarak kontrolcüdeki durum/günlük klasörleri (~/.local/share/sshpocket/<paket>/<ad>) yeni ada taşınır, hayalet sunucu kalmasın.
func (a *App) renameServer(oldName, newName string) (int, error) {
	if !reNewName.MatchString(oldName) || !reNewName.MatchString(newName) || len(newName) > 40 {
		return http.StatusBadRequest, fmt.Errorf("geçersiz ad: küçük harfle başlamalı; yalnızca küçük harf, rakam ve _ (en çok 40)")
	}
	if oldName == newName {
		return http.StatusBadRequest, fmt.Errorf("yeni ad eskisiyle aynı")
	}
	if st, err := os.Stat(filepath.Join(a.serversDir, oldName)); err != nil || !st.IsDir() {
		return http.StatusNotFound, fmt.Errorf("sunucu yok")
	}
	if _, err := os.Stat(filepath.Join(a.serversDir, newName)); err == nil {
		return http.StatusConflict, fmt.Errorf("bu adda bir sunucu zaten var")
	}
	if a.busy() {
		return http.StatusConflict, fmt.Errorf("çalışan bir eylem varken yeniden adlandırılamaz")
	}
	a.vmu.Lock()
	defer a.vmu.Unlock()
	if err := servers.Rename(a.serversDir, oldName, newName); err != nil {
		return http.StatusInternalServerError, err
	}
	// durum/günlük klasörlerini taşı (yalnızca hedef yoksa)
	pk, _ := os.ReadDir(a.stateDir)
	for _, p := range pk {
		if !p.IsDir() || !reName.MatchString(p.Name()) {
			continue
		}
		src, dst := filepath.Join(a.stateDir, p.Name(), oldName), filepath.Join(a.stateDir, p.Name(), newName)
		if st, err := os.Lstat(src); err == nil && st.IsDir() {
			if _, err := os.Lstat(dst); err != nil {
				_ = os.Rename(src, dst)
			}
		}
	}
	a.mu.Lock()
	delete(a.up, oldName)
	a.mu.Unlock()
	return 0, nil
}

type SrvFile struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsKey bool   `json:"isKey"`
}

// serverFiles: silinecek dosyaların adları (içerikleri okunmaz) ve kontrolcüde kalacak durum/günlük klasörleri
func (a *App) serverFiles(name string) ([]SrvFile, []string, error) {
	if !reNewName.MatchString(name) {
		return nil, nil, fmt.Errorf("geçersiz ad")
	}
	dir := filepath.Join(a.serversDir, name)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("sunucu yok")
	}
	files := []SrvFile{}
	for _, e := range ents {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, SrvFile{Name: e.Name(), Size: info.Size(), IsKey: strings.HasPrefix(e.Name(), "id_") && !strings.HasSuffix(e.Name(), ".pub")})
	}
	var state []string
	pk, _ := os.ReadDir(a.stateDir)
	for _, p := range pk {
		if p.IsDir() && reName.MatchString(p.Name()) {
			if st, err := os.Lstat(filepath.Join(a.stateDir, p.Name(), name)); err == nil && st.IsDir() {
				state = append(state, p.Name())
			}
		}
	}
	if state == nil {
		state = []string{}
	}
	return files, state, nil
}

// removeServer: `sshpocket-go server-remove` (Go) pty altında (Keychain soruları arayüze düşer).
// Motor "sunucu adını yazın" diye sorar: arayüzde zaten birebir yazılan ad (sunucu tarafında yeniden denetlenir) o istem belirince iletilir.
func (a *App) removeServer(name, confirm string, ack bool) (*Job, int, error) {
	if !reNewName.MatchString(name) {
		return nil, http.StatusBadRequest, fmt.Errorf("geçersiz ad")
	}
	if confirm != name {
		return nil, http.StatusBadRequest, fmt.Errorf("onay için sunucu adı birebir yazılmalı")
	}
	files, _, err := a.serverFiles(name)
	if err != nil {
		return nil, http.StatusNotFound, err
	}
	for _, f := range files {
		if f.IsKey && !ack {
			return nil, http.StatusBadRequest, fmt.Errorf("özel anahtarın silineceğini onaylamalısınız")
		}
	}
	job, err := a.newJob(name, "", "remove")
	if err != nil {
		return nil, http.StatusConflict, err
	}
	if err := a.spawn(job, a.engineRoot(), []string{selfExe(), "server-remove", a.serversDir, name}); err != nil {
		a.jmu.Lock()
		job.Done, job.Exit = true, 1
		a.jmu.Unlock()
		return nil, http.StatusInternalServerError, err
	}
	go func() {
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			a.jmu.Lock()
			has, done := strings.Contains(string(job.buf), "sunucu adını yazın"), job.Done
			a.jmu.Unlock()
			if has {
				_, _ = job.stdin.Write([]byte(name + "\n"))
				return
			}
			if done {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	return job, 0, nil
}

// purgeServerState: silinmiş sunucunun kontrolcüdeki durum/günlük klasörlerini kaldırır (yalnızca sunucu klasörü artık yoksa)
func (a *App) purgeServerState(name string) (int, error) {
	if !reNewName.MatchString(name) {
		return http.StatusBadRequest, fmt.Errorf("geçersiz ad")
	}
	if _, err := os.Stat(filepath.Join(a.serversDir, name)); err == nil {
		return http.StatusConflict, fmt.Errorf("sunucu hâlâ var")
	}
	n := 0
	pk, _ := os.ReadDir(a.stateDir)
	for _, p := range pk {
		if !p.IsDir() || !reName.MatchString(p.Name()) {
			continue
		}
		path := filepath.Join(a.stateDir, p.Name(), name)
		if st, err := os.Lstat(path); err == nil && st.IsDir() && st.Mode()&os.ModeSymlink == 0 {
			if err := os.RemoveAll(path); err == nil {
				n++
			}
		}
	}
	return 0, nil
}
