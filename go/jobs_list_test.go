package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJobSummariesNewestFirstAndNoOutput(t *testing.T) {
	a := testApp()
	j1 := &Job{ID: "a1", Server: "s1", Package: "p", Action: "preview", Started: "2026-01-01T10:00:00Z", Done: true, Exit: 0, buf: []byte("GIZLI-CIKTI")}
	j2 := &Job{ID: "b2", Server: "s2", Package: "p", Action: "kur", Started: "2026-01-01T11:00:00Z", buf: []byte("devam ediyor")}
	a.jobs["a1"], a.jobs["b2"] = j1, j2
	a.running = j2
	list, running := a.jobSummaries()
	if len(list) != 2 || list[0].ID != "b2" || list[1].ID != "a1" {
		t.Fatalf("yeni→eski sıralı olmalı: %+v", list)
	}
	if running != "b2" {
		t.Fatalf("çalışan iş b2 olmalı: %q", running)
	}
	// uç nokta çıktı sızdırmaz
	a.host, a.token = "127.0.0.1:9999", "j"
	rec := httptest.NewRecorder()
	a.jobsHandler(rec, httptest.NewRequest(http.MethodGet, "/api/jobs", nil))
	body := rec.Body.String()
	if strings.Contains(body, "GIZLI-CIKTI") || strings.Contains(body, "devam ediyor") {
		t.Errorf("iş çıktısı listeye sızmamalı: %s", body)
	}
	if !strings.Contains(body, `"running":"b2"`) {
		t.Errorf("running alanı yok: %s", body)
	}
	// iş bitince running boş
	j2.Done = true
	if _, running := a.jobSummaries(); running != "" {
		t.Errorf("biten iş running sayılmamalı: %q", running)
	}
}

func TestJobsEndpointNeedsAuth(t *testing.T) {
	a := testApp()
	a.host, a.token = "127.0.0.1:9999", "jeton"
	h := a.guard(a.jobsHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	req.Host = "127.0.0.1:9999"
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("çerezsiz istek reddedilmeli, kod %d", rec.Code)
	}
}
