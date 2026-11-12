package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"keylab/internal/servers"
)

// Sunucu envanteri yedeği (dışa/içe aktarma).
//
// Güvenlik ilkeleri (pocketui'deki eski sürümden bilinçli farklar):
//   - Dışa aktarma SIR içermez: private key dosyaları/yolları (KEY), knock dizisi ve parola/token/secret adlı değişkenler yedeğe girmez.
//     Dosya yine sunucu adreslerini içerir; kullanıcıya bu söylenir.
//   - İçe aktarma istemcinin söylediğini ASLA doğrudan yazmaz: yeni sunucu createServer'dan (aynı doğrulama), var olan sunucunun adresi
//     updateServerAddr'dan, değişkenler saveVar'dan (paket doğrulayıcısı, platform, uyarı onayı) geçer. server.env'e ham anahtar/değer
//     yazılmaz; yalnız IP/USER/PORT/OS. Böylece elle düzenlenmiş/kötü niyetli bir yedek ssh argümanı ya da yürütücü değişkeni sızdıramaz.
//   - Var olan sunucu yalnızca kullanıcı AÇIKÇA seçerse değişir; önce önizleme (apply=false) gelir, hiçbir şey yazılmaz.
//   - Boyut/sayı sınırları, denetim günlüğü (backup-audit.log, 0600).

const (
	backupFormat      = "keylab-inventory"
	backupVersion     = 1
	backupMaxServers  = 500
	backupMaxVarsEach = 400
)

var reSecretVar = regexp.MustCompile(`(?i)(pass|secret|token|private|credential)`)

type backupServer struct {
	Name string                       `json:"name"`
	Host string                       `json:"host"`
	User string                       `json:"user"`
	Port string                       `json:"port"`
	OS   string                       `json:"os,omitempty"`
	Vars map[string]map[string]string `json:"vars,omitempty"` // paket -> değişken -> değer
}

type backupFile struct {
	Format   string         `json:"format"`
	Version  int            `json:"version"`
	Exported string         `json:"exported"`
	Servers  []backupServer `json:"servers"`
}

type backupRow struct {
	Name    string   `json:"name"`
	Action  string   `json:"action"`      // create | update | same | invalid
	Status  string   `json:"status"`      // planned | ok | skipped | error
	AddrChg bool     `json:"addrChanged"` // update: host/user/port farklı
	VarsChg int      `json:"varsChanged"` // yazılacak/yazılan değişken sayısı
	Notes   []string `json:"notes,omitempty"`
}

// backupExport: yedeklenebilir (geçerli) sunucular; kaç gizli görünen değişkenin dışarıda bırakıldığı da döner.
func (a *App) backupExport() (backupFile, int) {
	f := backupFile{Format: backupFormat, Version: backupVersion, Exported: time.Now().UTC().Format(time.RFC3339), Servers: []backupServer{}}
	omitted := 0
	for _, s := range a.servers(false) {
		if !s.Valid {
			continue
		}
		dir := filepath.Join(a.serversDir, s.Name)
		osID, _ := servers.EnvGet(filepath.Join(dir, "server.env"), "OS")
		b := backupServer{Name: s.Name, Host: s.Host, User: s.User, Port: s.Port, OS: osID}
		files, _ := filepath.Glob(filepath.Join(dir, "*.vars"))
		sort.Strings(files)
		for _, vf := range files {
			pkg := strings.TrimSuffix(filepath.Base(vf), ".vars")
			if !reName.MatchString(pkg) {
				continue
			}
			for name, val := range readVarsFile(vf) {
				if reSecretVar.MatchString(name) {
					omitted++
					continue
				}
				if b.Vars == nil {
					b.Vars = map[string]map[string]string{}
				}
				if b.Vars[pkg] == nil {
					b.Vars[pkg] = map[string]string{}
				}
				b.Vars[pkg][name] = val
			}
		}
		f.Servers = append(f.Servers, b)
	}
	return f, omitted
}

