package platform

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Süreç/kaynak ölçümleri (KeyLab'ın kendi kullanımını göstermek için): süreç tablosu (ps), disk doluluğu, toplam bellek. İşletim sistemine özgü
// kısımlar procstat_unix.go / procstat_windows.go içindedir; burada yalnız ortak ayrıştırma vardır.

// Proc: bir sürecin anlık değerleri.
type Proc struct {
	PID, PPID  int
	CPU        float64 // % (tek çekirdeğe göre; ps pcpu)
	RSSKB      int64   // fiziksel bellek (KB)
	ElapsedSec int64   // çalışma süresi
	Name       string  // yürütülebilir dosyanın adı (yolsuz)
}

// ParsePS: `ps -axo pid=,ppid=,pcpu=,rss=,etime=,comm=` çıktısını ayrıştırır; bozuk satırlar atlanır.
func ParsePS(out string) []Proc {
	var procs []Proc
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		cpu, e3 := strconv.ParseFloat(strings.ReplaceAll(f[2], ",", "."), 64)
		rss, e4 := strconv.ParseInt(f[3], 10, 64)
		el, ok := parseEtime(f[4])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || !ok {
			continue
		}
		// comm yolu boşluk içerebilir ("/Applications/Google Chrome.app/…"): kalan alanlar birleştirilir, yalnız son bileşen alınır
		name := filepath.Base(strings.Join(f[5:], " "))
		procs = append(procs, Proc{PID: pid, PPID: ppid, CPU: cpu, RSSKB: rss, ElapsedSec: el, Name: name})
	}
	return procs
}

// parseEtime: [[gg-]ss:]dd:ss biçimi (ps etime) → saniye.
func parseEtime(s string) (int64, bool) {
	days := int64(0)
	if i := strings.IndexByte(s, '-'); i >= 0 {
		d, err := strconv.ParseInt(s[:i], 10, 64)
		if err != nil {
			return 0, false
		}
		days, s = d, s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var total int64
	for _, p := range parts {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		total = total*60 + n
	}
	return days*86400 + total, true
}
