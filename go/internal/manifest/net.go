package manifest

import (
	"fmt"
	"math/big"
	"net/netip"
	"strconv"
	"strings"
)

// component_run.py'nin ağ yardımcıları (Python ipaddress davranışı). IPv6 bölge kimlikleri (%eth0) Go'da
// ağ tanımında kabul edilmez.

func asciiDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ipv4Mask: ağ maskesi ya da joker (host) maskesi olarak "/a.b.c.d" biçiminden önek uzunluğu.
func ipv4MaskBits(s string) (int, bool) {
	a, err := netip.ParseAddr(s)
	if err != nil || !a.Is4() {
		return 0, false
	}
	b := a.As4()
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	if n, ok := contiguousOnes(v); ok {
		return n, true
	}
	if n, ok := contiguousOnes(v ^ 0xFFFFFFFF); ok {
		return n, true
	}
	return 0, false
}

func contiguousOnes(v uint32) (int, bool) {
	n := 0
	for i := 31; i >= 0 && v&(1<<uint(i)) != 0; i-- {
		n++
	}
	mask := uint32(0)
	if n > 0 {
		mask = ^uint32(0) << uint(32-n)
	}
	if v != mask {
		return 0, false
	}
	return n, true
}

// parseNetwork, ipaddress.ip_network(raw, strict=False) karşılığıdır: ağ adresine indirgenmiş önek.
func parseNetwork(raw string) (netip.Prefix, bool) {
	parts := strings.Split(raw, "/")
	if len(parts) > 2 || strings.Contains(raw, "%") {
		return netip.Prefix{}, false
	}
	addr, err := netip.ParseAddr(parts[0])
	if err != nil {
		return netip.Prefix{}, false
	}
	max := addr.BitLen()
	bits := max
	if len(parts) == 2 {
		ps := parts[1]
		ok := false
		if asciiDigits(ps) {
			if n, err := strconv.Atoi(ps); err == nil && n >= 0 && n <= max {
				bits, ok = n, true
			}
		}
		if !ok && addr.Is4() {
			if n, k := ipv4MaskBits(ps); k {
				bits, ok = n, true
			}
		}
		if !ok {
			return netip.Prefix{}, false
		}
	}
	p, err := addr.Prefix(bits)
	if err != nil {
		return netip.Prefix{}, false
	}
	return p, true
}

func isV4(p netip.Prefix) bool { return p.Addr().Is4() }

func subnetOf(sub, sup netip.Prefix) bool {
	return isV4(sub) == isV4(sup) && sup.Bits() <= sub.Bits() && sup.Contains(sub.Addr())
}

// NetAdd: 1. satır durum (added | duplicate | covered:<kapsayan> | replaced:<kaldırılanlar>), 2. satır sonuç listesi.
func NetAdd(list, newRaw string) (lines []string, code int) {
	var existing []string
	for _, x := range strings.Split(list, ",") {
		if x != "" {
			existing = append(existing, x)
		}
	}
	nw, ok := parseNetwork(newRaw)
	if !ok {
		return nil, 1
	}
	type ent struct {
		raw string
		net netip.Prefix
	}
	var nets []ent
	for _, x := range existing {
		p, ok := parseNetwork(x)
		if !ok {
			return nil, 1
		}
		nets = append(nets, ent{x, p})
	}
	join := strings.Join(existing, ",")
	for _, n := range nets {
		if n.net == nw {
			return []string{"duplicate", join}, 0
		}
	}
	for _, n := range nets {
		if subnetOf(nw, n.net) {
			return []string{"covered:" + n.raw, join}, 0
		}
	}
	var keep, removed []string
	for _, n := range nets {
		if isV4(n.net) == isV4(nw) && subnetOf(n.net, nw) {
			removed = append(removed, n.raw)
		} else {
			keep = append(keep, n.raw)
		}
	}
	status := "added"
	if len(removed) > 0 {
		status = "replaced:" + strings.Join(removed, ",")
	}
	return []string{status, strings.Join(append(keep, newRaw), ",")}, 0
}

// NetValid: adres geçerliyse kendisini yazar.
func NetValid(s string) (string, int) {
	if _, err := netip.ParseAddr(s); err != nil {
		return "", 1
	}
	return s, 0
}

func addrInt(a netip.Addr) *big.Int {
	b := a.AsSlice()
	return new(big.Int).SetBytes(b)
}

// NetPrefix: "adres/önek"; bitişik olmayan maske ya da hata için çıkış 1.
func NetPrefix(addrS, maskS string) (string, int) {
	addr, err := netip.ParseAddr(addrS)
	if err != nil {
		return "", 1
	}
	var mask *big.Int
	if strings.HasPrefix(strings.ToLower(maskS), "0x") {
		n, ok := new(big.Int).SetString(maskS[2:], 16)
		if !ok || n.Sign() < 0 || n.BitLen() > 128 {
			return "", 1
		}
		mask = n
	} else {
		ma, err := netip.ParseAddr(maskS)
		if err != nil {
			return "", 1
		}
		mask = addrInt(ma)
	}
	plen := 0
	for _, w := range mask.Bits() {
		for x := uint64(w); x != 0; x &= x - 1 {
			plen++
		}
	}
	maxLen := addr.BitLen()
	if plen > maxLen {
		return "", 1
	}
	want := new(big.Int).Lsh(big.NewInt(1), uint(plen))
	want.Sub(want, big.NewInt(1))
	want.Lsh(want, uint(maxLen-plen))
	if mask.Cmp(want) != 0 {
		return "", 1
	}
	return fmt.Sprintf("%s/%d", addr, plen), 0
}

// NetSuggest: "CIDR<TAB>açıklama" satırları.
func NetSuggest(addrS, plenS string) ([]string, int) {
	addr, err := netip.ParseAddr(addrS)
	if err != nil {
		return nil, 1
	}
	plen, err := strconv.Atoi(plenS)
	if err != nil || plen < 0 || plen > addr.BitLen() {
		return nil, 1
	}
	type cl struct{ cidr, label string }
	var out []cl
	if plen < addr.BitLen() {
		p, _ := addr.Prefix(plen)
		out = append(out, cl{p.String(), "bu ağdaki herkes"})
	} else if addr.Is4() {
		p, _ := addr.Prefix(24)
		out = append(out, cl{p.String(), "tahmini /24 (ağ maskesi bilinmiyor; doğrulayın)"})
	}
	out = append(out, cl{fmt.Sprintf("%s/%d", addr, addr.BitLen()), "yalnızca bu adres"})
	seen := map[string]bool{}
	var lines []string
	for _, c := range out {
		if !seen[c.cidr] {
			seen[c.cidr] = true
			lines = append(lines, c.cidr+"\t"+c.label)
		}
	}
	return lines, 0
}
