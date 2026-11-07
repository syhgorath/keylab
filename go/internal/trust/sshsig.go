// Package trust, sshpocket'in güven zincirini taşır: SSH imza doğrulaması (SSHSIG), components.lock,
// imzalı paket indeksi ve paketin güvenli kurulumu. Kabuk karşılıkları: Components.sh, ComponentsIndex.sh,
// ComponentsInstall.sh, component_index.py. Kabuk motoru taşıma bitene dek referans davranıştır.
package trust

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash"
	"io"
	"strings"
)

// SSHSIG biçimi (OpenSSH PROTOCOL.sshsig). Yalnızca ssh-ed25519 anahtarları ve sha256/sha512 özetleri desteklenir;
// başka her şey REDDEDİLİR (fail-closed). ssh-keygen'in kabul ettiği her imzayı kabul etmek hedef değildir;
// hedef: Go'nun kabul ettiği her imzayı ssh-keygen de kabul etmeli (testle doğrulanır).

const (
	sigMagic = "SSHSIG"
	armorBeg = "-----BEGIN SSH SIGNATURE-----"
	armorEnd = "-----END SSH SIGNATURE-----"
)

var (
	ErrArmor     = errors.New("imza biçimi geçersiz")
	ErrNamespace = errors.New("imza ad alanı uyuşmuyor")
	ErrNoSigner  = errors.New("imzacı izin verilenler arasında değil")
	ErrBadSig    = errors.New("imza doğrulanamadı")
)

// Signer, allowed_signers satırından çıkarılan imzacıdır.
type Signer struct {
	Principal  string
	Namespaces []string // boşsa ad alanı kısıtı yok
	Key        ed25519.PublicKey
}

type reader struct{ b []byte }

func (r *reader) u32() (uint32, bool) {
	if len(r.b) < 4 {
		return 0, false
	}
	v := binary.BigEndian.Uint32(r.b)
	r.b = r.b[4:]
	return v, true
}

func (r *reader) str() ([]byte, bool) {
	n, ok := r.u32()
	if !ok || uint64(n) > uint64(len(r.b)) {
		return nil, false
	}
	s := r.b[:n]
	r.b = r.b[n:]
	return s, true
}

func putStr(buf *bytes.Buffer, s []byte) {
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], uint32(len(s)))
	buf.Write(l[:])
	buf.Write(s)
}

// dearmor: "-----BEGIN SSH SIGNATURE-----" ... "-----END SSH SIGNATURE-----" gövdesini çözer.
func dearmor(sig []byte) ([]byte, error) {
	s := strings.ReplaceAll(string(sig), "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if !strings.HasPrefix(s, armorBeg+"\n") || !strings.HasSuffix(s, "\n"+armorEnd) {
		return nil, ErrArmor
	}
	body := s[len(armorBeg)+1 : len(s)-len(armorEnd)-1]
	body = strings.ReplaceAll(body, "\n", "")
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil || len(raw) == 0 {
		return nil, ErrArmor
	}
	return raw, nil
}

