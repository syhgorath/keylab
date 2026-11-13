package main

// Toplu işlem: seçilen sunucularda AYNI salt okunur eylemi çalıştırır; en çok `parallel` sunucu aynı anda, fazlası sırada (pending) bekler.
//
// Güvenlik tasarımı:
//   - Yalnız ai.Runnable eylemler (salt okunur, sudo'suz, onaysız, önkoşul tamam). Yazan, sudo'lu, yıkıcı ve onay isteyen eylemler
//     toplu yapılamaz; kapıları atlamak yerine hiç sunmayız.
//   - Her sunucu için çalıştırma, tek sunucudaki /api/run ile AYNI yoldan (authorizeRunItem + startJob → `keylab run`) geçer;
//     motorun kapıları, günlüğü ve sonuç kaydı değişmez.
//   - Yetki sunucuda yeniden denetlenir: istemcinin "uygun" demesine güvenilmez; başlatırken ve her sunucuda ayrı ayrı bakılır.
//   - Eşzamanlılık: parallel, Ayarlar'daki sınırı (a.maxJobs) aşamaz; sunucu başına tek iş kuralı geçerlidir (sunucular tekilleştirilir).
//   - Tek iş kuralı korunur: toplu iş sürerken başka bir iş (tek eylem, görev, kurulum) başlatılamaz; aralarda boşluk yoktur.
//   - Sınırlar: en çok maxBatchServers sunucu, sunucu başına batchJobTimeout.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	maxBatchServers = 200
	batchJobTimeout = 10 * time.Minute // salt okunur eylem; ssh asılırsa tüm toplu işi kilitlemesin
)

type BatchItem struct {
	Server string `json:"server"`
	State  string `json:"state"` // pending | running | ok | fail | skipped | cancelled
	Reason string `json:"reason,omitempty"`
	Job    string `json:"job,omitempty"`
	Exit   int    `json:"exit"`
}

type Batch struct {
	ID         string       `json:"id"`
	Package    string       `json:"package"`
	Action     string       `json:"action"`
	Started    string       `json:"started"`
	StopOnFail bool         `json:"stopOnFail"`
	Parallel   int          `json:"parallel"`
	Done       bool         `json:"done"`
	Cancelled  bool         `json:"cancelled"`
	Items      []*BatchItem `json:"items"`
	stopReason string       // ilk hatada dur: kalan bekleyenlerin iptal nedeni (bmu altında)
}

// batchEligible: sunucuda eylem toplu çalıştırılabilir mi? (karar HER ZAMAN sunucuda verilir)
func (a *App) batchEligible(server, pkg, action string) (bool, string) {
	ai, err := a.findAction(server, pkg, action)
	if err != nil {
		return false, err.Error()
	}
	if !ai.Runnable {
		if ai.Reason != "" {
			return false, ai.Reason
		}
		return false, "toplu işlem yalnız salt okunur, onaysız eylemleri çalıştırır"
	}
	return true, ""
}

// cleanServers: ad doğrulaması, tekilleştirme ve üst sınır
func cleanServers(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !reName.MatchString(s) {
			return nil, fmt.Errorf("geçersiz sunucu adı")
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("sunucu seçilmedi")
	}
	if len(out) > maxBatchServers {
		return nil, fmt.Errorf("en çok %d sunucu seçilebilir", maxBatchServers)
	}
	return out, nil
}

type batchOption struct {
	Package string `json:"package"`
	Action  string `json:"action"`
	OK      int    `json:"ok"`
	Total   int    `json:"total"`
}

// batchOptions: seçili sunucularda en az birinde toplu çalıştırılabilen (package, action) çiftleri
func (a *App) batchOptions(servers []string) []batchOption {
	cnt := map[[2]string]int{}
	for _, name := range servers {
		s, ok := a.findServerQuiet(name)
		if !ok || !s.Valid {
			continue
		}
		for _, p := range a.packageInfos(s) {
			for _, ac := range p.Actions {
				if ac.Runnable {
					cnt[[2]string{p.ID, ac.ID}]++
				}
			}
		}
	}
	out := []batchOption{}
	for k, n := range cnt {
		out = append(out, batchOption{k[0], k[1], n, len(servers)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		return out[i].Action < out[j].Action
	})
	return out
}

func (a *App) batchActive() bool { return a.batchOn.Load() }

// startBatch: ön denetim (uygun olmayanlar "skipped") sonra arka plan yürütücüsünü başlatır.
func (a *App) startBatch(pkg, action string, servers []string, stopOnFail bool, parallel int) (*Batch, int, error) {
	if !a.batchOn.CompareAndSwap(false, true) { // önce bayrak: bundan sonra tek tek iş başlatılamaz
		return nil, http.StatusConflict, fmt.Errorf("başka bir toplu işlem çalışıyor")
	}
	release := true
	defer func() {
		if release {
			a.batchOn.Store(false)
		}
	}()
	if a.anyRunning() {
		return nil, http.StatusConflict, fmt.Errorf("başka bir eylem çalışıyor")
	}
	if parallel < 1 {
		parallel = 1
	}
	if lim := a.maxJobs(); parallel > lim {
		parallel = lim // Ayarlar'daki eşzamanlı eylem sınırını aşamaz
	}
	b := &Batch{Package: pkg, Action: action, Started: time.Now().UTC().Format(time.RFC3339), StopOnFail: stopOnFail, Parallel: parallel}
	id := make([]byte, 8)
	_, _ = rand.Read(id)
	b.ID = hex.EncodeToString(id)
	eligible := 0
	for _, s := range servers {
		it := &BatchItem{Server: s, State: "pending"}
		if ok, why := a.batchEligible(s, pkg, action); !ok {
			it.State, it.Reason = "skipped", why
		} else {
			eligible++
		}
		b.Items = append(b.Items, it)
	}
	if eligible == 0 {
		return nil, http.StatusConflict, fmt.Errorf("seçili sunucuların hiçbirinde bu eylem toplu çalıştırılamaz")
	}
	a.bmu.Lock()
	a.batch = b
	a.bmu.Unlock()
	release = false
	go a.runBatch(b)
	return b, 0, nil
}

func (a *App) batchSet(b *Batch, it *BatchItem, state, reason, job string, exit int) {
	a.bmu.Lock()
	it.State, it.Reason, it.Job, it.Exit = state, reason, job, exit
	a.bmu.Unlock()
}

func (a *App) batchCancelled(b *Batch) bool {
	a.bmu.Lock()
	defer a.bmu.Unlock()
	return b.Cancelled
}

func (a *App) runBatch(b *Batch) {
	defer func() {
		a.bmu.Lock()
		b.Done = true
		a.bmu.Unlock()
		a.batchOn.Store(false)
	}()
	// iş havuzu: en çok b.Parallel sunucu aynı anda; kuyruk (bekleyenler) sırayla beslenir, bir iş bitince sıradaki başlar
	queue := make(chan *BatchItem)
	var wg sync.WaitGroup
	for w := 0; w < b.Parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range queue {
				a.runBatchItem(b, it)
			}
		}()
	}
	for _, it := range b.Items {
		a.bmu.Lock()
		st := it.State
		a.bmu.Unlock()
		if st == "pending" {
			queue <- it
		}
	}
	close(queue)
	wg.Wait()
}

