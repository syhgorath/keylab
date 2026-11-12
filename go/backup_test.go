package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// backupApp: itemRunApp'in "demo" paketi (demo_secim: a|b|c, demo_mod: native|docker) ve "vm1" sunucusuyla.
func backupApp(t *testing.T) *App {
	t.Helper()
	a, _ := itemRunApp(t)
	return a
}

func writeVars(t *testing.T, a *App, server, pkg, body string) {
	t.Helper()
	if err := os.WriteFile(a.varsFile(server, pkg), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBackupExportOmitsSecretsAndKeys(t *testing.T) {
	a := backupApp(t)
	// KEY yolu ve knock ayarı server.env'de; gizli görünen değişken .vars'ta
	os.WriteFile(filepath.Join(a.serversDir, "vm1", "server.env"), []byte("IP=127.0.0.1\nUSER=deploy\nPORT=1\nKEY=/home/u/.ssh/id_x\nKNOCK=keychain\nOS=ubuntu\n"), 0o600)
	writeVars(t, a, "vm1", "demo", "demo_secim=b\ndemo_db_password=cok-gizli\n")
	f, omitted := a.backupExport()
	if omitted != 1 {
		t.Fatalf("1 gizli değişken dışarıda kalmalı, %d", omitted)
	}
	if len(f.Servers) != 1 || f.Servers[0].Name != "vm1" || f.Servers[0].OS != "ubuntu" {
		t.Fatalf("sunucu: %+v", f.Servers)
	}
	raw := mustJSON(t, f)
	for _, bad := range []string{"cok-gizli", "demo_db_password", "id_x", "KEY", "KNOCK", "keychain"} {
		if strings.Contains(raw, bad) {
			t.Errorf("yedekte %q olmamalı:\n%s", bad, raw)
		}
	}
	if f.Servers[0].Vars["demo"]["demo_secim"] != "b" {
		t.Errorf("normal değişken yedekte olmalı: %+v", f.Servers[0].Vars)
	}
}

func TestBackupImportPreviewWritesNothing(t *testing.T) {
	a := backupApp(t)
	f := backupFile{Format: backupFormat, Version: 1, Servers: []backupServer{
		{Name: "yeni1", Host: "192.0.2.50", User: "deploy", Port: "22", Vars: map[string]map[string]string{"demo": {"demo_secim": "b"}}},
	}}
	rows, err := a.backupImport(f, false, nil)
	if err != nil || len(rows) != 1 || rows[0].Action != "create" || rows[0].Status != "planned" || rows[0].VarsChg != 1 {
		t.Fatalf("önizleme: %+v %v", rows, err)
	}
	if _, err := os.Stat(filepath.Join(a.serversDir, "yeni1")); err == nil {
		t.Fatal("önizleme diske yazmamalı")
	}
	// apply=true ama pick boş → yine hiçbir şey yazılmaz
	rows, _ = a.backupImport(f, true, nil)
	if rows[0].Status != "skipped" {
		t.Fatalf("seçilmeyen sunucu atlanmalı: %+v", rows[0])
	}
	if _, err := os.Stat(filepath.Join(a.serversDir, "yeni1")); err == nil {
		t.Fatal("seçilmeyen sunucu oluşturulmamalı")
	}
}

func TestBackupImportCreateAndUpdate(t *testing.T) {
	a := backupApp(t)
	f := backupFile{Format: backupFormat, Version: 1, Servers: []backupServer{
		{Name: "yeni1", Host: "192.0.2.50", User: "deploy", Port: "2222", Vars: map[string]map[string]string{"demo": {"demo_secim": "b"}}},
		{Name: "vm1", Host: "192.0.2.77", User: "deploy", Port: "22"},
	}}
	// vm1'in eski anahtarı/knock'u korunmalı
	os.WriteFile(filepath.Join(a.serversDir, "vm1", "server.env"), []byte("IP=127.0.0.1\nUSER=deploy\nPORT=1\nKEY=/home/u/.ssh/id_x\nKNOCK=keychain\n"), 0o600)
	rows, err := a.backupImport(f, true, []string{"yeni1", "vm1"})
	if err != nil {
		t.Fatal(err)
	}
	st := map[string]backupRow{}
	for _, r := range rows {
		st[r.Name] = r
	}
	if st["yeni1"].Status != "ok" || st["yeni1"].Action != "create" || st["vm1"].Status != "ok" || st["vm1"].Action != "update" || !st["vm1"].AddrChg {
		t.Fatalf("sonuç: %+v", rows)
	}
	b, _ := os.ReadFile(filepath.Join(a.serversDir, "yeni1", "server.env"))
	for _, w := range []string{"IP=192.0.2.50", "USER=deploy", "PORT=2222"} {
		if !strings.Contains(string(b), w) {
			t.Errorf("%q yok:\n%s", w, b)
		}
	}
	if v := readVarsFile(a.varsFile("yeni1", "demo")); v["demo_secim"] != "b" {
		t.Errorf("değişken yazılmadı: %v", v)
	}
	b, _ = os.ReadFile(filepath.Join(a.serversDir, "vm1", "server.env"))
	for _, w := range []string{"IP=192.0.2.77", "KEY=/home/u/.ssh/id_x", "KNOCK=keychain"} {
		if !strings.Contains(string(b), w) {
			t.Errorf("güncellemede %q korunmalı/yazılmalı:\n%s", w, b)
		}
	}
	// ikinci çalıştırma: değişiklik yok → same
	rows, _ = a.backupImport(f, false, nil)
	if rows[0].Action != "same" || rows[1].Action != "same" {
		t.Errorf("tekrar çalıştırma 'same' vermeli: %+v", rows)
	}
}

func TestBackupImportRejectsDangerousInput(t *testing.T) {
	a := backupApp(t)
	f := backupFile{Format: backupFormat, Version: 1, Servers: []backupServer{
		{Name: "../kacis", Host: "192.0.2.1", User: "x", Port: "22"},
		{Name: "kotu1", Host: "-oProxyCommand=id", User: "x", Port: "22"},
		{Name: "kotu2", Host: "192.0.2.2", User: "root; id", Port: "22"},
		{Name: "kotu3", Host: "192.0.2.3", User: "x", Port: "99999"},
		{Name: "tmp1", Host: "192.0.2.4", User: "x", Port: "22", Vars: map[string]map[string]string{
			"demo":      {"demo_secim": "z", "yok_degisken": "1", "demo_mod": "docker"},
			"olmayanpk": {"y": "1"},
		}},
	}}
	rows, err := a.backupImport(f, true, []string{"../kacis", "kotu1", "kotu2", "kotu3", "tmp1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows[:4] {
		if r.Status != "error" || r.Action != "invalid" {
			t.Errorf("%s reddedilmeliydi: %+v", r.Name, r)
		}
	}
	for _, n := range []string{"kacis", "kotu1", "kotu2", "kotu3"} {
		if _, err := os.Stat(filepath.Join(a.serversDir, n)); err == nil {
			t.Errorf("%s oluşmamalıydı", n)
		}
	}
	if _, err := os.Stat(filepath.Join(a.serversDir, "..", "kacis")); err == nil {
		t.Error("yol kaçışı: sunucu dizininin dışına yazıldı")
	}
	// tmp1: sunucu oluşur ama geçersiz değişkenler (kötü değer, pakette olmayan ad, olmayan paket) yazılmaz; geçerli olan yazılır
	r := rows[4]
	if r.Status != "ok" {
		t.Fatalf("tmp1: %+v", r)
	}
	v := readVarsFile(a.varsFile("tmp1", "demo"))
	if v["demo_mod"] != "docker" {
		t.Errorf("geçerli değişken yazılmalı: %v", v)
	}
	if _, bad := v["demo_secim"]; bad || v["yok_degisken"] != "" {
		t.Errorf("geçersiz değişken yazılmamalı: %v", v)
	}
	if _, err := os.Stat(a.varsFile("tmp1", "olmayanpk")); err == nil {
		t.Error("olmayan paket için vars dosyası oluşmamalı")
	}
	if len(r.Notes) < 3 {
		t.Errorf("reddedilen değişkenler için not beklenir: %v", r.Notes)
	}
}

func TestBackupImportFormatAndLimits(t *testing.T) {
	a := backupApp(t)
	for name, f := range map[string]backupFile{
		"biçim yok":  {Version: 1, Servers: []backupServer{{Name: "a", Host: "1.1.1.1", User: "x"}}},
		"sürüm":      {Format: backupFormat, Version: 9, Servers: []backupServer{{Name: "a", Host: "1.1.1.1", User: "x"}}},
		"boş liste":  {Format: backupFormat, Version: 1},
		"çok sunucu": {Format: backupFormat, Version: 1, Servers: make([]backupServer, backupMaxServers+1)},
	} {
		if _, err := a.backupImport(f, false, nil); err == nil {
			t.Errorf("%s: reddedilmeliydi", name)
		}
	}
	// aynı ad iki kez
	f := backupFile{Format: backupFormat, Version: 1, Servers: []backupServer{{Name: "ikiz", Host: "192.0.2.9", User: "x"}, {Name: "ikiz", Host: "192.0.2.10", User: "x"}}}
	rows, _ := a.backupImport(f, true, []string{"ikiz"})
	if rows[0].Status != "ok" || rows[1].Status != "skipped" {
		t.Errorf("aynı ad: %+v", rows)
	}
	if b, _ := os.ReadFile(filepath.Join(a.serversDir, "ikiz", "server.env")); !strings.Contains(string(b), "192.0.2.9") {
		t.Errorf("ilk kayıt kullanılmalı:\n%s", b)
	}
}

func TestBackupAuditLog(t *testing.T) {
	a := backupApp(t)
	a.backupExport()
	a.backupAudit("deneme")
	st, err := os.Stat(filepath.Join(a.stateDir, "backup-audit.log"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("denetim günlüğü 0600 olmalı: %v %v", st, err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Yol kaçışı: yedekteki paket/değişken anahtarları dosya yoluna akar; geçersizse HİÇBİR dosya işleminden önce reddedilmeli.
func TestBackupImportRejectsTraversalInPackageAndVarNames(t *testing.T) {
	a := backupApp(t)
	// sunucu dizininin DIŞINDA, "../secret" paket anahtarıyla okunabilecek bir .vars dosyası
	outside := filepath.Join(a.serversDir, "secret.vars")
	if err := os.WriteFile(outside, []byte("demo_secim=zzz\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]map[string]map[string]string{
		"paket ../secret":     {"../secret": {"demo_secim": "a"}},
		"paket mutlak yol":    {"/etc/passwd": {"demo_secim": "a"}},
		"paket nokta":         {"..": {"demo_secim": "a"}},
		"paket boş":           {"": {"demo_secim": "a"}},
		"değişken yol":        {"demo": {"../x": "a"}},
		"değişken büyük harf": {"demo": {"DEMO_SECIM": "a"}},
	}
	for name, vars := range cases {
		f := backupFile{Format: backupFormat, Version: 1, Servers: []backupServer{{Name: "vm1", Host: "192.0.2.50", User: "deploy", Port: "22", Vars: vars}}}
		for _, apply := range []bool{false, true} {
			rows, err := a.backupImport(f, apply, []string{"vm1"})
			if err != nil || len(rows) != 1 || rows[0].Action != "invalid" || rows[0].Status != "error" {
				t.Errorf("%s (apply=%v) reddedilmeliydi: %+v %v", name, apply, rows, err)
			}
		}
	}
	// reddedilen yedek hiçbir şeyi değiştirmemeli (adres aynı kaldı)
	if b, _ := os.ReadFile(filepath.Join(a.serversDir, "vm1", "server.env")); !strings.Contains(string(b), "IP=127.0.0.1") {
		t.Errorf("reddedilen yedek sunucuyu değiştirmemeli:\n%s", b)
	}
	// geçerli adlar hâlâ çalışır
	f := backupFile{Format: backupFormat, Version: 1, Servers: []backupServer{{Name: "vm1", Host: "127.0.0.1", User: "deploy", Port: "1", Vars: map[string]map[string]string{"demo": {"demo_mod": "docker"}}}}}
	rows, _ := a.backupImport(f, false, nil)
	if len(rows) != 1 || rows[0].Action != "update" {
		t.Errorf("geçerli yedek önizlemesi bozuldu: %+v", rows)
	}
}
