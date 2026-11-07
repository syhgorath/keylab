package trust

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ssh-keygen -Y verify referanstır. Özellik: Go'nun KABUL ettiği her imzayı ssh-keygen de kabul etmeli;
// geçerli imzaların ikisi de kabul etmeli.

type sshEnv struct {
	dir, key, pub string
}

func needSSHKeygen(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen yok")
	}
}

func newKey(t *testing.T, dir, name string) (priv, pubLine string) {
	t.Helper()
	priv = filepath.Join(dir, name)
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "", "-f", priv).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v %s", err, out)
	}
	b, _ := os.ReadFile(priv + ".pub")
	f := strings.Fields(string(b))
	return priv, f[0] + " " + f[1]
}

func sign(t *testing.T, priv, ns, hashAlg string, data []byte) []byte {
	t.Helper()
	dir := t.TempDir()
	df := filepath.Join(dir, "d")
	os.WriteFile(df, data, 0o600)
	args := []string{"-Y", "sign", "-f", priv, "-n", ns}
	if hashAlg != "" {
		args = append(args, "-O", "hashalg="+hashAlg)
	}
	args = append(args, df)
	if out, err := exec.Command("ssh-keygen", args...).CombinedOutput(); err != nil {
		t.Fatalf("sign: %v %s", err, out)
	}
	sig, _ := os.ReadFile(df + ".sig")
	return sig
}

func sshVerify(t *testing.T, signersText, principal, ns string, sig, data []byte) bool {
	t.Helper()
	dir := t.TempDir()
	sf, gf := filepath.Join(dir, "signers"), filepath.Join(dir, "sig")
	os.WriteFile(sf, []byte(signersText), 0o600)
	os.WriteFile(gf, sig, 0o600)
	cmd := exec.Command("ssh-keygen", "-Y", "verify", "-f", sf, "-I", principal, "-n", ns, "-s", gf)
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run() == nil
}

func goVerify(signersText, principal, ns string, sig, data []byte) bool {
	return Verify(ParseAllowedSigners(signersText), principal, ns, sig, bytes.NewReader(data)) == nil
}

func TestSSHSigValidAndNegativeMatchKeygen(t *testing.T) {
	needSSHKeygen(t)
	dir := t.TempDir()
	priv, pub := newKey(t, dir, "k1")
	_, pub2 := newKey(t, dir, "k2")
	data := []byte("paket içeriği\x00\x01\xff" + strings.Repeat("x", 100000))
	signers := "sshpocket-component namespaces=\"sshpocket-component\" " + pub + "\n"
	for _, h := range []string{"", "sha256", "sha512"} {
		sig := sign(t, priv, "sshpocket-component", h, data)
		cases := []struct {
			name, signers, principal, ns string
			sig, data                    []byte
			want                         bool
		}{
			{"geçerli", signers, "sshpocket-component", "sshpocket-component", sig, data, true},
			{"kısıtsız ad alanı", "sshpocket-component " + pub + "\n", "sshpocket-component", "sshpocket-component", sig, data, true},
			{"yorumlu anahtar", "sshpocket-component namespaces=\"sshpocket-component\" " + pub + " bir yorum\n", "sshpocket-component", "sshpocket-component", sig, data, true},
			{"yorum satırları + boş", "# x\n\n" + signers, "sshpocket-component", "sshpocket-component", sig, data, true},
			{"veri değişti", signers, "sshpocket-component", "sshpocket-component", sig, append(append([]byte(nil), data...), 'x'), false},
			{"bir bayt eksik", signers, "sshpocket-component", "sshpocket-component", sig, data[:len(data)-1], false},
			{"yanlış ad alanı arg", signers, "sshpocket-component", "sshpocket-index", sig, data, false},
			{"yanlış ilke", signers, "baska", "sshpocket-component", sig, data, false},
			{"yanlış anahtar", "sshpocket-component namespaces=\"sshpocket-component\" " + pub2 + "\n", "sshpocket-component", "sshpocket-component", sig, data, false},
			{"ad alanı seçeneği dışlıyor", "sshpocket-component namespaces=\"sshpocket-index\" " + pub + "\n", "sshpocket-component", "sshpocket-component", sig, data, false},
			{"boş imzacı listesi", "# yok\n", "sshpocket-component", "sshpocket-component", sig, data, false},
			{"boş imza", signers, "sshpocket-component", "sshpocket-component", nil, data, false},
			{"çöp imza", signers, "sshpocket-component", "sshpocket-component", []byte("çöp"), data, false},
		}
		for _, c := range cases {
			ref := sshVerify(t, c.signers, c.principal, c.ns, c.sig, c.data)
			got := goVerify(c.signers, c.principal, c.ns, c.sig, c.data)
			if ref != c.want {
				t.Fatalf("[%s] %s: ssh-keygen beklenmeyen sonuç %v (test varsayımı yanlış)", h, c.name, ref)
			}
			if got != ref {
				t.Errorf("[%s] %s: ssh-keygen=%v go=%v", h, c.name, ref, got)
			}
		}
		// ad alanı imzada başka
		other := sign(t, priv, "sshpocket-index", h, data)
		if goVerify(signers, "sshpocket-component", "sshpocket-component", other, data) {
			t.Errorf("[%s] başka ad alanıyla imzalı kabul edildi", h)
		}
	}
}

