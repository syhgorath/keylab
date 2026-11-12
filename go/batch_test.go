package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// batchApp: 3 geçerli sunucu (s1..s3) + salt okunur "durum", yazan "kur" ve sudo'lu "tara" eylemli paket.
func batchApp(t *testing.T) *App {
	t.Helper()
	comp, srvs, state := t.TempDir(), t.TempDir(), t.TempDir()
	writePkg(t, comp, "demo", "0.1.0", `{"id":"demo","version":"0.1.0","name":"Demo","description":"d",
	 "actions":{"durum":{"effects":[],"requires_sudo":false,"confirm":"none"},
	            "kur":{"effects":["installs-package"],"requires_sudo":true,"confirm":"yes-no"},
	            "tara":{"effects":[],"requires_sudo":true,"confirm":"none"}},
	 "vars":[]}`)
	for _, n := range []string{"s1", "s2", "s3"} {
		d := filepath.Join(srvs, n)
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(d, "server.env"), []byte("IP=10.0.0.1\nUSER=root\nPORT=22\n"), 0o600)
	}
	a := &App{serversDir: srvs, compDir: comp, stateDir: state, jobs: map[string]*Job{}, up: map[string]bool{}}
	return a
}

// fakeJob: sunucu adına göre davranan sahte komut (son argüman sunucu adıdır)
func fakeJob(t *testing.T, script string) {
	t.Helper()
	old := jobArgv
	jobArgv = func(exe, servers, comps, state, pkg, action, server, itemVar, item string) []string {
		return []string{"bash", "-c", `s="$1"; ` + script, "x", server}
	}
	t.Cleanup(func() { jobArgv = old })
}

func needScript(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script yok")
	}
}

