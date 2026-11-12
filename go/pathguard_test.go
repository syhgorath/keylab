package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerPathStaysInsideServersDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "servers")
	a := &App{serversDir: root}
	for _, bad := range [][]string{{".."}, {"."}, {"../x"}, {"a", "..", "..", "b"}, {"a/../../b"}, {"/etc/passwd"}, {"vm1", "../../x.vars"}} {
		if p := a.serverPath(bad...); p != "" && !strings.HasPrefix(p, root+string(os.PathSeparator)) && p != root {
			t.Errorf("serverPath(%q) dışarı taşmamalı: %q", bad, p)
		}
	}
	for _, bad := range [][]string{{".."}, {"../x"}, {"a", "..", "..", "b"}, {"vm1", "../../x.vars"}} {
		if p := a.serverPath(bad...); p != "" {
			t.Errorf("serverPath(%q) boş dönmeli, %q döndü", bad, p)
		}
	}
	if p := a.serverPath("vm1", "server.env"); p != filepath.Join(root, "vm1", "server.env") {
		t.Errorf("geçerli yol bozuldu: %q", p)
	}
	if p := a.serverPath("vm1"); p != filepath.Join(root, "vm1") {
		t.Errorf("sunucu klasörü bozuldu: %q", p)
	}
	if p := a.serverPath("."); p != "" {
		t.Errorf("serverPath('.') serversDir'in kendisi: boş dönmeli: %q", p)
	}
	// göreli serversDir
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	tmp := t.TempDir()
	os.Chdir(tmp)
	rel := &App{serversDir: "servers"}
	if p := rel.serverPath("vm1", "a.vars"); !strings.HasSuffix(p, filepath.Join("servers", "vm1", "a.vars")) || !filepath.IsAbs(p) {
		t.Errorf("göreli serversDir mutlak yola çevrilmeli: %q", p)
	}
	if p := rel.serverPath("..", "x"); p != "" {
		t.Errorf("göreli kökte de kaçış reddedilmeli: %q", p)
	}
}

func TestVarsFileAndSaveVarUseGate(t *testing.T) {
	a, _ := itemRunApp(t)
	if p := a.varsFile("..", "demo"); p != "" {
		t.Errorf("varsFile('..') boş dönmeli: %q", p)
	}
	if p := a.varsFile("vm1", "../demo"); p != "" {
		t.Errorf("varsFile paket yolu kaçışı boş dönmeli: %q", p)
	}
	if p := a.varsFile("vm1", "demo"); p != filepath.Join(a.serversDir, "vm1", "demo.vars") {
		t.Errorf("geçerli varsFile bozuldu: %q", p)
	}
}