// Bozulmuş imzalar: Go kabul ediyorsa ssh-keygen de kabul etmeli (güvenlik özelliği).
func TestSSHSigMutationsNeverMoreLenient(t *testing.T) {
	needSSHKeygen(t)
	dir := t.TempDir()
	priv, pub := newKey(t, dir, "k")
	data := []byte("veri")
	signers := "p namespaces=\"n\" " + pub + "\n"
	sig := sign(t, priv, "n", "sha512", data)
	lines := strings.Split(strings.TrimSpace(string(sig)), "\n")
	raw, err := base64.StdEncoding.DecodeString(strings.Join(lines[1:len(lines)-1], ""))
	if err != nil {
		t.Fatal(err)
	}
	armor := func(b []byte) []byte {
		enc := base64.StdEncoding.EncodeToString(b)
		var sb strings.Builder
		sb.WriteString(armorBeg + "\n")
		for len(enc) > 70 {
			sb.WriteString(enc[:70] + "\n")
			enc = enc[70:]
		}
		sb.WriteString(enc + "\n" + armorEnd + "\n")
		return []byte(sb.String())
	}
	if !goVerify(signers, "p", "n", armor(raw), data) || !sshVerify(t, signers, "p", "n", armor(raw), data) {
		t.Fatal("yeniden zırhlanan geçerli imza kabul edilmedi")
	}
	accepted := 0
	for i := 0; i < len(raw); i++ {
		for _, bit := range []byte{0x01, 0x80} {
			m := append([]byte(nil), raw...)
			m[i] ^= bit
			s := armor(m)
			if goVerify(signers, "p", "n", s, data) {
				accepted++
				if !sshVerify(t, signers, "p", "n", s, data) {
					t.Errorf("Go bayt %d bit %x bozulmuş imzayı kabul etti, ssh-keygen etmedi", i, bit)
				}
			}
		}
	}
	// kesme ve ek veri
	for _, n := range []int{0, 1, 5, 6, 9, 20, len(raw) - 1} {
		if n < len(raw) && goVerify(signers, "p", "n", armor(raw[:n]), data) {
			t.Errorf("kesik imza (%d) kabul edildi", n)
		}
	}
	if goVerify(signers, "p", "n", armor(append(append([]byte(nil), raw...), 0)), data) {
		t.Error("sonda fazladan veri kabul edildi")
	}
	// zırh bozuklukları
	good := string(armor(raw))
	for name, s := range map[string]string{
		"BEGIN yok":   strings.Replace(good, armorBeg, "-----BEGIN X-----", 1),
		"END yok":     strings.Replace(good, armorEnd, "-----END X-----", 1),
		"boşluk":      strings.Replace(good, "\n", " \n", 1),
		"base64 hata": strings.Replace(good, "A", "!", 1),
	} {
		if goVerify(signers, "p", "n", []byte(s), data) && !sshVerify(t, signers, "p", "n", []byte(s), data) {
			t.Errorf("zırh bozukluğu %q: Go kabul etti, ssh-keygen etmedi", name)
		}
	}
	t.Logf("bozulmuş %d varyanttan Go'nun kabul ettiği: %d", len(raw)*2, accepted)
	_ = fmt.Sprint
}

func TestAllowedSignersParsing(t *testing.T) {
	needSSHKeygen(t)
	dir := t.TempDir()
	_, pub := newKey(t, dir, "k")
	f := strings.Fields(pub)
	for text, want := range map[string]int{
		"p " + pub:                               1,
		"p namespaces=\"a,b\" " + pub + " yorum": 1,
		"# yorum\n\np " + pub + "\r\n":           1,
		"p valid-before=\"2000\" " + pub:         0, // seçenek desteklenmez → yok sayılır
		"p cert-authority " + pub:                0,
		"*@x " + pub:                             0,
		"p ssh-rsa " + f[1]:                      0,
		"p ssh-ed25519 AAAA":                     0,
		"p":                                      0,
		"p namespaces=a " + pub:                  0,
		"p namespaces=\"\" " + pub:               0,
		"p " + pub + "\nq " + pub:                2,
	} {
		if got := len(ParseAllowedSigners(text)); got != want {
			t.Errorf("%q: %d imzacı, beklenen %d", text, got, want)
		}
	}
}
