package runner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Sahte ssh-add: -l ajan dosyasını basar (boşsa 1); ekleme SSH_ASKPASS'ı çalıştırıp çıktısı "gizli" ise anahtarı ajana yazar.
const fakeAddScript = `#!/bin/bash
A="$FAKE_AGENT_FILE"
if [ "$1" = "-l" ]; then
  [ "${FAKE_NO_AGENT:-}" = 1 ] && exit 2
  [ -s "$A" ] && { cat "$A"; exit 0; }; echo "The agent has no identities." ; exit 1
fi
echo "ssh-add $*" >> "$FAKE_LOG"
pw="$("$SSH_ASKPASS" "Enter passphrase:")"
echo "askpass-cevap=[$pw]" >> "$FAKE_LOG"
if [ "$pw" = "gizli" ]; then ssh-keygen -lf "$1" > "$A"; exit 0; fi
# yanlış parola: gerçek ssh-add boş cevap alana kadar sorar
pw2="$("$SSH_ASKPASS" "Enter passphrase:")"
echo "askpass-ikinci=[$pw2]" >> "$FAKE_LOG"
exit 1
`

// Sahte security: depo dosyası; find → yazdırır, add → "-w" için stdin'den okur.
const fakeSecScript = `#!/bin/bash
S="$FAKE_KEYCHAIN"
case "$1" in
  find-generic-password) [ -s "$S" ] && { cat "$S"; exit 0; }; exit 44 ;;
  add-generic-password) echo "sec $*" >> "$FAKE_LOG"; read -r v; printf '%s\n' "$v" > "$S"; exit 0 ;;
esac
exit 1
`

func agentEnv(t *testing.T, keychain string) (priv, logf, agentf string) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ssh-add"), []byte(fakeAddScript), 0o755)
	os.WriteFile(filepath.Join(dir, "security"), []byte(fakeSecScript), 0o755)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	logf = filepath.Join(dir, "log")
	agentf = filepath.Join(dir, "agent")
	kc := filepath.Join(dir, "kc")
	if keychain != "" {
		os.WriteFile(kc, []byte(keychain+"\n"), 0o600)
	}
	t.Setenv("FAKE_LOG", logf)
	t.Setenv("FAKE_AGENT_FILE", agentf)
	t.Setenv("FAKE_KEYCHAIN", kc)
	t.Setenv("FAKE_NO_AGENT", "")
	priv, _ = realKey(t, dir)
	return
}

// Test ikilisi kendisi askpass olarak çağrılır.
func TestMain(m *testing.M) {
	if h, code := AskpassMain(os.Stdout); h {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func adder(answers ...string) (*AgentAdder, *bytes.Buffer) {
	var errb bytes.Buffer
	i := 0
	a := &AgentAdder{Err: &errb, Ask: func(p string) (string, bool) {
		errb.WriteString(p)
		if i >= len(answers) {
			return "", false
		}
		i++
		return answers[i-1], true
	}}
	a.Keyring = func(args ...string) *exec.Cmd {
		c := exec.Command("security", args...)
		return c
	}
	return a, &errb
}

func TestAddRetryUsesKeychainOnce(t *testing.T) {
	priv, logf, agentf := agentEnv(t, "gizli")
	a, errb := adder()
	if err := a.AddRetry("WEB1", priv); err != nil {
		t.Fatalf("eklenemedi: %v %s", err, errb)
	}
	if b, _ := os.ReadFile(agentf); !strings.Contains(string(b), "SHA256:") {
		t.Fatalf("ajana yazılmadı: %q", b)
	}
	lg, _ := os.ReadFile(logf)
	if !strings.Contains(string(lg), "askpass-cevap=[gizli]") {
		t.Errorf("askpass parolayı vermedi: %s", lg)
	}
	if strings.Contains(errb.String(), "gizli") {
		t.Errorf("parola iletilere sızdı: %s", errb)
	}
}

func TestAddRetryWrongPassphraseAsksOnceThenStops(t *testing.T) {
	priv, logf, _ := agentEnv(t, "yanlis")
	// 'h' → Keychain'e dokunmaz
	a, _ := adder("h")
	if err := a.AddRetry("WEB1", priv); err == nil {
		t.Fatal("yanlış parolada başarı döndü")
	}
	lg, _ := os.ReadFile(logf)
	if !strings.Contains(string(lg), "askpass-ikinci=[]") {
		t.Errorf("askpass ikinci çağrıda boş dönmedi (ssh-add sonsuz sorar): %s", lg)
	}
	if strings.Contains(string(lg), "sec add-generic-password") {
		t.Errorf("'h' yanıtında Keychain'e yazıldı: %s", lg)
	}
}

func TestAddRetryFixesKeychainWithE(t *testing.T) {
	priv, logf, agentf := agentEnv(t, "yanlis")
	a, errb := adder("e")
	// security -w: stdin'den okunur (gerçekte terminal); testte stdin'e doğru parola ver
	r, w, _ := os.Pipe()
	w.WriteString("gizli\n")
	w.Close()
	oldin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldin }()
	if err := a.AddRetry("WEB1", priv); err != nil {
		t.Fatalf("düzeltme sonrası eklenemedi: %v\n%s", err, errb)
	}
	if b, _ := os.ReadFile(agentf); !strings.Contains(string(b), "SHA256:") {
		t.Fatal("ajana yazılmadı")
	}
	lg, _ := os.ReadFile(logf)
	if !strings.Contains(string(lg), "sec add-generic-password -U -a USBMonitor_WEB1_Passphrase -s USBMonitor -w") {
		t.Errorf("security argümanları kabukla aynı değil: %s", lg)
	}
	if !strings.Contains(errb.String(), "PASSPHRASE'ini") {
		t.Errorf("açıklama notu gösterilmedi")
	}
}

func TestAddRetryEOFAndNoAgent(t *testing.T) {
	priv, _, _ := agentEnv(t, "yanlis")
	a, _ := adder() // girdi yok → EOF
	if err := a.AddRetry("WEB1", priv); err == nil {
		t.Error("EOF'ta başarı")
	}
	t.Setenv("FAKE_NO_AGENT", "1")
	asked := false
	a2 := &AgentAdder{Err: &bytes.Buffer{}, Ask: func(string) (string, bool) { asked = true; return "e", true }}
	if err := a2.AddRetry("WEB1", priv); err == nil || asked {
		t.Errorf("ajan yokken parola suçlandı/soruldu: err=%v asked=%v", err, asked)
	}
}

func TestAskpassAlreadyInAgent(t *testing.T) {
	priv, _, agentf := agentEnv(t, "gizli")
	fp := fingerprintOf(priv)
	os.WriteFile(agentf, []byte("256 "+fp+" x\n"), 0o600)
	a, errb := adder()
	if err := a.AddRetry("WEB1", priv); err != nil || !strings.Contains(errb.String(), "zaten agent'ta") {
		t.Errorf("zaten ajandayken: %v %s", err, errb)
	}
}
