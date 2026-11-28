package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"keylab/internal/manifest"
)

// Tek tık zinciri (arayüzdeki "Kur"): önizleme → uygulama → kontrol TEK koşuda, aynı terminalde. Her adım kendi kapılarından geçer, kendi
// günlüğünü ve sonuç kaydını yazar; bir adım başarısızsa sonrakiler çalışmaz. Sudo parolası bir kez sorulur: yalnız bu sürecin belleğinde tutulur,
// Ansible'a boru ile (--become-password-file /dev/fd/3) verilir; diske, günlüğe, komut satırına ya da ortam değişkenine yazılmaz.

// Chain: zincirin adımları.
type Chain struct {
	Pre    []string // eylemin önkoşulları (salt okunur; önizleme)
	Action string   // değiştiren eylem
	Post   []string // eylemin dokunduğu ayar kartlarının okumaları (kontrol)
	Ask    bool     // inceleme zinciri: eylemin e/h onayını düğme vermez, önizlemeden sonra terminalde sorulur
}

// Steps: koşulma sırasıyla bütün adımlar.
func (c Chain) Steps() []string {
	return append(append(append([]string(nil), c.Pre...), c.Action), c.Post...)
}

// ChainPlan: zinciri manifestten çıkarır. İki tür vardır: bir ayar kartının uygulama eylemi ("Kur": düğme e/h onayıdır, sonunda kontrol) ve
// kartı olmayan, önizlemesi (requires) bulunan e/h onaylı eylem ("Önizle ve uygula": e/h terminalde sorulur; yalnız `<kart>_kaldir` eyleminde sonunda o kart okunur). Yıkıcı eylem
// ancak sunucu adı onayı istiyorsa ve bir karta bağlıysa kabul edilir (o onay terminalde elle verilir).
func ChainPlan(info []string, action, platform string) (Chain, error) {
	c := Chain{Action: action}
	if !Mutating(info, action) {
		return c, errors.New("Zincir yalnız değişiklik yapan eylem içindir.")
	}
	reads, linked := affectedReads(info, action)
	c.Ask = !linked
	if c.Ask && (manifest.ActionGet(info, action, "confirm") != "yes-no" || manifest.ActionGet(info, action, "requires") == "") {
		return c, fmt.Errorf("'%s' zincirlenmez: bir ayar kartının uygulama eylemi ya da önizlemesi olan e/h onaylı bir eylem olmalı.", action)
	}
	// uygulama kaldırma (`<kart>_kaldir`): ayar eylemi değildir ama sonunda o kart yeniden okunur ki rozet eski kalmasın
	if card, removal := strings.CutSuffix(action, "_kaldir"); c.Ask && removal {
		reads = cardReads(info, card)
	}
	if has(strings.Split(manifest.ActionGet(info, action, "effects"), ","), "destructive") && manifest.ActionGet(info, action, "confirm") != "type-server-name" {
		return c, fmt.Errorf("'%s' yıkıcı ve sunucu adı onayı istemiyor; zincirlenmez.", action)
	}
	for _, r := range strings.Split(manifest.ActionGet(info, action, "requires"), ",") {
		if r == "" {
			continue
		}
		if Mutating(info, r) {
			return c, fmt.Errorf("Önkoşul '%s' değişiklik yapıyor; zincirlenmez.", r)
		}
		c.Pre = append(c.Pre, r)
	}
	for _, r := range reads {
		confirm := manifest.ActionGet(info, r, "confirm")
		if !has(c.Post, r) && !has(c.Pre, r) && !Mutating(info, r) && (confirm == "" || confirm == "none") && manifest.ActionOnPlatform(info, r, platform) {
			c.Post = append(c.Post, r)
		}
	}
	return c, nil
}

// affectedReads: eylemin dokunduğu okumalar: onu apply_via'da sayan ayarların okuması ile o ayarların kartlarındaki (diğer ayarların ve sağlık
// özetinin) okumalar. İlgisiz kartlar kontrol edilmez: paket büyüdükçe tek tık uzamasın. linked: eylem en az bir ayarın uygulama eylemi.
func affectedReads(info []string, action string) (reads []string, linked bool) {
	ids, cards := map[string]bool{}, map[string]bool{} // "setting.<id>" ve "setting_card.<id>"
	for _, line := range info {
		key, val, ok := strings.Cut(line, "=")
		if id, apply := strings.CutSuffix(key, ".apply"); ok && apply && strings.HasPrefix(id, "setting.") && has(strings.Split(val, ","), action) {
			ids[id], linked = true, true
		}
	}
	for _, line := range info {
		key, val, ok := strings.Cut(line, "=")
		if id, card := strings.CutSuffix(key, ".card"); ok && card && ids[id] {
			cards["setting_card."+val] = true
		}
	}
	for _, line := range info {
		key, val, ok := strings.Cut(line, "=")
		if id, card := strings.CutSuffix(key, ".card"); ok && card && cards["setting_card."+val] {
			ids[id] = true
		}
	}
	for _, line := range info {
		key, val, ok := strings.Cut(line, "=")
		if id, read := strings.CutSuffix(key, ".read"); ok && read && (ids[id] || cards[id]) {
			reads = append(reads, val)
		}
	}
	return reads, linked
}

