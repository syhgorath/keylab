package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCfg(t *testing.T, body string) string {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "ansible.cfg"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSSHOptionsKnownHosts(t *testing.T) {
	t.Setenv("KEYLAB_KNOWN_HOSTS", "")
	t.Setenv("SSHPOCKET_KNOWN_HOSTS", "")
	if strings.Contains(strings.Join(SSHOptions(), " "), "UserKnownHostsFile") {
		t.Fatal("ayarsızken known_hosts seçeneği eklendi")
	}
	t.Setenv("KEYLAB_KNOWN_HOSTS", "/usb/state/known_hosts")
	if got := strings.Join(SSHOptions(), " "); !strings.HasSuffix(got, "-o UserKnownHostsFile=/usb/state/known_hosts") || !strings.Contains(got, "StrictHostKeyChecking=yes") {
		t.Fatalf("SSHOptions: %q", got)
	}
}

func TestAnsibleExtraArgsEnv(t *testing.T) {
	cfg := "[defaults]\nssh_extra_args = YANLIS BOLUM\n\n[ssh_connection]\n# yorum\nssh_extra_args = -o ServerAliveInterval=15 -o ServerAliveCountMax=3\n\n[diger]\nssh_extra_args = BASKA\n"
	t.Setenv("KEYLAB_KNOWN_HOSTS", "")
	t.Setenv("SSHPOCKET_KNOWN_HOSTS", "")
	if v, ok := ansibleExtraArgsEnv(writeCfg(t, cfg)); ok || v != "" {
		t.Fatalf("known_hosts yönlendirilmemişken ortam ayarlandı: %q %v", v, ok)
	}
	t.Setenv("KEYLAB_KNOWN_HOSTS", "/usb/state/known_hosts")
	os.Unsetenv("ANSIBLE_SSH_EXTRA_ARGS")
	// paketin [ssh_connection] değeri KORUNUR, doğru bölümden okunur, sonuna known_hosts eklenir
	want := "-o ServerAliveInterval=15 -o ServerAliveCountMax=3 -o UserKnownHostsFile=/usb/state/known_hosts"
	if v, ok := ansibleExtraArgsEnv(writeCfg(t, cfg)); !ok || v != want {
		t.Fatalf("birleşik: %q %v", v, ok)
	}
	// cfg yok / bölüm yok: yalnız known_hosts
	if v, _ := ansibleExtraArgsEnv(t.TempDir()); v != "-o UserKnownHostsFile=/usb/state/known_hosts" {
		t.Fatalf("cfg yok: %q", v)
	}
	// kullanıcının kendi ortam değişkeni varsa o korunur (ini yerine)
	t.Setenv("ANSIBLE_SSH_EXTRA_ARGS", "-o Foo=bar")
	if v, _ := ansibleExtraArgsEnv(writeCfg(t, cfg)); v != "-o Foo=bar -o UserKnownHostsFile=/usb/state/known_hosts" {
		t.Fatalf("kullanıcı değişkeni: %q", v)
	}
	// boşluklu yol tek tırnakla (Ansible shlex ile böler)
	t.Setenv("KEYLAB_KNOWN_HOSTS", "/Volumes/MY USB/state/known_hosts")
	os.Unsetenv("ANSIBLE_SSH_EXTRA_ARGS")
	if v, _ := ansibleExtraArgsEnv(t.TempDir()); v != "-o UserKnownHostsFile='/Volumes/MY USB/state/known_hosts'" {
		t.Fatalf("boşluklu yol: %q", v)
	}
}

func TestAnsibleEnvCarriesKnownHostsOnlyWhenRedirected(t *testing.T) {
	ec := &ExecCtx{PkgDir: writeCfg(t, "[ssh_connection]\nssh_extra_args = -o ServerAliveInterval=15\n"), BinDir: "/venv/bin"}
	has := func(env []string, prefix string) string {
		for _, kv := range env {
			if strings.HasPrefix(kv, prefix) {
				return kv
			}
		}
		return ""
	}
	t.Setenv("KEYLAB_KNOWN_HOSTS", "")
	t.Setenv("SSHPOCKET_KNOWN_HOSTS", "")
	os.Unsetenv("ANSIBLE_SSH_EXTRA_ARGS")
	env := ec.ansibleEnv("30")
	if has(env, "ANSIBLE_SSH_EXTRA_ARGS=") != "" || has(env, "ANSIBLE_HOST_KEY_CHECKING=") != "ANSIBLE_HOST_KEY_CHECKING=True" || has(env, "ANSIBLE_TIMEOUT=") != "ANSIBLE_TIMEOUT=30" {
		t.Fatalf("yönlendirmesiz ortam: %v", env[len(env)-6:])
	}
	t.Setenv("KEYLAB_KNOWN_HOSTS", "/usb/kh")
	if got := has(ec.ansibleEnv("30"), "ANSIBLE_SSH_EXTRA_ARGS="); got != "ANSIBLE_SSH_EXTRA_ARGS=-o ServerAliveInterval=15 -o UserKnownHostsFile=/usb/kh" {
		t.Fatalf("yönlendirmeli: %q", got)
	}
}
