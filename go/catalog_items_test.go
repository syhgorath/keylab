package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePkg(t *testing.T, comp, id, ver, manifest string) {
	t.Helper()
	d := filepath.Join(comp, id, ver)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(d, "component.json"), []byte(manifest), 0o644)
	os.WriteFile(filepath.Join(comp, id, "current"), []byte(ver+"\n"), 0o644)
}

func TestPackageInfosExposeItems(t *testing.T) {
	comp := t.TempDir()
	long := strings.Repeat("ü", 600)
	writePkg(t, comp, "demo", "0.1.0", `{"id":"demo","version":"0.1.0","name":"Demo","description":"d","item_var":"demo_secim","mode_var":"demo_mod",
	 "items":[
	  {"id":"docker","label":"Docker","description":"Konteyner","modes":["native","docker"],"actions":["preview","kur"],"params":["demo_docker_surum"]},
	  {"id":"../kotu","label":"x"},
	  {"id":"uzun","label":"`+long+`","description":"`+long+`","modes":["a","b","c","d","e","f","g"]}
	 ],
	 "actions":{"durum":{"effects":[],"requires_sudo":false,"confirm":"none"}},
	 "vars":[{"name":"demo_secim","type":"string","default":"","group":"Seçim","description":"x"}]}`)
	writePkg(t, comp, "duz", "1.0.0", `{"id":"duz","version":"1.0.0","name":"Düz","description":"d","actions":{"durum":{"effects":[]}},"vars":[]}`)
	a := &App{serversDir: t.TempDir(), compDir: comp, stateDir: t.TempDir()}
	infos := a.packageInfos(Server{Name: "web1"})
	by := map[string]PackageInfo{}
	for _, p := range infos {
		by[p.ID] = p
	}
	d, ok := by["demo"]
	if !ok {
		t.Fatalf("demo paketi yok: %+v", infos)
	}
	if d.ItemVar != "demo_secim" || d.ModeVar != "demo_mod" {
		t.Errorf("item_var/mode_var: %q %q", d.ItemVar, d.ModeVar)
	}
	if len(d.Items) != 2 || d.Items[0].ID != "docker" || d.Items[1].ID != "uzun" { // "../kotu" elenir
		t.Fatalf("öğeler: %+v", d.Items)
	}
	if got := d.Items[0]; got.Label != "Docker" || len(got.Modes) != 2 || got.Actions[1] != "kur" || got.Params[0] != "demo_docker_surum" {
		t.Errorf("docker öğesi: %+v", got)
	}
	if n := len([]rune(d.Items[1].Label)); n != 80 {
		t.Errorf("etiket kısaltılmadı: %d", n)
	}
	if n := len([]rune(d.Items[1].Description)); n != 400 {
		t.Errorf("açıklama kısaltılmadı: %d", n)
	}
	if len(d.Items[1].Modes) != 5 {
		t.Errorf("mod sayısı sınırlanmadı: %d", len(d.Items[1].Modes))
	}
	if p := by["duz"]; len(p.Items) != 0 || p.ItemVar != "" {
		t.Errorf("items olmayan pakette öğe var: %+v", p)
	}
}
