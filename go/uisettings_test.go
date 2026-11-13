package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func settingsApp(t *testing.T) *App {
	t.Helper()
	a := testApp()
	a.stateDir = t.TempDir()
	return a
}

func postSettings(a *App, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.uiSettingsHandler(rec, httptest.NewRequest(http.MethodPost, "/api/ui-settings", strings.NewReader(body)))
	return rec
}

func TestMaxJobsDefaultAndSetting(t *testing.T) {
	a := settingsApp(t)
	if a.maxJobs() != defaultMaxJobs {
		t.Fatalf("varsayılan %d olmalı: %d", defaultMaxJobs, a.maxJobs())
	}
	if rec := postSettings(a, `{"maxJobs":2}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"maxJobs":2`) {
		t.Fatalf("kaydetme: %d %s", rec.Code, rec.Body.String())
	}
	// ayar GERÇEKTEN sunucu sınırını değiştirir
	for _, s := range []string{"s1", "s2"} {
		if _, err := a.newJob(s, "p", "durum"); err != nil {
			t.Fatalf("%s başlamalı: %v", s, err)
		}
	}
	if _, err := a.newJob("s3", "p", "durum"); err == nil || !strings.Contains(err.Error(), "en çok 2") {
		t.Errorf("2 sınırında 3. iş reddedilmeli: %v", err)
	}
	// sınırı yükseltince yeni iş başlar
	postSettings(a, `{"maxJobs":3}`)
	if _, err := a.newJob("s3", "p", "durum"); err != nil {
		t.Errorf("3'e yükseltince 3. iş başlamalı: %v", err)
	}
}

func TestMaxJobsLoweringKeepsRunningJobs(t *testing.T) {
	a := settingsApp(t)
	postSettings(a, `{"maxJobs":4}`)
	for _, s := range []string{"s1", "s2", "s3"} {
		if _, err := a.newJob(s, "p", "durum"); err != nil {
			t.Fatal(err)
		}
	}
	postSettings(a, `{"maxJobs":1}`)
	if n := len(a.runningLocked()); n != 3 {
		t.Errorf("düşürmek çalışan işleri durdurmamalı: %d", n)
	}
	if _, err := a.newJob("s4", "p", "durum"); err == nil {
		t.Error("düşürülmüş sınırda yeni iş başlamamalı")
	}
}

func TestMaxJobsRangeValidation(t *testing.T) {
	a := settingsApp(t)
	for _, body := range []string{`{"maxJobs":0}`, `{"maxJobs":17}`, `{"maxJobs":-3}`, `{"maxJobs":"x"}`, `{}`, `nope`} {
		if rec := postSettings(a, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: 400 beklenir, %d", body, rec.Code)
		}
	}
	if a.maxJobs() != defaultMaxJobs {
		t.Errorf("geçersiz istek ayarı değiştirmemeli: %d", a.maxJobs())
	}
	if _, err := os.Stat(a.uiSettingsPath()); err == nil {
		t.Error("geçersiz istek dosya yazmamalı")
	}
	for _, n := range []string{`{"maxJobs":1}`, `{"maxJobs":16}`} {
		if rec := postSettings(a, n); rec.Code != 200 {
			t.Errorf("%s sınır değeri kabul edilmeli: %d", n, rec.Code)
		}
	}
}

func TestMaxJobsPersistsAcrossRestart(t *testing.T) {
	a := settingsApp(t)
	postSettings(a, `{"maxJobs":7}`)
	st, err := os.Stat(a.uiSettingsPath())
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("dosya yok ya da izin yanlış: %v", err)
	}
	b := testApp()
	b.stateDir = a.stateDir
	if b.maxJobs() != defaultMaxJobs {
		t.Fatal("yükleme öncesi varsayılan olmalı")
	}
	b.loadUISettings()
	if b.maxJobs() != 7 {
		t.Errorf("yeniden başlatınca 7 okunmalı: %d", b.maxJobs())
	}
}

func TestMaxJobsCorruptFileFallsBack(t *testing.T) {
	for _, content := range []string{"{bozuk", `{"maxJobs":999}`, `{"maxJobs":0}`, ""} {
		a := settingsApp(t)
		os.WriteFile(a.uiSettingsPath(), []byte(content), 0o600)
		a.loadUISettings()
		if a.maxJobs() != defaultMaxJobs {
			t.Errorf("%q: varsayılana düşmeli: %d", content, a.maxJobs())
		}
	}
}

func TestUISettingsNeedsAuthAndMethods(t *testing.T) {
	a := settingsApp(t)
	a.host, a.token = "127.0.0.1:9999", "jeton"
	h := a.guard(a.uiSettingsHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/ui-settings", nil)
	req.Host = "127.0.0.1:9999"
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("çerezsiz: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	a.uiSettingsHandler(rec, httptest.NewRequest(http.MethodDelete, "/api/ui-settings", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE: %d", rec.Code)
	}
}
