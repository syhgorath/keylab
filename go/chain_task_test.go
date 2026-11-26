//go:build !windows

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tek tık zinciri görev terminali: yalnız bir ayar kartının uygulama eylemi, önkoşul/onay aranmadan "run -chain" ile açılır.
func TestOpenChainTask(t *testing.T) {
	a := batchApp(t)
	// "baska": değiştiren ama hiçbir ayar kartının uygulama eylemi olmayan eylem
	writePkg(t, a.compDir, "demo", "0.2.0", strings.Replace(setPkg, `"apply":{`, `"baska":{"cmd":"x","effects":["changes-config"],"confirm":"yes-no"},
  "apply":{`, 1))
	old := taskProgram
	taskProgram = func(string) string { return "echo" }
	t.Cleanup(func() { taskProgram = old })

	// aynı eylem zincirsiz: snapshot/onay olmadan açılmaz
	if _, code, err := a.openTask("s1", "demo", "apply", "", false, "", false, 24, 80); err == nil || code < 400 {
		t.Fatalf("zincirsiz yazan eylem onaysız açıldı: %d %v", code, err)
	}
	for _, c := range []struct {
		server, action, item, want string
	}{
		{"s1", "durum", "", "uygulama eylemi değil"},
		{"s1", "baska", "", "uygulama eylemi değil"},
		{"s1", "yokeylem", "", "bulunamadı"},
		{"yok", "apply", "", "sunucu yok"},
		{"s1", "apply", "docker", "öğesiyle"},
	} {
		if ts, code, err := a.openTaskX(true, c.server, "demo", c.action, c.item, false, "", false, 24, 80); err == nil || ts != nil || code < 400 || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: %d %v", c, code, err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/term/task", strings.NewReader(`{"server":"s1","package":"demo","action":"apply","chain":true}`))
	rec := httptest.NewRecorder()
	a.termTaskHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var ts *TermSession
	a.tmu.Lock()
	for _, s := range a.terms {
		ts = s
	}
	a.tmu.Unlock()
	endTask(t, ts)
	waitDone(t, ts, 5*time.Second)
	out := taskOutput(ts)
	if !strings.Contains(out, "run -chain -servers ") || !strings.Contains(strings.TrimSpace(out), "demo apply s1") {
		t.Fatalf("komut: %q", out)
	}
	b, _ := os.ReadFile(filepath.Join(a.stateDir, "terminal-audit.log"))
	if !strings.Contains(string(b), "chain:demo.apply\ts1\t"+ts.ID) {
		t.Fatalf("denetim günlüğü: %q", b)
	}
}