// runBatchItem: tek sunucu. İptal ya da "ilk hatada dur" işaretliyse hiç başlatmadan iptal eder.
func (a *App) runBatchItem(b *Batch, it *BatchItem) {
	a.bmu.Lock()
	cancelled, reason := b.Cancelled, b.stopReason
	a.bmu.Unlock()
	if cancelled {
		a.batchSet(b, it, "cancelled", "iptal edildi", "", 0)
		return
	}
	if reason != "" {
		a.batchSet(b, it, "cancelled", reason, "", 0)
		return
	}
	failed := func() {
		if b.StopOnFail {
			a.bmu.Lock()
			if b.stopReason == "" {
				b.stopReason = "önceki sunucuda hata: durduruldu"
			}
			a.bmu.Unlock()
		}
	}
	j, _, err := a.startJobX(true, it.Server, b.Package, b.Action, "", false, "", false)
	if err != nil { // durum arada değişmiş olabilir (kapı kapandı): bu sunucuyu atla, toplu işi bozma
		a.batchSet(b, it, "skipped", err.Error(), "", 0)
		return
	}
	a.batchSet(b, it, "running", "", j.ID, 0)
	deadline := time.Now().Add(batchJobTimeout)
	canceled, timedOut := false, false
	for {
		a.jmu.Lock()
		done, exit := j.Done, j.Exit
		a.jmu.Unlock()
		if done {
			switch {
			case exit == 0:
				a.batchSet(b, it, "ok", "", j.ID, 0)
			case timedOut:
				a.batchSet(b, it, "fail", "zaman aşımı (10 dk)", j.ID, exit)
				failed()
			case canceled:
				a.batchSet(b, it, "cancelled", "iptal edildi", j.ID, exit)
			default:
				a.batchSet(b, it, "fail", fmt.Sprintf("çıkış kodu %d", exit), j.ID, exit)
				failed()
			}
			return
		}
		if !canceled && (a.batchCancelled(b) || time.Now().After(deadline)) {
			canceled, timedOut = true, !a.batchCancelled(b)
			if j.cancel != nil {
				j.cancel()
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (a *App) batchSnapshot() *Batch {
	a.bmu.Lock()
	defer a.bmu.Unlock()
	if a.batch == nil {
		return nil
	}
	c := *a.batch
	c.Items = make([]*BatchItem, len(a.batch.Items))
	for i, it := range a.batch.Items {
		x := *it
		c.Items[i] = &x
	}
	return &c
}

// POST /api/batch/options {servers}  → {options:[…]}
// POST /api/batch {package, action, servers, stopOnFail} → {batch}
// GET  /api/batch → {batch|null}
// POST /api/batch/cancel
func (a *App) batchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(map[string]any{"batch": a.batchSnapshot()})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET/POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Package, Action string
		Servers         []string
		StopOnFail      bool
		Parallel        int
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req) != nil ||
		!reName.MatchString(req.Package) || !reAction.MatchString(req.Action) {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	servers, err := cleanServers(req.Servers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b, code, err := a.startBatch(req.Package, req.Action, servers, req.StopOnFail, req.Parallel)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"batch": b.ID})
}

func (a *App) batchOptionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Servers []string }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req) != nil {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	servers, err := cleanServers(req.Servers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"options": a.batchOptions(servers), "total": len(servers)})
}

func (a *App) batchCancelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	a.bmu.Lock()
	b := a.batch
	if b == nil || b.Done {
		a.bmu.Unlock()
		http.Error(w, "çalışan toplu işlem yok", http.StatusNotFound)
		return
	}
	b.Cancelled = true
	a.bmu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
