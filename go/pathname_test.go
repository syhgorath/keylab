package main

import (
	"os"
	"path/filepath"
	"testing"
)

// "." ve ".." sunucu/paket adı olarak kabul edilirse servers/<ad>/<paket>.vars yolu sunucu dizininin DIŞINA taşar.
func TestReNameRejectsPathLikeNames(t *testing.T) {
	for _, bad := range []string{".", "..", ".gizli", "-x", "-", "", "a/b", `a\b`, "../x", "x/..", "a b", "ü", "a;b"} {
		if reName.MatchString(bad) {
			t.Errorf("reName %q kabul etmemeli", bad)
		}
	}
	for _, ok := range []string{"web1", "linux-hardening", "kurulum", "fizik.proxmox", "a_b", "9node", "_x", "Ab-1.2"} {
		if !reName.MatchString(ok) {
			t.Errorf("reName %q kabul etmeli", ok)
		}
	}
}

func TestSaveVarRejectsDotServerAndWritesNothingOutside(t *testing.T) {
	a, _ := itemRunApp(t)
	parent := filepath.Dir(a.serversDir)
	for _, name := range []string{"..", ".", "../servers", "vm1/../..", ""} {
		if _, code, err := a.saveVar(name, "demo", "demo_mod", "docker", "set", false); err == nil || code != 400 {
			t.Errorf("saveVar(%q) reddedilmeli: kod=%d hata=%v", name, code, err)
		}
	}
	if _, code, err := a.saveVar("vm1", "../demo", "demo_mod", "docker", "set", false); err == nil || code != 400 {
		t.Errorf("paket adında yol reddedilmeli: kod=%d hata=%v", code, err)
	}
	for _, p := range []string{filepath.Join(parent, "demo.vars"), filepath.Join(a.serversDir, "demo.vars"), filepath.Join(parent, "..", "demo.vars")} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("sunucu dizininin dışına/yanlış yere dosya yazıldı: %s", p)
		}
	}
	// geçerli kullanım bozulmadı
	if _, code, err := a.saveVar("vm1", "demo", "demo_mod", "docker", "set", false); err != nil {
		t.Errorf("geçerli yazma reddedildi (%d): %v", code, err)
	}
}
