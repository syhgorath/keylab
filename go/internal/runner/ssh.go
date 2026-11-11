package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ssh yardımcıları (ComponentsGates.sh ve ComponentsRunCLI.sh). Host key doğrulaması KAPATILMAZ; parola sorulmaz (BatchMode);
// uzak komutlar SABİT ve salt okunurdur. Komutlar PATH'teki ssh/ssh-keygen/ssh-add ile çalışır (testte sahte araçla değiştirilir).

// SSHOptions, her ssh çağrısına eklenen sabit seçeneklerdir.
func SSHOptions() []string {
	return []string{"-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=10",
		"-o", "PreferredAuthentications=publickey", "-o", "PasswordAuthentication=no"}
}

func runCapture(name string, args []string, stdin io.Reader) (stdout, stderr []byte, err error) {
	cmd := exec.Command(name, args...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr, cmd.Stdin = &so, &se, stdin
	err = cmd.Run()
	return so.Bytes(), se.Bytes(), err
}

// KnownHost: sunucunun host key'i daha önce kabul edilmiş mi (ssh-keygen -F). knownHosts boşsa
// SSHPOCKET_KNOWN_HOSTS ya da ~/.ssh/known_hosts.
func KnownHost(host, port, knownHosts string) error {
	if knownHosts == "" {
		knownHosts = os.Getenv("SSHPOCKET_KNOWN_HOSTS")
	}
	if knownHosts == "" {
		knownHosts = filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	}
	q := host
	if port != "22" {
		q = "[" + host + "]:" + port
	}
	if _, err := os.Stat(knownHosts); err == nil {
		if _, _, err := runCapture("ssh-keygen", []string{"-F", q, "-f", knownHosts}, nil); err == nil {
			return nil
		}
	}
	return fmt.Errorf("Sunucunun host key'i tanınmıyor (%s). Önce sshpocket ile bir kez bağlanıp host key'i kabul edin.", q)
}

// Agent: anahtar ssh-agent'ta mı (değilse addRetry ile eklemeyi dener; addRetry nil olabilir).
func Agent(prefix, key string, addRetry func(prefix, key string) error) error {
	out, _, _ := runCapture("ssh-keygen", []string{"-lf", key}, nil)
	fields := strings.Fields(firstLine(string(out)))
	if len(fields) < 2 {
		return fmt.Errorf("Anahtar okunamadı: %s", key)
	}
	fp := fields[1]
	inAgent := func() bool {
		o, _, _ := runCapture("ssh-add", []string{"-l"}, nil)
		return strings.Contains(string(o), fp)
	}
	if inAgent() {
		return nil
	}
	if addRetry != nil {
		_ = addRetry(prefix, key)
	}
	if inAgent() {
		return nil
	}
	return errors.New("Anahtar ssh-agent'ta değil (ssh-add -l). Erişim doğrulanamayacağı için durduruldu.")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

var reOSID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)

// osField, `awk -F= '$1=="KEY" { gsub(/"/, "", $2); print tolower($2); exit }'` karşılığıdır.
func osField(text, key string) string {
	for _, l := range strings.Split(text, "\n") {
		f := strings.Split(l, "=")
		if f[0] == key {
			v := ""
			if len(f) > 1 {
				v = f[1]
			}
			return strings.ToLower(strings.ReplaceAll(v, `"`, ""))
		}
	}
	return ""
}

// DetectProfile: "ubuntu-26.04" (ID-VERSION_ID). Hata durumunda ssh'ın stderr metni de verilir (ipucu için).
func DetectProfile(host, port, user, key string) (profile, sshStderr string, err error) {
	args := append(SSHOptions(), "-p", port, "-i", key, user+"@"+host, "cat /etc/os-release")
	out, se, rerr := runCapture("ssh", args, nil)
	if rerr != nil {
		return "", string(se), rerr
	}
	id, ver := osField(string(out), "ID"), osField(string(out), "VERSION_ID")
	if !reOSID.MatchString(id) || !reOSID.MatchString(ver) {
		return "", string(se), errors.New("profil okunamadı")
	}
	return id + "-" + ver, string(se), nil
}

// DetectSudoWS: sunucuda /usr/bin/sudo.ws çalıştırılabilir mi (Ubuntu'da klasik sudo; sudo-rs yanında).
func DetectSudoWS(host, port, user, key string) bool {
	args := append(SSHOptions(), "-p", port, "-i", key, user+"@"+host, "test -x /usr/bin/sudo.ws && echo SSHPOCKET_SUDO_WS=yes")
	out, _, err := runCapture("ssh", args, nil)
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(out), "\n") {
		if l == "SSHPOCKET_SUDO_WS=yes" {
			return true
		}
	}
	return false
}

