package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repo(t *testing.T) string {
	t.Helper()
	r, _ := filepath.Abs("../../..")
	if _, err := os.Stat(filepath.Join(r, "referans/kabuk/sistem/Helpers/ComponentsReport.sh")); err != nil {
		t.Skip("kabuk motoru yok")
	}
	return r
}

var libs = []string{"Validate.sh", "ModuleFiles.sh", "SSHHelper.sh", "SSHModule.sh", "SSHModuleEdit.sh", "Components.sh", "ComponentsInstall.sh", "ComponentsManifest.sh", "ComponentsRun.sh", "ComponentsVars.sh", "ComponentsGates.sh", "Servers.sh", "ComponentsOnboard.sh", "ComponentsReport.sh"}

type out struct {
	code int
	o, e string
}

func shellReport(t *testing.T, r, comps, srvs string, args ...string) out {
	t.Helper()
	script := `export SISTEM_PATH="$1"; shift; for l in ` + strings.Join(libs, " ") + `; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; "$@"`
	cmd := exec.Command("bash", append([]string{"-c", script, "_", filepath.Join(r, "referans/kabuk/sistem"), "comp_report_pkg"}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8", "SSHPOCKET_COMPONENTS_DIR="+comps, "SSHPOCKET_SERVERS_DIR="+srvs)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	c := 0
	if ee, ok := err.(*exec.ExitError); ok {
		c = ee.ExitCode()
	}
	return out{c, so.String(), se.String()}
}

func goReport(comps, srvs string, args ...string) out { return goReportMax(0, comps, srvs, args...) }

func goReportMax(max int, comps, srvs string, args ...string) out {
	var so, se bytes.Buffer
	old := ""
	if len(args) > 1 {
		old = args[1]
	}
	c := Pkg(&so, &se, Options{ComponentsDir: comps, ServersDir: srvs, Max: max}, args[0], old)
	return out{c, so.String(), se.String()}
}

const noDiff = "  Değişken ve kural farkı yok.\n"

// kabuk hatası 1 (any bayrağı hiç kurulmaz): Go'da değişiklik varken bu satır yazılmaz; karşılaştırmadan önce kabuktaki satır atılır.
func norm(s string) string { return strings.ReplaceAll(s, noDiff, "") }

type mf = map[string]any

func baseManifest(t *testing.T, r string) mf {
	b, err := os.ReadFile(filepath.Join(r, "components/utility/0.1.0/component.json"))
	if err != nil {
		t.Skip("örnek paket yok")
	}
	var m mf
	json.Unmarshal(b, &m)
	return m
}

func clone(m mf) mf {
	b, _ := json.Marshal(m)
	var c mf
	json.Unmarshal(b, &c)
	return c
}

func install(t *testing.T, comps, id string, versions map[string]mf, current string) {
	for v, m := range versions {
		m["id"], m["version"] = id, v
		d := filepath.Join(comps, id, v)
		os.MkdirAll(d, 0o755)
		b, _ := json.MarshalIndent(m, "", "  ")
		os.WriteFile(filepath.Join(d, "component.json"), b, 0o644)
	}
	os.WriteFile(filepath.Join(comps, id, "current"), []byte(current+"\n"), 0o644)
}

func vars(m mf) []any { return m["vars"].([]any) }

func TestReportMatchesShell(t *testing.T) {
	r := repo(t)
	base := baseManifest(t, r)
	srvs := t.TempDir()
	type sc struct {
		name         string
		mut          func(m mf)
		old          string
		goOnlyChange bool // bilinçli fark: kabuk yanlış davranır; yalnızca Go çıktısı beklenenle karşılaştırılır
	}
	addVar := func(m mf, v mf) { m["vars"] = append(vars(m), v) }
	scs := []sc{
		{"fark yok", func(m mf) {}, "", false},
		{"yeni değişken varsayılanlı", func(m mf) {
			addVar(m, mf{"name": "utility_yeni", "type": "string", "default": "abc", "group": "Z", "description": "d"})
		}, "", false},
		{"yeni değişken varsayılansız", func(m mf) {
			addVar(m, mf{"name": "utility_yeni", "type": "string", "default": "", "group": "Z", "description": "d"})
		}, "", false},
		{"yeni değişken uzun varsayılan", func(m mf) {
			addVar(m, mf{"name": "utility_yeni", "type": "string", "default": strings.Repeat("ş", 60), "group": "Z", "description": "d"})
		}, "", false},
		{"kalkan değişken", func(m mf) { m["vars"] = vars(m)[1:] }, "", false},
		{"tür değişti", func(m mf) {
			vars(m)[0].(map[string]any)["type"] = "bool"
			vars(m)[0].(map[string]any)["default"] = false
		}, "", false},
		{"varsayılan değişti (dolu→dolu)", func(m mf) {
			vars(m)[0].(map[string]any)["default"] = "Europe/Berlin"
		}, "", false},
	}
	for _, s := range scs {
		comps := t.TempDir()
		oldM := clone(base)
		// eski sürümde varsayılan dolu olsun ki dolu→dolu değişim mümkün olsun
		if s.name == "varsayılan değişti (dolu→dolu)" {
			vars(oldM)[0].(map[string]any)["default"] = "Europe/Istanbul"
		}
		newM := clone(oldM)
		s.mut(newM)
		install(t, comps, "utility", map[string]mf{"0.1.0": oldM, "0.1.1": newM}, "0.1.1")
		sh := shellReport(t, r, comps, srvs, "utility", s.old)
		g := goReport(comps, srvs, "utility", s.old)
		if g.code != sh.code || norm(sh.o) != norm(g.o) && !strings.Contains(g.o, "Değişkenler:") || g.e != sh.e {
			t.Errorf("%s: kabuk=%d go=%d\nkabuk: %q\ngo:    %q", s.name, sh.code, g.code, sh.o, g.o)
		}
		// değişiklik varsa Go "farkı yok" yazmamalı; yoksa yazmalı
		changed := s.name != "fark yok"
		if changed == strings.Contains(g.o, noDiff) {
			t.Errorf("%s: 'fark yok' satırı beklenmedik: %q", s.name, g.o)
		}
		if changed && norm(sh.o) != norm(g.o) {
			t.Errorf("%s: değişiklik satırları farklı\nkabuk: %q\ngo:    %q", s.name, sh.o, g.o)
		}
		if !changed && sh.o != g.o {
			t.Errorf("%s: farklı\nkabuk: %q\ngo:    %q", s.name, sh.o, g.o)
		}
	}
}

func TestReportCapAndAnyFlag(t *testing.T) {
	r := repo(t)
	base := baseManifest(t, r)
	comps := t.TempDir()
	oldM, newM := clone(base), clone(base)
	vars(newM)[0].(map[string]any)["default"] = "Europe/Berlin" // eski varsayılan boş
	install(t, comps, "utility", map[string]mf{"0.1.0": oldM, "0.1.1": newM}, "0.1.1")
	g := goReport(comps, t.TempDir(), "utility", "")
	want := "    varsayılan değişti: utility_timezone: \"\" → \"Europe/Berlin\"\n"
	if !strings.Contains(g.o, want) {
		t.Errorf("Go doğru sütunları göstermeli:\n%s", g.o)
	}
	sh := shellReport(t, r, comps, t.TempDir(), "utility", "")
	if !strings.Contains(sh.o, want) {
		t.Errorf("varsayılan değişimi satırı kabukta farklı:\n%s", sh.o)
	}
	// 12'den fazla satır: Go sınırlar
	big := clone(base)
	for i := 0; i < 20; i++ {
		vars(big)[0] = vars(big)[0]
		big["vars"] = append(vars(big), mf{"name": fmt.Sprintf("utility_v%02d", i), "type": "string", "default": "", "group": "Z", "description": "d"})
	}
	comps = t.TempDir()
	install(t, comps, "utility", map[string]mf{"0.1.0": clone(base), "0.1.1": big}, "0.1.1")
	g = goReport(comps, t.TempDir(), "utility", "")
	if !strings.Contains(g.o, "    … ve 8 tane daha\n") || strings.Count(g.o, "yeni değişken") != 12 {
		t.Errorf("sınır uygulanmadı:\n%s", g.o)
	}
}

func TestReportEdgeCasesMatchShell(t *testing.T) {
	r := repo(t)
	base := baseManifest(t, r)
	srvs := t.TempDir()
	// kurulu değil
	comps := t.TempDir()
	sh, g := shellReport(t, r, comps, srvs, "utility"), goReport(comps, srvs, "utility")
	if sh != g {
		t.Errorf("kurulu değil: %+v %+v", sh, g)
	}
	// tek sürüm
	install(t, comps, "utility", map[string]mf{"0.1.0": clone(base)}, "0.1.0")
	sh, g = shellReport(t, r, comps, srvs, "utility"), goReport(comps, srvs, "utility")
	if sh != g {
		t.Errorf("tek sürüm: %+v %+v", sh, g)
	}
	// eski == yeni
	sh, g = shellReport(t, r, comps, srvs, "utility", "0.1.0"), goReport(comps, srvs, "utility", "0.1.0")
	if sh != g {
		t.Errorf("aynı sürüm: %+v %+v", sh, g)
	}
	// eski bildirim bozuk / yok
	install(t, comps, "utility", map[string]mf{"0.1.1": clone(base)}, "0.1.1")
	os.WriteFile(filepath.Join(comps, "utility", "0.1.0", "component.json"), []byte("{bozuk"), 0o644)
	sh, g = shellReport(t, r, comps, srvs, "utility"), goReport(comps, srvs, "utility")
	if sh != g {
		t.Errorf("bozuk eski: %+v %+v", sh, g)
	}
	sh, g = shellReport(t, r, comps, srvs, "utility", "0.0.9"), goReport(comps, srvs, "utility", "0.0.9")
	if sh != g {
		t.Errorf("olmayan eski: %+v %+v", sh, g)
	}
}

func TestReportRealPackagesMatchShell(t *testing.T) {
	r := repo(t)
	src := filepath.Join(r, "components/linux-hardening")
	comps := t.TempDir()
	for _, v := range []string{"0.12.0", "0.12.1", "0.13.0"} {
		b, err := os.ReadFile(filepath.Join(src, v, "component.json"))
		if err != nil {
			t.Skip("gerçek paket yok")
		}
		os.MkdirAll(filepath.Join(comps, "linux-hardening", v), 0o755)
		os.WriteFile(filepath.Join(comps, "linux-hardening", v, "component.json"), b, 0o644)
	}
	os.WriteFile(filepath.Join(comps, "linux-hardening", "current"), []byte("0.13.0\n"), 0o644)
	// sunucular: değişkenleri eksik / dolu
	srvs := t.TempDir()
	mk := func(n, vars string) {
		os.MkdirAll(filepath.Join(srvs, n), 0o700)
		os.WriteFile(filepath.Join(srvs, n, "server.env"), []byte("IP=1.1.1.1\nUSER=u\n"), 0o600)
		if vars != "" {
			os.WriteFile(filepath.Join(srvs, n, "linux-hardening.vars"), []byte(vars), 0o600)
		}
	}
	mk("bos", "hardening_x=1\n")
	mk("yok", "")
	mk("dolu", "hardening_platform=ubuntu\nhardening_admin_user=u\n")
	for _, old := range []string{"", "0.12.0", "0.12.1"} {
		sh, g := shellReport(t, r, comps, srvs, "linux-hardening", old), goReportMax(1000, comps, srvs, "linux-hardening", old)
		if sh.code != g.code || sh.e != g.e || norm(sh.o) != norm(g.o) {
			t.Errorf("eski=%q:\nkabuk: %q\ngo:    %q", old, sh.o, g.o)
		}
	}
}
