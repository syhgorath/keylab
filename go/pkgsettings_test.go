package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const setPkg = `{"id":"demo","version":"0.2.0","sshpocket_api":1,"name":"Demo","description":"d",
 "actions":{
  "durum":{"cmd":"x","effects":[],"outputs":{"format":"keyvalue-line","keys":["timezone"]}},
  "ntp_durum":{"cmd":"x","effects":[],"outputs":{"format":"keyvalue-line","keys":["ntp_kaynak_1","ntp_kaynak_2","ntp_senkron","liste"]}},
  "apply":{"cmd":"x","effects":["changes-config"],"confirm":"yes-no"},
  "pve_oku":{"cmd":"x","effects":[],"platforms":["proxmox-host"],"outputs":{"format":"keyvalue-line","keys":["pve"]}}
 },
 "vars":[
  {"name":"demo_ntp","type":"list[string]","default":[],"group":"NTP","description":"x"},
  {"name":"demo_zone","type":"string","default":"","group":"Zaman","description":"y"},
  {"name":"demo_adim","type":"bool","default":true,"group":"NTP","description":"z"},
  {"name":"demo_uyg","type":"string","default":"","group":"Uyg","description":"u"}
 ],
 "setting_cards":[{"id":"saat","label":"Saat","description":"d"},{"id":"../kotu","label":"x"}],
 "settings":[
  {"id":"ntp","label":"NTP sunucuları","var":"demo_ntp","risk":["time-jump"],"apply_via":["apply"],
   "state":{"action":"ntp_durum","keys":["ntp_kaynak_1","ntp_kaynak_2"],"compare":"set","strip_prefix":["server ","pool "]}},
  {"id":"zone","label":"Saat dilimi","var":"demo_zone","apply_via":["apply"],"state":{"action":"durum","key":"timezone"}},
  {"id":"adim","label":"Saat adımlama","var":"demo_adim","apply_via":["apply"],"card":"saat"},
  {"id":"uyg","label":"Uygulamalar","var":"demo_uyg","apply_via":["apply"],"state":{"action":"ntp_durum","key":"liste","compare":"set"}},
  {"id":"pve","label":"PVE","var":"demo_uyg","apply_via":["apply"],"state":{"action":"pve_oku","key":"pve"}}
 ]}`

// result.<eylem> dosyası ve results günlüğü
type setRead struct {
	action, ver, outcome string
	epoch                int64
	keys                 []string
}

func setSettings(t *testing.T, vars string, reads ...setRead) map[string]SettingInfo {
	t.Helper()
	comp, srv, state := t.TempDir(), t.TempDir(), t.TempDir()
	writePkg(t, comp, "demo", "0.2.0", setPkg)
	os.MkdirAll(filepath.Join(srv, "web1"), 0o700)
	if vars != "" {
		os.WriteFile(filepath.Join(srv, "web1", "demo.vars"), []byte(vars), 0o600)
	}
	sdir := filepath.Join(state, "demo", "web1")
	os.MkdirAll(sdir, 0o700)
	var log strings.Builder
	for _, r := range reads {
		fmt.Fprintf(&log, "%d|%s|%s|%s\n", r.epoch, r.ver, r.action, r.outcome)
		if r.keys != nil {
			os.WriteFile(filepath.Join(sdir, "result."+r.action), []byte(fmt.Sprintf("%d\n%s\n", r.epoch, strings.Join(r.keys, "\n"))), 0o600)
		}
	}
	os.WriteFile(filepath.Join(sdir, "results"), []byte(log.String()), 0o600)
	a := &App{serversDir: srv, compDir: comp, stateDir: state}
	infos := a.packageInfos(Server{Name: "web1"})
	if len(infos) != 1 {
		t.Fatalf("paket: %+v", infos)
	}
	out := map[string]SettingInfo{}
	for _, s := range infos[0].Settings {
		out[s.ID] = s
	}
	return out
}

func ntpRead(epoch int64, keys ...string) setRead {
	return setRead{"ntp_durum", "0.2.0", "ok", epoch, keys}
}

const setVars = "demo_ntp=server b.example.org,a.example.org\ndemo_zone=Europe/Istanbul\ndemo_uyg=nginx,docker\n"

