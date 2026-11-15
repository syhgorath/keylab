package main

import (
	"encoding/json"
	"keylab/internal/manifest"
	"keylab/internal/runner"
	"keylab/internal/trust"
	"net/http"
	"path/filepath"
)

// GET /api/python?package=<id> → paket eylemi başlamadan ÖNCE arayüzün sorduğu: bu paket Python (Ansible) gerektiriyor mu ve bu bilgisayarda uygun
// Python var mı? Yoksa arayüz eylemi başlatmak yerine "şunu kurun" penceresi gösterir (eylem terminalde başlayıp hata verene kadar beklenmez).
// KeyLab Python'u ve Ansible'ı kendi içinde TAŞIMAZ (boyut ve lisans yükü); kurulumu da kendisi yapmaz. Hiçbir şey çalıştırılmaz/kurulmaz;
// yalnız aday yorumlayıcıların sürümü okunur (macOS'un "geliştirici araçları" saplaması atlanır).
type pythonStatus struct {
	Needed bool     `json:"needed"` // paket Python/Ansible gerektiriyor
	OK     bool     `json:"ok"`     // gerekmiyorsa ya da uygun Python varsa true
	Range  string   `json:"range"`
	Found  []string `json:"found"` // bulunan ama aralığa uymayan sürümler
	Stub   bool     `json:"stub"`  // macOS saplaması (Command Line Tools yok) görüldü
	Guide  string   `json:"guide"` // yoksa düz metin rehber
}

func (a *App) pythonStatusFor(pkg string) pythonStatus {
	st := pythonStatus{OK: true, Found: []string{}}
	lock, err := trust.LoadLock(filepath.Join(filepath.Dir(filepath.Clean(a.compDir)), "components.lock"))
	if err != nil {
		return st // paket doğrulanamıyorsa eylem kendi hatasını verir; burada engel olmayız
	}
	ver, mf, err := trust.Prepare(a.compDir, lock, pkg)
	if err != nil {
		return st
	}
	info, _, code := manifest.ValidateFile(mf, pkg, ver)
	if code != 0 || manifest.InfoGet(info, "runner") != "ansible" {
		return st
	}
	st.Needed = true
	st.Range = manifest.InfoGet(info, "requires.python")
	if st.Range == "" {
		st.Range = ">=3.12"
	}
	if _, ok := runner.PickPython(st.Range); ok {
		return st
	}
	st.OK = false
	st.Found, st.Stub = runner.PythonScan()
	st.Guide = runner.PythonMissingGuide(st.Range)
	return st
}

func (a *App) pythonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET", http.StatusMethodNotAllowed)
		return
	}
	pkg := r.URL.Query().Get("package")
	if !reName.MatchString(pkg) {
		http.Error(w, "geçersiz paket", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a.pythonStatusFor(pkg))
}
