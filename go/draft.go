// Sunucu ekleme taslağı: doğrular ve önizler, dosya yazmaz.
// (main.go'dan mekanik olarak bölündü; davranış değişmedi.)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

/* ---------- sunucu ekleme TASLAĞI: doğrular ve önizler, DOSYA YAZMAZ ---------- */

var (
	reNewName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)             // sshpocket module_name_valid
	reNewHost = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)            // ssh_validate_target
	reNewUser = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`) // '-' ile başlayamaz
)

type Profile struct {
	ID      string            `json:"id"`
	Support map[string]string `json:"support"` // paket → supported | planned
}

// paketlerin component.json profiles listelerinin birleşimi (OS= değeri bu kimliklerden biri olur)
func (a *App) profiles() []Profile {
	pkgs, _ := a.packages()
	byID := map[string]*Profile{}
	var order []string
	for _, p := range pkgs {
		if !reName.MatchString(p.ID) || !reVer.MatchString(p.Version) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.compDir, p.ID, p.Version, "component.json"))
		if err != nil {
			continue
		}
		var c struct {
			Profiles []struct{ ID, Status string } `json:"profiles"`
		}
		if json.Unmarshal(b, &c) != nil {
			continue
		}
		for _, pr := range c.Profiles {
			if !reName.MatchString(pr.ID) {
				continue
			}
			if byID[pr.ID] == nil {
				byID[pr.ID] = &Profile{ID: pr.ID, Support: map[string]string{}}
				order = append(order, pr.ID)
			}
			byID[pr.ID].Support[p.ID] = pr.Status
		}
	}
	sort.Strings(order)
	out := []Profile{}
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out
}

type Draft struct {
	OK       bool              `json:"ok"`
	Errors   map[string]string `json:"errors"`
	EnvPath  string            `json:"envPath"`
	EnvText  string            `json:"envText"`
	KeyPath  string            `json:"keyPath"`
	Commands []string          `json:"commands"`
}

func (a *App) draft(name, host, user, port, osID string) Draft {
	d := Draft{Errors: map[string]string{}}
	if name == "" {
		d.Errors["name"] = "Sunucu adı gerekli."
	} else if !reNewName.MatchString(name) {
		d.Errors["name"] = "Küçük harfle başlamalı; yalnızca küçük harf, rakam ve _ kullanılabilir."
	} else if len(name) > 40 {
		d.Errors["name"] = "En fazla 40 karakter."
	} else if _, err := os.Stat(filepath.Join(a.serversDir, name)); err == nil {
		d.Errors["name"] = "Bu adda bir sunucu zaten var (üzerine yazılmaz)."
	}
	if host == "" {
		d.Errors["host"] = "IP ya da ana makine adı gerekli."
	} else if !reNewHost.MatchString(host) || len(host) > 253 {
		d.Errors["host"] = "Yalnızca harf, rakam, . : _ - kullanılabilir."
	}
	if user == "" {
		d.Errors["user"] = "Kullanıcı gerekli."
	} else if !reNewUser.MatchString(user) || len(user) > 32 {
		d.Errors["user"] = "Harf, rakam, _ . - ; '-' ile başlayamaz."
	}
	if port == "" {
		port = "22"
	}
	if n := 0; !rePort.MatchString(port) {
		d.Errors["port"] = "Port bir sayı olmalı."
	} else if fmt.Sscanf(port, "%d", &n); n < 1 || n > 65535 {
		d.Errors["port"] = "Port 1-65535 arasında olmalı."
	}
	if osID != "" {
		known := false
		for _, p := range a.profiles() {
			if p.ID == osID {
				known = true
			}
		}
		if !known {
			d.Errors["os"] = "Bilinmeyen profil (paketlerin profiles listesinde yok)."
		}
	}
	if len(d.Errors) > 0 {
		return d
	}
	d.OK = true
	base := "servers/" + name
	d.EnvPath = base + "/server.env"
	d.KeyPath = base + "/id_ed25519_" + name
	env := "IP=" + host + "\nUSER=" + user + "\nPORT=" + port + "\n"
	if osID != "" {
		env += "OS=" + osID + "\n"
	}
	d.EnvText = env
	d.Commands = []string{
		"ssh-keygen -t ed25519 -a 100 -C " + name + " -f " + shQuote(d.KeyPath),
		"ssh-copy-id -i " + shQuote(d.KeyPath+".pub") + " -p " + port + " " + user + "@" + host,
	}
	return d
}
