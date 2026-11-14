package main

// Oturum bileti ve terminal akış bileti (Proxmox'taki "uzun ömürlü sır → kısa ömürlü bilet" fikri; yerel, tek kullanıcılı modelde).
//
//  1. Oturum bileti: açılış jetonu (?b=, tek kullanımlık) ya da elle açılan adres jetonu (?t=) bir ÇEREZ BİLETİ ile değiştirilir. Çerez
//     değeri jetonun kendisi değildir; rastgele bir bilettir, sunucuda yalnız özeti (SHA-256) tutulur, süresi dolar:
//     kayan boşta süre (sessionIdle; her yetkili istekte yenilenir, açık pencerenin 10 sn'lik yenilemesi canlı tutar) ve mutlak üst sınır
//     (sessionMax). Çalınan/artakalan bir çerez, pencere kapandıktan sonra boşta süre dolunca işe yaramaz. Süre dolunca arayüz 401 alır ve
//     kullanıcıya terminaldeki adresi (?t=) açması söylenir; jeton süreç boyunca geçerlidir.
//  2. Terminal akış bileti: SSE akışı (EventSource) özel başlık gönderemez; bu yüzden akış, oturum başına, KISA ömürlü (termTicketTTL),
//     TEK kullanımlık bir biletle açılır (POST /api/term/ticket ile alınır). Girdi/yeniden boyut/kapatma POST'tur ve X-Requested-With +
//     Origin denetimi taşır (bkz. guard).

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

const (
	sessionIdle   = 12 * time.Hour     // kayan boşta süre
	sessionMax    = 7 * 24 * time.Hour // mutlak üst sınır
	termTicketTTL = 30 * time.Second   // terminal akış bileti ömrü
)

type session struct{ created, exp time.Time }
type termTicket struct {
	id  string
	exp time.Time
}

func (a *App) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

func ticketKey(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newSession: yeni oturum bileti üretir (çereze yazılacak değer döner); süresi dolmuşları temizler.
func (a *App) newSession() string {
	v := randHex(24)
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	if a.sessions == nil {
		a.sessions = map[string]session{}
	}
	now := a.now()
	for k, s := range a.sessions {
		if !now.Before(s.exp) || !now.Before(s.created.Add(sessionMax)) {
			delete(a.sessions, k)
		}
	}
	a.sessions[ticketKey(v)] = session{created: now, exp: now.Add(sessionIdle)}
	return v
}

// sessionValid: bilet tanınıyor ve süresi dolmamışsa true; kayan süreyi yeniler.
func (a *App) sessionValid(v string) bool {
	if v == "" {
		return false
	}
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	k := ticketKey(v)
	s, ok := a.sessions[k]
	if !ok {
		return false
	}
	now := a.now()
	if !now.Before(s.exp) || !now.Before(s.created.Add(sessionMax)) {
		delete(a.sessions, k)
		return false
	}
	s.exp = now.Add(sessionIdle)
	a.sessions[k] = s
	return true
}

// issueTermTicket: terminal oturumu için kısa ömürlü, tek kullanımlık akış bileti.
func (a *App) issueTermTicket(id string) string {
	v := randHex(18)
	a.ttMu.Lock()
	defer a.ttMu.Unlock()
	if a.termTickets == nil {
		a.termTickets = map[string]termTicket{}
	}
	now := a.now()
	for k, t := range a.termTickets {
		if !now.Before(t.exp) {
			delete(a.termTickets, k)
		}
	}
	a.termTickets[ticketKey(v)] = termTicket{id: id, exp: now.Add(termTicketTTL)}
	return v
}

// redeemTermTicket: bilet o oturum için geçerliyse true ve bilet yanar (ikinci kullanım false).
func (a *App) redeemTermTicket(ticket, id string) bool {
	if ticket == "" {
		return false
	}
	a.ttMu.Lock()
	defer a.ttMu.Unlock()
	k := ticketKey(ticket)
	t, ok := a.termTickets[k]
	if !ok {
		return false
	}
	delete(a.termTickets, k) // doğru ya da yanlış oturum, her denemede yanar
	return t.id == id && a.now().Before(t.exp)
}

// POST /api/term/ticket {id} → {ticket}: var olan terminal oturumu için akış bileti.
func (a *App) termTicketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, "geçersiz istek", http.StatusBadRequest)
		return
	}
	if a.getTerm(req.ID) == nil {
		http.Error(w, "oturum yok", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"ticket": a.issueTermTicket(req.ID)})
}
