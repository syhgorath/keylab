package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"keylab/internal/platform"
)

func fakeTable(self int) []platform.Proc {
	return []platform.Proc{
		{PID: 1, PPID: 0, CPU: 0.1, RSSKB: 1000, Name: "launchd"},
		{PID: self, PPID: 1, CPU: 5, RSSKB: 40960, ElapsedSec: 120, Name: "keylab"},      // KeyLab süreci
		{PID: 9001, PPID: self, CPU: 0, RSSKB: 2048, ElapsedSec: 118, Name: "ssh-agent"}, // özel ajan
		{PID: 9002, PPID: self, CPU: 20, RSSKB: 10240, ElapsedSec: 30, Name: "keylab"},   // görev (keylab run)
		{PID: 9003, PPID: 9002, CPU: 60, RSSKB: 102400, ElapsedSec: 25, Name: "python3.13"},
		{PID: 9004, PPID: 9003, CPU: 1, RSSKB: 4096, ElapsedSec: 20, Name: "ssh"},
		{PID: 9005, PPID: self, CPU: 0.5, RSSKB: 512, Name: "ps"},                         // ölçüm yapan ps: gösterilmez
		{PID: 7777, PPID: 1, CPU: 90, RSSKB: 999999, ElapsedSec: 5, Name: "baskaprogram"}, // KeyLab'ın alt süreci DEĞİL
	}
}

func TestMonitorSampleCountsOnlyKeyLabProcessTree(t *testing.T) {
	a := testApp()
	dir := t.TempDir()
	a.serversDir, a.compDir, a.stateDir = filepath.Join(dir, "servers"), filepath.Join(dir, "components"), filepath.Join(dir, "state")
	a.psFn = func() ([]platform.Proc, error) { return fakeTable(os.Getpid()), nil }
	a.dirSizeFn = func(p string) int64 { return int64(len(filepath.Base(p))) * 1000 }
	a.portableFn = func() bool { return false }
	s := a.monitorSample()
	got := map[int]string{}
	for _, p := range s.Procs {
		got[p.PID] = p.Kind
	}
	for pid, kind := range map[int]string{os.Getpid(): "self", 9001: "agent", 9002: "task", 9003: "python", 9004: "ssh"} {
		if got[pid] != kind {
			t.Errorf("pid %d türü %q, beklenen %q (hepsi: %v)", pid, got[pid], kind, got)
		}
	}
	if _, has := got[7777]; has || len(s.Procs) != 5 {
		t.Fatalf("yalnız KeyLab ve alt süreçleri sayılmalı (ps ve başka programlar hariç): %d %v", len(s.Procs), got)
	}
	if _, has := got[9005]; has {
		t.Fatal("ölçümü yapan ps listelenmemeli")
	}
	wantCPU, wantRSS := 5.0+0+20+60+1, float64(40960+2048+10240+102400+4096)/1024
	if s.CPU != wantCPU || s.RSSMB != wantRSS || s.SelfRSSMB != 40 {
		t.Fatalf("toplamlar: cpu %v (beklenen %v) rss %v (beklenen %v) self %v", s.CPU, wantCPU, s.RSSMB, wantRSS, s.SelfRSSMB)
	}
	if s.Cores < 1 || s.Goroutines < 1 || s.HeapMB <= 0 || s.UptimeSec < 0 {
		t.Fatalf("çalışma zamanı değerleri: %+v", s)
	}
	ids := map[string]int64{}
	for _, d := range s.Dirs {
		ids[d.ID] = d.Bytes
	}
	if ids["servers"] != 7000 || ids["components"] != 10000 || ids["state"] != 5000 || len(s.Dirs) != 4 {
		t.Fatalf("klasörler: %+v", s.Dirs)
	}
}

func TestMonitorDirSizesAreCachedAndPsFailureIsReported(t *testing.T) {
	a := testApp()
	a.serversDir = t.TempDir()
	calls := 0
	a.dirSizeFn = func(string) int64 { calls++; return 1 }
	a.psFn = func() ([]platform.Proc, error) { return nil, errors.New("ps yok") }
	a.portableFn = func() bool { return false }
	s1 := a.monitorSample()
	n := calls
	a.monitorSample()
	if calls != n {
		t.Fatalf("klasör boyutları 30 sn önbelleklenmeli: %d → %d çağrı", n, calls)
	}
	if s1.Note == "" || s1.RSSMB <= 0 || len(s1.Procs) != 0 {
		t.Fatalf("ps başarısızsa not düşülmeli ve yedek olarak Go yığın boyutu gösterilmeli: %+v", s1)
	}
}

func TestMonitorHandlerJSONAndMethods(t *testing.T) {
	a := testApp()
	a.serversDir = t.TempDir()
	a.psFn = func() ([]platform.Proc, error) { return fakeTable(os.Getpid()), nil }
	a.dirSizeFn = func(string) int64 { return 1 }
	w := httptest.NewRecorder()
	a.monitorHandler(w, httptest.NewRequest(http.MethodGet, "/api/monitor", nil))
	var s monSample
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &s) != nil || s.PID != os.Getpid() || len(s.Procs) == 0 {
		t.Fatalf("yanıt: %d %s", w.Code, w.Body)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("önbellek başlığı yok")
	}
	w = httptest.NewRecorder()
	a.monitorHandler(w, httptest.NewRequest(http.MethodPost, "/api/monitor", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", w.Code)
	}
}

func TestDirSizeSumsFilesAndToleratesMissing(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(d, "x"), make([]byte, 100), 0o644)
	os.WriteFile(filepath.Join(d, "a", "b", "y"), make([]byte, 250), 0o644)
	if n := dirSize(d); n != 350 {
		t.Fatalf("dirSize %d", n)
	}
	if n := dirSize(filepath.Join(d, "yok")); n != 0 {
		t.Fatalf("olmayan klasör %d", n)
	}
}

func TestMonitorKind(t *testing.T) {
	for name, want := range map[string]string{"ssh-agent": "agent", "SSH-AGENT": "agent", "ssh": "ssh", "ssh-add": "ssh", "python3.12": "python", "ansible-playbook": "python", "keylab": "task", "sshpocket": "task", "bash": "other"} {
		if got := monitorKind(name, false); got != want {
			t.Errorf("%s → %s, beklenen %s", name, got, want)
		}
	}
	if monitorKind("keylab", true) != "self" {
		t.Error("kendi süreci self olmalı")
	}
}
