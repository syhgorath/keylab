package runner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"sshpocket-go/internal/manifest"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, _ := filepath.Abs("../../..")
	if _, err := os.Stat(filepath.Join(root, "referans/kabuk/sistem/Helpers/ComponentsGates.sh")); err != nil {
		t.Skip("kabuk motoru yok")
	}
	return root
}

type shOut struct {
	code           int
	stdout, stderr string
}

var gateLibs = []string{"Components.sh", "ComponentsInstall.sh", "ComponentsManifest.sh", "ComponentsRun.sh", "ComponentsVars.sh", "ComponentsGates.sh", "ComponentsExec.sh", "ComponentsRunCLI.sh", "ComponentsVenv.sh", "Validate.sh", "ModuleFiles.sh", "SSHModule.sh", "Servers.sh"}

// runShell: kütüphaneleri yükler, PRELUDE'u çalıştırır, işlevi çağırır; stdin verilebilir.
func runShell(t *testing.T, root string, env []string, stdin, prelude, fn string, args ...string) shOut {
	t.Helper()
	script := `export SISTEM_PATH="$1"; shift; for l in $LIBS; do . "$SISTEM_PATH/Helpers/$l" || exit 99; done; eval "$PRELUDE"; "$@"`
	cmd := exec.Command("bash", append([]string{"-c", script, "_", filepath.Join(root, "referans/kabuk/sistem"), fn}, args...)...)
	cmd.Env = append(os.Environ(), append(env, "LIBS="+strings.Join(gateLibs, " "), "PRELUDE="+prelude)...)
	cmd.Stdin = strings.NewReader(stdin)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return shOut{code, so.String(), se.String()}
}

func infoOf(t *testing.T, p string) []string {
	lines, errs, code := manifest.ValidateFile(p, "", "")
	if code != 0 {
		t.Fatalf("%s: %v", p, errs)
	}
	return lines
}

func realInfos(t *testing.T, root string) map[string][]string {
	m, _ := filepath.Glob(filepath.Join(root, "components/*/*/component.json"))
	out := map[string][]string{}
	for _, p := range m {
		rel, _ := filepath.Rel(filepath.Join(root, "components"), filepath.Dir(p))
		out[rel] = infoOf(t, p)
	}
	if len(out) == 0 {
		t.Skip("örnek paket yok")
	}
	return out
}

func errStr(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

func TestStatelessGatesMatchShell(t *testing.T) {
	root := repoRoot(t)
	for rel, info := range realInfos(t, root) {
		joined := strings.Join(info, "\n")
		for _, a := range manifest.InfoAll(info, "action") {
			label := rel + " " + a
			// NotRoot
			for _, user := range []string{"root", "deploy"} {
				sh := runShell(t, root, nil, "", "", "comp_gate_not_root", joined, a, user)
				if err := NotRoot(info, a, user); (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) {
					t.Errorf("%s not_root %s: kabuk=%d %q go=%v", label, user, sh.code, sh.stderr, err)
				}
			}
			// WarnApplies / RequiredApplies
			sh := runShell(t, root, nil, "", "", "comp_gate_warn_applies", joined, a)
			if (sh.code == 0) != WarnApplies(info, a) {
				t.Errorf("%s warn_applies", label)
			}
			sh = runShell(t, root, nil, "", "", "comp_gate_required_applies", joined, a)
			if (sh.code == 0) != RequiredApplies(info, a) {
				t.Errorf("%s required_applies", label)
			}
		}
		// Profile
		for _, d := range []string{"", "ubuntu-26.04", "debian-13", "arch-1", "ubuntu-24.04"} {
			sh := runShell(t, root, nil, "", "", "comp_gate_profile", joined, d)
			if err := Profile(info, d); (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) {
				t.Errorf("%s profile %q: kabuk=%d %q go=%v", rel, d, sh.code, sh.stderr, err)
			}
		}
	}
}

func TestConfirmMatchesShell(t *testing.T) {
	root := repoRoot(t)
	infos := realInfos(t, root)
	answers := []string{"", "e\n", "E\n", "evet\n", "Evet\n", "hayır\n", "h\n", "evett\n", "srv\n", "x\nsrv\n", "x\ny\nsrv\n", "x\ny\nz\n", "srv", "\n"}
	seen := map[string]bool{}
	for rel, info := range infos {
		for _, a := range manifest.InfoAll(info, "action") {
			mode := manifest.ActionGet(info, a, "confirm")
			if seen[rel+mode] && len(seen) > 6 {
				continue // her paket+kip için bir eylem yeter
			}
			seen[rel+mode] = true
			for _, ans := range answers {
				sh := runShell(t, root, nil, ans, "", "comp_gate_confirm", strings.Join(info, "\n"), a, "srv")
				var out bytes.Buffer
				rd := strings.NewReader(ans)
				ask := func(p string) (string, bool) {
					out.WriteString(p)
					line, err := readLine(rd)
					if err != nil {
						out.WriteString("\n")
						return "", false
					}
					return line, true
				}
				err := Confirm(info, a, "srv", ask, &out)
				got := out.String()
				if err != nil {
					got += err.Error() + "\n"
				}
				if (sh.code == 0) != (err == nil) || sh.stderr != got {
					t.Errorf("%s %s [%s] girdi %q:\n kabuk(%d)=%q\n go(%v)   =%q", rel, a, mode, ans, sh.code, sh.stderr, err == nil, got)
				}
			}
		}
	}
}

// readLine, bash `read -r` gibi: satır sonuna kadar okur; hiç bayt yoksa ya da satır sonsuz kalan parça varsa hata (EOF).
func readLine(r *strings.Reader) (string, error) {
	var sb strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			return sb.String(), fmt.Errorf("eof")
		}
		if b == '\n' {
			return sb.String(), nil
		}
		sb.WriteByte(b)
	}
}

