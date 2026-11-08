package manifest

import (
	"fmt"
	"strings"
	"testing"
)

// Küçük ama geçerli bir ansible bildirimi; %s yerine "sections" alanı (ya da boş) konur.
const secBase = `{
 "id":"demo","version":"0.1.0","sshpocket_api":1,"runner":"ansible","inventory_group":"demo",
 "name":"Demo","description":"d",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "requires":{"control":{"python":">=3.12","packages":["ansible-core"]},"target":{"python":">=3.9"}},
 "actions":{
  "preview":{"cmd":"ansible-playbook playbooks/preview.yml --check --diff","effects":[],"requires_sudo":true,"confirm":"none","warnings":["w"]},
  "apply":{"cmd":"ansible-playbook playbooks/apply.yml","effects":["changes-config"],"requires_sudo":true,"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "audit":{"cmd":"ansible-playbook playbooks/audit.yml","effects":["installs-package"],"requires_sudo":true,"confirm":"yes-no","warnings":["w"]}
 },
 "vars":[
  {"name":"demo_ntp","type":"string","default":"","group":"NTP","description":"x"},
  {"name":"demo_zone","type":"string","default":"","group":"Zaman","description":"y"}
 ]%s}`

func secManifest(sections string) []byte {
	extra := ""
	if sections != "" {
		extra = `,"sections":` + sections
	}
	return []byte(fmt.Sprintf(secBase, extra))
}

func TestSectionsAbsentIsFine(t *testing.T) {
	_, errs, code := Validate(secManifest(""), "demo", "0.1.0")
	if code != 0 {
		t.Fatalf("sections olmadan geçersiz: %v", errs)
	}
}

func TestSectionsValid(t *testing.T) {
	lines, errs, code := Validate(secManifest(`[
	 {"id":"ntp","title":"NTP","summary":"Saati senkron tutar.","installs":["chrony"],"touches":["/etc/chrony/x"],"vars_group":"NTP","plan_tokens":["ntp","paket"],"order":30},
	 {"id":"lynis","title":"Lynis","summary":"Denetim.","installs":["lynis"],"action":"audit"}
	]`), "demo", "0.1.0")
	if code != 0 {
		t.Fatalf("geçerli sections reddedildi: %v", errs)
	}
	got := strings.Join(lines, "\n")
	for _, want := range []string{
		"section.ntp=NTP", "section.ntp.installs=chrony", "section.ntp.vars_group=NTP",
		"section.ntp.plan_tokens=ntp,paket", "section.ntp.order=30", "section.lynis.action=audit",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("çıktıda %q yok:\n%s", want, got)
		}
	}
}

// Her geçersiz örnek REDDEDİLMELİ ve hata iletisi beklenen anahtar sözcüğü içermeli (boş test olmasın).
func TestSectionsInvalid(t *testing.T) {
	ok := `{"id":"a","title":"A","summary":"s"}`
	cases := []struct{ name, sections, want string }{
		{"boş liste", `[]`, "sections"},
		{"liste değil", `{"id":"a"}`, "sections"},
		{"nesne değil", `["a"]`, "nesne"},
		{"id yok", `[{"title":"A","summary":"s"}]`, "id"},
		{"id büyük harf", `[{"id":"A","title":"A","summary":"s"}]`, "id"},
		{"yinelenen id", `[` + ok + `,` + ok + `]`, "yinelenen"},
		{"başlık yok", `[{"id":"a","summary":"s"}]`, "title"},
		{"başlık çok uzun", `[{"id":"a","title":"` + strings.Repeat("x", 41) + `","summary":"s"}]`, "title"},
		{"özet yok", `[{"id":"a","title":"A"}]`, "summary"},
		{"özet çok uzun", `[{"id":"a","title":"A","summary":"` + strings.Repeat("x", 301) + `"}]`, "summary"},
		{"özet denetim karakteri", `[{"id":"a","title":"A","summary":"a\u0001b"}]`, "summary"},
		{"bilinmeyen alan", `[{"id":"a","title":"A","summary":"s","komut":"rm"}]`, "bilinmeyen"},
		{"tanımsız grup", `[{"id":"a","title":"A","summary":"s","vars_group":"Yok"}]`, "vars_group"},
		{"tanımsız eylem", `[{"id":"a","title":"A","summary":"s","action":"yok"}]`, "action"},
		{"installs liste değil", `[{"id":"a","title":"A","summary":"s","installs":"chrony"}]`, "installs"},
		{"installs virgüllü öğe", `[{"id":"a","title":"A","summary":"s","installs":["a,b"]}]`, "installs"},
		{"installs 11 öğe", `[{"id":"a","title":"A","summary":"s","installs":["1","2","3","4","5","6","7","8","9","10","11"]}]`, "installs"},
		{"plan_tokens artı", `[{"id":"a","title":"A","summary":"s","plan_tokens":["a+b"]}]`, "plan_tokens"},
		{"order sınır dışı", `[{"id":"a","title":"A","summary":"s","order":1000}]`, "order"},
		{"order metin", `[{"id":"a","title":"A","summary":"s","order":"1"}]`, "order"},
	}
	for _, c := range cases {
		_, errs, code := Validate(secManifest(c.sections), "demo", "0.1.0")
		if code == 0 {
			t.Errorf("%s: geçersiz sections kabul edildi", c.name)
			continue
		}
		if !strings.Contains(strings.Join(errs, "\n"), c.want) {
			t.Errorf("%s: hata %q içermeli, gelen: %v", c.name, c.want, errs)
		}
	}
}

func TestSectionsTooMany(t *testing.T) {
	var parts []string
	for i := 0; i < 31; i++ {
		parts = append(parts, fmt.Sprintf(`{"id":"s%d","title":"T","summary":"s"}`, i))
	}
	_, _, code := Validate(secManifest("["+strings.Join(parts, ",")+"]"), "demo", "0.1.0")
	if code == 0 {
		t.Fatal("31 kart kabul edildi")
	}
}