func fmtUTC(epoch int64) string { return time.Unix(epoch, 0).UTC().Format("15:04:05") }

var (
	reClockNum = regexp.MustCompile(`^[0-9]{1,12}$`)
	reClockTZ  = regexp.MustCompile(`^Timezone=([A-Za-z0-9_+/-]{1,64})$`)
	reClockNTP = regexp.MustCompile(`^NTPSynchronized=(yes|no)$`)
)

// shQuoteQ, bash `printf %q` karşılığıdır (yaygın durumlar): güvenli karakterler olduğu gibi, diğerleri ters bölüyle.
func shQuoteQ(s string) string {
	if s == "" {
		return "''"
	}
	var b strings.Builder
	for i, r := range s {
		switch {
		case r == '\n':
			return "$'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\t", "\\t", "\r", "\\r").Replace(s) + "'"
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("_./:=,@%+-", r):
			b.WriteRune(r)
		case i == 0 && r == '~', i == 0 && r == '#':
			b.WriteString("\\" + string(r))
		default:
			b.WriteString("\\" + string(r))
		}
	}
	return b.String()
}

// WarnClockSkew: sunucu saati yerel saatten 60 sn'den fazla farklıysa uyarı yazar; ENGELLEMEZ. now nil ise gerçek saat.
func WarnClockSkew(host, port, user, key string, now func() int64, out io.Writer) {
	args := append(SSHOptions(), "-p", port, "-i", key, user+"@"+host, "date +%s; timedatectl show -p Timezone -p NTPSynchronized 2>/dev/null")
	so, _, err := runCapture("ssh", args, nil)
	if err != nil {
		return
	}
	text := string(so)
	remoteS := firstLine(text)
	if !reClockNum.MatchString(remoteS) {
		return
	}
	remote, _ := strconv.ParseInt(remoteS, 10, 64)
	tz, ntp := "", ""
	for _, line := range strings.Split(text, "\n") {
		if m := reClockTZ.FindStringSubmatch(line); m != nil {
			tz = m[1]
		}
		if m := reClockNTP.FindStringSubmatch(line); m != nil {
			ntp = m[1]
		}
	}
	var localNow int64
	if now != nil {
		localNow = now()
	} else {
		localNow = time.Now().Unix()
	}
	diff := remote - localNow
	dir := "ileride"
	if diff < 0 {
		dir = "geride"
		diff = -diff
	}
	if diff <= 60 {
		return
	}
	min := (diff + 30) / 60
	qkey := shQuoteQ(key)
	tzPart := ""
	if tz != "" {
		tzPart = " (saat dilimi: " + tz + ")"
	}
	fmt.Fprintf(out, "  ⚠️  Sunucu saati yaklaşık %d dakika %s (snapshot geri alındıysa olağandır).\n", min, dir)
	fmt.Fprintf(out, "      Sunucu: %s UTC%s · Bu bilgisayar: %s UTC · Fark: %d dk %d sn %s\n", fmtUTC(remote), tzPart, fmtUTC(localNow), diff/60, diff%60, dir)
	fmt.Fprintln(out, "      apt \"Not live until\" hatası verebilir; audit/apply başarısız olabilir. Sunucuda TOTP/2FA varsa kodlar da reddedilir.")
	off := diff % 1800
	if diff >= 1800 && (off < 120 || off > 1680) {
		fmt.Fprintf(out, "      Fark tam saat dilimi farkına benziyor (~%d yarım saat): sunucu saati yerel saat olarak ayarlanmış olabilir. 'timedatectl' ile saat dilimini ve RTC ayarını kontrol edin.\n", (diff+900)/1800)
	}
	if ntp == "no" {
		fmt.Fprintln(out, "      NTP eşitlemesi kapalı/başarısız (NTPSynchronized=no): düzeltme kalıcı olmayabilir; chrony kaynaklarını kontrol edin.")
	}
	fmt.Fprintln(out, "      Düzeltme (hazır komut, sudo parolanızı siz yazarsınız; saat SIÇRAR):")
	fmt.Fprintf(out, "        ssh -t -p %s -i %s %s@%s 'sudo systemctl restart chrony; sleep 8; sudo chronyc makestep; date -u'\n", port, qkey, user, host)
	fmt.Fprintln(out, "      Ya da sunucuda: sudo chronyc makestep")
	fmt.Fprintln(out, "      Ya da paket seçeneği: linux-hardening için hardening_time_makestep=true (apply saati düzeltir; saati sıçratır, varsayılan kapalı).")
}

