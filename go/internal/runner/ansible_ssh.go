package runner

import (
	"keylab/internal/envx"
	"os"
	"path/filepath"
	"strings"
)

// ansibleExtraArgsEnv: taşınabilir (USB) modda ssh known_hosts dosyası KEYLAB_KNOWN_HOSTS ile USB'ye yönlendirilir; Ansible'ın kendi ssh'ı da aynı
// dosyayı kullanmalı (aksi hâlde host key denetimi başka dosyaya bakar). Ansible `ssh_extra_args`ı ANSIBLE_SSH_EXTRA_ARGS ortam değişkeni
// ini'yi EZER; bu yüzden paketin ansible.cfg'sindeki [ssh_connection] ssh_extra_args (ve kullanıcı ortamında varsa o değer) KORUNUP sonuna
// UserKnownHostsFile eklenir. Dönen ikinci değer false ise ortam değişkenine dokunulmaz (known_hosts yönlendirilmemiş).
func ansibleExtraArgsEnv(pkgDir string) (string, bool) {
	kh := envx.Get("KNOWN_HOSTS")
	if kh == "" {
		return "", false
	}
	base, ok := os.LookupEnv("ANSIBLE_SSH_EXTRA_ARGS")
	if !ok {
		base = iniSSHExtraArgs(filepath.Join(pkgDir, "ansible.cfg"))
	}
	opt := "-o UserKnownHostsFile=" + shQuoteArg(kh)
	if strings.TrimSpace(base) == "" {
		return opt, true
	}
	return strings.TrimSpace(base) + " " + opt, true
}

// iniSSHExtraArgs: ansible.cfg'nin [ssh_connection] bölümündeki ssh_extra_args değeri ("" = yok).
func iniSSHExtraArgs(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			in = l == "[ssh_connection]"
			continue
		}
		if !in || strings.HasPrefix(l, "#") || strings.HasPrefix(l, ";") {
			continue
		}
		if k, v, ok := strings.Cut(l, "="); ok && strings.TrimSpace(k) == "ssh_extra_args" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// shQuoteArg: Ansible argümanları shlex ile böler; boşluk içeren yol tek tırnakla korunur.
func shQuoteArg(s string) string {
	if !strings.ContainsAny(s, " \t'\"\\$`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
