package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// components.lock ayrıştırma ve doğrulama (Components.sh). Dosya ASLA çalıştırılmaz; her değer doğrulanır.
// Kilitte olmayan paket çalıştırılmaz.

const ComponentNamespace = "sshpocket-component"

var (
	reID       = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	reVersion  = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	reRepo     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	reSHA256   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reSigners  = regexp.MustCompile(`^sshpocket-component namespaces="sshpocket-component" ssh-ed25519 [A-Za-z0-9+/]+=*( [^\x00-\x1f\x7f]*)?$`)
	reLockHead = regexp.MustCompile(`^\[([^\]]*)\]$`)
	reLockKV   = regexp.MustCompile(`^([a-z0-9_]+)[ \t\n\v\f\r]*=[ \t\n\v\f\r]*(.*)$`)
)

func IDValid(s string) bool      { return reID.MatchString(s) }
func VersionValid(s string) bool { return reVersion.MatchString(s) }
func SHA256Valid(s string) bool  { return reSHA256.MatchString(s) }
func RepoValid(s string) bool    { return reRepo.MatchString(s) && !strings.Contains(s, "..") }
func SignersValid(s string) bool { return reSigners.MatchString(s) }

// LockEntry, kilitteki bir paketin dört alanıdır.
type LockEntry struct {
	ID, Repo, Version, SHA256, Signers string
}

// Lock, components.lock içeriğidir (dosya sırası korunur).
type Lock struct {
	Entries []LockEntry
}

func (l *Lock) Get(id string) (LockEntry, bool) {
	for _, e := range l.Entries {
		if e.ID == id {
			return e, true
		}
	}
	return LockEntry{}, false
}

func (l *Lock) IDs() []string {
	ids := make([]string, 0, len(l.Entries))
	for _, e := range l.Entries {
		ids = append(ids, e.ID)
	}
	return ids
}

// LoadLock, dosyayı katı biçimde okur ve doğrular: bilinmeyen anahtar, yinelenen kimlik/anahtar, bozuk değer ve
// dört alanın eksikliği hatadır. Dosya yoksa boş kilit döner (hiçbir paket bilinmez).
func LoadLock(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Lock{}, nil
		}
		return nil, err
	}
	lock := &Lock{}
	var cur *LockEntry
	seenIDs := map[string]bool{}
	seenKeys := map[string]bool{}
	have := map[string]map[string]bool{}
	text := string(data)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	flush := func() {
		if cur != nil {
			lock.Entries = append(lock.Entries, *cur)
		}
	}
	for i, line := range lines {
		no := i + 1
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := reLockHead.FindStringSubmatch(line); m != nil {
			id := m[1]
			if !IDValid(id) {
				return nil, fmt.Errorf("components.lock:%d: geçersiz paket kimliği", no)
			}
			if seenIDs[id] {
				return nil, fmt.Errorf("components.lock:%d: yinelenen paket: %s", no, id)
			}
			seenIDs[id] = true
			seenKeys = map[string]bool{}
			flush()
			cur = &LockEntry{ID: id}
			have[id] = map[string]bool{}
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("components.lock:%d: paket başlığı yok", no)
		}
		m := reLockKV.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("components.lock:%d: anlaşılmayan satır", no)
		}
		key, val := m[1], strings.TrimRight(m[2], " \t\n\v\f\r")
		if seenKeys[key] {
			return nil, fmt.Errorf("components.lock:%d: yinelenen anahtar: %s", no, key)
		}
		seenKeys[key] = true
		ok := false
		switch key {
		case "repo":
			ok = RepoValid(val)
			cur.Repo = val
		case "version":
			ok = VersionValid(val)
			cur.Version = val
		case "sha256":
			ok = SHA256Valid(val)
			cur.SHA256 = val
		case "signers":
			ok = SignersValid(val)
			cur.Signers = val
		default:
			return nil, fmt.Errorf("components.lock:%d: bilinmeyen anahtar: %s", no, key)
		}
		if !ok {
			return nil, fmt.Errorf("components.lock:%d: geçersiz değer (%s)", no, key)
		}
		have[cur.ID][key] = true
	}
	flush()
	for _, e := range lock.Entries {
		for _, k := range []string{"repo", "version", "sha256", "signers"} {
			if !have[e.ID][k] {
				return nil, fmt.Errorf("components.lock: %s için '%s' eksik", e.ID, k)
			}
		}
	}
	return lock, nil
}

// SignersFromLockLine, kilitteki signers satırını doğrulanmış imzacıya çevirir.
func SignersFromLockLine(line string) ([]Signer, error) {
	if !SignersValid(line) {
		return nil, fmt.Errorf("signers satırı geçersiz")
	}
	s := ParseAllowedSigners(line)
	if len(s) != 1 {
		return nil, fmt.Errorf("signers satırı geçersiz")
	}
	return s, nil
}

// FileSHA256, dosyanın sha256 özetidir.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VersionCmp: 1 (a>b), 0 (eşit), -1 (a<b). İkisi de X.Y.Z biçiminde olmalıdır; rakam dizileri sayısal karşılaştırılır.
func VersionCmp(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var x, y string
		if i < len(pa) {
			x = strings.TrimLeft(pa[i], "0")
		}
		if i < len(pb) {
			y = strings.TrimLeft(pb[i], "0")
		}
		if len(x) != len(y) {
			if len(x) > len(y) {
				return 1
			}
			return -1
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

// VerifyArchive, comp_verify karşılığıdır: paket kilitte olmalı, özet (beklenen ya da kilitteki) tutmalı,
// imza KİLİTTEKİ imzacıyla (ad alanı sshpocket-component) doğrulanmalı. expectedSHA yalnızca DOĞRULANMIŞ indeksten gelir.
func VerifyArchive(lock *Lock, id, archive, sigFile, expectedSHA string) error {
	if !IDValid(id) {
		return fmt.Errorf("geçersiz paket kimliği")
	}
	e, ok := lock.Get(id)
	if !ok {
		return fmt.Errorf("paket kilit dosyasında yok: %s", id)
	}
	want := expectedSHA
	if want != "" {
		if !SHA256Valid(want) {
			return fmt.Errorf("beklenen özet geçersiz (%s)", id)
		}
	} else {
		want = e.SHA256
	}
	got, err := FileSHA256(archive)
	if err != nil || got != want {
		return fmt.Errorf("özet beklenenle uyuşmuyor (%s)", id)
	}
	signers, err := SignersFromLockLine(e.Signers)
	if err != nil {
		return fmt.Errorf("signers satırı geçersiz (%s)", id)
	}
	sig, err := os.ReadFile(sigFile)
	if err != nil {
		return fmt.Errorf("imza doğrulanamadı (%s)", id)
	}
	f, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("imza doğrulanamadı (%s)", id)
	}
	defer f.Close()
	if err := Verify(signers, ComponentNamespace, ComponentNamespace, sig, f); err != nil {
		return fmt.Errorf("imza doğrulanamadı (%s)", id)
	}
	return nil
}
