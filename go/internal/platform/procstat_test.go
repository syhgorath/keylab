package platform

import "testing"

func TestParsePS(t *testing.T) {
	out := `    1     0   0.0   1234 10-02:03:04 /sbin/launchd
 4242   100  12.5  98765       05:03 /Users/x/Developer/keylab
 4243  4242   0.1   2048       00:07 /usr/bin/ssh-agent
 4300  4242   3,5   4096    01:02:03 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome
 bozuk satır
   77    1   x.y   100   00:01 /bin/kotu
`
	ps := ParsePS(out)
	if len(ps) != 4 {
		t.Fatalf("4 geçerli satır beklenirdi, %d: %+v", len(ps), ps)
	}
	if p := ps[0]; p.PID != 1 || p.PPID != 0 || p.RSSKB != 1234 || p.ElapsedSec != 10*86400+2*3600+3*60+4 || p.Name != "launchd" {
		t.Fatalf("launchd: %+v", p)
	}
	if p := ps[1]; p.CPU != 12.5 || p.ElapsedSec != 5*60+3 || p.Name != "keylab" {
		t.Fatalf("keylab: %+v", p)
	}
	if p := ps[3]; p.CPU != 3.5 || p.Name != "Google Chrome" || p.ElapsedSec != 3600+2*60+3 {
		t.Fatalf("boşluklu yol / virgüllü cpu / saatli süre: %+v", p)
	}
}

func TestParseEtime(t *testing.T) {
	for in, want := range map[string]int64{"00:07": 7, "05:03": 303, "01:02:03": 3723, "2-00:00:01": 2*86400 + 1} {
		if got, ok := parseEtime(in); !ok || got != want {
			t.Errorf("%q: %d %v, beklenen %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "7", "a:b", "1-2", "-1:00", "1:2:3:4"} {
		if _, ok := parseEtime(bad); ok {
			t.Errorf("%q geçerli sayıldı", bad)
		}
	}
}

func TestProcessTableSeesOurOwnProcess(t *testing.T) {
	ps, err := ProcessTable()
	if err != nil {
		t.Skipf("ps yok: %v", err)
	}
	found := false
	for _, p := range ps {
		if p.PID > 0 && p.RSSKB > 0 {
			found = true
			break
		}
	}
	if !found || len(ps) < 3 {
		t.Fatalf("gerçek süreç tablosu boş/anlamsız: %d kayıt", len(ps))
	}
	if total, free, ok := DiskUsage("."); !ok || total == 0 || free > total {
		t.Fatalf("disk: %d %d %v", total, free, ok)
	}
}
