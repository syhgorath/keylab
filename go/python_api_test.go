package main

import (
	"encoding/json"
	"keylab/internal/runner"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPythonHandlerValidatesInput(t *testing.T) {
	a := &App{compDir: filepath.Join(t.TempDir(), "components")}
	for _, tc := range []struct {
		method, q string
		want      int
	}{{"POST", "package=utility", 405}, {"GET", "package=../x", 400}, {"GET", "", 400}, {"GET", "package=utility", 200}} {
		w := httptest.NewRecorder()
		a.pythonHandler(w, httptest.NewRequest(tc.method, "/api/python?"+tc.q, nil))
		if w.Code != tc.want {
			t.Fatalf("%s %q: %d (beklenen %d)", tc.method, tc.q, w.Code, tc.want)
		}
	}
}

// Kurulu olmayan/doğrulanamayan paket: arayüz engellenmez (eylem kendi hatasını verir).
func TestPythonStatusUnknownPackageDoesNotBlock(t *testing.T) {
	a := &App{compDir: filepath.Join(t.TempDir(), "components")}
	st := a.pythonStatusFor("utility")
	if !st.OK || st.Needed {
		t.Fatalf("%+v", st)
	}
	w := httptest.NewRecorder()
	a.pythonHandler(w, httptest.NewRequest(http.MethodGet, "/api/python?package=utility", nil))
	var m map[string]any
	if json.Unmarshal(w.Body.Bytes(), &m) != nil || m["ok"] != true {
		t.Fatalf("yanıt: %s", w.Body.String())
	}
}

func TestPythonScanReportsVersionsOnce(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"python3.13", "python3"} { // aynı sürümü bildiren iki aday: bir kez listelenir
		os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\necho 3.13.1\n"), 0o755)
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin")
	t.Setenv("KEYLAB_PYTHON_CANDIDATES", "python3.13 python3 python3.99")
	found, _ := runner.PythonScan()
	if len(found) != 1 || found[0] != "3.13.1" {
		t.Fatalf("bulunan: %v", found)
	}
	t.Setenv("KEYLAB_PYTHON_CANDIDATES", "yok-boyle-python")
	if found, stub := runner.PythonScan(); len(found) != 0 || stub {
		t.Fatalf("boş beklenirdi: %v %v", found, stub)
	}
}
