package servers

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSecrets: platform.Secrets'in bellekteki sahtesi (Keychain aracı gerekmeden RemoveInteractive'in depo kullanımını sınar).
type fakeSecrets struct {
	avail bool
	data  map[string]string
	calls []string
}

func (f *fakeSecrets) rec(op, a string) { f.calls = append(f.calls, op+" "+a) }
func (f *fakeSecrets) Find(a string) (string, bool) {
	f.rec("find", a)
	v, ok := f.data[a]
	return v, ok
}
func (f *fakeSecrets) Has(a string) bool { f.rec("has", a); _, ok := f.data[a]; return ok }
func (f *fakeSecrets) Delete(a string) bool {
	f.rec("delete", a)
	_, ok := f.data[a]
	delete(f.data, a)
	return ok
}
func (f *fakeSecrets) Set(a, v string) error { f.rec("set", a); f.data[a] = v; return nil }
func (f *fakeSecrets) SetInteractive(a string, _ io.Reader, _, _ io.Writer) error {
	f.rec("setint", a)
	return nil
}
func (f *fakeSecrets) Available() bool { return f.avail }

func TestRemoveInteractiveUsesSecretsOnlyWhenAvailable(t *testing.T) {
	run := func(avail bool) (*fakeSecrets, string) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "web1"), 0o700)
		fs := &fakeSecrets{avail: avail, data: map[string]string{"USBMonitor_WEB1_Knock": "7000,8000", "USBMonitor_WEB1_Passphrase": "x"}}
		var out, errb bytes.Buffer
		code := RemoveInteractive(Console{In: bufio.NewReader(strings.NewReader("web1\ny\n")), Out: &out, Err: &errb}, fs, dir, "web1")
		if code != 0 {
			t.Fatalf("çıkış %d: %s %s", code, out.String(), errb.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "web1")); err == nil {
			t.Fatal("sunucu silinmedi")
		}
		return fs, out.String()
	}
	// depo aracı VAR: knock dizisi silinir, passphrase kaydı sorulup silinir
	fs, out := run(true)
	if got := strings.Join(fs.calls, "|"); got != "delete USBMonitor_WEB1_Knock|has USBMonitor_WEB1_Passphrase|delete USBMonitor_WEB1_Passphrase" {
		t.Fatalf("çağrılar: %s", got)
	}
	if !strings.Contains(out, "Knock dizisi Keychain'den silindi") || !strings.Contains(out, "Keychain kaydı silindi") {
		t.Fatalf("çıktı: %q", out)
	}
	// depo aracı YOK (macOS dışı): depoya hiç dokunulmaz, soru sorulmaz
	fs, out = run(false)
	if len(fs.calls) != 0 || strings.Contains(out, "Keychain") {
		t.Fatalf("araç yokken depo kullanıldı: calls=%v out=%q", fs.calls, out)
	}
}