var (
	reHintOffending = regexp.MustCompile(`(?s)Offending .* key in `)
	reHintNoHost    = regexp.MustCompile(`(?s)No .* host key is known`)
)

// SSHHint: bilinen ssh hatalarını açıklar (boş ise ipucu yok). Hata metni sunucudan gelebilir: denetim karakterleri
// temizlenir, kesilir; kabuğa HİÇ verilmez.
func SSHHint(stderrText []byte) string {
	if len(stderrText) == 0 {
		return ""
	}
	b := stderrText
	if len(b) > 2000 {
		b = b[:2000]
	}
	clean := make([]byte, 0, len(b))
	for _, c := range b {
		if (c <= 8) || (c >= 11 && c <= 31) || c == 127 {
			continue
		}
		clean = append(clean, c)
	}
	msg := string(clean)
	has := func(s string) bool { return strings.Contains(msg, s) }
	switch {
	case has("REMOTE HOST IDENTIFICATION HAS CHANGED") || reHintOffending.MatchString(msg):
		return "Sunucunun host key'i DEĞİŞMİŞ. Sunucu yeniden kurulduysa eski kaydı silin (ssh-keygen -R); aksi halde araya girilmiş olabilir, bağlanmayın."
	case reHintNoHost.MatchString(msg) || has("Host key verification failed"):
		return "Host key doğrulanamadı. Önce sshpocket'in Bağlan seçeneğiyle bu sunucuya bir kez bağlanıp host key'i kabul edin."
	case has("Too many authentication failures"):
		return "Çok fazla kimlik doğrulama denemesi (sunucuda MaxAuthTries düşük olabilir). ssh-agent'ta çok anahtar varsa ssh-add -D ile temizleyin."
	case has("Permission denied"):
		return "Anahtar reddedildi. Doğru anahtar mı (server.env KEY), sunucuda authorized_keys içinde mi, parolalı anahtar ajanda mı (ssh-add -l)?"
	case has("timed out"):
		return "Zaman aşımı: sunucuya ulaşılamıyor. VPN/ağ açık mı? Sunucuda güvenlik duvarı (ufw) bu ağdan SSH'a izin vermiyor olabilir (hardening_ssh_allow_from)."
	case has("No route to host") || has("Network is unreachable"):
		return "Ağ yolu yok: sunucunun ağına ulaşılamıyor. VPN açık mı, doğru ağda mısınız?"
	case has("Connection refused"):
		return "Bağlantı reddedildi: sshd çalışmıyor ya da port yanlış (server.env PORT)."
	case has("Could not resolve hostname"):
		return "Sunucu adı çözülemedi: server.env IP/host değerini kontrol edin."
	case has("Connection reset") || has("closed by remote host") || has("kex_exchange_identification"):
		return "Bağlantı sunucu tarafından kesildi. Çok sayıda başarısız deneme sonrası fail2ban engellemiş olabilir; biraz bekleyip yeniden deneyin."
	}
	first := firstLine(msg)
	if len(first) > 160 {
		first = first[:160]
	}
	if first == "" {
		return ""
	}
	return "ssh: " + first
}