// cardReads: bir ayar kartının okumaları: kartın sağlık özeti ve karttaki ayarlar.
func cardReads(info []string, card string) (reads []string) {
	ids := map[string]bool{"setting_card." + card: true}
	for _, line := range info {
		key, val, ok := strings.Cut(line, "=")
		if id, in := strings.CutSuffix(key, ".card"); ok && in && val == card {
			ids[id] = true
		}
	}
	for _, line := range info {
		key, val, ok := strings.Cut(line, "=")
		if id, read := strings.CutSuffix(key, ".read"); ok && read && ids[id] {
			reads = append(reads, val)
		}
	}
	return reads
}

func has(list []string, x string) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}

// runChain: adımları sırayla koşar; çıkış kodu ilk başarısız adımınkidir. record her koşulan adım için çağrılır.
func (ec *ExecCtx) runChain(info []string, id, ver string, c Chain, record func(action string, res ExecResult)) int {
	ec.chain = true
	steps := c.Steps()
	done := false // değiştiren eylem başarıyla bitti
	for i, a := range steps {
		label := "Kontrol"
		switch {
		case i < len(c.Pre):
			label = "Önizleme"
		case a == c.Action:
			label = "Uygulama"
		}
		ec.say("\n── Adım %d/%d · %s: %s ──\n", i+1, len(steps), label, a)
		// Düğmeye basmak yıkıcı OLMAYAN eylemin "devam edilsin mi" onayıdır; sunucu adı onayı ve değişken uyarıları yine terminalde sorulur.
		// (yıkıcı eylemde onay kuralı her zaman "type-server-name"dir: doğrulayıcı yükseltir, ChainPlan da denetler)
		ec.autoYes = !c.Ask && a == c.Action && manifest.ActionGet(info, a, "confirm") == "yes-no"
		res := ExecAction(ec, info, id, ver, a)
		ec.autoYes = false
		if res.RC != Refused {
			record(a, res)
		}
		if res.RC != 0 {
			if done {
				ec.say("⚠️  Uygulama tamamlandı; kontrol adımı başarısız (%s). Kartta \"Kontrol et\" ile yeniden okuyabilirsiniz.\n", a)
			} else {
				ec.say("⛔ %s adımında durdu (%s); sonraki adımlar çalıştırılmadı.\n", label, a)
			}
			return res.RC
		}
		done = done || a == c.Action
	}
	ec.say("\n✅ Tamamlandı: %s\n", strings.Join(steps, " → "))
	return 0
}

// becomePassword: sudo parolasını bu koşuda BİR KEZ sorar (terminalde yankısız). Boş yanıt: sunucu parola istemiyor.
func (ec *ExecCtx) becomePassword() (string, error) {
	if ec.secretAsked {
		return ec.secret, nil
	}
	restore, err := echoOff(ec.Stdin)
	if err != nil {
		return "", errors.New("Parola istemi açılamadı: terminalde yazılanlar gizlenemiyor.")
	}
	defer restore()
	ec.say("🔑 Sunucudaki sudo parolası (bu koşuda bir kez sorulur, yazarken görünmez; parola istemiyorsa yalnız Enter): ")
	type line struct {
		s   string
		err error
	}
	ch := make(chan line, 1)
	in := ec.input()
	go func() {
		s, err := in.ReadString('\n')
		ch <- line{s, err}
	}()
	select {
	case <-ec.ctx().Done():
		ec.say("\n")
		return "", errors.New("İptal edildi.")
	case l := <-ch:
		ec.say("\n")
		if l.err != nil {
			return "", errors.New("Parola alınamadı.")
		}
		ec.secret, ec.secretAsked = strings.TrimRight(l.s, "\r\n"), true
		return ec.secret, nil
	}
}

// echoOff: girdi gerçek bir terminalse yankıyı kapatır; dönen işlev geri açar. Terminal değilse (boru, test) bir şey yapmaz.
var echoOff = func(in any) (func(), error) {
	f, ok := in.(*os.File)
	if !ok {
		return func() {}, nil
	}
	if st, err := f.Stat(); err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return func() {}, nil
	}
	stty := func(arg string) error {
		cmd := exec.Command("stty", arg)
		cmd.Stdin = f
		return cmd.Run()
	}
	if err := stty("-echo"); err != nil {
		return nil, err
	}
	return func() { _ = stty("echo") }, nil
}

// passwordPipe: parolayı yalnız çocuk sürecin okuyacağı bir boruya koyar (yazma ucu hemen kapanır).
func passwordPipe(pw string) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	_, err = w.WriteString(pw + "\n")
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}
