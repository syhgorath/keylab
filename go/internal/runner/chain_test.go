package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"keylab/internal/manifest"
)

const chainPkg = `{
 "id":"demo","version":"0.1.0","sshpocket_api":1,"runner":"ansible","inventory_group":"demo","name":"Demo","description":"d",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "requires":{"control":{"python":">=3.12","packages":["ansible-core"]},"target":{"python":">=3.9"}},
 "actions":{
  "durum":{"cmd":"ansible-playbook playbooks/durum.yml","effects":[],"confirm":"none","warnings":["w"],"outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.durum","keys":["zone"]}},
  "kok":{"cmd":"ansible-playbook playbooks/kok.yml","effects":[],"requires_sudo":true,"confirm":"none","warnings":["w"],"outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.kok","keys":["zone"]}},
  "preview":{"cmd":"ansible-playbook playbooks/preview.yml --check","effects":[],"requires_sudo":true,"confirm":"none","warnings":["w"]},
  "apply":{"cmd":"ansible-playbook playbooks/apply.yml","effects":["changes-config"],"requires_sudo":true,"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "sil":{"cmd":"ansible-playbook playbooks/sil.yml","effects":["destructive"],"requires_sudo":true,"requires":["preview"],"confirm":"type-server-name","warnings":["w"]},
  "yikici_eh":{"cmd":"ansible-playbook playbooks/x.yml","effects":["destructive"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "saat_oku":{"cmd":"ansible-playbook playbooks/saat_oku.yml","effects":[],"confirm":"none","warnings":["w"],"outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.saat_oku","keys":["zone"]}},
  "saat_kardes":{"cmd":"ansible-playbook playbooks/saat_kardes.yml","effects":[],"confirm":"none","warnings":["w"],"outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.saat_kardes","keys":["zone"]}},
  "saat_ozet":{"cmd":"ansible-playbook playbooks/saat_ozet.yml","effects":[],"confirm":"none","warnings":["w"],"outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.saat_ozet","keys":["zone"]}},
  "serbest_oku":{"cmd":"ansible-playbook playbooks/serbest_oku.yml","effects":[],"confirm":"none","warnings":["w"],"outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.serbest_oku","keys":["zone"]}},
  "baska_serbest_oku":{"cmd":"ansible-playbook playbooks/baska_serbest_oku.yml","effects":[],"confirm":"none","warnings":["w"],"outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.baska_serbest_oku","keys":["zone"]}},
  "saat_ayarla":{"cmd":"ansible-playbook playbooks/saat_ayarla.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "saat_adim":{"cmd":"ansible-playbook playbooks/saat_adim.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "serbest_ayarla":{"cmd":"ansible-playbook playbooks/serbest_ayarla.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "baska":{"cmd":"ansible-playbook playbooks/baska.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]}
 },
 "vars":[{"name":"demo_zone","type":"string","default":"","group":"Zaman","description":"x"},{"name":"demo_saat","type":"string","default":"","group":"Zaman","description":"x"},{"name":"demo_adim","type":"string","default":"","group":"Zaman","description":"x"},{"name":"demo_serbest","type":"string","default":"","group":"Zaman","description":"x"},{"name":"demo_serbest2","type":"string","default":"","group":"Zaman","description":"x"}],
 "setting_cards":[{"id":"sistem","label":"Sistem","summary":{"action":"kok","items":[{"key":"zone","label":"Dilim"}]}},
   {"id":"saat","label":"Saat","summary":{"action":"saat_ozet","items":[{"key":"zone","label":"Dilim"}]}}],
 "settings":[
  {"id":"zone","card":"sistem","label":"Saat dilimi","var":"demo_zone","description":"d","apply_via":["apply","sil","yikici_eh"],"state":{"action":"durum","key":"zone"}},
  {"id":"serbest","label":"serbest","var":"demo_serbest","description":"d","apply_via":["apply"],"state":{"action":"serbest_oku","key":"zone"}},
  {"id":"kaynak","card":"saat","label":"kaynak","var":"demo_saat","description":"d","apply_via":["saat_ayarla"],"state":{"action":"saat_oku","key":"zone"}},
  {"id":"adim","card":"saat","label":"adim","var":"demo_adim","description":"d","apply_via":["saat_adim"],"state":{"action":"saat_kardes","key":"zone"}},
  {"id":"serbest2","label":"serbest2","var":"demo_serbest2","description":"d","apply_via":["serbest_ayarla"],"state":{"action":"baska_serbest_oku","key":"zone"}}
 ]}`

