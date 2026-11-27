package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"keylab/internal/manifest"
)

const pvePkg = `{
 "id":"demo","version":"0.1.0","sshpocket_api":1,"runner":"ansible","inventory_group":"demo","name":"Demo","description":"d",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "requires":{"control":{"python":">=3.12","packages":["ansible-core"]},"target":{"python":">=3.9"}},
 "platform_var":"demo_platform",
 "platform_choices":[{"id":"pve","label":"PVE","description":"d","apply_vars":{"demo_gercek":true},"apply_warnings":["hipervizoru degistirir"]}],
 "actions":{
  "preview":{"cmd":"ansible-playbook playbooks/preview.yml --check","effects":[],"confirm":"none","warnings":["w"]},
  "eh":{"cmd":"ansible-playbook playbooks/eh.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]},
  "adli":{"cmd":"ansible-playbook playbooks/adli.yml","effects":["changes-config"],"requires":["preview"],"confirm":"type-server-name","warnings":["w"]},
  "kartli":{"cmd":"ansible-playbook playbooks/kartli.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no","warnings":["w"]}
 },
 "vars":[
  {"name":"demo_platform","type":"enum","values":["auto","pve"],"default":"auto","group":"P","description":"x"},
  {"name":"demo_gercek","type":"bool","default":false,"group":"P","description":"y"},
  {"name":"demo_zone","type":"string","default":"","group":"P","description":"z"}
 ],
 "settings":[{"id":"zone","label":"Z","var":"demo_zone","description":"d","apply_via":["kartli","adli"]}]}`

// Seçili platformun gerçek-değişiklik değişkeni yalnız terminalde verilen onaydan sonra geçer: e/h ya da sunucu adı; kartın "Kur" düğmesi yetmez.
func TestPlatformApplyVarsNeedAskedConfirmation(t *testing.T) {
	const notice = "demo_gercek="
	for _, c := range []struct {
		action, stdin string
		ran, passed   bool
	}{
		{"eh", "e\n", true, true},
		{"eh", "\n", false, false},
		{"adli", "srv\n", true, true},
		{"kartli", "", true, false}, // düğme onaylar: platform değişkeni geçmez (paket gerçek koşuyu kendisi durdurur)
	} {
		w := newWorld(t)
		os.WriteFile(filepath.Join(w.bin, "ansible-playbook"), []byte(chainAnsible), 0o755)
		log := filepath.Join(t.TempDir(), "chain.log")
		t.Setenv("CHAIN_LOG", log)
		t.Setenv("CHAIN_FAIL", "")
		pkg := t.TempDir()
		os.WriteFile(filepath.Join(pkg, "component.json"), []byte(pvePkg), 0o600)
		info, errs, code := manifest.ValidateFile(filepath.Join(pkg, "component.json"), "demo", "0.1.0")
		if code != 0 {
			t.Fatalf("paket geçersiz: %v", errs)
		}
		vars := filepath.Join(t.TempDir(), "demo.vars")
		os.WriteFile(vars, []byte("demo_platform=pve\n"), 0o600)
		var out bytes.Buffer
		ec := &ExecCtx{PkgDir: pkg, BinDir: w.bin, Server: "srv", Host: "10.0.0.5", User: "deploy", Port: "22", Key: w.key, Prefix: "SRV", Profile: "ubuntu-26.04",
			ValuesFile: vars, Gates: &Gates{StateDir: t.TempDir()}, KnownHosts: w.known, Stdin: strings.NewReader(c.stdin), Stdout: &out, Stderr: &out, Ctx: context.Background()}
		plan, err := ChainPlan(info, c.action, "")
		if err != nil {
			t.Fatal(err)
		}
		ec.runChain(info, "demo", "0.1.0", plan, func(a string, r ExecResult) {
			f, _ := os.OpenFile(filepath.Join(ec.Gates.StateDir, "kayit"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			f.Close()
		})
		b, _ := os.ReadFile(log)
		ran := strings.Contains(string(b), c.action+"|")
		if ran != c.ran || strings.Contains(out.String(), notice) != c.passed && c.ran {
			t.Errorf("%s %q: koştu=%v (beklenen %v), değişken bildirimi beklenen=%v\n%s", c.action, c.stdin, ran, c.ran, c.passed, out.String())
		}
		if c.passed && strings.Index(out.String(), notice) > strings.LastIndex(out.String(), "adim "+c.action+" calisti") {
			t.Errorf("%s: bildirim onaydan önce gösterilmeli\n%s", c.action, out.String())
		}
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "demo.vars"), []byte("demo_platform=pve\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "component.json"), []byte(pvePkg), 0o600)
	info, _, _ := manifest.ValidateFile(filepath.Join(dir, "component.json"), "demo", "0.1.0")
	for action, want := range map[string][]string{"eh": {"demo_gercek=true"}, "adli": {"demo_gercek=true"}, "preview": nil} {
		if got := ApplyVars(info, action, filepath.Join(dir, "demo.vars")); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v (beklenen %v)", action, got, want)
		}
	}
}
