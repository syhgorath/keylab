package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Bu dosya ComponentsVars.sh'ın dosya işlevlerinin (comp_vars_get/set/unset/save/missing) Go karşılığıdır.
// Değerler servers/<ad>/<paket>.vars içinde düz "ad=değer" satırlarıdır; hiçbir şey source/eval edilmez.
//
// Bilinçli fark: kabuktaki `awk -v val="$v"` değerdeki ters bölü dizilerini yorumlar (örn. `C:\temp` içindeki `\t`
// sekmeye dönüşür). Go değeri OLDUĞU GİBİ yazar.

// VarsFile, comp_vars_file karşılığıdır.
func VarsFile(serverDir, pkg string) string { return filepath.Join(serverDir, pkg+".vars") }

func varNameOK(n string) bool { return reName.MatchString(n) }

// keyLine: awk'taki `sub(/^[ \t]+/, "", s)` sonrası satır ve `s ~ /^#/` / `index(s, k "=") == 1` denetimi.
func varLineMatches(line, key string) (rest string, matches bool) {
	s := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(s, "#") {
		return "", false
	}
	if strings.HasPrefix(s, key+"=") {
		return s[len(key)+1:], true
	}
	return "", false
}

func splitAwkLines(data string) []string {
	if data == "" {
		return nil
	}
	lines := strings.Split(data, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// VarsGet: değeri verir; yoksa ok=false (kabukta çıkış 1).
func VarsGet(file, name string) (string, bool) {
	if !varNameOK(name) {
		return "", false
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	for _, line := range splitAwkLines(string(data)) {
		if rest, ok := varLineMatches(line, name); ok {
			return strings.Trim(rest, " \t\n\v\f\r"), true
		}
	}
	return "", false
}

func writeAtomic(file string, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+".")
	if err != nil {
		return errors.New("geçici dosya oluşturulamadı")
	}
	name := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, file); err != nil {
		os.Remove(name)
		return err
	}
	os.Chmod(file, 0o600)
	return nil
}

// VarsSet: satırı değiştirir/ekler (diğer satırlar ve yorumlar korunur); izin 600, atomik yazım.
func VarsSet(file, name, value string) error {
	if !varNameOK(name) {
		return errors.New("geçersiz değişken adı")
	}
	if strings.ContainsAny(value, "\n\r\t") {
		return errors.New("değer satır sonu/sekme içeremez")
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return errors.New("değer denetim karakteri içeremez")
		}
	}
	var out strings.Builder
	if data, err := os.ReadFile(file); err == nil {
		done := false
		for _, line := range splitAwkLines(string(data)) {
			if _, ok := varLineMatches(line, name); ok {
				if !done {
					out.WriteString(name + "=" + value + "\n")
					done = true
				}
				continue
			}
			out.WriteString(line + "\n")
		}
		if !done {
			out.WriteString(name + "=" + value + "\n")
		}
	} else {
		out.WriteString(name + "=" + value + "\n")
	}
	return writeAtomic(file, out.String())
}

// VarsUnset: satırı siler (varsayılana döner). Dosya yoksa sorun değil.
func VarsUnset(file, name string) error {
	if !varNameOK(name) {
		return errors.New("geçersiz değişken adı")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var out strings.Builder
	for _, line := range splitAwkLines(string(data)) {
		if _, ok := varLineMatches(line, name); ok {
			continue
		}
		out.WriteString(line + "\n")
	}
	return writeAtomic(file, out.String())
}

// VarsSave: önce DOĞRULAR, geçerliyse yazar. warn true ise çağıran ayrıca onay isteyebilir.
func VarsSave(manifestPath, file, name, value string) (warn bool, err error) {
	w, errs, code := Check(manifestPath, name, value)
	if code != 0 {
		return false, errors.New(strings.Join(errs, "\n"))
	}
	if err := VarsSet(file, name, value); err != nil {
		return false, err
	}
	return w, nil
}

// VarsMissing: değeri de varsayılanı da olmayan ZORUNLU değişken adları.
func VarsMissing(manifestPath, file string) []string {
	lines, _, _ := Describe(manifestPath, "", false)
	var out []string
	for _, l := range lines {
		f := strings.Split(l, "\t")
		if len(f) < 4 {
			continue
		}
		name, req, dflt := f[0], f[2], f[3]
		if req != "yes" || dflt != "" {
			continue
		}
		if cur, ok := VarsGet(file, name); !ok || cur == "" {
			out = append(out, name)
		}
	}
	return out
}

// NetSuggestFor, comp_net_suggest karşılığıdır: "adres/önek" ya da yalnızca adres.
func NetSuggestFor(a string) ([]string, int) {
	addr, plen := a, ""
	if i := strings.Index(a, "/"); i >= 0 {
		addr = a[:i]
		plen = a[strings.LastIndex(a, "/")+1:]
	} else if strings.Contains(addr, ":") {
		plen = "128"
	} else {
		plen = "32"
	}
	if len(plen) < 1 || len(plen) > 3 || !asciiDigits(plen) {
		return nil, 1
	}
	return NetSuggest(addr, plen)
}

var _ = fmt.Sprint
