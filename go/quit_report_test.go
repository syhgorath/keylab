package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestQuitReportCodes(t *testing.T) {
	join := func(m map[string]any, k string) string { return strings.Join(m[k].([]string), ",") }
	g := quitReport(2, []string{"agent", "tmp"}, true, false)
	if join(g, "cleaned") != "terms,agent,tmp" || join(g, "remains") != "macos" {
		t.Fatalf("misafir: %v", g)
	}
	k := quitReport(0, []string{"tmp"}, false, true) // misafir değil: Keychain ve kullanıcının ajanı da kalır
	if join(k, "cleaned") != "tmp" || join(k, "remains") != "macos,keychain,user_agent" {
		t.Fatalf("misafir değil: %v", k)
	}
	if n := quitReport(0, nil, false, false); join(n, "remains") != "macos,user_agent" || join(n, "cleaned") != "" {
		t.Fatalf("Keychain desteklenmiyorsa: %v", n)
	}
}

func TestQuitResponseCarriesReportOnlyWhenPortable(t *testing.T) {
	exited := make(chan struct{}, 4)
	mk := func(portable bool) *App {
		a := &App{serversDir: "/Volumes/test/servers"}
		a.portableFn = func() bool { return portable }
		a.volumes = fakeVolumes{eject: func(io.Writer, string) int { return 0 }}
		a.exitFn = func() { exited <- struct{}{} }
		return a
	}
	// taşınabilir: ajan + geçici klasör temizlenir ve özet yanıtta
	a := mk(true)
	var order []string
	a.volumes = fakeVolumes{eject: func(io.Writer, string) int { order = append(order, "eject"); return 0 }}
	a.addCleanup("agent", func() { order = append(order, "ajan") })
	a.addCleanup("tmp", func() { order = append(order, "tmp") })
	code, m := quitCall(a, `{}`, http.MethodPost)
	if code != 200 {
		t.Fatalf("kod %d", code)
	}
	rep, ok := m["report"].(map[string]any)
	if !ok {
		t.Fatalf("özet yok: %v", m)
	}
	cl, _ := rep["cleaned"].([]any)
	if len(cl) != 2 || cl[0] != "agent" || cl[1] != "tmp" {
		t.Fatalf("cleaned: %v", rep["cleaned"])
	}
	rm, _ := rep["remains"].([]any)
	if len(rm) == 0 || rm[0] != "macos" {
		t.Fatalf("remains: %v", rep["remains"])
	}
	// sıra: önce USB çıkarma (anahtarlar ajandan silinir), SONRA ajan ve geçici klasör
	if strings.Join(order, ",") != "eject,ajan,tmp" {
		t.Fatalf("sıra: %v", order)
	}
	<-exited
	// taşınabilir değil: özet yok
	b := mk(false)
	_, m = quitCall(b, `{}`, http.MethodPost)
	if _, has := m["report"]; has {
		t.Fatalf("taşınabilir olmayanda özet var: %v", m)
	}
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("kapanmadı")
	}
}
