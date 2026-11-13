package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// /api/lynis bulgu listesi HİÇBİR durumda null olmamalı: arayüz `findings.filter(...)` çağırır (null'da TypeError, bulgu listesi çizilmez).
// Eski hata: bulgu dosyası (bulgular.tsv) yokken findings nil dilim döndürüyor, JSON'a "null" yazılıyordu.
func TestFindingsNeverNilJSON(t *testing.T) {
	a := &App{stateDir: t.TempDir()}
	report, list := a.findings("hic-olcum-yok")
	if list == nil {
		t.Fatal("bulgu dosyası yokken findings nil olmamalı (JSON null olur)")
	}
	if report != "" || len(list) != 0 {
		t.Errorf("boş sonuç beklenir: %q %v", report, list)
	}
	b, _ := json.Marshal(map[string]any{"findings": list})
	if !strings.Contains(string(b), `"findings":[]`) {
		t.Errorf("JSON boş dizi olmalı: %s", b)
	}
}
