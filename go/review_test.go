package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/manifest"
	"keylab/internal/runner"
)

const reviewPkg = `{"id":"demo","version":"0.2.0","sshpocket_api":1,"runner":"ansible","inventory_group":"demo","name":"Demo","description":"d",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "requires":{"control":{"python":">=3.12","packages":["ansible-core"]},"target":{"python":">=3.9"}},
 "actions":{
  "preview":{"cmd":"ansible-playbook playbooks/p.yml --check","effects":[],"requires_sudo":true,"confirm":"none","warnings":["w"]},
  "durum":{"cmd":"ansible-playbook playbooks/d.yml","effects":[],"confirm":"none","warnings":["w"],"outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.durum","keys":["zone"]}},
  "uygun":{"cmd":"ansible-playbook playbooks/a.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "kartli":{"cmd":"ansible-playbook playbooks/k.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "adli":{"cmd":"ansible-playbook playbooks/n.yml","effects":["changes-config"],"requires":["preview"],"confirm":"type-server-name","warnings":["w"]},
  "yikici":{"cmd":"ansible-playbook playbooks/y.yml","effects":["destructive"],"requires":["preview"],"confirm":"type-server-name","warnings":["w"]},
  "onizlemesiz":{"cmd":"ansible-playbook playbooks/o.yml","effects":["changes-config"],"confirm":"yes-no","warnings":["w"]},
  "degiskenli":{"cmd":"ansible-playbook playbooks/g.yml","effects":["changes-config"],"requires":["preview"],"requires_vars":["demo_zone"],"confirm":"yes-no","warnings":["w"]}
 },
 "vars":[{"name":"demo_zone","type":"string","default":"","group":"Zaman","description":"x"}],
 "settings":[{"id":"zone","label":"Saat dilimi","var":"demo_zone","description":"d","apply_via":["kartli"],"state":{"action":"durum","key":"zone"}}]}`

// "Önizle ve uygula" bayrağı yalnız kartı olmayan, önizlemeli, yıkıcı olmayan e/h eyleminde; motorun zincir kuralıyla aynı sonucu verir.
func TestReviewFlag(t *testing.T) {
	flags := func(pkg string) map[string]bool {
		a := batchApp(t)
		writePkg(t, a.compDir, "demo", "0.2.0", pkg)
		s, ok := a.findServerQuiet("s1")
		if !ok {
			t.Fatal("sunucu yok")
		}
		out := map[string]bool{}
		for _, p := range a.packageInfosFor(s, "") {
			if p.ID == "demo" {
				for _, x := range p.Actions {
					out[x.ID] = x.Review
				}
			}
		}
		return out
	}
	got := flags(reviewPkg)
	if len(got) != 8 {
		t.Fatalf("eylemler: %v", got)
	}
	for id, review := range got {
		if review != (id == "uygun") {
			t.Errorf("%s: review=%v", id, review)
		}
	}
	for name, pkg := range map[string]string{
		"betik paketi":              strings.Replace(reviewPkg, `"runner":"ansible"`, `"runner":"script"`, 1),
		"değiştiren önkoşul":        strings.Replace(reviewPkg, `"requires":["preview"],"confirm":"yes-no"`, `"requires":["onizlemesiz"],"confirm":"yes-no"`, 1),
		"platforma uymayan önkoşul": strings.Replace(reviewPkg, `"requires_sudo":true,`, `"requires_sudo":true,"platforms":["proxmox-host"],`, 1),
		"tanımsız önkoşul":          strings.Replace(reviewPkg, `"requires":["preview"],"confirm":"yes-no"`, `"requires":["yok"],"confirm":"yes-no"`, 1),
	} {
		if pkg == reviewPkg {
			t.Fatalf("%s: paket değişmedi", name)
		}
		if flags(pkg)["uygun"] {
			t.Errorf("%s: düğme çıkmamalı", name)
		}
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "component.json"), []byte(reviewPkg), 0o600)
	info, errs, code := manifest.ValidateFile(filepath.Join(dir, "component.json"), "demo", "0.2.0")
	if code != 0 {
		t.Fatalf("paket geçersiz: %v", errs)
	}
	for _, id := range []string{"uygun", "adli", "onizlemesiz"} {
		plan, err := runner.ChainPlan(info, id, "")
		if (err == nil && plan.Ask) != got[id] {
			t.Errorf("%s: arayüz ile motor uyuşmuyor: %+v %v", id, plan, err)
		}
	}
}

// Bildirilen sonuç anahtarları arayüze gider (katalog, uygulama kartını bunlardan tanır).
func TestActionOutKeys(t *testing.T) {
	a := batchApp(t)
	writePkg(t, a.compDir, "demo", "0.2.0", reviewPkg)
	s, ok := a.findServerQuiet("s1")
	if !ok {
		t.Fatal("sunucu yok")
	}
	for _, p := range a.packageInfosFor(s, "") {
		for _, x := range p.Actions {
			if p.ID == "demo" && x.ID == "durum" && !(len(x.OutKeys) == 1 && x.OutKeys[0] == "zone") {
				t.Errorf("durum anahtarları: %v", x.OutKeys)
			}
			if p.ID == "demo" && x.ID == "preview" && len(x.OutKeys) != 0 {
				t.Errorf("preview anahtarları: %v", x.OutKeys)
			}
		}
	}
}
