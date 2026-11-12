package eject

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/shellgold"
)

// Sahte araçlar: ssh-add -d yalnızca FAKE_DEL_OK içindeki .pub adlarında başarılı; diskutil davranışı FAKE_* ile seçilir.
func fakeBin(t *testing.T, withDiskutil bool) string {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "ssh-add"), []byte("#!/bin/bash\necho \"ssh-add $*\" >> \"$FAKE_LOG\"\ncase \"$FAKE_DEL_OK\" in *\"$(basename \"$2\")\"*) exit 0;; esac\nexit 1\n"), 0o755)
	if withDiskutil {
		os.WriteFile(filepath.Join(d, "diskutil"), []byte(`#!/bin/bash
echo "diskutil $*" >> "$FAKE_LOG"
case "$1" in
  info) echo "   Device Identifier:         disk9s1"; echo "   Device Node:  /dev/disk9s1" ;;
  eject) [ "${FAKE_EJECT_OK:-1}" = 1 ] && [ "$2" = disk9s1 ] && exit 0 || exit 1 ;;
  unmount) [ "${FAKE_UNMOUNT_OK:-1}" = 1 ] && exit 0 || exit 1 ;;
esac
`), 0o755)
	}
	return d
}

func mkUSB(t *testing.T) string {
	u := filepath.Join(t.TempDir(), "SSHUSB")
	for _, p := range []string{"servers/web1/id_ed25519_web1.pub", "servers/web1/baska.pub", "servers/db/id_ed25519_db.pub", "servers/db/notpub.txt", "servers/.gizli/x.pub"} {
		os.MkdirAll(filepath.Dir(filepath.Join(u, p)), 0o755)
		os.WriteFile(filepath.Join(u, p), []byte("ssh-ed25519 AAAA\n"), 0o644)
	}
	return u
}

func TestEjectMatchesShell(t *testing.T) {
	root, _ := filepath.Abs("../../..")
	if os.Getenv("KEYLAB_KAYIT_KABUK") == "1" { // kayıt: kabuk motoru gerekir; oynatmada gerekmez (altın dosya)
		if _, err := os.Stat(filepath.Join(root, "referans/kabuk/sistem/Helpers/Eject.sh")); err != nil {
			t.Skip("kabuk motoru yok")
		}
	}
	gold := shellgold.Open(t, "shell")
	type eg struct {
		Out  string
		Code int
	}
	type tc struct {
		name        string
		diskutil    bool
		delOK       string
		eject, umnt string
		noUSB       bool
	}
	for _, c := range []tc{
		{"hepsi tamam", true, "id_ed25519_web1.pub id_ed25519_db.pub", "1", "1", false},
		{"anahtar yok ajanda", true, "", "1", "1", false},
		{"eject başarısız, force unmount tamam", true, "id_ed25519_web1.pub", "0", "1", false},
		{"ikisi de başarısız", true, "id_ed25519_web1.pub", "0", "0", false},
		{"diskutil yok", false, "id_ed25519_web1.pub", "1", "1", false},
		{"usb yok", true, "", "1", "1", true},
	} {
		bin := fakeBin(t, c.diskutil)
		env := []string{"PATH=" + bin + ":/usr/bin:/bin", "FAKE_DEL_OK=" + c.delOK, "FAKE_EJECT_OK=" + c.eject, "FAKE_UNMOUNT_OK=" + c.umnt, "HOME=" + t.TempDir()}
		run := func(isShell bool) (string, string, int) {
			usb := mkUSB(t)
			if c.noUSB {
				usb = filepath.Join(t.TempDir(), "yok")
			}
			log := filepath.Join(t.TempDir(), "log")
			e := append(append([]string{}, env...), "FAKE_LOG="+log)
			var out string
			code := 0
			if isShell {
				script := `SISTEM_PATH="$1"; for l in SSHHelper.sh Eject.sh; do . "$SISTEM_PATH/Helpers/$l"; done; Log() { :; }; EjectUSB "$2"`
				cmd := exec.Command("/bin/bash", "-c", script, "_", filepath.Join(root, "referans/kabuk/sistem"), usb)
				cmd.Env = e
				var b bytes.Buffer
				cmd.Stdout, cmd.Stderr = &b, &b
				if err := cmd.Run(); err != nil {
					if ee, ok := err.(*exec.ExitError); ok {
						code = ee.ExitCode()
					}
				}
				out = b.String()
			} else {
				old := os.Environ()
				os.Clearenv()
				for _, kv := range e {
					k, v, _ := strings.Cut(kv, "=")
					os.Setenv(k, v)
				}
				wd, _ := os.Getwd()
				var b bytes.Buffer
				code = Run(&b, usb)
				os.Chdir(wd)
				os.Clearenv()
				for _, kv := range old {
					k, v, _ := strings.Cut(kv, "=")
					os.Setenv(k, v)
				}
				out = b.String()
			}
			lg, _ := os.ReadFile(log)
			norm := func(s string) string { return strings.ReplaceAll(s, usb, "<USB>") }
			return norm(out) + "|LOG|" + norm(string(lg)), "", code
		}
		sg := shellgold.Do(t, gold, c.name, func() eg { o, _, c := run(true); return eg{o, c} })
		so, sc := sg.Out, sg.Code
		go_, _, gc := run(false)
		if sc != gc || so != go_ {
			t.Errorf("%s: kabuk=%d go=%d\nkabuk: %q\ngo:    %q", c.name, sc, gc, so, go_)
		}
	}
}
