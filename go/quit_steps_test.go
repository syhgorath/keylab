package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keylab/internal/platform"
)

func quitStatus(a *App) (string, []quitStep) {
	w := httptest.NewRecorder()
	a.quitStatusHandler(w, httptest.NewRequest(http.MethodGet, "/api/quit/status", nil))
	var r struct {
		State string     `json:"state"`
		Steps []quitStep `json:"steps"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	return r.State, r.Steps
}

func stepsString(st []quitStep) string {
	var p []string
	for _, s := range st {
		p = append(p, s.ID+":"+s.State)
	}
	return strings.Join(p, ",")
}

// Çıkış adım adım kaydedilir: yanıt ve /api/quit/status aynı sırayı verir; süreç "Kapat" gelene dek yaşar.
func TestQuitRecordsStepsInOrderAndWaitsForFinish(t *testing.T) {
	platform.SetGuestMode(true)
	t.Cleanup(func() { platform.SetGuestMode(false) })
	exited := make(chan struct{}, 1)
	a := &App{serversDir: "/Volumes/test/servers", jobs: map[string]*Job{}}
	a.portableFn = func() bool { return true }
	a.volumes = fakeVolumes{eject: func(io.Writer, string) int { return 0 }}
	a.exitFn = func() { exited <- struct{}{} }
	a.addCleanup("agent", func() {})
	a.addCleanup("tmp", func() {})
	if st, _ := quitStatus(a); st != "" {
		t.Fatalf("başlamadan durum %q", st)
	}
	code, m := quitCall(a, `{}`, http.MethodPost)
	if code != 200 {
		t.Fatalf("kod %d", code)
	}
	want := "terms:skipped,usb:done,agent:done,tmp:done,keychain:info"
	st, steps := quitStatus(a)
	if st != "done" || stepsString(steps) != want {
		t.Fatalf("durum/adımlar: %s %s (beklenen %s)", st, stepsString(steps), want)
	}
	resp, _ := json.Marshal(m["steps"])
	var rs []quitStep
	_ = json.Unmarshal(resp, &rs)
	if stepsString(rs) != want {
		t.Fatalf("yanıttaki adımlar: %s", stepsString(rs))
	}
	if last := steps[len(steps)-1]; last.Detail != "guest" {
		t.Fatalf("misafirde keychain adımı 'guest' demeli: %+v", last)
	}
	if steps[1].Detail != "test" {
		t.Fatalf("usb adımı birim adını taşımalı: %+v", steps[1])
	}
	select {
	case <-exited:
		t.Fatal("adımlar bitince süreç kendiliğinden kapandı: kullanıcı 'Kapat'a basmadan pencere kapanmamalı")
	case <-time.After(900 * time.Millisecond):
	}
	if c := quitFinish(t, a); c != 200 {
		t.Fatalf("finish: %d", c)
	}
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("Kapat'tan sonra kapanmadı")
	}
}

// Bitmeden finish reddedilir; ikinci çıkış isteği 409 "quitting"; yalnız GET/POST kabul edilir.
func TestQuitFinishRefusedUntilDoneAndSecondQuitRejected(t *testing.T) {
	a := &App{serversDir: "/Users/x/servers", jobs: map[string]*Job{}}
	a.exitFn = func() {}
	if c := quitFinish(t, a); c != http.StatusConflict {
		t.Fatalf("başlamadan finish: %d", c)
	}
	if code, _ := quitCall(a, `{}`, http.MethodPost); code != 200 {
		t.Fatal("ilk çıkış başarısız")
	}
	if code, m := quitCall(a, `{}`, http.MethodPost); code != http.StatusConflict || m["error"] != "quitting" {
		t.Fatalf("ikinci çıkış: %d %v", code, m)
	}
	w := httptest.NewRecorder()
	a.quitStatusHandler(w, httptest.NewRequest(http.MethodPost, "/api/quit/status", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status POST: %d", w.Code)
	}
	w = httptest.NewRecorder()
	a.quitFinishHandler(w, httptest.NewRequest(http.MethodGet, "/api/quit/finish", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("finish GET: %d", w.Code)
	}
}

// Kullanıcı "Kapat"a basmazsa süreç zamanaşımıyla kapanır (pencere hiç kapatılmasa da süreç sonsuza dek yaşamaz).
func TestQuitAutoExitsAfterTimeout(t *testing.T) {
	exited := make(chan struct{}, 1)
	a := &App{serversDir: "/Users/x/servers", jobs: map[string]*Job{}, quitAutoExitAfter: 150 * time.Millisecond}
	a.exitFn = func() { exited <- struct{}{} }
	quitCall(a, `{}`, http.MethodPost)
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("zamanaşımında kapanmadı")
	}
}

// Kapatılan terminal sayısı adımda görünür; Keychain modunda keychain adımı 'kept' (dokunulmadı) der.
func TestQuitStepsReportTerminalCountAndKeychainKept(t *testing.T) {
	platform.SetGuestMode(false)
	a := &App{serversDir: "/Users/x/servers", jobs: map[string]*Job{}}
	a.exitFn = func() {}
	a.terms = map[string]*TermSession{"a": {}, "b": {}}
	quitCall(a, `{}`, http.MethodPost)
	_, steps := quitStatus(a)
	if stepsString(steps) != "terms:done,keychain:info" || steps[0].N != 2 || steps[1].Detail != "kept" {
		t.Fatalf("adımlar: %+v", steps)
	}
}
