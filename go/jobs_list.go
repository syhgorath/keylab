package main

import (
	"encoding/json"
	"net/http"
	"sort"
)

// GET /api/jobs: bellekteki işlerin özeti (ÇIKTI YOK; çıktı /api/run?id= ile alınır). Arayüz, sayfa yenilense ya da pencere arka plana
// alınsa bile çalışan işi bulup yeniden bağlanabilsin diye kullanır. Salt okunur; tek iş kuralı (a.running) değişmez.
type jobSummary struct {
	ID      string `json:"id"`
	Server  string `json:"server"`
	Package string `json:"package"`
	Action  string `json:"action"`
	Started string `json:"started"`
	Done    bool   `json:"done"`
	Exit    int    `json:"exit"`
}

func (a *App) jobSummaries() (list []jobSummary, running string) {
	a.jmu.Lock()
	defer a.jmu.Unlock()
	list = make([]jobSummary, 0, len(a.jobs))
	for _, j := range a.jobs {
		list = append(list, jobSummary{j.ID, j.Server, j.Package, j.Action, j.Started, j.Done, j.Exit})
	}
	sort.Slice(list, func(i, k int) bool { return list[i].Started > list[k].Started }) // yeni → eski
	if a.running != nil && !a.running.Done {
		running = a.running.ID
	}
	return list, running
}

func (a *App) jobsHandler(w http.ResponseWriter, r *http.Request) {
	list, running := a.jobSummaries()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jobs": list, "running": running})
}
