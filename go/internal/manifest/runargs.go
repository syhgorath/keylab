package manifest

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ComponentsManifest.sh erişimcileri ve ComponentsRun.sh girdi üreteçleri (VM'e DOKUNMAZ; yalnızca dosya/argüman üretir).

// InfoGet, comp_info_get: ilk eşleşen değer.
func InfoGet(info []string, key string) string {
	for _, l := range info {
		if strings.HasPrefix(l, key+"=") {
			return l[len(key)+1:]
		}
	}
	return ""
}

// InfoAll, comp_info_all: tüm eşleşen değerler.
func InfoAll(info []string, key string) []string {
	var out []string
	for _, l := range info {
		if strings.HasPrefix(l, key+"=") {
			out = append(out, l[len(key)+1:])
		}
	}
	return out
}

// ActionGet, comp_action_get.
func ActionGet(info []string, action, field string) string {
	return InfoGet(info, "action."+action+"."+field)
}

// ActionOnPlatform, comp_action_on_platform: platforms yoksa ya da platform bilinmiyorsa her zaman true.
func ActionOnPlatform(info []string, action, platform string) bool {
	pl := ActionGet(info, action, "platforms")
	if pl == "" || platform == "" {
		return true
	}
	return strings.Contains(","+pl+",", ","+platform+",")
}

var (
	reInvGroup = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
	reInvAlias = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	reInvHost  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.:_-]{0,252}$`)
	reInvUser  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]{0,63}$`)
	reInvPort  = regexp.MustCompile(`^[0-9]{1,5}$`)
)

func invSafe(s string) bool {
	return s != "" && !strings.ContainsAny(s, "\n\r'\"\\")
}

// WriteInventory: geçici Ansible inventory (INI) yazar; izin 600, yeni dosya (üzerine yazmaz).
// becomeExe boşsa eklenmez; yalnızca tam olarak /usr/bin/sudo.ws kabul edilir.
func WriteInventory(file, group, alias, host, user, port, key, becomeExe string) error {
	extra := ""
	switch becomeExe {
	case "":
	case "/usr/bin/sudo.ws":
		extra = " ansible_become_exe=/usr/bin/sudo.ws"
	default:
		return errors.New("geçersiz become-exe (yalnızca /usr/bin/sudo.ws)")
	}
	if !reInvGroup.MatchString(group) {
		return errors.New("geçersiz inventory grubu")
	}
	if !reInvAlias.MatchString(alias) {
		return errors.New("geçersiz sunucu adı")
	}
	if !reInvHost.MatchString(host) {
		return errors.New("geçersiz host/IP")
	}
	if !reInvUser.MatchString(user) {
		return errors.New("geçersiz kullanıcı")
	}
	if n, _ := strconv.Atoi(port); !reInvPort.MatchString(port) || n < 1 || n > 65535 {
		return errors.New("geçersiz port")
	}
	if !strings.HasPrefix(key, "/") || !invSafe(key) {
		return errors.New("anahtar yolu mutlak ve güvenli olmalı")
	}
	if _, err := os.Lstat(file); err == nil {
		return errors.New("inventory dosyası zaten var")
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("inventory yazılamadı")
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "[%s]\n%s ansible_host=%s ansible_user=%s ansible_port=%s ansible_private_key_file=%s%s\n",
		group, alias, host, user, port, "'"+key+"'", extra)
	if err != nil {
		return errors.New("inventory yazılamadı")
	}
	return nil
}

func runnerOwnedFlag(w string) bool {
	switch w {
	case "-i", "--inventory", "--limit", "-l", "-e", "--extra-vars":
		return true
	}
	for _, p := range []string{"--inventory=", "--limit=", "--extra-vars=", "-i=", "-l=", "-e="} {
		if strings.HasPrefix(w, p) {
			return true
		}
	}
	return false
}

// BuildArgv: ansible-playbook komutunu bir dizi olarak kurar (kabuk dizgesi ve eval YOK).
// Sıra: cmd sözcükleri, -i, --limit, -e @dosya, invoke.append. interactive: parola istemi bekleniyor mu.
func BuildArgv(info []string, action, inv, alias, varsFile string) (argv []string, interactive bool, err error) {
	if InfoGet(info, "runner") != "ansible" {
		return nil, false, errors.New("yalnızca ansible runner destekleniyor")
	}
	known := false
	for _, a := range InfoAll(info, "action") {
		if a == action {
			known = true
		}
	}
	if !known {
		return nil, false, fmt.Errorf("bilinmeyen eylem: %s", action)
	}
	if inv == "" || alias == "" || varsFile == "" {
		return nil, false, errors.New("inventory, sunucu adı ve değişken dosyası gerekli")
	}
	cmd := ActionGet(info, action, "cmd")
	if cmd == "" {
		return nil, false, errors.New("eylemin komutu yok")
	}
	for _, w := range strings.Fields(cmd) {
		if runnerOwnedFlag(w) {
			return nil, false, fmt.Errorf("eylem komutu çalıştırıcıya ait bayrağı içeriyor (%s)", w)
		}
		argv = append(argv, w)
	}
	argv = append(argv, "-i", inv, "--limit", alias, "-e", "@"+varsFile)
	if ActionGet(info, action, "interactive") == "yes" {
		interactive = true
	}
	for _, line := range InfoAll(info, "invoke") {
		if line == "" {
			continue
		}
		arg, rest, _ := strings.Cut(line, "|")
		val, rest, _ := strings.Cut(rest, "|")
		switch arg {
		case "-i", "--inventory", "--limit", "-l", "-e", "--extra-vars":
			continue
		}
		if val != "" {
			if strings.HasPrefix(val, "<") && strings.HasSuffix(val, ">") {
				return nil, false, fmt.Errorf("bilinmeyen yer tutucu: %s", val)
			}
			argv = append(argv, arg, val)
		} else {
			argv = append(argv, arg)
		}
		_, inter, _ := strings.Cut(rest, "|")
		if inter == "yes" {
			interactive = true
		}
	}
	if ActionGet(info, action, "sudo") == "yes" {
		has := false
		for _, w := range argv {
			if w == "--ask-become-pass" || w == "-K" {
				has = true
			}
		}
		if !has {
			argv = append(argv, "--ask-become-pass")
			interactive = true
		}
	}
	return argv, interactive, nil
}
