package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Arayüz testleri (jsdom): bkz. webtest/README.md. node ve webtest/node_modules yoksa atlanır (CI'da ayrı iş koşar).
func TestWebUI(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node yok")
	}
	if _, err := os.Stat("webtest/node_modules/jsdom"); err != nil {
		t.Skip("webtest/node_modules yok (cd webtest && npm ci --ignore-scripts)")
	}
	cmd := exec.Command(node, "--test")
	cmd.Dir = "webtest"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("arayüz testleri başarısız:\n%s", tail(string(out), 4000))
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + strings.TrimLeft(s[len(s)-n:], "\n")
}