func TestWarnVarsMatchesShell(t *testing.T) {
	root := repoRoot(t)
	for _, in := range []string{"", "e\n", "h\n", "e\ne\n", "e\nh\n"} {
		for _, names := range [][]string{nil, {"a"}, {"a", "b"}} {
			sh := runShell(t, root, nil, in, "", "comp_gate_warn_vars", names...)
			var out bytes.Buffer
			rd := strings.NewReader(in)
			err := WarnVars(names, func(p string) (string, bool) {
				out.WriteString(p)
				l, e := readLine(rd)
				if e != nil {
					out.WriteString("\n")
					return "", false
				}
				return l, true
			})
			got := out.String()
			if err != nil {
				got += err.Error() + "\n"
			}
			if (sh.code == 0) != (err == nil) || sh.stderr != got {
				t.Errorf("%q %v: kabuk(%d)=%q go(%v)=%q", in, names, sh.code, sh.stderr, err == nil, got)
			}
		}
	}
}

func TestVarGatesMatchShell(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	for rel, info := range realInfos(t, root) {
		joined := strings.Join(info, "\n")
		pv := manifest.InfoGet(info, "platform_var")
		var plats []string
		for _, c := range manifest.InfoAll(info, "platform_choice") {
			plats = append(plats, c)
		}
		contents := []string{"\x00", "", "x=1\n"}
		for _, p := range plats {
			contents = append(contents, fmt.Sprintf("%s=%s\n", pv, p))
			// platform + bazı requires_vars dolu
			contents = append(contents, fmt.Sprintf("%s=%s\nhardening_ssh_allow_from=10.0.0.0/24\nhardening_pve_apply=true\n", pv, p))
		}
		for ci, c := range contents {
			vf := filepath.Join(dir, fmt.Sprintf("v%d.vars", ci))
			os.Remove(vf)
			if c != "\x00" {
				os.WriteFile(vf, []byte(c), 0o600)
			}
			for _, a := range manifest.InfoAll(info, "action") {
				label := fmt.Sprintf("%s %s vars#%d", rel, a, ci)
				sh := runShell(t, root, nil, "", "", "comp_gate_requires_vars", joined, a, vf)
				err := RequiresVars(info, a, vf)
				if (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) {
					t.Errorf("%s requires_vars: kabuk=%d %q go=%v", label, sh.code, sh.stderr, err)
				}
				sh = runShell(t, root, nil, "", "", "comp_gate_apply_vars", joined, a, vf)
				got := ApplyVars(info, a, vf)
				if want := splitNonEmpty(sh.stdout); !reflect.DeepEqual(want, got) {
					t.Errorf("%s apply_vars: kabuk=%q go=%q", label, want, got)
				}
				if len(got) > 0 {
					shn := runShell(t, root, nil, "", "", "comp_gate_apply_vars_notice", append([]string{joined, vf}, got...)...)
					var out bytes.Buffer
					ApplyVarsNotice(info, vf, got, &out)
					if shn.stderr != out.String() {
						t.Errorf("%s apply_vars_notice:\n kabuk=%q\n go   =%q", label, shn.stderr, out.String())
					}
				}
			}
		}
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestRunSetsMatchesShell(t *testing.T) {
	root := repoRoot(t)
	info := infoOf(t, filepath.Join(root, "components/kurulum/0.1.0/component.json"))
	joined := strings.Join(info, "\n")
	iv, mv := manifest.InfoGet(info, "item_var"), manifest.InfoGet(info, "mode_var")
	items := manifest.InfoAll(info, "item")
	if iv == "" || len(items) == 0 {
		t.Skip("kurulum paketinde katalog yok")
	}
	first := strings.SplitN(items[0], "|", 2)[0]
	var sets []string
	sets = append(sets, "", iv+"="+first, iv+"=yokyazilim", mv+"=x", iv+"="+first+"\n"+mv+"=zz", "\nfoo=bar", iv+"="+first+"\nfoo=bar", iv+"="+first+"\n"+iv+"=baska")
	if mv != "" {
		modes := manifest.InfoGet(info, "item."+first+".modes")
		for _, m := range strings.Split(modes, ",") {
			if m != "" {
				sets = append(sets, iv+"="+first+"\n"+mv+"="+m)
			}
		}
	}
	if pp := manifest.InfoGet(info, "item."+first+".params"); pp != "" {
		for _, p := range strings.Split(pp, ",") {
			sets = append(sets, iv+"="+first+"\n"+p+"=1", p+"=1")
		}
	}
	for _, a := range manifest.InfoAll(info, "action") {
		for _, rs := range sets {
			sh := runShell(t, root, []string{"COMP_CTX_RUNSET=" + rs}, "", "", "comp_gate_run_sets", joined, a)
			got, err := RunSets(info, a, rs)
			if (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) ||
				(err == nil && !reflect.DeepEqual(splitNonEmpty(sh.stdout), got)) {
				t.Errorf("%s %q: kabuk(%d) out=%q err=%q | go out=%q err=%v", a, rs, sh.code, sh.stdout, sh.stderr, got, err)
			}
		}
	}
}

func TestStateRecordRequiresMatchShell(t *testing.T) {
	root := repoRoot(t)
	state := t.TempDir()
	env := []string{"SSHPOCKET_STATE_DIR=" + state}
	g := &Gates{StateDir: state}
	info := infoOf(t, filepath.Join(root, "components/utility/0.1.0/component.json"))
	joined := strings.Join(info, "\n")

	// Record geçerli/geçersiz girdiler: yan yana, ayrı durum klasörleri
	stateSh, stateGo := t.TempDir(), t.TempDir()
	gg := &Gates{StateDir: stateGo}
	for _, c := range [][5]string{
		{"utility", "srv", "0.1.0", "preview", "ok"}, {"utility", "srv", "0.1.0", "apply", "fail"}, {"UTILITY", "srv", "0.1.0", "x", "ok"},
		{"utility", "-bad", "0.1.0", "x", "ok"}, {"utility", "srv", "0.1", "x", "ok"}, {"utility", "srv", "0.1.0", "X", "ok"}, {"utility", "srv", "0.1.0", "x", "maybe"},
	} {
		sh := runShell(t, root, []string{"SSHPOCKET_STATE_DIR=" + stateSh}, "", "", "comp_gate_record", c[0], c[1], c[2], c[3], c[4])
		err := gg.Record(c[0], c[1], c[2], c[3], c[4])
		if (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) {
			t.Errorf("record %v: kabuk=%d %q go=%v", c, sh.code, sh.stderr, err)
		}
	}
	norm := func(root string) map[string]string {
		out := map[string]string{}
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			rel, _ := filepath.Rel(root, p)
			st, _ := os.Stat(p)
			if d.IsDir() {
				out[rel+"/"] = fmt.Sprintf("%o", st.Mode().Perm())
				return nil
			}
			b, _ := os.ReadFile(p)
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			for i, l := range lines { // epoch'u at
				if f := strings.SplitN(l, "|", 2); len(f) == 2 {
					lines[i] = "T|" + f[1]
				}
			}
			out[rel] = fmt.Sprintf("%o %v", st.Mode().Perm(), lines)
			return nil
		})
		return out
	}
	if a, b := norm(stateSh), norm(stateGo); !reflect.DeepEqual(a, b) {
		t.Errorf("durum klasörü farkı\n kabuk=%v\n go   =%v", a, b)
	}

	// requires / age: farklı zaman damgaları
	now := time.Now().Unix()
	rdir := filepath.Join(state, "utility", "srv")
	os.MkdirAll(rdir, 0o700)
	row := func(ts int64, ver, act, res string) string { return fmt.Sprintf("%d|%s|%s|%s\n", ts, ver, act, res) }
	cases := map[string]string{
		"taze ok":        row(now-10, "0.1.0", "preview", "ok"),
		"bayat":          row(now-2000, "0.1.0", "preview", "ok"),
		"tam sınır":      row(now-1800, "0.1.0", "preview", "ok"),
		"sınır+1":        row(now-1801, "0.1.0", "preview", "ok"),
		"başarısız":      row(now-10, "0.1.0", "preview", "fail"),
		"başka sürüm":    row(now-10, "0.2.0", "preview", "ok"),
		"başka eylem":    row(now-10, "0.1.0", "durum", "ok"),
		"gelecek 30 sn":  row(now+30, "0.1.0", "preview", "ok"),
		"gelecek 120 sn": row(now+120, "0.1.0", "preview", "ok"),
		"bozuk damga":    "abc|0.1.0|preview|ok\n",
		"satır sonu yok": fmt.Sprintf("%d|0.1.0|preview|ok", now-10),
		"karışık":        row(now-5000, "0.1.0", "preview", "ok") + row(now-20, "0.1.0", "preview", "fail") + row(now-15, "0.1.0", "preview", "ok"),
		"fazla alan":     fmt.Sprintf("%d|0.1.0|preview|ok|ekstra\n", now-10),
		"boş":            "",
	}
	for name, content := range cases {
		os.WriteFile(filepath.Join(rdir, "results"), []byte(content), 0o600)
		for _, act := range []string{"apply", "preview"} {
			sh := runShell(t, root, env, "", "", "comp_gate_requires", joined, "utility", "srv", "0.1.0", act)
			err := g.Requires(info, "utility", "srv", "0.1.0", act)
			if (sh.code == 0) != (err == nil) || (err != nil && strings.TrimSpace(sh.stderr) != err.Error()) {
				t.Errorf("requires [%s] %s: kabuk=%d %q go=%v", name, act, sh.code, sh.stderr, err)
			}
		}
		for _, act := range []string{"preview", "durum"} {
			sh := runShell(t, root, env, "", "", "comp_gate_requires_age", "utility", "srv", "0.1.0", act)
			age, ok := g.RequiresAge("utility", "srv", "0.1.0", act)
			if (sh.code == 0) != ok {
				t.Errorf("age [%s] %s: kabuk=%d %q go=%v", name, act, sh.code, sh.stdout, ok)
			} else if ok {
				var shAge int64
				fmt.Sscan(sh.stdout, &shAge)
				if d := shAge - age; d < -2 || d > 2 {
					t.Errorf("age [%s] %s: kabuk=%d go=%d", name, act, shAge, age)
				}
			}
		}
	}
	// geçersiz sunucu adı
	if err := g.Requires(info, "utility", "-bad", "0.1.0", "apply"); err == nil || err.Error() != "geçersiz sunucu adı" {
		t.Errorf("geçersiz sunucu: %v", err)
	}
}
