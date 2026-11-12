package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keylab/internal/servers"
)

func knockFx(t *testing.T, env string) (*Servers, string, string) {
	t.Helper()
	mgrEnv(t) // PATH'e sahte security/ssh-add
	kc := filepath.Join(t.TempDir(), "kc")
	t.Setenv("FAKE_KC", kc)
	argv := filepath.Join(t.TempDir(), "argv.log")
	t.Setenv("FAKE_ARGV_LOG", argv)
	root := filepath.Join(t.TempDir(), "servers")
	mkServer(t, root, "kn", env, false)
	return &Servers{Dir: root}, kc, argv
}

func kcLines(kc string) string { b, _ := os.ReadFile(kc); return string(b) }

func TestKnockSaveStatusDisable(t *testing.T) {
	s, kc, argv := knockFx(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")

	st, err := s.KnockStatus("kn")
	if err != nil || st.Enabled || st.HasSeq || st.Proto != "tcp" || st.Delay != "300" {
		t.Fatalf("başlangıç durumu: %+v %v", st, err)
	}
	if err := s.KnockSave("kn", "udp", "450", "7001,7002,7003"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(kcLines(kc), "USBMonitor_KN_Knock=7001,7002,7003") {
		t.Errorf("dizi Keychain'de değil: %q", kcLines(kc))
	}
	st, _ = s.KnockStatus("kn")
	if !st.Enabled || !st.HasSeq || st.Proto != "udp" || st.Delay != "450" {
		t.Errorf("kayıttan sonra durum: %+v", st)
	}
	env, _ := os.ReadFile(filepath.Join(s.Dir, "kn", "server.env"))
	if strings.Contains(string(env), "7001") {
		t.Errorf("dizi server.env'e sızmış:\n%s", env)
	}
	// dizi hiçbir zaman komut satırında görünmez
	if a, _ := os.ReadFile(argv); strings.Contains(string(a), "7001") {
		t.Errorf("dizi komut satırında göründü: %q", a)
	}

	if err := s.KnockDisable("kn"); err != nil {
		t.Fatal(err)
	}
	st, _ = s.KnockStatus("kn")
	if st.Enabled || st.HasSeq {
		t.Errorf("kapattıktan sonra durum: %+v", st)
	}
}

// USB başka bir Mac'e takılınca: server.env'de KNOCK=keychain var ama dizi o Mac'in Keychain'inde yok.
func TestKnockStatusOtherMac(t *testing.T) {
	s, _, _ := knockFx(t, "IP=192.0.2.9\nUSER=u\nPORT=22\nKNOCK=keychain\nKNOCK_PROTO=tcp\nKNOCK_DELAY_MS=300\n")
	st, err := s.KnockStatus("kn")
	if err != nil || !st.Enabled || st.HasSeq {
		t.Fatalf("Enabled=true, HasSeq=false beklenirdi: %+v %v", st, err)
	}
}

func TestKnockSaveRejectsBadInput(t *testing.T) {
	cases := []struct{ name, proto, delay, seq, want string }{
		{"kn", "icmp", "300", "7001", "protokol"},
		{"kn", "tcp", "5001", "7001", "bekleme"},
		{"kn", "tcp", "abc", "7001", "bekleme"},
		{"kn", "tcp", "300", "", "dizi"},
		{"kn", "tcp", "300", "7001,", "dizi"},
		{"kn", "tcp", "300", "0", "dizi"},
		{"kn", "tcp", "300", "65536", "dizi"},
		{"kn", "tcp", "300", "1,2,3,4,5,6,7,8,9,10,11", "dizi"},
		{"kn", "tcp", "300", "7001, 7002", "dizi"},
		{"../kn", "tcp", "300", "7001", "geçersiz"},
		{"yok", "tcp", "300", "7001", "sunucu yok"},
	}
	for _, c := range cases {
		s, kc, _ := knockFx(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")
		err := s.KnockSave(c.name, c.proto, c.delay, c.seq)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: hata %q içermeli, gelen %v", c, c.want, err)
		}
		if kcLines(kc) != "" {
			t.Errorf("%+v: reddedilen girdi Keychain'e yazılmış: %q", c, kcLines(kc))
		}
		env, _ := os.ReadFile(filepath.Join(s.Dir, "kn", "server.env"))
		if strings.Contains(string(env), "KNOCK") {
			t.Errorf("%+v: reddedilen girdi server.env'i değiştirmiş:\n%s", c, env)
		}
	}
}

func TestKnockSaveFailureModes(t *testing.T) {
	// 1) security yazamıyor
	s, _, _ := knockFx(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "security"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if err := s.KnockSave("kn", "tcp", "300", "7001"); err != ErrKnockStore {
		t.Errorf("ErrKnockStore beklenirdi: %v", err)
	}
	if st, _ := s.KnockStatus("kn"); st.Enabled {
		t.Error("kaydedilemeyince knock etkinleşmemeli")
	}
	// 2) yazıyor görünüp saklamıyor (doğrulama başarısız)
	s2, _, _ := knockFx(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")
	bin2 := t.TempDir()
	delLog := filepath.Join(t.TempDir(), "silindi")
	os.WriteFile(filepath.Join(bin2, "security"), []byte("#!/bin/sh\ncase \"$1\" in -i) cat >/dev/null; exit 0;; find-generic-password) echo YANLIS; exit 0;; delete-generic-password) echo sil >> "+delLog+"; exit 0;; esac\nexit 0\n"), 0o755)
	t.Setenv("PATH", bin2+":"+os.Getenv("PATH"))
	if err := s2.KnockSave("kn", "tcp", "300", "7001"); err != ErrKnockVerify {
		t.Errorf("ErrKnockVerify beklenirdi: %v", err)
	}
	if b, _ := os.ReadFile(delLog); !strings.Contains(string(b), "sil") {
		t.Error("doğrulanamayan kayıt Keychain'den silinmeliydi")
	}
	if st, _ := s2.KnockStatus("kn"); st.Enabled {
		t.Error("doğrulanamayınca knock etkinleşmemeli")
	}
}

var _ = servers.KnockAccount

// Dizi boş bırakılırsa kayıtlı dizi korunur, yalnızca protokol ve bekleme güncellenir.
func TestKnockSaveEmptyKeepsSequence(t *testing.T) {
	s, kc, _ := knockFx(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")
	if err := s.KnockSave("kn", "tcp", "300", "7001,7002,7003"); err != nil {
		t.Fatal(err)
	}
	before := kcLines(kc)
	if err := s.KnockSave("kn", "udp", "120", ""); err != nil {
		t.Fatalf("boş dizi + kayıtlı dizi kabul edilmeliydi: %v", err)
	}
	if kcLines(kc) != before || !strings.Contains(kcLines(kc), "7001,7002,7003") {
		t.Errorf("kayıtlı dizi değişmemeliydi:\nönce %q\nsonra %q", before, kcLines(kc))
	}
	st, _ := s.KnockStatus("kn")
	if !st.Enabled || !st.HasSeq || st.Proto != "udp" || st.Delay != "120" {
		t.Errorf("protokol/bekleme güncellenmeliydi: %+v", st)
	}
	// geçersiz protokol/bekleme yine reddedilir ve hiçbir şeyi değiştirmez
	if err := s.KnockSave("kn", "icmp", "120", ""); err == nil {
		t.Error("geçersiz protokol boş dizide de reddedilmeli")
	}
	if st, _ := s.KnockStatus("kn"); st.Proto != "udp" {
		t.Errorf("reddedilen istek ayarı değiştirmiş: %+v", st)
	}
}

func TestKnockSaveEmptyWithoutStoredSequence(t *testing.T) {
	// başka Mac: bayrak var, dizi bu Mac'te yok → boş dizi kabul edilmez, env değişmez
	s, kc, _ := knockFx(t, "IP=192.0.2.9\nUSER=u\nPORT=22\nKNOCK=keychain\nKNOCK_PROTO=tcp\nKNOCK_DELAY_MS=300\n")
	err := s.KnockSave("kn", "udp", "999", "")
	if err == nil || !strings.Contains(err.Error(), "dizi") {
		t.Fatalf("dizi gerekli hatası beklenirdi: %v", err)
	}
	if kcLines(kc) != "" {
		t.Errorf("Keychain'e bir şey yazılmamalı: %q", kcLines(kc))
	}
	if st, _ := s.KnockStatus("kn"); st.Proto != "tcp" || st.Delay != "300" {
		t.Errorf("env değişmemeliydi: %+v", st)
	}
}

func TestMaskSequence(t *testing.T) {
	cases := map[string]string{
		"7001,7002,7003": "7••1, 7••2, 7••3",
		"7123,45678":     "7••3, 4•••8",
		"123":            "1•3",
		"80":             "••",
		"7,80,9000":      "•, ••, 9••0",
		"65535":          "6•••5",
		"":               "",
		"7001,abc":       "",
		"7001, 7002":     "", // boşluklu dizi geçersiz
		"0":              "",
	}
	for in, want := range cases {
		if got := MaskSequence(in); got != want {
			t.Errorf("MaskSequence(%q) = %q, beklenen %q", in, got, want)
		}
	}
	// ortadaki rakamlar hiçbir koşulda sızmaz
	for _, seq := range []string{"7123,45678", "1234567"[:5]} {
		got := MaskSequence(seq)
		for _, mid := range []string{"12", "56", "123", "567"} {
			if strings.Contains(got, mid) {
				t.Errorf("MaskSequence(%q) = %q içinde orta rakamlar var: %s", seq, got, mid)
			}
		}
	}
}

func TestKnockStatusHint(t *testing.T) {
	s, _, _ := knockFx(t, "IP=192.0.2.9\nUSER=u\nPORT=22\n")
	if st, _ := s.KnockStatus("kn"); st.Hint != "" {
		t.Errorf("dizi yokken ipucu olmamalı: %q", st.Hint)
	}
	if err := s.KnockSave("kn", "tcp", "300", "7001,7002,7003"); err != nil {
		t.Fatal(err)
	}
	st, _ := s.KnockStatus("kn")
	if st.Hint != "7••1, 7••2, 7••3" {
		t.Errorf("ipucu = %q", st.Hint)
	}
}
