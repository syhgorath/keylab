package platform

import "testing"

func TestMountRoot(t *testing.T) {
	for in, want := range map[string]string{
		"/Volumes/test/servers":       "/Volumes/test",
		"/Volumes/My USB/servers/":    "/Volumes/My USB",
		"/Volumes/test":               "/Volumes/test",
		"/Volumes/":                   "",
		"/Volumes/../etc/servers":     "",
		"/Users/x/servers":            "",
		"../servers":                  "",
		"":                            "",
		"/Volumesx/test/servers":      "",
		"/Volumes/test/../../etc/foo": "",
	} {
		if got := (DiskutilVolumes{}).MountRoot(in); got != want {
			t.Errorf("MountRoot(%q)=%q, beklenen %q", in, got, want)
		}
	}
}