// sahte ansible: koşan playbook'u, argümanları ve (varsa) fd 3'ten okunan parolayı kaydeder
const chainAnsible = `#!/bin/bash
pb=""; for a in "$@"; do case "$a" in playbooks/*) pb="${a#playbooks/}"; pb="${pb%.yml}" ;; esac; done
pw="-"
case " $* " in *" --become-password-file /dev/fd/3 "*) IFS= read -r pw </dev/fd/3 ;; esac
echo "$pb|$pw|$*" >> "$CHAIN_LOG"
echo "adim $pb calisti"
case "$pb" in durum|kok|*_oku|saat_*) echo "SSHPOCKET_RESULT demo.$pb zone=UTC" ;; esac
[ "$pb" = "$CHAIN_FAIL" ] && exit 4
exit 0
`

type chainRun struct {
	rc       int
	out      string
	calls    []string // "playbook|parola|argümanlar"
	state    string
	recorded []string
}

func chainInfo(t *testing.T) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "component.json"), []byte(chainPkg), 0o600)
	info, errs, code := manifest.ValidateFile(filepath.Join(dir, "component.json"), "demo", "0.1.0")
	if code != 0 {
		t.Fatalf("paket geçersiz: %v", errs)
	}
	return dir, info
}

func runTestChain(t *testing.T, action, stdin, fail string) chainRun {
	t.Helper()
	w := newWorld(t)
	os.WriteFile(filepath.Join(w.bin, "ansible-playbook"), []byte(chainAnsible), 0o755)
	log := filepath.Join(t.TempDir(), "chain.log")
	t.Setenv("CHAIN_LOG", log)
	t.Setenv("CHAIN_FAIL", fail)
	pkg, info := chainInfo(t)
	plan, err := ChainPlan(info, action, "")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	r := chainRun{state: t.TempDir()}
	ec := &ExecCtx{PkgDir: pkg, BinDir: w.bin, Server: "srv", Host: "10.0.0.5", User: "deploy", Port: "22", Key: w.key, Prefix: "SRV", Profile: "ubuntu-26.04",
		Gates: &Gates{StateDir: r.state}, KnownHosts: w.known, Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &out, Ctx: context.Background()}
	r.rc = ec.runChain(info, "demo", "0.1.0", plan, func(a string, res ExecResult) { r.recorded = append(r.recorded, a) })
	r.out = out.String()
	if b, err := os.ReadFile(log); err == nil {
		r.calls = strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	return r
}

func (r chainRun) ran() (names, passwords []string) {
	for _, c := range r.calls {
		f := strings.SplitN(c, "|", 3)
		names, passwords = append(names, f[0]), append(passwords, f[1])
	}
	return
}

func TestChainPlan(t *testing.T) {
	_, info := chainInfo(t)
	c, err := ChainPlan(info, "apply", "")
	if err != nil || !reflect.DeepEqual(c.Pre, []string{"preview"}) || c.Action != "apply" {
		t.Fatalf("%+v %v", c, err)
	}
	// yalnız eylemin dokunduğu kartlar (ayar ve özet okumaları) ile aynı eyleme bağlı kartsız ayarlar okunur; ilgisiz kartlar okunmaz
	for action, want := range map[string][]string{
		"apply":          {"durum", "kok", "serbest_oku"},
		"saat_ayarla":    {"saat_kardes", "saat_oku", "saat_ozet"}, // aynı karttaki başka eylemin ayarı da kartın parçasıdır
		"saat_adim":      {"saat_kardes", "saat_oku", "saat_ozet"},
		"serbest_ayarla": {"baska_serbest_oku"}, // kartsız ayar: yalnız kendi okuması
	} {
		got, err := ChainPlan(info, action, "")
		post := slices.Clone(got.Post)
		slices.Sort(post)
		if err != nil || !reflect.DeepEqual(post, want) {
			t.Errorf("%s kontrol adımları: %v %v (beklenen %v)", action, post, err, want)
		}
	}
	if _, err := ChainPlan(info, "sil", ""); err != nil {
		t.Fatalf("ad onaylı yıkıcı eylem zincirlenebilmeli: %v", err)
	}
	// yıkıcı eyleme "yes-no" yazılsa da doğrulayıcı onayı sunucu adına yükseltir
	if got := manifest.ActionGet(info, "yikici_eh", "confirm"); got != "type-server-name" {
		t.Fatalf("yikici_eh onayı: %s", got)
	}
	// doğrulayıcı atlansa bile zincir reddeder
	raw := append([]string(nil), info...)
	for i, l := range raw {
		if l == "action.yikici_eh.confirm=type-server-name" {
			raw[i] = "action.yikici_eh.confirm=yes-no"
		}
	}
	if _, err := ChainPlan(raw, "yikici_eh", ""); err == nil || !strings.Contains(err.Error(), "zincirlenmez") {
		t.Fatalf("adsız yıkıcı eylem: %v", err)
	}
	for action, want := range map[string]string{"durum": "değişiklik yapan", "baska": "ayar kartının"} {
		if _, err := ChainPlan(info, action, ""); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v (beklenen %q)", action, err, want)
		}
	}
}