// ParsePublicKey: "ssh-ed25519 AAAA..." alanından anahtarı çıkarır.
func ParsePublicKey(keyType, b64 string) (ed25519.PublicKey, error) {
	if keyType != "ssh-ed25519" {
		return nil, errors.New("yalnızca ssh-ed25519 desteklenir")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	r := &reader{raw}
	t, ok := r.str()
	if !ok || string(t) != "ssh-ed25519" {
		return nil, errors.New("anahtar türü uyuşmuyor")
	}
	k, ok := r.str()
	if !ok || len(k) != ed25519.PublicKeySize || len(r.b) != 0 {
		return nil, errors.New("anahtar biçimi geçersiz")
	}
	return ed25519.PublicKey(append([]byte(nil), k...)), nil
}

// Verify, imzayı (armorlu SSHSIG) ve ad alanını doğrular; imzalayan anahtar signers içinde principal+namespace
// için izinli olmalıdır. data okunabilir akış: özet hesaplanır.
func Verify(signers []Signer, principal, namespace string, sig []byte, data io.Reader) error {
	raw, err := dearmor(sig)
	if err != nil {
		return err
	}
	if len(raw) < len(sigMagic) || string(raw[:len(sigMagic)]) != sigMagic {
		return ErrArmor
	}
	r := &reader{raw[len(sigMagic):]}
	ver, ok := r.u32()
	if !ok || ver != 1 {
		return ErrArmor
	}
	pkBlob, ok1 := r.str()
	ns, ok2 := r.str()
	reserved, ok3 := r.str()
	hashAlg, ok4 := r.str()
	sigBlob, ok5 := r.str()
	if !(ok1 && ok2 && ok3 && ok4 && ok5) || len(r.b) != 0 {
		return ErrArmor
	}
	if string(ns) != namespace {
		return ErrNamespace
	}
	var h hash.Hash
	switch string(hashAlg) {
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		return ErrArmor
	}
	// anahtar: ssh-ed25519 genel anahtar blobu
	kr := &reader{pkBlob}
	kt, ok := kr.str()
	if !ok || string(kt) != "ssh-ed25519" {
		return ErrBadSig
	}
	kb, ok := kr.str()
	if !ok || len(kb) != ed25519.PublicKeySize || len(kr.b) != 0 {
		return ErrBadSig
	}
	pub := ed25519.PublicKey(kb)
	// izinli imzacı
	allowed := false
	for _, s := range signers {
		if s.Principal != principal || !bytes.Equal(s.Key, pub) {
			continue
		}
		if len(s.Namespaces) > 0 {
			found := false
			for _, n := range s.Namespaces {
				if n == namespace {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		allowed = true
		break
	}
	if !allowed {
		return ErrNoSigner
	}
	// imza blobu
	sr := &reader{sigBlob}
	st, ok := sr.str()
	if !ok || string(st) != "ssh-ed25519" {
		return ErrBadSig
	}
	sv, ok := sr.str()
	if !ok || len(sv) != ed25519.SignatureSize || len(sr.b) != 0 {
		return ErrBadSig
	}
	if _, err := io.Copy(h, data); err != nil {
		return err
	}
	var signed bytes.Buffer
	signed.WriteString(sigMagic)
	putStr(&signed, ns)
	putStr(&signed, reserved)
	putStr(&signed, hashAlg)
	putStr(&signed, h.Sum(nil))
	if !ed25519.Verify(pub, signed.Bytes(), sv) {
		return ErrBadSig
	}
	return nil
}

// ParseAllowedSigners, ssh allowed_signers biçimini okur: "ilke [seçenekler] ssh-ed25519 AAAA [yorum]".
// Yalnızca `namespaces="a,b"` seçeneği desteklenir; başka seçenekli (valid-before, cert-authority…), başka anahtar
// türlü, ilke deseni içeren (*, ?, !) ya da bozuk satırlar YOK SAYILIR (ssh-keygen bunların bir kısmını kabul eder;
// Go daha katıdır). Yorum (#) ve boş satırlar atlanır.
func ParseAllowedSigners(text string) []Signer {
	var out []Signer
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		f := splitSignerFields(t)
		if len(f) < 3 {
			continue
		}
		principal := f[0]
		if strings.ContainsAny(principal, "*?!,\"") {
			continue
		}
		rest := f[1:]
		var nss []string
		if strings.HasPrefix(rest[0], "namespaces=") {
			v := strings.TrimPrefix(rest[0], "namespaces=")
			if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
				continue
			}
			bad := false
			for _, n := range strings.Split(v[1:len(v)-1], ",") {
				if n == "" || strings.ContainsAny(n, "*?!") {
					bad = true
					break
				}
				nss = append(nss, n)
			}
			if bad {
				continue
			}
			rest = rest[1:]
		}
		if len(rest) < 2 || strings.Contains(rest[0], "=") {
			continue
		}
		key, err := ParsePublicKey(rest[0], rest[1])
		if err != nil {
			continue
		}
		out = append(out, Signer{Principal: principal, Namespaces: nss, Key: key})
	}
	return out
}

// splitSignerFields, boşlukla ayırır ama çift tırnak içindeki boşlukları korur.
func splitSignerFields(s string) []string {
	var f []string
	var cur strings.Builder
	inQ := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQ = !inQ
			cur.WriteByte(c)
		case (c == ' ' || c == '\t') && !inQ:
			if cur.Len() > 0 {
				f = append(f, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		f = append(f, cur.String())
	}
	return f
}
