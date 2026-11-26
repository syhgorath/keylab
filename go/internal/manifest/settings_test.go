package manifest

import (
	"fmt"
	"strings"
	"testing"
)

// %s: "settings" alanının değeri
const setBase = `{
 "id":"demo","version":"0.1.0","sshpocket_api":1,"runner":"ansible","inventory_group":"demo",
 "name":"Demo","description":"d",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "requires":{"control":{"python":">=3.12","packages":["ansible-core"]},"target":{"python":">=3.9"}},
 "actions":{
  "durum":{"cmd":"ansible-playbook playbooks/durum.yml","effects":[],"confirm":"none","warnings":["w"],
    "outputs":{"format":"keyvalue-line","keys":["zone","k1","k2"]}},
  "kok":{"cmd":"ansible-playbook playbooks/kok.yml","effects":[],"requires_sudo":true,"confirm":"none","warnings":["w"],
    "outputs":{"format":"keyvalue-line","keys":["zone"]}},
  "preview":{"cmd":"ansible-playbook playbooks/preview.yml --check --diff","effects":[],"requires_sudo":true,"confirm":"none","warnings":["w"]},
  "apply":{"cmd":"ansible-playbook playbooks/apply.yml","effects":["changes-config"],"requires_sudo":true,"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "sil":{"cmd":"ansible-playbook playbooks/sil.yml","effects":["destructive"],"requires_sudo":true,"requires":["preview"],"confirm":"type-server-name","warnings":["w"]}
 },
 "vars":[
  {"name":"demo_ntp","type":"list[string]","default":[],"group":"NTP","description":"x"},
  {"name":"demo_zone","type":"string","default":"","group":"Zaman"}
 ],
 "settings":%s}`

func setValidate(settings string) ([]string, []string, int) {
	return Validate([]byte(fmt.Sprintf(setBase, settings)), "demo", "0.1.0")
}

const setGood = `[
 {"id":"ntp","label":"NTP sunucuları","var":"demo_ntp","description":"d","risk":["time-jump"],"risk_note":"n","apply_via":["apply"],
  "state":{"action":"durum","keys":["k1","k2"],"compare":"set","source":"result","strip_prefix":["server ","pool "]}},
 {"id":"zone","label":"Saat dilimi","var":"demo_zone","description":"d","apply_via":["apply"],"state":{"action":"durum","key":"zone","absent":"yok","ignore":["otomatik"],"not_applicable":["dagitim","ic"]}},
 {"id":"duz","label":"Okunmayan","var":"demo_zone","description":"d","apply_via":["apply"]}
]`

func TestSettingsValid(t *testing.T) {
	lines, errs, code := setValidate(setGood)
	if code != 0 {
		t.Fatalf("geçerli settings reddedildi: %v", errs)
	}
	got := "\n" + strings.Join(lines, "\n") + "\n"
	for _, want := range []string{"setting.ntp=NTP sunucuları", "setting.ntp.var=demo_ntp", "setting.ntp.apply=apply", "setting.ntp.risk=time-jump",
		"setting.ntp.read=durum", "setting.ntp.keys=k1,k2", "setting.ntp.compare=set", "setting.zone.compare=exact", "setting.duz.var=demo_zone"} {
		if !strings.Contains(got, "\n"+want+"\n") {
			t.Errorf("satır yok: %q", want)
		}
	}
	if strings.Contains(got, "setting.duz.read=") {
		t.Error("state'siz ayarda read satırı olmamalı")
	}
}

