//go:build !windows

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sahte saatli uygulama
func clockApp(start time.Time) (*App, *time.Time) {
	now := start
	a := &App{host: "127.0.0.1:9999", token: "jeton"}
	a.clock = func() time.Time { return now }
	return a, &now
}

func TestSessionTicketIsNotTheTokenAndValidates(t *testing.T) {
	a, _ := clockApp(time.Unix(1_000_000, 0))
	v := a.newSession()
	if v == "" || v == a.token || len(v) < 32 {
		t.Fatalf("oturum bileti jeton olmamalı, yeterince uzun olmalı: %q", v)
	}
	if !a.sessionValid(v) {
		t.Error("yeni bilet geçerli olmalı")
	}
	for _, bad := range []string{"", "jeton", v + "x", strings.ToUpper(v) + "z"} {
		if a.sessionValid(bad) {
			t.Errorf("geçersiz bilet kabul edildi: %q", bad)
		}
	}
	if other := a.newSession(); other == v {
		t.Error("her oturum farklı bilet almalı")
	}
	// sunucuda bilet DEĞİL, özeti tutulur
	for k := range a.sessions {
		if k == v {
			t.Error("bilet düz metin saklanmamalı")
		}
	}
}

func TestSessionSlidingIdleAndAbsoluteExpiry(t *testing.T) {
	a, now := clockApp(time.Unix(1_000_000, 0))
	v := a.newSession()
	// kayan boşta süre: her yetkili istek yeniler
	for i := 0; i < 20; i++ {
		*now = now.Add(sessionIdle - time.Minute)
		if !a.sessionValid(v) {
			t.Fatalf("etkinlik süreyi yenilemeli (adım %d)", i)
		}
		if i == 12 { // 7 günü aşmadan önce
			break
		}
	}
	// boşta süre dolunca geçersiz ve silinir
	*now = now.Add(sessionIdle + time.Second)
	if a.sessionValid(v) {
		t.Error("boşta süre dolunca bilet geçersiz olmalı")
	}
	if len(a.sessions) != 0 {
		t.Error("süresi dolan bilet silinmeli")
	}
	// mutlak üst sınır: sürekli etkinlik olsa bile
	b, n2 := clockApp(time.Unix(2_000_000, 0))
	w := b.newSession()
	for el := time.Duration(0); el < sessionMax-time.Hour; el += time.Hour {
		*n2 = n2.Add(time.Hour)
		if !b.sessionValid(w) {
			t.Fatalf("üst sınıra kadar geçerli olmalı (%v)", el)
		}
	}
	*n2 = n2.Add(2 * time.Hour)
	if b.sessionValid(w) {
		t.Error("mutlak üst sınırdan sonra etkinlik olsa da geçersiz olmalı")
	}
}

func TestSessionPurgesExpiredOnNew(t *testing.T) {
	a, now := clockApp(time.Unix(1_000_000, 0))
	for i := 0; i < 5; i++ {
		a.newSession()
	}
	*now = now.Add(sessionIdle + time.Minute)
	a.newSession()
	if len(a.sessions) != 1 {
		t.Errorf("süresi dolanlar temizlenmeli: %d kayıt", len(a.sessions))
	}
}

func TestGuardUsesSessionTicketNotToken(t *testing.T) {
	a, now := clockApp(time.Unix(1_000_000, 0))
	h := a.guard(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	do := func(cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Host = a.host
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}
	v := a.newSession()
	if do(v).Code != http.StatusOK {
		t.Error("geçerli bilet kabul edilmeli")
	}
	if rec := do(a.token); rec.Code != http.StatusUnauthorized {
		t.Errorf("jetonun kendisi çerez olarak kabul edilmemeli (kod %d)", rec.Code)
	}
	if do("").Code != http.StatusUnauthorized {
		t.Error("çerezsiz istek reddedilmeli")
	}
	*now = now.Add(sessionIdle + time.Minute)
	if rec := do(v); rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "süresi doldu") {
		t.Errorf("süresi dolan bilet 401 ve açıklayıcı ileti vermeli: %d %q", rec.Code, rec.Body.String())
	}
}