func TestChainRunsAllStepsWithOnePassword(t *testing.T) {
	const pw = "deneme-degeri-123"
	r := runTestChain(t, "apply", pw+"\n", "")
	names, passwords := r.ran()
	if r.rc != 0 || len(names) != 5 || names[0] != "preview" || names[1] != "apply" {
		t.Fatalf("rc=%d adımlar=%v\n%s", r.rc, names, r.out)
	}
	for i, n := range names {
		want := pw
		if n == "durum" || n == "serbest_oku" {
			want = "-" // sudo istemeyen okumaya parola verilmez
		}
		if passwords[i] != want {
			t.Errorf("%s adımına parola: %q (beklenen %q)", n, passwords[i], want)
		}
	}
	if n := strings.Count(r.out, "Sunucudaki sudo parolası"); n != 1 {
		t.Errorf("parola %d kez soruldu", n)
	}
	if !reflect.DeepEqual(r.recorded, names) {
		t.Errorf("kayıtlar %v", r.recorded)
	}
	if !strings.Contains(r.out, "(Kur düğmesiyle onaylandı)") || !strings.Contains(r.out, "✅ Tamamlandı") {
		t.Errorf("çıktı:\n%s", r.out)
	}
	// parola ekrana, günlüklere, durum dosyalarına ve komut satırına yazılmaz
	if strings.Contains(r.out, pw) {
		t.Error("parola çıktıda")
	}
	filepath.WalkDir(r.state, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, _ := os.ReadFile(p); strings.Contains(string(b), pw) {
				t.Errorf("parola dosyada: %s", p)
			}
		}
		return nil
	})
	for _, c := range r.calls {
		if strings.Contains(strings.SplitN(c, "|", 3)[2], pw) || strings.Contains(c, "--ask-become-pass") {
			t.Errorf("komut satırı: %s", c)
		}
	}
}

func TestChainStopsAtFailedStep(t *testing.T) {
	r := runTestChain(t, "apply", "x\n", "preview")
	if names, _ := r.ran(); r.rc != 4 || !reflect.DeepEqual(names, []string{"preview"}) {
		t.Fatalf("önizleme düşünce uygulama koşmamalı: rc=%d %v", r.rc, names)
	}
	if !strings.Contains(r.out, "Önizleme adımında durdu") {
		t.Errorf("çıktı:\n%s", r.out)
	}
	r = runTestChain(t, "apply", "x\n", "apply")
	if names, _ := r.ran(); r.rc != 4 || !reflect.DeepEqual(names, []string{"preview", "apply"}) {
		t.Fatalf("uygulama düşünce kontrol koşmamalı: rc=%d %v", r.rc, names)
	}
	r = runTestChain(t, "apply", "x\n", "durum")
	if r.rc != 4 || !strings.Contains(r.out, "Uygulama tamamlandı; kontrol adımı başarısız") {
		t.Fatalf("rc=%d\n%s", r.rc, r.out)
	}
}

func TestChainEmptyPasswordAndNoInput(t *testing.T) {
	r := runTestChain(t, "apply", "\n", "")
	if r.rc != 0 {
		t.Fatalf("rc=%d\n%s", r.rc, r.out)
	}
	for _, c := range r.calls {
		if strings.Contains(c, "become-pass") {
			t.Errorf("boş yanıt: parola bayrağı kalkmalı: %s", c)
		}
	}
	r = runTestChain(t, "apply", "", "")
	if r.rc != Refused || len(r.calls) != 0 {
		t.Fatalf("girdi yoksa hiçbir şey koşmamalı: rc=%d %v", r.rc, r.calls)
	}
}

func TestChainDestructiveStillAsksServerName(t *testing.T) {
	r := runTestChain(t, "sil", "x\nyanlis\nyanlis\nyanlis\n", "")
	if names, _ := r.ran(); r.rc != Refused || !reflect.DeepEqual(names, []string{"preview"}) {
		t.Fatalf("ad uyuşmadan yıkıcı eylem koşmamalı: rc=%d %v\n%s", r.rc, names, r.out)
	}
	r = runTestChain(t, "sil", "x\nsrv\n", "")
	if names, _ := r.ran(); r.rc != 0 || len(names) != 4 || names[1] != "sil" || strings.Contains(r.out, "düğmesiyle") {
		t.Fatalf("rc=%d %v\n%s", r.rc, names, r.out)
	}
}