func TestSettingsRejects(t *testing.T) {
	one := func(extra string) string {
		return `[{"id":"a","label":"A","var":"demo_zone","apply_via":["apply"]` + extra + `}]`
	}
	st := func(body string) string { return one(`,"state":{` + body + `}`) }
	for _, tc := range []struct{ name, settings, want string }{
		{"liste değil", `{}`, "settings 1-40"},
		{"boş liste", `[]`, "settings 1-40"},
		{"nesne değil", `["x"]`, "nesne olmalı"},
		{"bilinmeyen alan", one(`,"cmd":"rm"`), "bilinmeyen alan: cmd"},
		{"kötü id", `[{"id":"../x","label":"A","var":"demo_zone","apply_via":["apply"]}]`, "id küçük harfle"},
		{"yinelenen id", `[{"id":"a","label":"A","var":"demo_zone","apply_via":["apply"]},{"id":"a","label":"B","var":"demo_zone","apply_via":["apply"]}]`, "yinelenen id"},
		{"etiket yok", `[{"id":"a","var":"demo_zone","apply_via":["apply"]}]`, "label geçersiz metin"},
		{"değişken yok", `[{"id":"a","label":"A","var":"yok","apply_via":["apply"]}]`, "vars[] içinde"},
		{"apply yok", `[{"id":"a","label":"A","var":"demo_zone"}]`, "apply_via"},
		{"apply tanımsız", `[{"id":"a","label":"A","var":"demo_zone","apply_via":["yok"]}]`, "apply_via"},
		{"risk sözlük dışı", one(`,"risk":["tehlike"]`), "risk şunlardan"},
		{"risk yinelenen", one(`,"risk":["time-jump","time-jump"]`), "risk şunlardan"},
		{"state action yok", st(`"action":"yok","key":"zone"`), "action tanımlı"},
		{"state yazan eylem", st(`"action":"apply","key":"zone"`), "salt okunur olmalı"},
		{"state anahtar bildirilmemiş", st(`"action":"durum","key":"baska"`), "outputs.keys listesinde yok"},
		{"keys içinde bildirilmemiş", st(`"action":"durum","keys":["k1","k9"],"compare":"set"`), "'k9' anahtarı"},
		{"key ve keys", st(`"action":"durum","key":"zone","keys":["k1"]`), "tam biri"},
		{"ikisi de yok", st(`"action":"durum"`), "tam biri"},
		{"compare sözlük dışı", st(`"action":"durum","key":"zone","compare":"yakın"`), "compare şunlardan"},
		{"keys exact", st(`"action":"durum","keys":["k1"]`), "keys ile compare"},
		{"summary kaynağı", st(`"action":"durum","key":"zone","source":"summary"`), "source yalnızca"},
		{"strip_prefix uzun", st(`"action":"durum","key":"zone","strip_prefix":["123456789012345678901"]`), "strip_prefix"},
		{"state bilinmeyen alan", st(`"action":"durum","key":"zone","path":"/etc"`), "bilinmeyen alan: path"},
		{"absent boşluklu", st(`"action":"durum","key":"zone","absent":"hiç yok"`), "absent sonuç satırı"},
		{"absent keys ile", st(`"action":"durum","keys":["k1"],"compare":"set","absent":"yok"`), "absent yalnızca"},
		{"ignore boş liste", st(`"action":"durum","key":"zone","ignore":[]`), "ignore en çok"},
		{"ignore metin değil", st(`"action":"durum","key":"zone","ignore":[1]`), "ignore en çok"},
		{"ignore çok uzun", st(`"action":"durum","key":"zone","ignore":["a","b","c","d","e","f"]`), "ignore en çok"},
		{"not_applicable keys ile", st(`"action":"durum","keys":["k1"],"compare":"set","not_applicable":["ic"]`), "not_applicable yalnızca"},
		{"not_applicable boş", st(`"action":"durum","key":"zone","not_applicable":[]`), "not_applicable en çok"},
		{"not_applicable boşluklu", st(`"action":"durum","key":"zone","not_applicable":["iç ağ"]`), "not_applicable en çok"},
		{"not_applicable unknown", st(`"action":"durum","key":"zone","not_applicable":["unknown"]`), "not_applicable en çok"},
		{"not_applicable absent ile aynı", st(`"action":"durum","key":"zone","absent":"yok","not_applicable":["yok"]`), "not_applicable en çok"},
		{"kart tanımsız", one(`,"card":"yok"`), "card, setting_cards"},
	} {
		_, errs, code := setValidate(tc.settings)
		if code == 0 || !strings.Contains(strings.Join(errs, "\n"), tc.want) {
			t.Errorf("%s: kod %d, hata %v (beklenen %q)", tc.name, code, errs, tc.want)
		}
	}
}

