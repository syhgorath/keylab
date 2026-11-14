package platform

import (
	"io"
	"keylab/internal/eject"
	"path/filepath"
	"strings"
)

// DiskutilVolumes: macOS birimleri (/Volumes/<ad>; çıkarma `diskutil` ile, internal/eject).
type DiskutilVolumes struct{}

// MountRoot: yol /Volumes/<ad>/... ise "/Volumes/<ad>", değilse "".
func (DiskutilVolumes) MountRoot(path string) string {
	p := filepath.ToSlash(filepath.Clean(path))
	const pre = "/Volumes/"
	if !strings.HasPrefix(p, pre) {
		return ""
	}
	name := strings.SplitN(strings.TrimPrefix(p, pre), "/", 2)[0]
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return pre + name
}

func (DiskutilVolumes) Eject(w io.Writer, root string) int { return eject.Run(w, root) }

func (DiskutilVolumes) WillDefer(root string) bool { return eject.WillDefer(root) }
