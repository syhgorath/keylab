package platform

import "testing"

func TestAppBundlePath(t *testing.T) {
	for exe, want := range map[string]string{
		"/Applications/KeyLab.app/Contents/MacOS/KeyLab":                                "/Applications/KeyLab.app",
		"/Volumes/My USB/KeyLab.app/Contents/MacOS/KeyLab":                              "/Volumes/My USB/KeyLab.app",
		"/Users/x/Developer/keylabtest/keylab":                                          "",
		"/private/var/folders/x/AppTranslocation/ID/d/KeyLab.app/Contents/MacOS/KeyLab": "/private/var/folders/x/AppTranslocation/ID/d/KeyLab.app",
	} {
		got, ok := AppBundlePath(exe)
		if got != want || ok != (want != "") {
			t.Errorf("%q → %q %v, beklenen %q", exe, got, ok, want)
		}
	}
	if !IsTranslocated("/private/var/folders/x/AppTranslocation/ID/d/KeyLab.app") || IsTranslocated("/Applications/KeyLab.app") {
		t.Error("translocation tespiti")
	}
}