func TestSettingCards(t *testing.T) {
	one := `[{"id":"a","label":"A","var":"demo_zone","apply_via":["apply"],"card":"saat"}]`
	with := func(cards, settings string) ([]string, []string, int) {
		m := strings.Replace(fmt.Sprintf(setBase, settings), `"settings":`, `"setting_cards":`+cards+`,"settings":`, 1)
		return Validate([]byte(m), "demo", "0.1.0")
	}
	lines, errs, code := with(`[{"id":"saat","label":"Saat","description":"d"}]`, one)
	if code != 0 {
		t.Fatalf("geçerli kart reddedildi: %v", errs)
	}
	got := "\n" + strings.Join(lines, "\n") + "\n"
	for _, want := range []string{"setting_card.saat=Saat", "setting.a.card=saat"} {
		if !strings.Contains(got, "\n"+want+"\n") {
			t.Errorf("satır yok: %q", want)
		}
	}
	for _, tc := range []struct{ name, cards, settings, want string }{
		{"liste değil", `{}`, one, "setting_cards 1-10"},
		{"nesne değil", `[1]`, one, "nesne olmalı"},
		{"bilinmeyen alan", `[{"id":"saat","label":"S","cmd":"x"}]`, one, "bilinmeyen alan: cmd"},
		{"kötü id", `[{"id":"../x","label":"S"}]`, one, "id geçersiz"},
		{"yinelenen", `[{"id":"saat","label":"S"},{"id":"saat","label":"T"}]`, one, "yinelenmiş"},
		{"etiket yok", `[{"id":"saat"}]`, one, "label geçersiz"},
		{"boş kart", `[{"id":"saat","label":"S"},{"id":"bos","label":"B"}]`, one, "kartını kullanan ayar yok"},
	} {
		_, errs, code := with(tc.cards, tc.settings)
		if code == 0 || !strings.Contains(strings.Join(errs, "\n"), tc.want) {
			t.Errorf("%s: kod %d, hata %v (beklenen %q)", tc.name, code, errs, tc.want)
		}
	}
	sum := func(body string) ([]string, []string, int) {
		return with(`[{"id":"saat","label":"S","summary":`+body+`}]`, one)
	}
	lines, errs, code = sum(`{"action":"durum","items":[{"key":"zone","label":"Dilim","ok":"x"},{"key":"k1","label":"K","max_abs":5,"unit":"ms"}]}`)
	if code != 0 {
		t.Fatalf("geçerli özet reddedildi: %v", errs)
	}
	got = "\n" + strings.Join(lines, "\n") + "\n"
	for _, want := range []string{"setting_card.saat.read=durum", "setting_card.saat.keys=zone,k1"} {
		if !strings.Contains(got, "\n"+want+"\n") {
			t.Errorf("özet satırı yok: %q", want)
		}
	}
	item := func(body string) string { return `{"action":"durum","items":[` + body + `]}` }
	for _, tc := range []struct{ name, body, want string }{
		{"nesne değil", `[]`, "summary nesne"},
		{"bilinmeyen alan", `{"action":"durum","items":[{"key":"zone","label":"L"}],"cmd":"x"}`, "bilinmeyen alan: cmd"},
		{"yazan eylem", `{"action":"apply","items":[{"key":"zone","label":"L"}]}`, "salt okunur"},
		{"tanımsız eylem", `{"action":"yok","items":[{"key":"zone","label":"L"}]}`, "salt okunur"},
		{"boş liste", `{"action":"durum","items":[]}`, "items 1-6"},
		{"anahtar bildirilmemiş", item(`{"key":"baska","label":"L"}`), "outputs.keys listesinde"},
		{"anahtar yinelenmiş", item(`{"key":"zone","label":"L"},{"key":"zone","label":"M"}`), "yinelenmiş"},
		{"etiket yok", item(`{"key":"zone"}`), "label geçersiz"},
		{"ok ve sınır", item(`{"key":"zone","label":"L","ok":"x","max_abs":1}`), "birlikte"},
		{"negatif sınır", item(`{"key":"zone","label":"L","max_abs":-1}`), "max_abs sıfır"},
		{"uzun birim", item(`{"key":"zone","label":"L","unit":"123456789"}`), "unit geçersiz"},
		{"öğe alanı", item(`{"key":"zone","label":"L","path":"/"}`), "bilinmeyen alan: path"},
	} {
		_, errs, code := sum(tc.body)
		if code == 0 || !strings.Contains(strings.Join(errs, "\n"), tc.want) {
			t.Errorf("özet %s: kod %d, hata %v (beklenen %q)", tc.name, code, errs, tc.want)
		}
	}
	lines, _, _ = sum(`{"action":"kok","items":[{"key":"zone","label":"L"}]}`)
	c := &pkgChecker{}
	c.checkSettings(parseInfo(lines))
	found := false
	for _, r := range c.results {
		found = found || (r.level == "warn" && strings.Contains(r.text, "kart saat: özet eylemi 'kok' sudo"))
	}
	if !found {
		t.Errorf("sudo isteyen özet uyarılmadı: %+v", c.results)
	}
	alone := strings.Replace(fmt.Sprintf(setBase, setGood), `"settings":`, `"setting_cards":[],"yok":`, 1)
	if _, errs, code := Validate([]byte(alone), "demo", "0.1.0"); code == 0 || !strings.Contains(strings.Join(errs, "\n"), "yalnızca settings") {
		t.Errorf("settings olmadan setting_cards kabul edildi: %v", errs)
	}
}

