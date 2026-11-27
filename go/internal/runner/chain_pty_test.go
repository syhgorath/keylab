//go:build darwin || linux

package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// Gerçek terminalde: önizleme akarken basılan "e" sonradan sorulan e/h sorusunu YANITLAMAZ; yanıt soru göründükten sonra yazılan satırdır.
func TestReviewChainDropsTypeahead(t *testing.T) {
	for _, c := range []struct {
		answer string
		ran    int
	}{{"\n", 1}, {"e\n", 2}} {
		w := newWorld(t)
		hold := filepath.Join(t.TempDir(), "birak")
		// önizleme, test tuşlara basana kadar bekler
		script := strings.Replace(chainAnsible, `echo "adim $pb calisti"`, `echo "adim $pb calisti"
if [ "$pb" = preview ]; then : > "$CHAIN_HOLD.basladi"; i=0; while [ ! -f "$CHAIN_HOLD" ] && [ $i -lt 200 ]; do sleep 0.05; i=$((i+1)); done; fi`, 1)
		os.WriteFile(filepath.Join(w.bin, "ansible-playbook"), []byte(script), 0o755)
		log := filepath.Join(t.TempDir(), "chain.log")
		t.Setenv("CHAIN_LOG", log)
		t.Setenv("CHAIN_FAIL", "")
		t.Setenv("CHAIN_HOLD", hold)
		pkg, info := chainInfo(t)
		plan, err := ChainPlan(info, "baska", "")
		if err != nil {
			t.Fatal(err)
		}
		master, slave := testPTY(t)
		go io.Copy(io.Discard, master) // yankı birikip yazmayı tıkamasın
		out := &lockedBuf{}
		ec := &ExecCtx{PkgDir: pkg, BinDir: w.bin, Server: "srv", Host: "10.0.0.5", User: "deploy", Port: "22", Key: w.key, Prefix: "SRV", Profile: "ubuntu-26.04",
			Gates: &Gates{StateDir: t.TempDir()}, KnownHosts: w.known, Stdin: slave, Stdout: out, Stderr: out, Ctx: context.Background()}
		done := make(chan int, 1)
		go func() { done <- ec.runChain(info, "demo", "0.1.0", plan, func(string, ExecResult) {}) }()
		waitFor := func(what string, ok func() bool) {
			for end := time.Now().Add(10 * time.Second); !ok(); time.Sleep(10 * time.Millisecond) {
				if time.Now().After(end) {
					t.Fatalf("beklenen olmadı: %s\n%s", what, out.String())
				}
			}
		}
		says := func(s string) func() bool { return func() bool { return strings.Contains(out.String(), s) } }
		waitFor("parola istemi", says("sudo parolası"))
		master.WriteString("x\n")
		waitFor("önizleme", func() bool { _, err := os.Stat(hold + ".basladi"); return err == nil })
		master.WriteString("e\n") // önizleme akarken basılan tuş
		time.Sleep(200 * time.Millisecond)
		os.WriteFile(hold, nil, 0o600)
		waitFor("onay sorusu", says("Devam edilsin mi?"))
		time.Sleep(200 * time.Millisecond)
		if strings.Contains(out.String(), "İptal edildi") || strings.Contains(out.String(), "adim baska calisti") {
			t.Fatalf("önceden basılan tuş soruyu yanıtladı:\n%s", out.String())
		}
		master.WriteString(c.answer)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("zincir bitmedi:\n%s", out.String())
		}
		b, _ := os.ReadFile(log)
		if n := len(strings.Split(strings.TrimSpace(string(b)), "\n")); n != c.ran {
			t.Errorf("yanıt %q: %d adım koştu (beklenen %d)\n%s", c.answer, n, c.ran, out.String())
		}
		master.Close()
		slave.Close()
	}
}
