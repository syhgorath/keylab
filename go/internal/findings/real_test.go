package findings

import (
	"os"
	"path/filepath"
	"testing"
)

// Gerçek durum klasöründeki verilerle (varsa) Python ile karşılaştırma.
func TestRealStateMatchesPython(t *testing.T) {
	pyF := pyScriptLive(t, "component_findings.py")
	pyS := pyScriptLive(t, "component_scores.py")
	state := filepath.Join(os.Getenv("HOME"), ".local/share/sshpocket")
	srvs, _ := filepath.Glob(filepath.Join(state, "*", "*"))
	n := 0
	for _, d := range srvs {
		pkg, srv := filepath.Base(filepath.Dir(d)), filepath.Base(d)
		for _, f := range []string{"bulgular.tsv"} {
			code, out := runPyLive(t, pyF, state, pkg, srv, f)
			res, err := Load(state, pkg, srv, f)
			gc := 0
			if err == ErrBadHeader {
				gc = 3
			} else if err != nil {
				gc = 2
			}
			if code != gc || out != res.Format() {
				t.Errorf("bulgu %s/%s: python=%d go=%d", pkg, srv, code, gc)
			}
			n++
		}
		code, out := runPyLive(t, pyS, state, pkg, srv)
		sc, err := Scores(state, pkg, srv)
		if err != nil || code != 0 || out != FormatScores(sc) {
			t.Errorf("puan %s/%s: python=%d %q go=%q", pkg, srv, code, out, FormatScores(sc))
		}
		n++
	}
	t.Logf("%d gerçek kontrol", n)
	if n == 0 {
		t.Skip("gerçek durum verisi yok")
	}
}