func TestSettingsStatus(t *testing.T) {
	want := func(name string, s SettingInfo, status string, actual ...string) {
		t.Helper()
		if s.Status != status || strings.Join(s.Actual, " ") != strings.Join(actual, " ") {
			t.Errorf("%s: durum=%q gerçek=%v not=%q (beklenen %q %v)", name, s.Status, s.Actual, s.Note, status, actual)
		}
	}
	// hiç okunmamış
	g := setSettings(t, setVars)
	want("okunmamış", g["ntp"], setUnknown)
	want("okunmayan ayar", g["adim"], setUnreadable)
	want("uygulanmayan platform", g["pve"], setUnreadable)
	if g["pve"].Note != "bu sunucuda okunamıyor" || g["pve"].ReadAction != "" {
		t.Errorf("platform notu: %+v", g["pve"])
	}
	if g["adim"].Note != "" {
		t.Errorf("okuma bildirmeyen ayarda not olmamali: %q", g["adim"].Note)
	}
	if g["ntp"].Group != "NTP" || g["ntp"].ReadAction != "ntp_durum" || g["adim"].ReadAction != "" || g["adim"].Desired[0] != "true" {
		t.Errorf("alanlar: %+v / %+v", g["ntp"], g["adim"])
	}
	if strings.Join(g["ntp"].Desired, "|") != "server b.example.org|a.example.org" {
		t.Errorf("istenen liste: %v", g["ntp"].Desired)
	}
	// uyumlu: önek atılır, sıra önemsiz; '/' sonuç kuralıyla karşılaştırılır; tek anahtarlı '+' listesi
	g = setSettings(t, setVars,
		ntpRead(100, "ntp_kaynak_1=a.example.org", "ntp_kaynak_2=b.example.org", "liste=docker+nginx"),
		setRead{"durum", "0.2.0", "ok", 100, []string{"timezone=Europe_Istanbul"}})
	want("liste uyumlu", g["ntp"], setMatch, "a.example.org", "b.example.org")
	want("saat dilimi uyumlu", g["zone"], setMatch, "Europe_Istanbul")
	want("+ listesi", g["uyg"], setMatch, "docker", "nginx")
	if g["ntp"].Stale || g["ntp"].ReadTime == "" {
		t.Errorf("eskimiş/okuma zamanı: %+v", g["ntp"])
	}
	// sapma: eksik kaynak, farklı dilim
	g = setSettings(t, setVars,
		ntpRead(100, "ntp_kaynak_1=a.example.org", "liste=yok"),
		setRead{"durum", "0.2.0", "ok", 100, []string{"timezone=Etc_UTC"}})
	want("liste sapma", g["ntp"], setDrift, "a.example.org")
	want("dilim sapma", g["zone"], setDrift, "Etc_UTC")
	want("boş + listesi", g["uyg"], setDrift)
	// hiçbir liste anahtarı yok: okundu, sunucuda kaynak yok → sapma; tek anahtar yoksa bilinmiyor
	g = setSettings(t, setVars, ntpRead(100, "ntp_senkron=no"))
	want("kaynak yok", g["ntp"], setDrift)
	want("tek anahtar yok", g["uyg"], setUnknown)
	// paket "unknown" bildirirse bilinmiyor
	g = setSettings(t, setVars, ntpRead(100, "ntp_kaynak_1=unknown"))
	want("unknown", g["ntp"], setUnknown)
	// son okuma başarısız: eski değerlerle karar verilmez
	g = setSettings(t, setVars, ntpRead(100, "ntp_kaynak_1=a.example.org", "ntp_kaynak_2=b.example.org"), setRead{"ntp_durum", "0.2.0", "fail", 200, nil})
	want("başarısız", g["ntp"], setUnknown)
	// eski paket sürümünden okuma
	g = setSettings(t, setVars, setRead{"ntp_durum", "0.1.0", "ok", 100, []string{"ntp_senkron=yes"}})
	want("eski sürüm", g["ntp"], setUnknown)
	if !strings.Contains(g["ntp"].Note, "0.1.0") {
		t.Errorf("not: %q", g["ntp"].Note)
	}
	// istenen boş: yönetilmiyor (okunmamış olsa da)
	g = setSettings(t, "")
	want("yönetilmiyor", g["ntp"], setUnmanaged)
	if g["ntp"].Note != "" || g["ntp"].Desired == nil || g["ntp"].Actual == nil {
		t.Errorf("boş alanlar: %+v", g["ntp"])
	}
	g = setSettings(t, "", ntpRead(100, "ntp_kaynak_1=a.example.org"))
	want("yönetilmiyor ama okunan görünür", g["ntp"], setUnmanaged, "a.example.org")
	// okumadan sonra değiştiren eylem: eskimiş
	g = setSettings(t, setVars, ntpRead(100, "ntp_kaynak_1=a.example.org"), setRead{"apply", "0.2.0", "ok", 300, nil})
	if !g["ntp"].Stale {
		t.Errorf("eskimiş değil: %+v", g["ntp"])
	}
	// geçersiz değer dosyası: sapma/uyum uydurulmaz
	g = setSettings(t, "demo_adim=belki\n", ntpRead(100, "ntp_kaynak_1=a.example.org"))
	want("geçersiz değer", g["ntp"], setUnknown)
}

func TestSettingCardsInInfo(t *testing.T) {
	comp := t.TempDir()
	writePkg(t, comp, "demo", "0.2.0", setPkg)
	a := &App{serversDir: t.TempDir(), compDir: comp, stateDir: t.TempDir()}
	p := a.packageInfos(Server{Name: "web1"})[0]
	if len(p.SettingCards) != 1 || p.SettingCards[0] != (SettingCard{ID: "saat", Label: "Saat", Description: "d"}) {
		t.Fatalf("kartlar: %+v", p.SettingCards)
	}
	for _, s := range p.Settings {
		if want := map[string]string{"adim": "saat"}[s.ID]; s.Card != want {
			t.Errorf("%s kartı %q (beklenen %q)", s.ID, s.Card, want)
		}
	}
}

func TestSettingMatches(t *testing.T) {
	long := strings.Repeat("a", 70)
	for _, c := range []struct {
		compare         string
		desired, actual []string
		want            bool
	}{
		{"ordered", []string{"a", "b"}, []string{"b", "a"}, false},
		{"ordered", []string{"a", "b"}, []string{"a", "b"}, true},
		{"set", []string{"a", "a", "b"}, []string{"b", "a"}, true},
		{"set", []string{"a"}, []string{"a", "b"}, false},
		{"exact", []string{long}, []string{long[:64]}, true},
		{"exact", []string{"a b"}, []string{"a_b"}, true},
		{"present", []string{"x"}, []string{"y"}, true},
		{"present", []string{"x"}, nil, false},
	} {
		if got := settingMatches(c.compare, c.desired, c.actual, nil); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}
