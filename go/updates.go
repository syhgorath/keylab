// Paket güncelleme: imzalı indeks denetimi ve yükseltme.
// (main.go'dan mekanik olarak bölündü; davranış değişmedi.)
package main

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"keylab/internal/trust"
)

/* ---------- paket güncelleme: imzalı indeks denetimi ve yükseltme, sshpocket'in kendi motoruyla ---------- */

type UpdateRow struct {
	ID        string `json:"id"`
	Installed string `json:"installed"` // "-" = kurulu değil
	Verdict   string `json:"verdict"`   // upgrade | install | current | yanked | needs-sshpocket | other-repo | below-floor | conflict | unknown
	Version   string `json:"version"`   // önerilen sürüm (varsa)
	Text      string `json:"text"`      // motorun insan okunur kararı
}

type UpdateReport struct {
	OK     bool        `json:"ok"`
	Serial string      `json:"serial"`
	Valid  string      `json:"valid"`
	Rows   []UpdateRow `json:"rows"`
	Raw    string      `json:"raw"`
	Exit   int         `json:"exit"`
}

// checkUpdates: imzalı paket indeksini Go güven katmanıyla (internal/trust) alır ve doğrular; yalnızca raporlar, kurmaz.
// Doğrulama sırası: indir → imza (index_signers) → şema/süre → seri (geri alma yok) → durumu ilerlet.
func (a *App) checkUpdates() (UpdateReport, error) {
	root := a.engineRoot()
	a.jmu.Lock()
	busy := a.running != nil && !a.running.Done
	a.jmu.Unlock()
	if busy {
		return UpdateReport{}, fmt.Errorf("çalışan bir eylem varken güncelleme denetlenemez")
	}
	lock, err := trust.LoadLock(filepath.Join(root, "components.lock"))
	if err != nil {
		return UpdateReport{Rows: []UpdateRow{}, Raw: err.Error(), Exit: 1}, nil
	}
	c := &trust.Client{
		ComponentsDir: a.compDir,
		SignersFile:   signersPath(root, "index_signers"),
		SSHPocketVer:  trust.ReadVersionFile(filepath.Join(root, "VERSION")),
		Download:      a.idxDL,
	}
	idx, err := c.Refresh()
	if errors.Is(err, trust.ErrDisabled) {
		return UpdateReport{Rows: []UpdateRow{}, Raw: "ℹ️  paket indeksi kapalı (index_signers boş).", Exit: 0}, nil
	}
	if err != nil {
		return UpdateReport{Rows: []UpdateRow{}, Raw: err.Error(), Exit: 1}, nil
	}
	rep := UpdateReport{OK: true, Serial: strconv.FormatInt(idx.Serial, 10), Valid: idx.ValidUntil, Rows: []UpdateRow{}}
	var raw strings.Builder
	fmt.Fprintf(&raw, "Paket indeksi (seri %s, geçerlilik: %s)\n", rep.Serial, rep.Valid)
	for _, r := range c.Report(idx, lock) {
		inst := r.Installed
		if inst == "" {
			inst = "-"
		}
		row := UpdateRow{ID: r.ID, Installed: inst, Verdict: r.Kind, Text: r.Text()}
		switch r.Kind {
		case "upgrade", "install", "needs-sshpocket":
			row.Version = r.Detail
		}
		rep.Rows = append(rep.Rows, row)
		fmt.Fprintf(&raw, "  %-20s kurulu %-8s %s\n", r.ID, inst, row.Text)
	}
	rep.Raw = raw.String()
	return rep, nil
}

// startUpgrade: indeksin gösterdiği sürümü Go güven katmanıyla (internal/trust) indirip kurar. Sunucu tarafı yeniden
// denetim: indeks yeniden alınıp imzası doğrulanır; karar upgrade/install olmalı ve sürüm kullanıcının onayladığıyla
// AYNI olmalıdır (onay ile kurulum arasında indeks değiştiyse durur). İmza, özet, depo, sürüm kapıları trust.InstallArchive'dadır.
// İş, arayüzün mevcut iş izleyicisine akar (job.buf); kurulum atomiktir, iptal edilemez.
func (a *App) startUpgrade(pkg, version string) (*Job, int, error) {
	if !trust.IDValid(pkg) || !trust.VersionValid(version) {
		return nil, http.StatusBadRequest, fmt.Errorf("geçersiz paket kimliği ya da sürüm")
	}
	job, err := a.newJob("", pkg, "upgrade")
	if err != nil {
		return nil, http.StatusConflict, err
	}
	job.cancel = func() {}
	emit := func(format string, args ...any) {
		a.jmu.Lock()
		job.buf = append(job.buf, []byte(fmt.Sprintf(format, args...)+"\n")...)
		a.jmu.Unlock()
	}
	go func() {
		exit := 1
		defer func() {
			a.jmu.Lock()
			job.Done, job.Exit = true, exit
			a.jmu.Unlock()
		}()
		root := a.engineRoot()
		lock, err := trust.LoadLock(filepath.Join(root, "components.lock"))
		if err != nil {
			emit("✖ %v", err)
			return
		}
		c := &trust.Client{
			ComponentsDir: a.compDir,
			SignersFile:   signersPath(root, "index_signers"),
			SSHPocketVer:  trust.ReadVersionFile(filepath.Join(root, "VERSION")),
			Download:      a.idxDL,
		}
		emit("İmzalı paket indeksi alınıyor ve doğrulanıyor…")
		idx, err := c.Refresh()
		if err != nil {
			emit("✖ %v", err)
			return
		}
		emit("✔ indeks doğrulandı (seri %d, geçerlilik %s)", idx.Serial, idx.ValidUntil)
		v := c.Eval(idx, lock, pkg)
		if v.Kind != "upgrade" && v.Kind != "install" {
			emit("✖ %s: kurulabilir ya da yükseltilebilir sürüm yok (%s)", pkg, v.Text())
			return
		}
		if v.Detail != version {
			emit("✖ %s: indeks değişmiş (onayladığınız %s, şimdi %s); güncellemeyi yeniden denetleyin", pkg, version, v.Detail)
			return
		}
		if old := trust.InstalledVersion(a.compDir, pkg); old == "" {
			emit("%s %s indirilip imzası ve özeti doğrulanarak kurulacak…", pkg, version)
		} else {
			emit("%s %s → %s indirilip imzası ve özeti doğrulanarak kurulacak…", pkg, old, version)
		}
		msg, err := trust.InstallFromIndex(a.compDir, lock, idx, pkg, a.idxDL)
		if err != nil {
			emit("✖ %v", err)
			emit("Eski sürüm olduğu gibi duruyor.")
			return
		}
		emit("✔ %s", msg)
		emit("Not: sürüm farkı ve sunucu etkisi raporu bu yolda henüz yok; yeni sürümün CHANGELOG'una bakın.")
		exit = 0
	}()
	return job, 0, nil
}
