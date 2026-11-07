package servers

import (
	"os"
	"strings"
)

// SSHConfigImport.sh: ~/.ssh/config içindeki Host kayıtları. Include/Match blokları desteklenmez (Match sonrası ayarlar atlanır).
// Private key'ler KOPYALANMAZ; IdentityFile yolu server.env'e KEY olarak yazılır.

// HostEntry: joker içermeyen tek bir Host takma adı ve ilk bulunan ayarları.
type HostEntry struct {
	Alias, Host, User, Port, IdentityFile string
}

func isTab(r byte) bool { return r == ' ' || r == '\t' }

// ParseSSHConfig, ssh_config_hosts (awk) karşılığı. Host boşsa takma ad kullanılır; kullanıcı/port boş kalabilir (çağıran varsayılan koyar).
func ParseSSHConfig(file string) ([]HostEntry, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var out []HostEntry
	var aliases []string
	var host, user, port, idf string
	inblock := false
	flush := func() {
		for _, a := range aliases {
			h := host
			if h == "" {
				h = a
			}
			out = append(out, HostEntry{a, h, user, port, idf})
		}
		aliases = nil
	}
	text := string(b)
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		line = strings.TrimLeft(line, " \t")
		if line == "" || line[0] == '#' {
			continue
		}
		// kw: ilk " \t=" öncesi; val: anahtar sözcük + boşluk + isteğe bağlı "=" + boşluk sonrası
		i := 0
		for i < len(line) && !isTab(line[i]) && line[i] != '=' {
			i++
		}
		kw := line[:i]
		val := line
		if j := 0; true {
			for j < len(val) && !isTab(val[j]) && val[j] != '=' {
				j++
			}
			// awk: ^[^ \t=]+[ \t]*=?[ \t]*  (ilk sözcük zorunlu; yoksa eşleşme olmaz)
			if j > 0 {
				for j < len(val) && isTab(val[j]) {
					j++
				}
				if j < len(val) && val[j] == '=' {
					j++
				}
				for j < len(val) && isTab(val[j]) {
					j++
				}
				val = val[j:]
			}
		}
		val = strings.TrimPrefix(val, `"`)
		val = strings.TrimSuffix(val, `"`)
		k := strings.ToLower(kw)
		switch {
		case k == "host":
			flush()
			host, user, port, idf = "", "", "", ""
			for _, p := range splitTabs(val) {
				if p != "" && !strings.ContainsAny(p, "*?!") {
					aliases = append(aliases, p)
				}
			}
			inblock = true
		case k == "match":
			flush()
			inblock = false
		case !inblock:
		case k == "hostname" && host == "":
			host = val
		case k == "user" && user == "":
			user = val
		case k == "port" && port == "":
			port = val
		case k == "identityfile" && idf == "":
			idf = val
		}
	}
	flush()
	return out, nil
}

// splitTabs: awk split(val, parts, /[ \t]+/) — baştaki boşluk boş ilk parça verir (çağıran boşları atlar).
func splitTabs(s string) []string {
	var parts []string
	cur := strings.Builder{}
	in := false
	for i := 0; i < len(s); i++ {
		if isTab(s[i]) {
			if !in {
				parts = append(parts, cur.String())
				cur.Reset()
				in = true
			}
			continue
		}
		in = false
		cur.WriteByte(s[i])
	}
	parts = append(parts, cur.String())
	return parts
}

// ModuleNameFromAlias, ssh_config_module_name: küçük harf; [a-z0-9] dışındaki her BAYT "_"; harfle başlamazsa "h_" öneki.
func ModuleNameFromAlias(alias string) string {
	l := strings.ToLower(alias)
	b := make([]byte, len(l))
	for i := 0; i < len(l); i++ {
		c := l[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '\n' {
			b[i] = c
		} else {
			b[i] = '_'
		}
	}
	n := string(b)
	if n == "" || n[0] < 'a' || n[0] > 'z' {
		n = "h_" + n
	}
	return n
}

// ImportResult: tek takma adın içe aktarma sonucu.
type ImportResult struct {
	Name    string
	Skipped bool  // zaten var
	Err     error // geçersiz değer
}

// ImportHosts, ImportSSHConfig.sh'ın yazma kısmı: seçilen girdileri servers/ altına yazar (kullanıcı boşsa defaultUser, port boşsa 22).
func ImportHosts(serversDir string, sel []HostEntry, defaultUser string) []ImportResult {
	var res []ImportResult
	for _, e := range sel {
		name := ModuleNameFromAlias(e.Alias)
		if _, err := os.Lstat(serversDir + "/" + name); err == nil {
			res = append(res, ImportResult{Name: name, Skipped: true})
			continue
		}
		user, port := e.User, e.Port
		if user == "" {
			user = defaultUser
		}
		if port == "" {
			port = "22"
		}
		err := Write(serversDir, name, e.Host, user, port, e.IdentityFile, "")
		res = append(res, ImportResult{Name: name, Err: err})
	}
	return res
}
