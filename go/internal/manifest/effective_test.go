package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveVarsPrecedence(t *testing.T) {
	dir := t.TempDir()
	mf := filepath.Join(dir, "component.json")
	os.WriteFile(mf, []byte(`{"id":"demo","version":"0.1.0","name":"d","description":"d","runner":"script",
	 "vars":[
	  {"name":"demo_secim","type":"enum","values":["a","b","c"],"default":"a","description":"x"},
	  {"name":"demo_mod","type":"enum","values":["native","docker"],"default":"native","description":"x"},
	  {"name":"demo_port","type":"int","default":8080,"description":"x"},
	  {"name":"demo_bos","type":"string","description":"x"}
	 ]}`), 0o644)
	vals := filepath.Join(dir, "demo.vars")

	// dosya yok: varsayılanlar
	got, err := EffectiveVars(mf, filepath.Join(dir, "yok.vars"), nil, "demo_secim", "demo_mod", "demo_port", "demo_bos")
	if err != nil {
		t.Fatal(err)
	}
	if got["demo_secim"] != "a" || got["demo_mod"] != "native" || got["demo_port"] != "8080" {
		t.Errorf("varsayılanlar: %v", got)
	}
	if _, ok := got["demo_bos"]; ok {
		t.Errorf("değeri olmayan değişken sonuçta: %v", got)
	}
	// değer dosyası varsayılanı ezer
	os.WriteFile(vals, []byte("demo_secim=b\ndemo_mod=docker\n"), 0o600)
	got, _ = EffectiveVars(mf, vals, nil, "demo_secim", "demo_mod")
	if got["demo_secim"] != "b" || got["demo_mod"] != "docker" {
		t.Errorf("değer dosyası: %v", got)
	}
	// koşuya özel değer dosyayı ezer
	got, _ = EffectiveVars(mf, vals, map[string]string{"demo_secim": "c"}, "demo_secim", "demo_mod")
	if got["demo_secim"] != "c" || got["demo_mod"] != "docker" {
		t.Errorf("forced: %v", got)
	}
	// geçersiz değer hata verir (sessizce varsayılana düşmez)
	if _, err := EffectiveVars(mf, vals, map[string]string{"demo_secim": "zzz"}, "demo_secim"); err == nil {
		t.Error("geçersiz enum değeri kabul edildi")
	}
	// tanımsız değişken hata verir
	if _, err := EffectiveVars(mf, vals, map[string]string{"yok_degisken": "1"}, "demo_secim"); err == nil {
		t.Error("tanımsız değişken kabul edildi")
	}
}
