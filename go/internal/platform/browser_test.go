package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestChromiumCandidatesOrder(t *testing.T) {
	c := (MacBrowser{}).ChromiumCandidates("/Users/x")
	want := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Users/x/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Users/x/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Users/x/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Users/x/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	if len(c) != len(want) {
		t.Fatalf("aday sayısı %d, beklenen %d: %v", len(c), len(want), c)
	}
	for i := range want {
		if c[i] != want[i] {
			t.Errorf("aday %d: %q, beklenen %q", i, c[i], want[i])
		}
	}
}

func TestMacBrowserOpenURL(t *testing.T) {
	bin, log := t.TempDir(), filepath.Join(t.TempDir(), "calls.log")
	script := "#!/bin/bash\necho \"$*\" >> \"" + log + "\"\n"
	if err := os.WriteFile(filepath.Join(bin, "open"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if err := DefaultBrowser().OpenURL("http://127.0.0.1:1234/?b=abc"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ { // Start beklemez
		if b, _ := os.ReadFile(log); strings.TrimSpace(string(b)) != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	b, _ := os.ReadFile(log)
	if got := strings.TrimSpace(string(b)); got != "http://127.0.0.1:1234/?b=abc" {
		t.Fatalf("open argümanları: %q", got)
	}
}
