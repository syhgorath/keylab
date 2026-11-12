//go:build !windows

package main

import (
	"strings"
	"testing"
	"time"
)

func waitDone(t *testing.T, ts *TermSession, d time.Duration) {
	t.Helper()
	select {
	case <-ts.exited:
	case <-time.After(d):
		t.Fatal("süreç zamanında bitmedi")
	}
}

func startTest(t *testing.T, a *App, script string) *TermSession {
	t.Helper()
	ts, err := startTermProcess([]string{"/bin/sh", "-c", script}, t.TempDir(), termEnv(), 24, 80)
	if err != nil {
		t.Fatal(err)
	}
	ts.Server = "test"
	if a != nil {
		a.tmu.Lock()
		if a.terms == nil {
			a.terms = map[string]*TermSession{}
		}
		a.terms[ts.ID] = ts
		a.tmu.Unlock()
	}
	go ts.readLoop()
	go ts.waitLoop(a)
	return ts
}

func TestTermOutputAndExitCode(t *testing.T) {
	ts := startTest(t, nil, "echo merhaba; exit 3")
	waitDone(t, ts, 5*time.Second)
	time.Sleep(100 * time.Millisecond) // readLoop son parçayı bitirsin
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.exit != 3 || !ts.done {
		t.Fatalf("çıkış kodu=%d done=%v", ts.exit, ts.done)
	}
	if !strings.Contains(string(ts.ring), "merhaba") {
		t.Fatalf("çıktı yok: %q", ts.ring)
	}
}

func TestTermInputReachesProcess(t *testing.T) {
	ts := startTest(t, nil, "read x; echo GELDI:$x")
	if ok, err := ts.write([]byte("abc\n")); !ok || err != nil {
		t.Fatalf("yazma: %v %v", ok, err)
	}
	waitDone(t, ts, 5*time.Second)
	time.Sleep(100 * time.Millisecond)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if !strings.Contains(string(ts.ring), "GELDI:abc") {
		t.Fatalf("girdi işlenmedi: %q", ts.ring)
	}
}

func TestTermWriteAfterDone(t *testing.T) {
	ts := startTest(t, nil, "exit 0")
	waitDone(t, ts, 5*time.Second)
	if ok, _ := ts.write([]byte("x")); ok {
		t.Fatal("bitmiş oturuma yazıldı")
	}
}

// Regresyon (pocketui'de bulundu): reapTerms a.tmu tutarken closeTerm de a.tmu istiyordu → kilitlenme.
func TestReapOnceDoesNotDeadlock(t *testing.T) {
	a := &App{}
	idle := startTest(t, a, "sleep 30") // boşta kalacak
	fin := startTest(t, a, "exit 0")    // bitmiş
	waitDone(t, fin, 5*time.Second)
	finished := make(chan struct{})
	go func() {
		// sıfır eşikler: hepsi temizlenmeli
		a.reapOnce(time.Now().Add(time.Hour), 0, 0)
		// kilit serbest kalmış olmalı: yeni bir açılış/liste çağrısı takılmamalı
		a.tmu.Lock()
		_ = len(a.terms)
		a.tmu.Unlock()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("reapOnce kilitlendi")
	}
	a.tmu.Lock()
	n := len(a.terms)
	a.tmu.Unlock()
	if n != 0 {
		t.Fatalf("oturumlar temizlenmedi: %d", n)
	}
	waitDone(t, idle, 6*time.Second) // sleep SIGHUP ile ölmeli
}

func TestTermKillEscalates(t *testing.T) {
	// SIGHUP'u yok sayan süreç 2 sn sonra SIGKILL ile sonlandırılmalı
	ts := startTest(t, nil, "trap '' HUP; while :; do sleep 1; done")
	time.Sleep(300 * time.Millisecond)
	ts.kill()
	waitDone(t, ts, 6*time.Second)
}

func TestTermSubscriberLimit(t *testing.T) {
	ts := startTest(t, nil, "sleep 30")
	defer ts.kill()
	var chans []chan termEvent
	for i := 0; i < maxSubs; i++ {
		ch, _, _, _, ok := ts.subscribe()
		if !ok {
			t.Fatalf("%d. abone reddedildi", i)
		}
		chans = append(chans, ch)
	}
	if _, _, _, _, ok := ts.subscribe(); ok {
		t.Fatal("sınır aşıldı ama abone kabul edildi")
	}
	ts.unsubscribe(chans[0])
	if _, _, _, _, ok := ts.subscribe(); !ok {
		t.Fatal("yer açılınca abone reddedildi")
	}
}

func TestTermEnvAllowlist(t *testing.T) {
	t.Setenv("SSHPOCKET_SECRET", "x")
	t.Setenv("KEYLAB_TOKEN", "x")
	t.Setenv("ANSIBLE_X", "x")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "x")
	t.Setenv("SSH_AUTH_SOCK", "/tmp/agent.sock")
	env := strings.Join(termEnv(), "\n")
	for _, bad := range []string{"SSHPOCKET_", "KEYLAB_", "ANSIBLE_", "AWS_"} {
		if strings.Contains(env, bad) {
			t.Errorf("ortamda %s var:\n%s", bad, env)
		}
	}
	for _, want := range []string{"TERM=xterm-256color", "SSH_AUTH_SOCK=/tmp/agent.sock", "LANG=en_US.UTF-8"} {
		if !strings.Contains(env, want) {
			t.Errorf("ortamda %s yok", want)
		}
	}
}

func TestOpenTermRejectsUnknownServer(t *testing.T) {
	a := &App{serversDir: t.TempDir(), stateDir: t.TempDir()}
	if _, code, err := a.openTerm("yok", 24, 80); err == nil || code != 404 {
		t.Fatalf("beklenen 404, gelen %d %v", code, err)
	}
	if _, code, err := a.openTerm("../etc", 24, 80); err == nil || code != 400 {
		t.Fatalf("geçersiz ad: beklenen 400, gelen %d %v", code, err)
	}
}
