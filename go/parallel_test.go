package main

import (
	"strings"
	"testing"
)

func TestParallelJobsOnDifferentServers(t *testing.T) {
	a := testApp()
	j1, err := a.newJob("s1", "p", "durum")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.newJob("s2", "p", "durum"); err != nil {
		t.Errorf("farklı sunucuda paralel iş başlamalı: %v", err)
	}
	// aynı sunucuda ikinci iş reddedilir
	if _, err := a.newJob("s1", "p", "baska"); err == nil || !strings.Contains(err.Error(), "bu sunucuda") {
		t.Errorf("aynı sunucuda ikinci iş reddedilmeli: %v", err)
	}
	// biten iş yeni işe yer açar
	a.jmu.Lock()
	j1.Done = true
	a.jmu.Unlock()
	if _, err := a.newJob("s1", "p", "yeni"); err != nil {
		t.Errorf("biten işten sonra aynı sunucuda iş başlamalı: %v", err)
	}
}

func TestParallelJobsCap(t *testing.T) {
	a := testApp()
	for i := 0; i < maxParallelJobs; i++ {
		if _, err := a.newJob("s"+string(rune('a'+i)), "p", "durum"); err != nil {
			t.Fatalf("%d. iş başlamalı: %v", i+1, err)
		}
	}
	if _, err := a.newJob("sz", "p", "durum"); err == nil || !strings.Contains(err.Error(), "aynı anda") {
		t.Errorf("üst sınırı aşan iş reddedilmeli: %v", err)
	}
}

func TestExclusiveJobsRunAlone(t *testing.T) {
	a := testApp()
	// paket kurulumu (sunucu yok) başka iş varken başlamaz
	if _, err := a.newJob("s1", "p", "durum"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.newJob("", "utility", "upgrade"); err == nil {
		t.Error("çalışan iş varken genel iş (kurulum) başlamamalı")
	}
	if _, err := a.newJob("s2", "", "remove"); err == nil {
		t.Error("çalışan iş varken sunucu silme başlamamalı")
	}
	// genel iş çalışırken hiçbir iş başlamaz
	b := testApp()
	if _, err := b.newJob("", "utility", "upgrade"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.newJob("s1", "p", "durum"); err == nil || !strings.Contains(err.Error(), "genel") {
		t.Errorf("genel iş sürerken iş başlamamalı: %v", err)
	}
}

func TestBusyHelpers(t *testing.T) {
	a := testApp()
	if a.anyRunning() || a.serverBusy("s1") {
		t.Fatal("başta meşgul olmamalı")
	}
	j, _ := a.newJob("s1", "p", "durum")
	if !a.anyRunning() || !a.serverBusy("s1") || a.serverBusy("s2") {
		t.Errorf("s1 meşgul, s2 değil: any=%v s1=%v s2=%v", a.anyRunning(), a.serverBusy("s1"), a.serverBusy("s2"))
	}
	a.jmu.Lock()
	j.Done = true
	a.jmu.Unlock()
	if a.anyRunning() || a.serverBusy("s1") {
		t.Error("biten iş meşgul sayılmamalı")
	}
	// genel iş her sunucuyu meşgul eder
	if _, err := a.newJob("", "utility", "upgrade"); err != nil {
		t.Fatal(err)
	}
	if !a.serverBusy("herhangi") {
		t.Error("genel iş her sunucuyu meşgul etmeli")
	}
}

func TestJobSummariesListsAllRunning(t *testing.T) {
	a := testApp()
	a.jobs["a"] = &Job{ID: "a", Server: "s1", Started: "2026-01-01T10:00:00Z"}
	a.jobs["b"] = &Job{ID: "b", Server: "s2", Started: "2026-01-01T11:00:00Z"}
	a.jobs["c"] = &Job{ID: "c", Server: "s3", Started: "2026-01-01T12:00:00Z", Done: true}
	_, running, ids := a.jobSummaries()
	if len(ids) != 2 || ids[0] != "b" || ids[1] != "a" || running != "b" {
		t.Errorf("çalışanlar [b a], running b olmalı: %v %q", ids, running)
	}
}
