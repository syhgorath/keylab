package main

import (
	"encoding/json"
	"io/fs"
	"keylab/internal/platform"
	"keylab/internal/runner"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Kaynaklar sayfası (GET /api/monitor): KeyLab'ın KENDİ kullanımı — süreci ve alt süreçleri (özel ssh-agent, görev terminalleri, ansible/ssh…) için CPU ve RAM,
// diskte kapladığı yer (sunucular/paketler/durum/önbellek klasörleri; taşınabilirde geçici olanlar işaretli) ve çalışanlar. macOS Etkinlik Monitörü gibi
// ama yalnız KeyLab'a ait. Notlar: pencerenin kendisi (WebKit) macOS'un ayrı süreçleridir ve burada sayılmaz; CPU yüzdesi tek çekirdeğe göredir (ps pcpu).
// Ölçümler yalnız bu uç çağrılınca yapılır (arayüz Kaynaklar sayfası açıkken 2 sn'de bir); klasör boyutları 30 sn önbelleklenir.

var processStart = time.Now()

const dirSizeTTL = 30 * time.Second

type monProc struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"` // self | agent | ssh | python | task | other
	CPU        float64 `json:"cpu"`
	RSSMB      float64 `json:"rssMB"`
	ElapsedSec int64   `json:"elapsedSec"`
}

type monDir struct {
	ID    string `json:"id"` // servers | components | state | cache
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	Temp  bool   `json:"temp"` // geçici klasör: çıkışta silinir (taşınabilir mod)
}

type monSample struct {
	PID        int       `json:"pid"`
	UptimeSec  int64     `json:"uptimeSec"`
	Cores      int       `json:"cores"`
	MemTotalMB float64   `json:"memTotalMB"`
	CPU        float64   `json:"cpu"`   // KeyLab + alt süreçler, tek çekirdeğe göre %
	RSSMB      float64   `json:"rssMB"` // KeyLab + alt süreçler
	SelfRSSMB  float64   `json:"selfRssMB"`
	Goroutines int       `json:"goroutines"`
	HeapMB     float64   `json:"heapMB"`
	Terms      int       `json:"terms"`
	Tasks      int       `json:"tasks"`
	Procs      []monProc `json:"procs"`
	Dirs       []monDir  `json:"dirs"`
	Volume     *struct {
		Path       string `json:"path"`
		TotalBytes uint64 `json:"totalBytes"`
		FreeBytes  uint64 `json:"freeBytes"`
	} `json:"volume,omitempty"`
	Note string `json:"note,omitempty"` // ölçüm kısmen yapılamadıysa (ör. ps yok)
}

type dirSizeEntry struct {
	at    time.Time
	bytes int64
}

// monitorKind: süreç adından kullanıcıya gösterilecek tür.
func monitorKind(name string, self bool) string {
	n := strings.ToLower(name)
	switch {
	case self:
		return "self"
	case n == "ssh-agent":
		return "agent"
	case n == "ssh" || n == "ssh-add" || n == "scp" || n == "ssh-keygen":
		return "ssh"
	case strings.HasPrefix(n, "python") || strings.HasPrefix(n, "ansible"):
		return "python"
	case n == "keylab" || n == "sshpocket" || n == "keylab.exe":
		return "task"
	}
	return "other"
}

// descendants: root'un tüm alt süreçleri (root dahil), PID sırasıyla.
func monDescendants(table []platform.Proc, root int) []platform.Proc {
	kids := map[int][]platform.Proc{}
	byPID := map[int]platform.Proc{}
	for _, p := range table {
		kids[p.PPID] = append(kids[p.PPID], p)
		byPID[p.PID] = p
	}
	var out []platform.Proc
	seen := map[int]bool{}
	var walk func(pid int)
	walk = func(pid int) {
		if seen[pid] {
			return
		}
		seen[pid] = true
		if p, ok := byPID[pid]; ok {
			out = append(out, p)
		}
		for _, c := range kids[pid] {
			walk(c.PID)
		}
	}
	walk(root)
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

// dirSize: klasörün toplam bayt boyutu (okunamayanlar atlanır; en çok 300 bin kayıt).
func dirSize(path string) int64 {
	var total int64
	n := 0
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if n++; n > 300000 {
			return filepath.SkipAll
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func (a *App) cachedDirSize(path string) int64 {
	a.monMu.Lock()
	if e, ok := a.monDirs[path]; ok && time.Since(e.at) < dirSizeTTL {
		a.monMu.Unlock()
		return e.bytes
	}
	a.monMu.Unlock()
	size := dirSize
	if a.dirSizeFn != nil {
		size = a.dirSizeFn
	}
	b := size(path)
	a.monMu.Lock()
	if a.monDirs == nil {
		a.monDirs = map[string]dirSizeEntry{}
	}
	a.monDirs[path] = dirSizeEntry{time.Now(), b}
	a.monMu.Unlock()
	return b
}

func (a *App) monitorSample() monSample {
	pid := os.Getpid()
	s := monSample{PID: pid, UptimeSec: int64(time.Since(processStart).Seconds()), Cores: runtime.NumCPU(), Procs: []monProc{}}
	s.MemTotalMB = float64(platform.TotalMemoryBytes()) / (1 << 20)
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.Goroutines, s.HeapMB = runtime.NumGoroutine(), float64(ms.HeapAlloc)/(1<<20)
	ps := platform.ProcessTable
	if a.psFn != nil {
		ps = a.psFn
	}
	table, err := ps()
	if err != nil {
		s.Note = "süreç tablosu okunamadı: " + err.Error()
		s.RSSMB = s.HeapMB
	} else {
		for _, p := range monDescendants(table, pid) {
			if p.Name == "ps" && p.PID != pid { // ölçümü yapan ps'in kendisi
				continue
			}
			mp := monProc{PID: p.PID, Name: p.Name, Kind: monitorKind(p.Name, p.PID == pid), CPU: p.CPU, RSSMB: float64(p.RSSKB) / 1024, ElapsedSec: p.ElapsedSec}
			s.Procs = append(s.Procs, mp)
			s.CPU += mp.CPU
			s.RSSMB += mp.RSSMB
			if p.PID == pid {
				s.SelfRSSMB = mp.RSSMB
			}
		}
	}
	a.tmu.Lock()
	for _, ts := range a.terms {
		ts.mu.Lock()
		if !ts.done {
			if ts.Kind == "task" {
				s.Tasks++
			} else {
				s.Terms++
			}
		}
		ts.mu.Unlock()
	}
	a.tmu.Unlock()
	portable, cacheDir := a.isPortable(), runner.CacheDir()
	for _, d := range []monDir{
		{ID: "servers", Path: a.serversDir}, {ID: "components", Path: a.compDir}, {ID: "state", Path: a.stateDir},
		{ID: "cache", Path: cacheDir, Temp: portable && os.Getenv("KEYLAB_CACHE_DIR") != ""},
	} {
		if d.Path == "" {
			continue
		}
		d.Bytes = a.cachedDirSize(d.Path)
		s.Dirs = append(s.Dirs, d)
	}
	if total, free, ok := platform.DiskUsage(a.serversDir); ok {
		s.Volume = &struct {
			Path       string `json:"path"`
			TotalBytes uint64 `json:"totalBytes"`
			FreeBytes  uint64 `json:"freeBytes"`
		}{a.serversDir, total, free}
	}
	return s
}

// GET /api/monitor
func (a *App) monitorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(a.monitorSample())
}