// items[].settings BİLEREK desteklenmez (eski sürümler paketi reddeder): üst düzey kullanılmalı.
func TestSettingsInsideItemsRejected(t *testing.T) {
	m := strings.Replace(fmt.Sprintf(setBase, setGood), `"settings":`, `"item_var":"demo_zone","items":[{"id":"a","label":"A","settings":[]}],"settings":`, 1)
	_, errs, code := Validate([]byte(m), "demo", "0.1.0")
	if code == 0 || !strings.Contains(strings.Join(errs, "\n"), "bilinmeyen alan: settings") {
		t.Fatalf("items[].settings kabul edildi: %d %v", code, errs)
	}
}

func TestSettingsCheckerWarnings(t *testing.T) {
	lines, errs, code := setValidate(`[
	 {"id":"iyi","label":"İ","var":"demo_ntp","apply_via":["apply"]},
	 {"id":"kotu","label":"K","var":"demo_zone","apply_via":["sil"],"state":{"action":"kok","key":"zone"}}
	]`)
	if code != 0 {
		t.Fatalf("geçersiz: %v", errs)
	}
	c := &pkgChecker{}
	c.checkSettings(parseInfo(lines))
	var got []string
	for _, r := range c.results {
		got = append(got, r.level+" "+r.text)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{"ok iyi", "warn kotu: okuma eylemi 'kok' sudo istiyor", "warn kotu: yıkıcı eylem 'sil'", "warn kotu: açıklama yok"} {
		if !strings.Contains(all, want) {
			t.Errorf("beklenen sonuç yok: %q\n%s", want, all)
		}
	}
	if len(got) != 4 {
		t.Errorf("sonuç sayısı %d:\n%s", len(got), all)
	}
}
