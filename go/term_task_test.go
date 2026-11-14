//go:build !windows

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// taskOutput: görev oturumunun şimdiye dek yazdığı çıktı (halka tamponu).
func taskOutput(ts *TermSession) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return string(ts.ring)
}

func waitOut(t *testing.T, ts *TermSession, sub string) {
	t.Helper()
	for i := 0; i < 80; i++ {
		if strings.Contains(taskOutput(ts), sub) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("çıktıda %q yok: %q", sub, taskOutput(ts))
}

// Görev terminali: eylem bir PTY oturumu olarak başlar, terminalden girdi alır (parola/onay), bitince çıkış kodu görünür; sekme bilgisi taşır.
func TestOpenTaskRunsActionInTerminalAndTakesInput(t *testing.T) {
	a := batchApp(t)
	fakeJob(t, `echo "BASLADI $1"; printf 'Parola: '; read -r pw; echo; echo "ALINDI:${#pw}"; exit 3`)
	ts, code, err := a.openTask("s1", "demo", "durum", "", false, "", false, 24, 80)
	if err != nil || ts == nil {
		t.Fatalf("başlamadı (%d): %v", code, err)
	}
	endTask(t, ts)
	if ts.Kind != "task" || ts.Server != "s1" || ts.Pkg != "demo" || ts.Action != "durum" || ts.Title != "demo · durum" {
		t.Fatalf("oturum bilgisi: %+v", ts)
	}
	waitOut(t, ts, "BASLADI s1")
	waitOut(t, ts, "Parola:")
	if ok, err := ts.write([]byte("sekiz\n")); err != nil || !ok {
		t.Fatalf("girdi yazılamadı: %v %v", ok, err)
	}
	waitOut(t, ts, "ALINDI:5")
	waitDone(t, ts, 5*time.Second)
	ts.mu.Lock()
	exit := ts.exit
	ts.mu.Unlock()
	if exit != 3 {
		t.Fatalf("çıkış kodu %d (3 bekleniyordu)", exit)
	}
	// terminal denetim günlüğü görev adını kaydeder (girdiyi ASLA)
	b, _ := os.ReadFile(filepath.Join(a.stateDir, "terminal-audit.log"))
	if !strings.Contains(string(b), "task:demo.durum\ts1\t"+ts.ID) || strings.Contains(string(b), "sekiz") {
		t.Fatalf("denetim günlüğü: %q", b)
	}
}

func TestOpenTaskGatesAndSingleTaskPerServer(t *testing.T) {
	a := batchApp(t)
	fakeJob(t, `sleep 30`)
	// kapılar: değişiklik yapan eylem onay/önkoşul olmadan açılmaz (sunucu tarafı denetimi arayüze güvenmez)
	if ts, code, err := a.openTask("s1", "demo", "kur", "", false, "", false, 24, 80); err == nil || ts != nil || code < 400 {
		t.Fatalf("onaysız yazan eylem açıldı: %d %v", code, err)
	}
	if _, code, err := a.openTask("yok", "demo", "durum", "", false, "", false, 24, 80); err == nil || code < 400 {
		t.Fatalf("olmayan sunucu: %d %v", code, err)
	}
	ts, _, err := a.openTask("s1", "demo", "durum", "", false, "", false, 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	endTask(t, ts)
	if _, code, err := a.openTask("s1", "demo", "durum", "", false, "", false, 24, 80); err == nil || code != http.StatusConflict || !strings.Contains(err.Error(), "zaten çalışan") {
		t.Fatalf("aynı sunucuda ikinci görev: %d %v", code, err)
	}
	// başka sunucuda paralel çalışabilir
	ts2, _, err := a.openTask("s2", "demo", "durum", "", false, "", false, 24, 80)
	if err != nil {
		t.Fatalf("başka sunucuda görev: %v", err)
	}
	endTask(t, ts2)
	if n := a.runningTasks(); n != 2 {
		t.Fatalf("çalışan görev sayısı %d", n)
	}
}

func TestTaskSessionsAreNotReapedWhileRunning(t *testing.T) {
	a := batchApp(t)
	fakeJob(t, `sleep 30`)
	ts, _, err := a.openTask("s1", "demo", "durum", "", false, "", false, 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	endTask(t, ts)
	// çalışan görev, abonesiz ve çok eski olsa da boşta diye KAPATILMAZ (yarım uygulama sunucuyu yarım bırakır)
	a.reapOnce(time.Now().Add(24*time.Hour), time.Minute, time.Hour)
	if a.getTerm(ts.ID) == nil {
		t.Fatal("çalışan görev boşta diye temizlendi")
	}
	// kıyas: aynı koşulda boşta bir ssh oturumu temizlenir
	ssh := startTest(t, a, "sleep 30")
	a.reapOnce(time.Now().Add(24*time.Hour), time.Minute, time.Hour)
	if a.getTerm(ssh.ID) != nil {
		t.Fatal("boşta ssh oturumu temizlenmedi")
	}
	waitDone(t, ssh, 6*time.Second)
}

func TestTermTaskHandlerAndList(t *testing.T) {
	a := batchApp(t)
	fakeJob(t, `echo MERHABA; sleep 30`)
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.termTaskHandler(w, httptest.NewRequest(http.MethodPost, "/api/term/task", strings.NewReader(body)))
		return w
	}
	if w := post(`{"server":"s1","package":"demo","action":"durum"}`); w.Code != 200 {
		t.Fatalf("istek: %d %s", w.Code, w.Body)
	} else {
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		if m["kind"] != "task" || m["title"] != "demo · durum" || m["pkg"] != "demo" || m["action"] != "durum" || m["server"] != "s1" || m["id"] == "" {
			t.Fatalf("yanıt: %v", m)
		}
		endTask(t, a.getTerm(m["id"].(string)))
	}
	// liste görev bilgisini taşır (sayfa yenilenince sekme geri gelsin)
	w := httptest.NewRecorder()
	a.termListHandler(w, httptest.NewRequest(http.MethodGet, "/api/term/list", nil))
	var list []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["kind"] != "task" || list[0]["title"] != "demo · durum" {
		t.Fatalf("liste: %v", list)
	}
	// geçersiz istekler
	for _, body := range []string{`{`, `{"server":"../x","package":"demo","action":"durum"}`, `{"server":"s1","package":"Demo!","action":"durum"}`, `{"server":"s1","package":"demo","action":"x y"}`} {
		if w := post(body); w.Code != 400 {
			t.Errorf("%q: %d", body, w.Code)
		}
	}
	w = httptest.NewRecorder()
	a.termTaskHandler(w, httptest.NewRequest(http.MethodGet, "/api/term/task", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", w.Code)
	}
}

func TestQuitCountsRunningTasks(t *testing.T) {
	a := batchApp(t)
	fakeJob(t, `sleep 30`)
	ts, _, err := a.openTask("s1", "demo", "durum", "", false, "", false, 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	endTask(t, ts)
	exited := make(chan struct{}, 1)
	a.exitFn = func() { exited <- struct{}{} }
	code, m := quitCall(a, `{}`, http.MethodPost)
	if code != http.StatusConflict || m["error"] != "running" || m["jobs"] != float64(1) {
		t.Fatalf("çalışan görevle Çıkış onaysız: %d %v", code, m)
	}
	select {
	case <-exited:
		t.Fatal("onaysız çıkıldı")
	case <-time.After(700 * time.Millisecond):
	}
}

// endTask: test bitince görevi öldürür ve çıkışını (denetim satırı dahil) BEKLER; yoksa t.TempDir temizliği çocuğun son yazımıyla yarışır.
func endTask(t *testing.T, ts *TermSession) {
	t.Helper()
	t.Cleanup(func() {
		ts.kill()
		select {
		case <-ts.exited:
		case <-time.After(5 * time.Second):
		}
	})
}
