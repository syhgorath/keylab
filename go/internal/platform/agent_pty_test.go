package platform

import (
	"runtime"
	"strings"
	"testing"
)

func TestAgentArgv(t *testing.T) {
	if got := strings.Join((KeychainAgent{}).AddKeyArgv("/k ey"), "\x00"); got != "ssh-add\x00--apple-use-keychain\x00/k ey" {
		t.Errorf("macOS ajan komutu: %q", got)
	}
	if got := strings.Join((OpenSSHAgent{}).AddKeyArgv("/k ey"), "\x00"); got != "ssh-add\x00/k ey" {
		t.Errorf("düz ssh-add: %q", got)
	}
	_, mac := DefaultAgent().(KeychainAgent)
	if mac != (runtime.GOOS == "darwin") {
		t.Errorf("DefaultAgent işletim sistemine uymuyor (%s): %T", runtime.GOOS, DefaultAgent())
	}
}

func TestPTYWrap(t *testing.T) {
	if got := strings.Join((BSDScriptPTY{}).Wrap([]string{"ssh-add", "/k ey"}), "\x00"); got != "script\x00-q\x00/dev/null\x00ssh-add\x00/k ey" {
		t.Errorf("BSD script: %q", got)
	}
	// Linux: tek komut dizesi, her parça tek tırnakla (içindeki tek tırnak kaçışlı)
	got := (UtilLinuxScriptPTY{}).Wrap([]string{"ssh", "-o", "X=it's", "a b"})
	if len(got) != 4 || got[0] != "script" || got[1] != "-qfec" || got[3] != "/dev/null" || got[2] != `'ssh' '-o' 'X=it'\''s' 'a b'` {
		t.Errorf("util-linux script: %q", got)
	}
	// Windows: pty yok
	if (WindowsPTY{}).TerminalSupported() || strings.Join((WindowsPTY{}).Wrap([]string{"a"}), "") != "a" {
		t.Error("Windows pty davranışı")
	}
	if !(BSDScriptPTY{}).TerminalSupported() || !(UtilLinuxScriptPTY{}).TerminalSupported() {
		t.Error("Unix'te gömülü terminal desteklenmeli")
	}
	switch runtime.GOOS {
	case "darwin":
		if _, ok := DefaultPTY().(BSDScriptPTY); !ok {
			t.Errorf("macOS DefaultPTY: %T", DefaultPTY())
		}
	case "windows":
		if _, ok := DefaultPTY().(WindowsPTY); !ok {
			t.Errorf("Windows DefaultPTY: %T", DefaultPTY())
		}
	default:
		if _, ok := DefaultPTY().(UtilLinuxScriptPTY); !ok {
			t.Errorf("Linux DefaultPTY: %T", DefaultPTY())
		}
	}
}

func TestSupportFlags(t *testing.T) {
	mac := runtime.GOOS == "darwin"
	if SecretsSupported() != mac || DefaultBrowser().Supported() != mac {
		t.Errorf("destek bayrakları (%s): secrets=%v browser=%v", runtime.GOOS, SecretsSupported(), DefaultBrowser().Supported())
	}
}
