package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/manifest"
)

const scopeManifest = `{
 "id":"demo","version":"0.1.0","sshpocket_api":1,"runner":"ansible","inventory_group":"demo","name":"Demo","description":"d",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "requires":{"control":{"python":">=3.12","packages":["ansible-core"]},"target":{"python":">=3.9"}},
 "actions":{
  "preview":{"cmd":"ansible-playbook playbooks/preview.yml --check --diff","effects":[],"requires_sudo":true,"confirm":"none","warnings":["w"]},
  "kur":{"cmd":"ansible-playbook playbooks/kur.yml","effects":["installs-package"],"requires_sudo":true,"requires":["preview"],"confirm":"yes-no","warnings":["w"]}
 },
 "item_var":"demo_secim","mode_var":"demo_mod",
 "items":[
  {"id":"a","label":"A","modes":["native","docker"],"actions":["preview","kur"]},
  {"id":"b","label":"B","modes":["native","docker"],"actions":["preview","kur"]}
 ],
 "vars":[
  {"name":"demo_secim","type":"enum","values":["a","b"],"default":"a","group":"Seçim","description":"x"},
  {"name":"demo_mod","type":"enum","values":["native","docker"],"default":"native","group":"Seçim","description":"x"}
 ]}`

func scopePkg(t *testing.T, manifestText string) (dir string, info []string) {
	t.Helper()
	dir = t.TempDir()
	mf := filepath.Join(dir, "component.json")
	os.WriteFile(mf, []byte(manifestText), 0o644)
	lines, errs, code := manifest.ValidateFile(mf, "", "")
	if code != 0 {
		t.Fatalf("örnek paket geçersiz: %v", errs)
	}
	return dir, lines
}

func TestScopeForEffectiveSelection(t *testing.T) {
	dir, info := scopePkg(t, scopeManifest)
	vals := filepath.Join(t.TempDir(), "demo.vars")
	ec := &ExecCtx{PkgDir: dir, ValuesFile: vals}

	if got, err := ec.scopeFor(info, "kur"); err != nil || got != "a:native" { // dosya yok: varsayılanlar
		t.Fatalf("varsayılan: %q %v", got, err)
	}
	os.WriteFile(vals, []byte("demo_secim=b\ndemo_mod=docker\n"), 0o600)
	if got, _ := ec.scopeFor(info, "kur"); got != "b:docker" {
		t.Errorf("değer dosyası: %q", got)
	}
	ec.RunSet = "demo_secim=a"
	if got, _ := ec.scopeFor(info, "kur"); got != "a:docker" { // koşuya özel öğe, moddan değer dosyası
		t.Errorf("koşuya özel: %q", got)
	}
	ec.RunSet = "demo_secim=a\ndemo_mod=native"
	if got, _ := ec.scopeFor(info, "preview"); got != "a:native" {
		t.Errorf("koşuya özel öğe+mod: %q", got)
	}
	for _, bad := range []string{"demo_secim=yok", "demo_mod=docker", "demo_secim=a\ndemo_mod=zzz", "baska=1"} {
		ec.RunSet = bad
		if _, err := ec.scopeFor(info, "kur"); err == nil {
			t.Errorf("geçersiz seçim kabul edildi: %q", bad)
		}
	}
	ec.RunSet = ""
	// değer dosyasında katalogda olmayan öğe (ör. elle bozulmuş): enum reddeder
	os.WriteFile(vals, []byte("demo_secim=zzz\n"), 0o600)
	if _, err := ec.scopeFor(info, "kur"); err == nil {
		t.Error("katalogda olmayan değer dosyası seçimi kabul edildi")
	}
}

func TestScopeForPackageWithoutItems(t *testing.T) {
	plain := strings.Replace(strings.Split(scopeManifest, ` "item_var"`)[0], `"warnings":["w"]}
 },`, `"warnings":["w"]}
 },`, 1)
	plain = strings.TrimRight(plain, ",\n ") + `,"vars":[]}`
	dir, info := scopePkg(t, plain)
	ec := &ExecCtx{PkgDir: dir}
	if got, err := ec.scopeFor(info, "kur"); err != nil || got != "" {
		t.Errorf("öğesiz paket kapsamı %q %v", got, err)
	}
}

// Önkoşul kapısı YALNIZ aynı kapsamdaki başarılı kaydı sayar.
func TestRequiresScopedMatchesOnlySameScope(t *testing.T) {
	info := []string{"action=preview", "action=kur", "action.kur.requires=preview"}
	state := t.TempDir()
	now := int64(1_800_000_000)
	g := &Gates{StateDir: state, Now: func() int64 { return now }}
	if err := g.RecordScoped("demo", "srv", "0.1.0", "preview", "ok", "a:native"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		scope string
		ok    bool
	}{{"a:native", true}, {"b:native", false}, {"a:docker", false}, {"", false}}
	for _, c := range cases {
		err := g.RequiresScoped(info, "demo", "srv", "0.1.0", "kur", c.scope)
		if (err == nil) != c.ok {
			t.Errorf("kapsam %q: beklenen ok=%v, gelen %v", c.scope, c.ok, err)
		}
	}
	// eski (kapsamsız) kayıt yalnız kapsamsız denetimi karşılar; kapsamlı denetimi karşılamaz
	g2 := &Gates{StateDir: t.TempDir(), Now: func() int64 { return now }}
	if err := g2.Record("demo", "srv", "0.1.0", "preview", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := g2.RequiresScoped(info, "demo", "srv", "0.1.0", "kur", ""); err != nil {
		t.Errorf("kapsamsız kayıt kapsamsız denetimi karşılamalı: %v", err)
	}
	if err := g2.RequiresScoped(info, "demo", "srv", "0.1.0", "kur", "a:native"); err == nil {
		t.Error("kapsamsız eski kayıt kapsamlı denetimi karşıladı (öğe/mod atlanabilir)")
	}
	// başarısız kayıt ve eski kayıt (TTL) sayılmaz
	g3 := &Gates{StateDir: t.TempDir(), Now: func() int64 { return now }}
	g3.RecordScoped("demo", "srv", "0.1.0", "preview", "fail", "a:native")
	if err := g3.RequiresScoped(info, "demo", "srv", "0.1.0", "kur", "a:native"); err == nil {
		t.Error("başarısız önizleme önkoşulu karşıladı")
	}
	g4 := &Gates{StateDir: g.StateDir, Now: func() int64 { return now + 31*60 }}
	if err := g4.RequiresScoped(info, "demo", "srv", "0.1.0", "kur", "a:native"); err == nil {
		t.Error("TTL'i aşan önizleme önkoşulu karşıladı")
	}
	// kapsam biçimi doğrulanır
	if err := g.RecordScoped("demo", "srv", "0.1.0", "preview", "ok", "a|b"); err == nil {
		t.Error("geçersiz kapsam kaydedildi")
	}
	// age: kapsama göre
	if _, ok := g.RequiresAgeScoped("demo", "srv", "0.1.0", "preview", "b:native"); ok {
		t.Error("başka kapsamın yaşı döndü")
	}
	if age, ok := g.RequiresAgeScoped("demo", "srv", "0.1.0", "preview", "a:native"); !ok || age != 0 {
		t.Errorf("yaş: %d %v", age, ok)
	}
}