func waitBatch(t *testing.T, a *App) *Batch {
	t.Helper()
	end := time.Now().Add(20 * time.Second)
	for time.Now().Before(end) {
		if b := a.batchSnapshot(); b != nil && b.Done {
			return b
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("toplu işlem bitmedi")
	return nil
}

func states(b *Batch) string {
	var out []string
	for _, it := range b.Items {
		out = append(out, it.Server+"="+it.State)
	}
	return strings.Join(out, " ")
}

func TestBatchOptionsOnlyRunnable(t *testing.T) {
	a := batchApp(t)
	opts := a.batchOptions([]string{"s1", "s2", "yok"})
	if len(opts) != 1 || opts[0].Action != "durum" || opts[0].OK != 2 || opts[0].Total != 3 {
		t.Fatalf("yalnız salt okunur 'durum' (2/3) beklenir: %+v", opts)
	}
}

func TestBatchRefusesWriteAndSudoAndUnknown(t *testing.T) {
	a := batchApp(t)
	for _, act := range []string{"kur", "tara", "olmayan"} {
		if ok, _ := a.batchEligible("s1", "demo", act); ok {
			t.Errorf("%s toplu çalıştırılabilir sayıldı", act)
		}
		if _, code, err := a.startBatch("demo", act, []string{"s1", "s2"}, false); err == nil || code != http.StatusConflict {
			t.Errorf("%s: toplu işlem başlamamalı (kod %d, err %v)", act, code, err)
		}
		if a.batchActive() {
			t.Errorf("%s: reddedilen toplu işlem bayrağı açık bıraktı", act)
		}
	}
	if ok, _ := a.batchEligible("s1", "demo", "durum"); !ok {
		t.Error("durum uygun olmalı")
	}
	if ok, _ := a.batchEligible("yok", "demo", "durum"); ok {
		t.Error("olmayan sunucu uygun sayıldı")
	}
}

func TestBatchRunsSequentiallyAndContinuesOnFail(t *testing.T) {
	needScript(t)
	a := batchApp(t)
	fakeJob(t, `echo run-$s; [ "$s" = s2 ] && exit 3; exit 0`)
	b, _, err := a.startBatch("demo", "durum", []string{"s1", "s2", "s3", "s3"}, false) // tekrar eden ad bir kez
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Items) != 4 { // startBatch tekilleştirmez: handler cleanServers ile yapar
		t.Logf("not: %d öğe", len(b.Items))
	}
	done := waitBatch(t, a)
	if got := states(done); !strings.Contains(got, "s1=ok") || !strings.Contains(got, "s2=fail") || !strings.Contains(got, "s3=ok") {
		t.Errorf("hatada devam etmeli: %s", got)
	}
	if a.batchActive() {
		t.Error("bitince bayrak kapanmalı")
	}
	if _, err := a.newJob("s", "p", "a"); err != nil {
		t.Errorf("bitince tek iş başlatılabilmeli: %v", err)
	}
}

func TestBatchStopOnFail(t *testing.T) {
	needScript(t)
	a := batchApp(t)
	fakeJob(t, `[ "$s" = s1 ] && exit 2; exit 0`)
	if _, _, err := a.startBatch("demo", "durum", []string{"s1", "s2", "s3"}, true); err != nil {
		t.Fatal(err)
	}
	got := states(waitBatch(t, a))
	if got != "s1=fail s2=cancelled s3=cancelled" {
		t.Errorf("ilk hatada durmalı: %s", got)
	}
}

func TestBatchBlocksOtherJobsWhileRunningAndOnlyOneBatch(t *testing.T) {
	needScript(t)
	a := batchApp(t)
	fakeJob(t, `sleep 1; exit 0`)
	if _, _, err := a.startBatch("demo", "durum", []string{"s1", "s2"}, false); err != nil {
		t.Fatal(err)
	}
	// işler arasındaki boşlukta bile tek tek iş başlatılamaz
	for i := 0; i < 30; i++ {
		if _, err := a.newJob("s", "p", "a"); err == nil {
			t.Fatal("toplu iş sürerken tek iş başlatılabildi")
		}
		time.Sleep(60 * time.Millisecond)
	}
	if _, code, err := a.startBatch("demo", "durum", []string{"s3"}, false); err == nil || code != http.StatusConflict {
		t.Errorf("ikinci toplu işlem reddedilmeli: %d %v", code, err)
	}
	waitBatch(t, a)
}

func TestBatchRefusedWhenJobRunning(t *testing.T) {
	a := batchApp(t)
	if _, err := a.newJob("s1", "demo", "durum"); err != nil {
		t.Fatal(err)
	}
	if _, code, err := a.startBatch("demo", "durum", []string{"s1"}, false); err == nil || code != http.StatusConflict {
		t.Errorf("çalışan iş varken toplu işlem başlamamalı: %d %v", code, err)
	}
	if a.batchActive() {
		t.Error("reddedilen toplu işlem bayrağı açık bıraktı")
	}
}

func TestBatchCancel(t *testing.T) {
	needScript(t)
	a := batchApp(t)
	fakeJob(t, `echo basladi; sleep 60`)
	if _, _, err := a.startBatch("demo", "durum", []string{"s1", "s2", "s3"}, false); err != nil {
		t.Fatal(err)
	}
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) {
		if b := a.batchSnapshot(); b != nil && b.Items[0].State == "running" {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	rec := httptest.NewRecorder()
	a.batchCancelHandler(rec, httptest.NewRequest(http.MethodPost, "/api/batch/cancel", nil))
	if rec.Code != 200 {
		t.Fatalf("iptal: %d", rec.Code)
	}
	got := states(waitBatch(t, a))
	if got != "s1=cancelled s2=cancelled s3=cancelled" {
		t.Errorf("iptalde hepsi cancelled olmalı: %s", got)
	}
	rec = httptest.NewRecorder()
	a.batchCancelHandler(rec, httptest.NewRequest(http.MethodPost, "/api/batch/cancel", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("biten işi iptal: %d", rec.Code)
	}
}

func TestBatchHandlerValidationAndAuth(t *testing.T) {
	a := batchApp(t)
	a.host, a.token = "127.0.0.1:9999", "jeton"
	post := func(h http.HandlerFunc, body string) int {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec.Code
	}
	if c := post(a.batchHandler, `{"package":"demo","action":"durum","servers":["../x"]}`); c != 400 {
		t.Errorf("yol kaçışı adı: %d", c)
	}
	if c := post(a.batchHandler, `{"package":"demo","action":"durum","servers":[]}`); c != 400 {
		t.Errorf("boş liste: %d", c)
	}
	if c := post(a.batchHandler, `{"package":"../d","action":"durum","servers":["s1"]}`); c != 400 {
		t.Errorf("kötü paket: %d", c)
	}
	big := make([]string, maxBatchServers+1)
	for i := range big {
		big[i] = "s" + strings.Repeat("a", i%5) + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26))
	}
	if _, err := cleanServers(big); err == nil {
		t.Error("üst sınır aşıldı ama kabul edildi")
	}
	if got, err := cleanServers([]string{"a", "b", "a"}); err != nil || len(got) != 2 {
		t.Errorf("tekilleştirme: %v %v", got, err)
	}
	// çerezsiz istek reddedilir
	h := a.guard(a.batchHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/batch", nil)
	req.Host = "127.0.0.1:9999"
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("çerezsiz: %d", rec.Code)
	}
}
