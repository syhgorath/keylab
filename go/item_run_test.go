package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const itemRunManifest = `{"id":"demo","version":"0.1.0","name":"Demo","description":"d","item_var":"demo_secim","mode_var":"demo_mod",
 "items":[
  {"id":"a","label":"A","modes":["native","docker"],"actions":["preview","kur"]},
  {"id":"b","label":"B","modes":["native","docker"],"actions":["preview","kur"]},
  {"id":"c","label":"C","modes":["native"],"actions":["preview"]}
 ],
 "actions":{
  "durum":{"effects":[],"requires_sudo":false,"confirm":"none"},
  "preview":{"effects":[],"requires_sudo":false,"confirm":"none"},
  "kur":{"effects":["installs-package"],"requires_sudo":true,"requires":["preview"],"confirm":"yes-no","warnings":["w"]}
 },
 "vars":[
  {"name":"demo_secim","type":"enum","values":["a","b","c"],"default":"a","description":"x"},
  {"name":"demo_mod","type":"enum","values":["native","docker"],"default":"native","description":"x"}
 ]}`

func itemRunApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	comp, srvs, state := filepath.Join(dir, "components"), filepath.Join(dir, "servers"), filepath.Join(dir, "state")
	writePkg(t, comp, "demo", "0.1.0", itemRunManifest)
	writePkg(t, comp, "duz", "1.0.0", `{"id":"duz","version":"1.0.0","name":"Düz","description":"d","actions":{"durum":{"effects":[],"requires_sudo":false,"confirm":"none"}},"vars":[]}`)
	os.MkdirAll(filepath.Join(srvs, "vm1"), 0o700)
	// bağlantı hemen reddedilsin (yoklama beklemesin)
	os.WriteFile(filepath.Join(srvs, "vm1", "server.env"), []byte("IP=127.0.0.1\nUSER=deploy\nPORT=1\n"), 0o600)
	os.MkdirAll(filepath.Join(state, "demo", "vm1"), 0o700)
	a := &App{serversDir: srvs, compDir: comp, stateDir: state, jobs: map[string]*Job{}, up: map[string]bool{}, conn: map[string]string{}}
	return a, filepath.Join(state, "demo", "vm1", "results")
}

func TestAuthorizeRunItemValidation(t *testing.T) {
	a, _ := itemRunApp(t)
	cases := []struct {
		name, pkg, action, item string
	}{
		{"katalogda olmayan öğe", "demo", "preview", "z"},
		{"öğenin eylemi değil", "demo", "durum", "a"},
		{"öğe eylem listesinde yok", "demo", "kur", "c"},
		{"öğesiz pakette öğe", "duz", "durum", "a"},
		{"olmayan paket", "yok", "preview", "a"},
	}
	for _, c := range cases {
		_, _, code, err := a.authorizeRunItem("vm1", c.pkg, c.action, c.item, true, "evet", false)
		if err == nil || code != 409 {
			t.Errorf("%s: reddedilmeliydi (kod %d, hata %v)", c.name, code, err)
		}
	}
	// geçerli: salt okunur önizleme çalıştırılabilir, item_var döner
	ai, iv, code, err := a.authorizeRunItem("vm1", "demo", "preview", "a", false, "", false)
	if err != nil || code != 0 || iv != "demo_secim" || !ai.Runnable {
		t.Errorf("geçerli öğe eylemi: %+v %q %d %v", ai, iv, code, err)
	}
}

// Öğe eylemi, YALNIZ o öğenin kapsamındaki taze önizlemeyle yetkilendirilir.
func TestAuthorizeRunItemScopedPrereqs(t *testing.T) {
	a, results := itemRunApp(t)
	os.WriteFile(results, []byte(fmt.Sprintf("%d|0.1.0|preview|ok|a:native\n", time.Now().Unix()-30)), 0o600)

	if _, _, code, err := a.authorizeRunItem("vm1", "demo", "kur", "a", true, "evet", false); err != nil {
		t.Fatalf("a için önizleme var, kurulum yetkilenmeliydi: %v (kod %d)", err, code)
	}
	if _, _, code, err := a.authorizeRunItem("vm1", "demo", "kur", "b", true, "evet", false); err == nil || code != 409 {
		t.Errorf("b için önizleme yok, reddedilmeliydi (kod %d, hata %v)", code, err)
	}
	// seçim değişkeni b olsa bile öğe isteği a ise kapsam a'dır (istemci seçimi değer dosyasından bağımsızdır)
	os.WriteFile(filepath.Join(a.serversDir, "vm1", "demo.vars"), []byte("demo_secim=b\n"), 0o600)
	if _, _, _, err := a.authorizeRunItem("vm1", "demo", "kur", "a", true, "evet", false); err != nil {
		t.Errorf("değer dosyası b iken a öğesi için istek yetkilenmeli: %v", err)
	}
	// mod değişince (docker) a:native önizlemesi yetmez
	os.WriteFile(filepath.Join(a.serversDir, "vm1", "demo.vars"), []byte("demo_mod=docker\n"), 0o600)
	if _, _, code, err := a.authorizeRunItem("vm1", "demo", "kur", "a", true, "evet", false); err == nil || code != 409 {
		t.Errorf("mod docker iken native önizlemesi kabul edildi (kod %d, hata %v)", code, err)
	}
	// onaylar yine zorunlu
	os.Remove(filepath.Join(a.serversDir, "vm1", "demo.vars"))
	if _, _, code, err := a.authorizeRunItem("vm1", "demo", "kur", "a", false, "evet", false); err == nil || code != 400 {
		t.Errorf("snapshot onayı olmadan kabul edildi (kod %d, hata %v)", code, err)
	}
}

func TestPackageInfosItemStates(t *testing.T) {
	a, results := itemRunApp(t)
	os.WriteFile(results, []byte(fmt.Sprintf("%d|0.1.0|preview|ok|a:native\n", time.Now().Unix()-30)), 0o600)
	s, _ := a.findServer("vm1")
	var demo PackageInfo
	for _, p := range a.packageInfos(s) {
		if p.ID == "demo" {
			demo = p
		}
	}
	by := map[string]ItemInfo{}
	for _, it := range demo.Items {
		by[it.ID] = it
	}
	if !by["a"].States["kur"].Write || !by["a"].States["kur"].PrereqsOK {
		t.Errorf("a: kur durumu: %+v", by["a"].States["kur"])
	}
	if by["b"].States["kur"].Write || by["b"].States["kur"].PrereqsOK || by["b"].States["kur"].Reason == "" {
		t.Errorf("b: kur önkoşulsuz kapalı olmalı: %+v", by["b"].States["kur"])
	}
	if !by["b"].States["preview"].Runnable {
		t.Errorf("b: önizleme çalıştırılabilir olmalı: %+v", by["b"].States["preview"])
	}
	if _, ok := by["c"].States["kur"]; ok {
		t.Errorf("c'nin eylem listesinde olmayan eylem durumda: %v", by["c"].States)
	}
}

func TestRunArgvItemSelection(t *testing.T) {
	got := runArgv("/x/keylab", "/s", "/c", "/st", "demo", "kur", "vm1", "demo_secim", "b")
	want := []string{"/x/keylab", "run", "-servers", "/s", "-components", "/c", "-state", "/st", "-set", "demo_secim=b", "demo", "kur", "vm1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("argv:\n%q\n%q", got, want)
	}
	if got := runArgv("/x/keylab", "/s", "/c", "/st", "demo", "kur", "vm1", "demo_secim", ""); strings.Contains(strings.Join(got, " "), "-set") {
		t.Errorf("öğesiz çağrıda -set var: %q", got)
	}
}
