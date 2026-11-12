package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sshCfg = `Host web1 web-alias
    HostName 192.0.2.10
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_web

Host *
    ServerAliveInterval 30

Host kotu
    HostName bad host;rm -rf
    User x

Host db
    HostName db.example.test
`

func sshApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	if err := os.WriteFile(cfg, []byte(sshCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return &App{serversDir: filepath.Join(dir, "servers"), host: "127.0.0.1:9999", token: "j", jobs: map[string]*Job{}, up: map[string]bool{}}, cfg
}

func TestSSHConfigList(t *testing.T) {
	a, cfg := sshApp(t)
	ex, rows, err := a.sshConfigList(cfg)
	if err != nil || !ex {
		t.Fatalf("ex=%v err=%v", ex, err)
	}
	var names []string
	for _, r := range rows {
		names = append(names, r.Alias)
		if r.Added {
			t.Errorf("%s henüz eklenmemiş olmalı", r.Alias)
		}
	}
	if got := strings.Join(names, ","); got != "web1,web-alias,kotu,db" {
		t.Errorf("takma adlar: %s", got)
	}
	if ex, rows, err := a.sshConfigList(filepath.Join(t.TempDir(), "yok")); ex || err != nil || len(rows) != 0 {
		t.Errorf("dosya yokken: ex=%v err=%v", ex, err)
	}
}

func TestSSHConfigImport(t *testing.T) {
	a, cfg := sshApp(t)
	res, err := a.sshConfigImport(cfg, []string{"web1", "web1", "kotu", "db", "olmayan"})
	if err != nil {
		t.Fatal(err)
	}
	st := map[string]string{}
	for _, r := range res {
		st[r.Alias] = r.Status
	}
	if len(res) != 4 || st["web1"] != "imported" || st["db"] != "imported" || st["kotu"] != "error" || st["olmayan"] != "error" {
		t.Fatalf("sonuç: %+v", res)
	}
	b, _ := os.ReadFile(filepath.Join(a.serversDir, "web1", "server.env"))
	for _, w := range []string{"IP=192.0.2.10", "USER=deploy", "PORT=2222", "KEY=~/.ssh/id_web"} {
		if !strings.Contains(string(b), w) {
			t.Errorf("%q yok:\n%s", w, b)
		}
	}
	if _, err := os.Stat(filepath.Join(a.serversDir, "kotu")); err == nil {
		t.Error("geçersiz host için klasör oluşmamalıydı")
	}
	b, _ = os.ReadFile(filepath.Join(a.serversDir, "db", "server.env"))
	if !strings.Contains(string(b), "USER=root") || !strings.Contains(string(b), "PORT=22") {
		t.Errorf("varsayılanlar:\n%s", b)
	}
	// ikinci kez: var olan atlanır, üzerine yazılmaz
	os.WriteFile(filepath.Join(a.serversDir, "db", "server.env"), []byte("IP=198.51.100.1\nUSER=el\nPORT=22\n"), 0o600)
	res, _ = a.sshConfigImport(cfg, []string{"db"})
	if len(res) != 1 || res[0].Status != "skipped" {
		t.Fatalf("var olan atlanmalı: %+v", res)
	}
	if b, _ := os.ReadFile(filepath.Join(a.serversDir, "db", "server.env")); !strings.Contains(string(b), "USER=el") {
		t.Error("var olan sunucunun üzerine yazıldı")
	}
}
