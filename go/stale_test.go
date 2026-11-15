package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

type staleFx struct {
	root   string
	procs  map[int]string
	killed []int
}

func (f *staleFx) env() staleEnv {
	return staleEnv{root: f.root, self: 4242, procs: func() (map[int]string, error) { return f.procs, nil },
		kill: func(pid int) error { f.killed = append(f.killed, pid); return nil }, owned: func(fs.FileInfo) bool { return true }}
}

func newStaleFx(t *testing.T) *staleFx {
	t.Helper()
	return &staleFx{root: t.TempDir(), procs: map[int]string{4242: "/Applications/KeyLab.app/Contents/MacOS/KeyLab", 1: "/sbin/launchd"}}
}

// mk: root/keylab-<id> klasörü (owner=0 ise owner.pid yok = eski sürümden kalma) ve içinde ajan soketi + önbellek.
func (f *staleFx) mk(t *testing.T, id string, owner int) string {
	t.Helper()
	d := filepath.Join(f.root, "keylab-"+id)
	os.MkdirAll(filepath.Join(d, "cache"), 0o700)
	os.WriteFile(filepath.Join(d, "cache", "x"), []byte("venv"), 0o600)
	if owner > 0 {
		os.WriteFile(filepath.Join(d, ownerFile), []byte(strconv.Itoa(owner)+"\n"), 0o600)
	}
	return d
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

// Sahibi ölü: klasör silinir ve SOKETİ O KLASÖRÜ GÖSTEREN özel ajan durdurulur; başka ajanlara dokunulmaz.
func TestStaleDeadOwnerCleanedAndOnlyItsAgentKilled(t *testing.T) {
	f := newStaleFx(t)
	d := f.mk(t, "111", 9999) // 9999 yok → ölü
	other := f.mk(t, "222", 4243)
	f.procs[4243] = "/Volumes/USB/KeyLab.app/Contents/MacOS/KeyLab" // 222'nin sahibi canlı
	f.procs[7001] = "ssh-agent -D -a " + filepath.Join(d, "agent.sock")
	f.procs[7002] = "ssh-agent -D -a " + filepath.Join(other, "agent.sock") // canlı oturumun ajanı
	f.procs[7003] = "/usr/bin/ssh-agent -l"                                 // kullanıcının kendi ajanı
	rep := cleanStaleSessions(f.env())
	if rep.Dirs != 1 || rep.Agents != 1 || exists(d) || !exists(other) {
		t.Fatalf("rapor %+v, ölü klasör var=%v, canlı klasör var=%v", rep, exists(d), exists(other))
	}
	if len(f.killed) != 1 || f.killed[0] != 7001 {
		t.Fatalf("yalnız ölü oturumun ajanı durdurulmalı: %v", f.killed)
	}
}

// Sahibi canlı ya da bu süreç: dokunulmaz. PID başka bir programa geçmişse (PID yeniden kullanımı) KeyLab değildir → eski sayılır.
func TestStaleLiveSelfAndReusedPID(t *testing.T) {
	f := newStaleFx(t)
	live := f.mk(t, "1", 4243)
	f.procs[4243] = "/Users/x/Developer/keylabtest/keylab -no-open"
	self := f.mk(t, "2", 4242)
	reused := f.mk(t, "3", 5555)
	f.procs[5555] = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	rep := cleanStaleSessions(f.env())
	if !exists(live) || !exists(self) || exists(reused) || rep.Dirs != 1 {
		t.Fatalf("canlı=%v kendi=%v yeniden-kullanılan-pid=%v rapor=%+v", exists(live), exists(self), exists(reused), rep)
	}
}

// owner.pid'siz (eski sürümden kalma) klasör: başka bir KeyLab çalışıyorsa DOKUNULMAZ (onun klasörü olabilir); hiçbiri yoksa temizlenir.
func TestStaleLegacyDirOnlyWhenNoOtherKeyLab(t *testing.T) {
	f := newStaleFx(t)
	legacy := f.mk(t, "9", 0)
	f.procs[67004] = "/Applications/KeyLab.app/Contents/MacOS/KeyLab"
	if rep := cleanStaleSessions(f.env()); rep.Dirs != 0 || !exists(legacy) {
		t.Fatalf("çalışan başka KeyLab varken eski klasör silindi: %+v", rep)
	}
	delete(f.procs, 67004)
	if rep := cleanStaleSessions(f.env()); rep.Dirs != 1 || exists(legacy) {
		t.Fatalf("başka KeyLab yokken eski klasör temizlenmeliydi: %+v", rep)
	}
}

// Yanlış şeye dokunma: sembolik bağ (hedefi silinmez), başka adlar, başkasının klasörü, süreç tablosu okunamıyorsa hiçbir şey.
func TestStaleNeverTouchesSymlinksOtherNamesForeignOwnersOrWhenBlind(t *testing.T) {
	f := newStaleFx(t)
	target := t.TempDir()
	os.WriteFile(filepath.Join(target, "onemli"), []byte("x"), 0o600)
	os.Symlink(target, filepath.Join(f.root, "keylab-5"))
	other := filepath.Join(f.root, "keylab-abc") // rakam değil
	os.MkdirAll(other, 0o700)
	os.MkdirAll(filepath.Join(f.root, "baska-123"), 0o700)
	foreign := f.mk(t, "7", 9999)
	env := f.env()
	env.owned = func(fi fs.FileInfo) bool { return fi.Name() != "keylab-7" } // keylab-7 başka kullanıcının
	rep := cleanStaleSessions(env)
	if rep.Dirs != 0 || !exists(filepath.Join(target, "onemli")) || !exists(other) || !exists(filepath.Join(f.root, "baska-123")) || !exists(foreign) {
		t.Fatalf("yanlış şeye dokunuldu: %+v", rep)
	}
	dead := f.mk(t, "8", 9999)
	blind := f.env()
	blind.procs = func() (map[int]string, error) { return nil, os.ErrNotExist }
	if rep := cleanStaleSessions(blind); rep.Dirs != 0 || !exists(dead) {
		t.Fatalf("süreç tablosu okunamıyorken silindi: %+v", rep)
	}
}

func TestIsKeyLabProcess(t *testing.T) {
	for cmd, want := range map[string]bool{
		"/Applications/KeyLab.app/Contents/MacOS/KeyLab": true, "/Volumes/My USB/KeyLab.app/Contents/MacOS/KeyLab": true,
		"./keylab -no-open": true, "/x/sshpocket": true, "ssh-agent -D -a /var/folders/x/T/keylab-123/agent.sock": false,
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome": false, "/usr/bin/vim keylab.go": false,
	} {
		if got := isKeyLabProcess(cmd); got != want {
			t.Errorf("%q → %v, beklenen %v", cmd, got, want)
		}
	}
}
