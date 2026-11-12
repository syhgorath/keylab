package main

import (
	"os/exec"
	"strings"
	"testing"
)

// js/core/html.js (html“ etiketli şablon) davranış testi: node varsa çalışır (jstest/html_test.js).
func TestHTMLTemplateJS(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node yok")
	}
	out, err := exec.Command(node, "jstest/html_test.js").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "ok") {
		t.Fatalf("html.js testi: %v\n%s", err, out)
	}
}
