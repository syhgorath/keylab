package envx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetOrder(t *testing.T) {
	t.Setenv("KEYLAB_X1", "")
	t.Setenv("SSHPOCKET_X1", "eski")
	if Get("X1") != "eski" {
		t.Fatal("eski ad yedek olmalı")
	}
	t.Setenv("KEYLAB_X1", "yeni")
	if Get("X1") != "yeni" {
		t.Fatal("yeni ad kazanmalı")
	}
}

func TestPickDir(t *testing.T) {
	d := t.TempDir()
	yeni, eski := filepath.Join(d, "keylab"), filepath.Join(d, "sshpocket")
	if PickDir(yeni, eski) != yeni {
		t.Fatal("ikisi de yokken yeni")
	}
	os.Mkdir(eski, 0o755)
	if PickDir(yeni, eski) != eski {
		t.Fatal("yalnız eski varken eski")
	}
	os.Mkdir(yeni, 0o755)
	if PickDir(yeni, eski) != yeni {
		t.Fatal("yeni varsa yeni")
	}
}
