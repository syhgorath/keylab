// Package servers: servers/<ad>/server.env veri katmanı (Validate.sh, ModuleFiles.sh server_write_files, Servers.sh, SSHModule.sh
// _ssh_env_get, SSHModuleEdit.sh _ssh_env_set, ModuleManager.sh list/remove/rename, SSHKeyCopy.sh yardımcıları).
//
// Yalnızca YENİ biçim (servers/<ad>/server.env) desteklenir. Eski biçim (sistem/Modules/<ad>/.env + üretilmiş modül) ve --migrate
// taşınmadı: proje hiç dağıtılmadı, eski düzeni kullanan yok (kullanıcı kararı: kabuk geliştirmesi durdu, Go yeni düzenle başlar).
// Hiçbir dosya `source` edilmez; server.env güvenli ayrıştırıcıyla okunur.
package servers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	reName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	reIP   = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)
	reUser = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
	rePort = regexp.MustCompile(`^[0-9]+$`)
)

// ErrExists: hedef sunucu zaten var (kabukta server_write_files çıkış kodu 3).
var ErrExists = errors.New("hedef zaten var")

// NameValid, module_name_valid.
func NameValid(n string) bool { return reName.MatchString(n) }

// Prefix, module_prefix: büyük harf (ASCII ad kuralı nedeniyle strings.ToUpper yeterli).
func Prefix(n string) string { return strings.ToUpper(n) }

// ValidateTarget, ssh_validate_target: hata iletisi kabuğun stderr çıktısıyla aynıdır.
func ValidateTarget(ip, user, port string) error {
	if !reIP.MatchString(ip) {
		return fmt.Errorf("geçersiz IP/host: '%s'", ip)
	}
	if !reUser.MatchString(user) {
		return fmt.Errorf("geçersiz kullanıcı: '%s'", user)
	}
	if !rePort.MatchString(port) {
		return fmt.Errorf("geçersiz port: '%s'", port)
	}
	// Bilinçli fark: kabukta taşan sayılar ([ -lt ] hata verir) doğrulamayı geçerdi; Go reddeder.
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("port aralık dışı: '%s'", port)
	}
	return nil
}

// ---------- server.env okuma/yazma ----------

// EnvGet, _ssh_env_get: eval/source olmadan tek anahtar okur.
func EnvGet(file, want string) (string, bool) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimLeft(line, " \t\n\v\f\r")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		key = strings.Trim(key, " \t\n\v\f\r")
		value = strings.Trim(value, " \t\n\v\f\r")
		switch {
		case len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"':
			value = value[1 : len(value)-1]
		case len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'':
			value = value[1 : len(value)-1]
		default:
			for i := 0; i+1 < len(value); i++ {
				if (value[i] == ' ' || value[i] == '\t') && value[i+1] == '#' {
					value = value[:i]
					break
				}
			}
			value = strings.TrimRight(value, " \t\n\v\f\r")
		}
		if key == want {
			return value, true
		}
	}
	return "", false
}

// EnvSet, _ssh_env_set: anahtarın İLK satırını değiştirir (yoksa sona ekler); diğer satırlar/yorumlar korunur; izin 600; atomik.
func EnvSet(file, want, value string) error {
	if strings.ContainsAny(want, "=\n") || strings.ContainsRune(value, '\n') {
		return errors.New("geçersiz anahtar/değer")
	}
	var out strings.Builder
	found := false
	if b, err := os.ReadFile(file); err == nil {
		text := string(b)
		lines := strings.Split(text, "\n")
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for _, line := range lines {
			key := ""
			trimmed := strings.TrimLeft(line, " \t\n\v\f\r")
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				if i := strings.Index(trimmed, "="); i >= 0 {
					key = strings.TrimRight(trimmed[:i], " \t\n\v\f\r")
				}
			}
			if key == want && !found {
				out.WriteString(want + "=" + value + "\n")
				found = true
			} else {
				out.WriteString(line + "\n")
			}
		}
	}
	if !found {
		out.WriteString(want + "=" + value + "\n")
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+".")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Chmod(0o600); err == nil {
		_, err = tmp.WriteString(out.String())
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, file); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Get, srv_get (yeni biçim): servers/<ad> klasöründen anahtar.
func Get(dir, key string) (string, bool) { return EnvGet(filepath.Join(dir, "server.env"), key) }

// Set, srv_set (yeni biçim).
func Set(dir, key, value string) error { return EnvSet(filepath.Join(dir, "server.env"), key, value) }

// ---------- listeleme / yazma ----------