func (a *App) backupAudit(line string) {
	if a.stateDir == "" {
		return
	}
	_ = os.MkdirAll(a.stateDir, 0o700)
	fh, err := os.OpenFile(filepath.Join(a.stateDir, "backup-audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer fh.Close()
	fmt.Fprintf(fh, "%s %s\n", time.Now().UTC().Format(time.RFC3339), line)
}

// backupImport: apply=false → yalnız önizleme (disk değişmez). apply=true → pick listesindeki adlar uygulanır.
func (a *App) backupImport(f backupFile, apply bool, pick []string) ([]backupRow, error) {
	if f.Format != backupFormat || f.Version != backupVersion {
		return nil, fmt.Errorf("tanınmayan yedek biçimi (beklenen: %s v%d)", backupFormat, backupVersion)
	}
	if len(f.Servers) == 0 || len(f.Servers) > backupMaxServers {
		return nil, fmt.Errorf("yedekte 1–%d sunucu olmalı", backupMaxServers)
	}
	picked := map[string]bool{}
	for _, n := range pick {
		picked[n] = true
	}
	var rows []backupRow
	seen := map[string]bool{}
	for _, s := range f.Servers {
		row := backupRow{Name: s.Name}
		if seen[s.Name] {
			row.Action, row.Status = "invalid", "skipped"
			row.Notes = append(row.Notes, "yedekte aynı ad birden fazla kez geçiyor; ilki kullanıldı")
			rows = append(rows, row)
			continue
		}
		seen[s.Name] = true
		port := s.Port
		if port == "" {
			port = "22"
		}
		d := a.draftIgnoringExisting(s.Name, s.Host, s.User, port, s.OS)
		nv := 0
		for _, vs := range s.Vars {
			nv += len(vs)
		}
		if nv > backupMaxVarsEach {
			row.Action, row.Status = "invalid", "error"
			row.Notes = append(row.Notes, fmt.Sprintf("çok fazla değişken (%d > %d)", nv, backupMaxVarsEach))
			rows = append(rows, row)
			continue
		}
		for _, k := range []string{"name", "host", "user", "port"} {
			if m, bad := d[k]; bad {
				row.Action, row.Status = "invalid", "error"
				row.Notes = append(row.Notes, m)
			}
		}
		if row.Action == "invalid" {
			rows = append(rows, row)
			continue
		}
		osID := s.OS
		if _, bad := d["os"]; bad {
			osID = ""
			row.Notes = append(row.Notes, "işletim sistemi profili tanınmadı; boş bırakılacak")
		}
		dir := filepath.Join(a.serversDir, s.Name)
		exists := false
		if _, err := os.Lstat(dir); err == nil {
			exists = true
		}
		type vchg struct{ pkg, name, val string }
		var changes []vchg
		if exists {
			env := filepath.Join(dir, "server.env")
			ip, _ := servers.EnvGet(env, "IP")
			us, _ := servers.EnvGet(env, "USER")
			pt, _ := servers.EnvGet(env, "PORT")
			if pt == "" {
				pt = "22"
			}
			row.AddrChg = ip != s.Host || us != s.User || pt != port
			for pkg, vs := range s.Vars {
				cur := readVarsFile(a.varsFile(s.Name, pkg))
				for name, val := range vs {
					if old, ok := cur[name]; !ok || old != val {
						changes = append(changes, vchg{pkg, name, val})
					}
				}
			}
			row.VarsChg = len(changes)
			if !row.AddrChg && row.VarsChg == 0 {
				row.Action, row.Status = "same", "skipped"
				rows = append(rows, row)
				continue
			}
			row.Action = "update"
		} else {
			row.Action = "create"
			for pkg, vs := range s.Vars {
				for name, val := range vs {
					changes = append(changes, vchg{pkg, name, val})
				}
			}
			row.VarsChg = len(changes)
		}
		row.Status = "planned"
		if !apply {
			rows = append(rows, row)
			continue
		}
		if !picked[s.Name] {
			row.Status = "skipped"
			rows = append(rows, row)
			continue
		}
		// --- uygula: her yazma, uygulamanın kendi doğrulamasından geçer ---
		failed := false
		if row.Action == "create" {
			if code, err := a.createServer(s.Name, s.Host, s.User, port, osID); err != nil {
				row.Status = "error"
				row.Notes = append(row.Notes, fmt.Sprintf("oluşturulamadı (%d): %v", code, err))
				rows = append(rows, row)
				continue
			}
		} else if row.AddrChg {
			if code, err := a.updateServerAddr(s.Name, s.Host, s.User, port); err != nil {
				row.Status = "error"
				row.Notes = append(row.Notes, fmt.Sprintf("adres güncellenemedi (%d): %v", code, err))
				rows = append(rows, row)
				continue
			}
		}
		applied := 0
		sort.Slice(changes, func(i, j int) bool { return changes[i].pkg+"/"+changes[i].name < changes[j].pkg+"/"+changes[j].name })
		for _, c := range changes {
			if _, code, err := a.saveVar(s.Name, c.pkg, c.name, c.val, "set", false); err != nil {
				failed = true
				why := err.Error()
				if code == http.StatusConflict && strings.HasPrefix(why, "uyarı") {
					why = "riskli değer (onay gerekir); atlandı"
				}
				row.Notes = append(row.Notes, fmt.Sprintf("%s/%s yazılmadı: %s", c.pkg, c.name, why))
				continue
			}
			applied++
		}
		row.VarsChg = applied
		row.Status = "ok"
		if failed {
			row.Notes = append(row.Notes, "bazı değişkenler uygulanamadı (üstte listelendi)")
		}
		rows = append(rows, row)
	}
	if apply {
		var ok, errs int
		var names []string
		for _, r := range rows {
			switch r.Status {
			case "ok":
				ok++
				names = append(names, r.Name)
			case "error":
				errs++
			}
		}
		a.backupAudit(fmt.Sprintf("import ok=%d error=%d sunucular=%s", ok, errs, strings.Join(names, ",")))
	}
	return rows, nil
}

// draftIgnoringExisting: draft'ın "bu ad zaten var" hatası dışındaki alan hataları (var olan sunucu burada hata değil, güncelleme adayı).
func (a *App) draftIgnoringExisting(name, host, user, port, osID string) map[string]string {
	d := a.draft(name, host, user, port, osID)
	out := map[string]string{}
	for k, v := range d.Errors {
		if k == "name" && strings.Contains(v, "zaten var") {
			continue
		}
		out[k] = v
	}
	return out
}

func (a *App) backupExportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { // özel başlık zorunluluğu (CSRF) yalnız POST'ta uygulanır
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	f, omitted := a.backupExport()
	a.backupAudit(fmt.Sprintf("export sunucu=%d gizli_degisken_disarida=%d", len(f.Servers), omitted))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"file": f, "omittedSecrets": omitted})
}

func (a *App) backupImportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		File  backupFile `json:"file"`
		Apply bool       `json:"apply"`
		Pick  []string   `json:"pick"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req) != nil {
		http.Error(w, "geçersiz yedek dosyası", http.StatusBadRequest)
		return
	}
	if len(req.Pick) > backupMaxServers {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	rows, err := a.backupImport(req.File, req.Apply, req.Pick)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "applied": req.Apply, "results": rows})
}
