//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// termServerApp: USB motoru gerektirmeyen deneme uygulaması; verilen adla geçerli bir sunucu klasörü vardır
func termServerApp(t *testing.T, name string) *App {
	t.Helper()
	root := t.TempDir()
	a := &App{serversDir: filepath.Join(root, "servers"), compDir: filepath.Join(root, "components"), stateDir: filepath.Join(root, "state"), jobs: map[string]*Job{}, up: map[string]bool{}}
	for _, d := range []string{filepath.Join(a.serversDir, name), a.compDir, a.stateDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(a.serversDir, name, "server.env"), []byte("IP=192.0.2.10\nUSER=admin\nPORT=22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return a
}

// gerçek ssh-keygen görev terminalinde: parola istemleri (iki kez) boş yanıtlanır; anahtar oluşur, 0600; ikinci deneme üzerine yazmaz
func TestKeygenTask(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen yok")
	}
	a := termServerApp(t, "kvm")
	ts, code, err := a.openServerTask("keygen", "kvm", 24, 80)
	if err != nil {
		t.Fatalf("başlamadı (%d): %v", code, err)
	}
	endTask(t, ts)
	if ts.Kind != "task" || ts.Pkg != "" || ts.Action != "keygen" || ts.Title != "keygen" {
		t.Fatalf("oturum bilgisi: %+v", ts)
	}
	for _, prompt := range []string{"Enter passphrase", "Enter same passphrase"} {
		waitOut(t, ts, prompt)
		if ok, err := ts.write([]byte("\n")); err != nil || !ok {
			t.Fatalf("yanıt yazılamadı: %v %v", ok, err)
		}
	}
	waitDone(t, ts, 15*time.Second)
	if ts.exit != 0 {
		t.Fatalf("ssh-keygen çıkış %d: %s", ts.exit, taskOutput(ts))
	}
	key := a.serversDir + "/kvm/id_ed25519_kvm"
	if st, err := os.Stat(key); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("özel anahtar yok ya da izin yanlış: %v", err)
	}
	if _, err := os.Stat(key + ".pub"); err != nil {
		t.Error("genel anahtar yok")
	}
	before, _ := os.ReadFile(key)
	if _, code, err := a.openServerTask("keygen", "kvm", 24, 80); err == nil || code != 409 {
		t.Errorf("var olan anahtarın üzerine yazılmamalı (kod %d)", code)
	}
	if after, _ := os.ReadFile(key); string(before) != string(after) {
		t.Error("anahtar değişti")
	}
	if _, code, _ := a.openServerTask("bilinmeyen", "kvm", 24, 80); code != 400 {
		t.Errorf("bilinmeyen adım: kod %d", code)
	}
	if _, code, _ := a.openServerTask("keycopy", "yok_boyle", 24, 80); code != 404 {
		t.Errorf("olmayan sunucu: kod %d", code)
	}
}

// silme: kapılar ve motorun ad sorusunun otomatik yanıtlanması (motor yerine sahte komut); çalışan eylem varken silinmez
func TestRemoveServerInTerminal(t *testing.T) {
	a := termServerApp(t, "silinecek")
	_ = os.WriteFile(a.serversDir+"/silinecek/id_ed25519_silinecek", []byte("GIZLI"), 0o600)
	fake := filepath.Join(t.TempDir(), "motor")
	os.WriteFile(fake, []byte("#!/bin/sh\nprintf 'Onaylamak için sunucu adını yazın: '; read -r ad; echo \"ALINDI:$1:$3:$ad\"; read -r k; echo \"KEYCHAIN:$k\"\n"), 0o755)
	old := taskProgram
	taskProgram = func(string) string { return fake }
	t.Cleanup(func() { taskProgram = old })

	for _, c := range []struct {
		confirm string
		ack     bool
		code    int
	}{{"yanlis", true, 400}, {"silinecek", false, 400}} {
		if _, code, err := a.openRemove("silinecek", c.confirm, c.ack, 24, 80); err == nil || code != c.code {
			t.Errorf("%+v: %d %v", c, code, err)
		}
	}
	a.jobs["busy"] = &Job{ID: "busy", Server: "baska"}
	if _, code, err := a.openRemove("silinecek", "silinecek", true, 24, 80); err == nil || code != 409 {
		t.Errorf("çalışan iş varken silme: %d %v", code, err)
	}
	delete(a.jobs, "busy")
	ts, code, err := a.openRemove("silinecek", "silinecek", true, 24, 80)
	if err != nil {
		t.Fatalf("%d %v", code, err)
	}
	endTask(t, ts)
	if _, code, err := a.openRemove("silinecek", "silinecek", true, 24, 80); err == nil || code != 409 {
		t.Errorf("görev sürerken ikinci silme: %d %v", code, err)
	}
	waitOut(t, ts, "ALINDI:server-remove:silinecek:silinecek")
	if strings.Contains(taskOutput(ts), "KEYCHAIN:") {
		t.Fatal("ad dışındaki soruyu kullanıcı yanıtlamalı")
	}
	if ok, err := ts.write([]byte("h\n")); err != nil || !ok {
		t.Fatal(ok, err)
	}
	waitOut(t, ts, "KEYCHAIN:h")
	b, _ := os.ReadFile(filepath.Join(a.stateDir, "terminal-audit.log"))
	if !strings.Contains(string(b), "server:remove\tsilinecek\t"+ts.ID) {
		t.Fatalf("denetim günlüğü: %q", b)
	}
}