// Names, srv_names: server.env'i olan, adı kurala uyan sunucular (sıralı).
func Names(serversDir string) []string {
	ents, err := os.ReadDir(serversDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		if !NameValid(n) {
			continue
		}
		if st, err := os.Stat(filepath.Join(serversDir, n)); err != nil || !st.IsDir() {
			continue
		}
		if st, err := os.Stat(filepath.Join(serversDir, n, "server.env")); err != nil || !st.Mode().IsRegular() {
			continue
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Write, server_write_files: servers/<ad>/server.env yazar (dizin 0700, dosya 0600). Hedef varsa ErrExists, hiçbir şeyi değiştirmez.
// Anahtar üretmez.
func Write(serversDir, name, ip, user, port, key, osID string) error {
	if !NameValid(name) {
		return fmt.Errorf("geçersiz sunucu adı: '%s'", name)
	}
	if err := ValidateTarget(ip, user, port); err != nil {
		return err
	}
	if strings.Contains(key+osID, "\n") {
		return errors.New("geçersiz değer (satır sonu)")
	}
	dir := filepath.Join(serversDir, name)
	if _, err := os.Lstat(dir); err == nil {
		return ErrExists
	}
	if err := os.MkdirAll(serversDir, 0o700); err == nil {
		os.Chmod(serversDir, 0o700)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	os.Chmod(dir, 0o700)
	var b strings.Builder
	b.WriteString("# SSH bağlantı bilgileri (gizli bilgi buraya YAZILMAZ)\n")
	b.WriteString("IP=" + ip + "\nUSER=" + user + "\nPORT=" + port + "\n")
	if key != "" {
		b.WriteString("KEY=" + key + "\n")
	}
	if osID != "" {
		b.WriteString("OS=" + osID + "\n")
	}
	f := filepath.Join(dir, "server.env")
	if err := os.WriteFile(f, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Chmod(f, 0o600)
}

// Summary, server_summary: "kullanıcı@ip[:port]" (bilgi eksik/geçersizse boş).
func Summary(serversDir, name string) string {
	if !NameValid(name) {
		return ""
	}
	d := filepath.Join(serversDir, name)
	ip, ok := Get(d, "IP")
	if !ok {
		return ""
	}
	user, _ := Get(d, "USER")
	port, _ := Get(d, "PORT")
	if ip == "" || user == "" {
		return ""
	}
	p := port
	if p == "" {
		p = "22"
	}
	if ValidateTarget(ip, user, p) != nil {
		return ""
	}
	if port != "" && port != "22" {
		return user + "@" + ip + ":" + port
	}
	return user + "@" + ip
}

// KeyPath: sunucunun özel anahtar yolu (KEY yoksa klasördeki id_ed25519_<ad>; "~" ev dizinine açılır).
func KeyPath(serversDir, name string) string {
	d := filepath.Join(serversDir, name)
	key, _ := Get(d, "KEY")
	if key == "" {
		return filepath.Join(d, "id_ed25519_"+name)
	}
	if key[0] == '~' {
		key = os.Getenv("HOME") + key[1:]
	}
	return key
}

// ListLine, ModuleManager.sh _list_line (etiketsiz): "  ad<18 genişlik> kullanıcı@ip:port".
func ListLine(serversDir, name string) string {
	d := filepath.Join(serversDir, name)
	ip, _ := Get(d, "IP")
	user, _ := Get(d, "USER")
	port, _ := Get(d, "PORT")
	if ip == "" {
		ip = "(ayar yok)"
	}
	if user != "" {
		user += "@"
	}
	if port != "" {
		port = ":" + port
	}
	return fmt.Sprintf("  %-18s %s%s%s", name, user, ip, port)
}

// ---------- yeniden adlandırma / silme ----------

// Rename, cmd_rename: klasörü taşır, id_ed25519_<eski>[.pub] dosyalarını yeni ada çevirir; hata olursa geri alır.
// Keychain kayıtları TAŞINMAZ (çağıran uyarır).
func Rename(serversDir, oldName, newName string) error {
	if !NameValid(oldName) {
		return fmt.Errorf("Geçersiz sunucu adı: '%s'", oldName)
	}
	if st, err := os.Stat(filepath.Join(serversDir, oldName)); err != nil || !st.IsDir() {
		return fmt.Errorf("Sunucu yok: %s", oldName)
	}
	if !NameValid(newName) {
		return fmt.Errorf("Geçersiz yeni ad: '%s'", newName)
	}
	if _, err := os.Lstat(filepath.Join(serversDir, newName)); err == nil {
		return fmt.Errorf("Hedef zaten var: %s", newName)
	}
	oldDir, newDir := filepath.Join(serversDir, oldName), filepath.Join(serversDir, newName)
	if err := os.Rename(oldDir, newDir); err != nil {
		return errors.New("Yeniden adlandırılamadı")
	}
	for _, ext := range []string{"", ".pub"} {
		from := filepath.Join(newDir, "id_ed25519_"+oldName+ext)
		if st, err := os.Stat(from); err == nil && st.Mode().IsRegular() {
			if err := os.Rename(from, filepath.Join(newDir, "id_ed25519_"+newName+ext)); err != nil {
				os.Rename(newDir, oldDir)
				return errors.New("Key dosyası yeniden adlandırılamadı; geri alınıyor")
			}
		}
	}
	return nil
}

// Remove, cmd_remove'un dosya kısmı (onay ve Keychain temizliği çağıranda): klasörü siler, simgesel bağlara girmez.
func Remove(serversDir, name string) error {
	if !NameValid(name) {
		return fmt.Errorf("Geçersiz sunucu adı: '%s'", name)
	}
	dir := filepath.Join(serversDir, name)
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("Sunucu yok: %s", name)
	}
	return os.RemoveAll(dir)
}

// ---------- ssh-copy-id yardımcısı ----------

// ManualKeyHelp, _ssh_manual_key_help: public key satırı için sunucuda çalıştırılacak tek satırlık komut.
func ManualKeyHelp(pubFile string) (string, error) {
	b, err := os.ReadFile(pubFile)
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(string(b), "\n")
	esc := strings.ReplaceAll(line, "'", `'\''`)
	return "\n📋 Elle ekleme (sunucunun konsolunda / başka bir oturumda çalıştırın):\n\n" +
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh && touch ~/.ssh/authorized_keys && { grep -qxF '" + esc +
		"' ~/.ssh/authorized_keys || echo '" + esc + "' >> ~/.ssh/authorized_keys; } && chmod 600 ~/.ssh/authorized_keys\n\n" +
		"Sonra menüden 1) Bağlan ile key ile girişi deneyin.\n", nil
}

func joinDir(serversDir, name string) string { return filepath.Join(serversDir, name) }

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