func TestTermTicketOneTimeShortLivedAndBoundToSession(t *testing.T) {
	a, now := clockApp(time.Unix(1_000_000, 0))
	tk := a.issueTermTicket("t1")
	if !a.redeemTermTicket(tk, "t1") {
		t.Fatal("geçerli bilet kabul edilmeli")
	}
	if a.redeemTermTicket(tk, "t1") {
		t.Error("bilet tek kullanımlık olmalı")
	}
	tk2 := a.issueTermTicket("t1")
	if a.redeemTermTicket(tk2, "t2") {
		t.Error("başka oturum için bilet geçersiz olmalı")
	}
	if a.redeemTermTicket(tk2, "t1") {
		t.Error("yanlış oturumla denenen bilet da yanmalı")
	}
	tk3 := a.issueTermTicket("t1")
	*now = now.Add(termTicketTTL + time.Second)
	if a.redeemTermTicket(tk3, "t1") {
		t.Error("süresi dolan bilet reddedilmeli")
	}
	for _, bad := range []string{"", "yok", "t1"} {
		if a.redeemTermTicket(bad, "t1") {
			t.Errorf("bilinmeyen bilet kabul edildi: %q", bad)
		}
	}
	a.issueTermTicket("t9")
	*now = now.Add(time.Hour)
	a.issueTermTicket("t9")
	if len(a.termTickets) != 1 {
		t.Errorf("süresi dolan biletler temizlenmeli: %d kayıt", len(a.termTickets))
	}
}

func TestTermStreamNeedsTicket(t *testing.T) {
	a := &App{host: "127.0.0.1:9999", token: "jeton"}
	ts := startTest(t, a, "sleep 5")
	defer ts.kill()
	get := func(q string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/term/stream?"+q, nil)
		rec := httptest.NewRecorder()
		a.termStreamHandler(rec, req)
		return rec.Code
	}
	if c := get("id=" + ts.ID); c != http.StatusForbidden {
		t.Errorf("biletsiz akış 403 olmalı: %d", c)
	}
	if c := get("id=" + ts.ID + "&ticket=uydurma"); c != http.StatusForbidden {
		t.Errorf("sahte biletle akış 403 olmalı: %d", c)
	}
	// başka oturumun bileti bu oturumda geçmez
	other := a.issueTermTicket("baska")
	if c := get("id=" + ts.ID + "&ticket=" + other); c != http.StatusForbidden {
		t.Errorf("başka oturumun bileti 403 olmalı: %d", c)
	}
	// geçerli bilet ama oturum yok → 404 (ve bilet yanar)
	tk := a.issueTermTicket("yok")
	if c := get("id=yok&ticket=" + tk); c != http.StatusNotFound {
		t.Errorf("olmayan oturum 404 olmalı: %d", c)
	}
}

func TestTermTicketHandler(t *testing.T) {
	a := &App{host: "127.0.0.1:9999", token: "jeton"}
	ts := startTest(t, a, "sleep 5")
	defer ts.kill()
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/term/ticket", strings.NewReader(body))
		rec := httptest.NewRecorder()
		a.termTicketHandler(rec, req)
		return rec
	}
	if rec := httptest.NewRecorder(); true {
		a.termTicketHandler(rec, httptest.NewRequest(http.MethodGet, "/api/term/ticket", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET reddedilmeli: %d", rec.Code)
		}
	}
	if rec := post(`{"id":"yok"}`); rec.Code != http.StatusNotFound {
		t.Errorf("olmayan oturum 404 olmalı: %d", rec.Code)
	}
	if rec := post(`bozuk`); rec.Code != http.StatusBadRequest {
		t.Errorf("bozuk gövde 400 olmalı: %d", rec.Code)
	}
	rec := post(`{"id":"` + ts.ID + `"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ticket":"`) || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("bilet verilmeli: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
	}
	// verilen bilet akışta bir kez geçer
	var out struct{ Ticket string }
	if err := jsonDecode(rec.Body.Bytes(), &out); err != nil || out.Ticket == "" {
		t.Fatalf("yanıt çözülemedi: %v", err)
	}
	if !a.redeemTermTicket(out.Ticket, ts.ID) {
		t.Error("verilen bilet o oturum için geçerli olmalı")
	}
}

func jsonDecode(b []byte, v any) error { return json.Unmarshal(b, v) }
