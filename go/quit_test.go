package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestUSBRoot(t *testing.T) {
	for in, want := range map[string]string{
		"/Volumes/test/servers":       "/Volumes/test",
		"/Volumes/My USB/servers/":    "/Volumes/My USB",
		"/Volumes/test":               "/Volumes/test",
		"/Volumes/":                   "",
		"/Volumes/../etc/servers":     "",
		"/Users/x/servers":            "",
		"../servers":                  "",
		"":                            "",
		"/Volumesx/test/servers":      "",
		"/Volumes/test/../../etc/foo": "",
	} {
		if got := usbRoot(in); got != want {
			t.Errorf("usbRoot(%q)=%q, beklenen %q", in, got, want)
		}
	}
	if usbName("") != "" || usbName("/Volumes/test") != "test" {
		t.Error("usbName")
	}
}

func quitCall(a *App, body string, method string) (int, map[string]any) {
	w := httptest.NewRecorder()
	a.quitHandler(w, httptest.NewRequest(method, "/api/quit", strings.NewReader(body)))
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func TestQuitHandlerEjectsUSBAndExits(t *testing.T) {
	var ejectedPath string
	exited := make(chan struct{}, 1)
	a := &App{serversDir: "/Volumes/test/servers", jobs: map[string]*Job{"done": {Done: true}}}
	a.ejectFn = func(w io.Writer, usb string) int {
		ejectedPath = usb
		io.WriteString(w, "✅ USB çıkarıldı")
		return 0
	}
	a.exitFn = func() { exited <- struct{}{} }
	code, m := quitCall(a, `{}`, http.MethodPost)
	if code != 200 || m["ok"] != true || m["usb"] != "test" || m["ejected"] != true || m["log"] != "✅ USB çıkarıldı" {
		t.Fatalf("yanıt: %d %v", code, m)
	}
	if ejectedPath != "/Volumes/test" {
		t.Errorf("eject yolu %q", ejectedPath)
	}
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("uygulama kapanmadı")
	}
}

func TestQuitHandlerNoUSBJustExits(t *testing.T) {
	var ejectCalls atomic.Int32
	exited := make(chan struct{}, 1)
	a := &App{serversDir: "/Users/x/servers"}
	a.ejectFn = func(io.Writer, string) int { ejectCalls.Add(1); return 0 }
	a.exitFn = func() { exited <- struct{}{} }
	code, m := quitCall(a, `{}`, http.MethodPost)
	if code != 200 || m["usb"] != "" || m["ejected"] != false || ejectCalls.Load() != 0 {
		t.Fatalf("yanıt: %d %v eject=%d", code, m, ejectCalls.Load())
	}
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		t.Fatal("uygulama kapanmadı")
	}
}

func TestQuitHandlerEjectFailureReported(t *testing.T) {
	exited := make(chan struct{}, 1)
	a := &App{serversDir: "/Volumes/test/servers"}
	a.ejectFn = func(w io.Writer, usb string) int { io.WriteString(w, "⚠️  USB çıkarılamadı"); return 1 }
	a.exitFn = func() { exited <- struct{}{} }
	_, m := quitCall(a, `{}`, http.MethodPost)
	if m["ok"] != true || m["ejected"] != false || m["usb"] != "test" || m["log"] != "⚠️  USB çıkarılamadı" {
		t.Fatalf("yanıt: %v", m)
	}
	<-exited
}

func TestQuitHandlerRunningJobNeedsForce(t *testing.T) {
	var ejectCalls atomic.Int32
	exited := make(chan struct{}, 1)
	a := &App{serversDir: "/Volumes/test/servers", jobs: map[string]*Job{"j1": {Done: false}}}
	a.ejectFn = func(io.Writer, string) int { ejectCalls.Add(1); return 0 }
	a.exitFn = func() { exited <- struct{}{} }
	code, m := quitCall(a, `{}`, http.MethodPost)
	if code != http.StatusConflict || m["error"] != "running" || m["jobs"] != float64(1) {
		t.Fatalf("çalışan iş varken 409 beklenirdi: %d %v", code, m)
	}
	time.Sleep(700 * time.Millisecond)
	select {
	case <-exited:
		t.Fatal("onaysız kapandı")
	default:
	}
	if ejectCalls.Load() != 0 {
		t.Error("onaysız eject çağrıldı")
	}
	code, m = quitCall(a, `{"force":true}`, http.MethodPost)
	if code != 200 || m["ok"] != true || ejectCalls.Load() != 1 {
		t.Fatalf("force ile: %d %v", code, m)
	}
	<-exited
}

func TestQuitHandlerRejects(t *testing.T) {
	a := &App{}
	if code, _ := quitCall(a, `{}`, http.MethodGet); code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", code)
	}
	if code, _ := quitCall(a, `{`, http.MethodPost); code != http.StatusBadRequest {
		t.Errorf("bozuk gövde: %d", code)
	}
}

func TestQuitHandlerDefersEjectWhenRunningFromTheUSB(t *testing.T) {
	old := ejectDeferred
	t.Cleanup(func() { ejectDeferred = old })
	ejectDeferred = func(string) bool { return true }
	exited := make(chan struct{}, 1)
	a := &App{serversDir: "/Volumes/test/servers"}
	a.ejectFn = func(w io.Writer, usb string) int { io.WriteString(w, "📤 ertelendi"); return 0 }
	a.exitFn = func() { exited <- struct{}{} }
	_, m := quitCall(a, `{}`, http.MethodPost)
	if m["deferred"] != true || m["ejected"] != false || m["usb"] != "test" {
		t.Fatalf("ertelenen çıkarma: %v", m)
	}
	<-exited
}
