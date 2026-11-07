// Package manifest, component.json doğrulayıcısının (sistem/Helpers/component_manifest.py) Go karşılığıdır.
// Kabuk motoru taşıma bitene dek referans davranıştır; çıktı biçimi ("anahtar=değer" satırları) ve hata
// iletileri sırasıyla birebir korunur ki iki uygulama karşılaştırılabilsin.
package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// obj, Python dict gibi ekleme sırasını korur; yinelenen anahtarda konum ilk, değer son görülendir.
type obj struct {
	keys []string
	m    map[string]any
}

func newObj() *obj { return &obj{m: map[string]any{}} }

func (o *obj) set(k string, v any) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}
func (o *obj) get(k string) any  { return o.m[k] }
func (o *obj) has(k string) bool { _, ok := o.m[k]; return ok }
func (o *obj) len() int          { return len(o.keys) }

// getDef, Python'un d.get(k, def) karşılığıdır: anahtar VARSA (null dahil) değerini, yoksa def'i verir.
func (o *obj) getDef(k string, def any) any {
	if v, ok := o.m[k]; ok {
		return v
	}
	return def
}

// decode, sıralı nesneler (*obj), []any, string, bool, nil, int64 (tamsayı yazımı) ve float64 üretir.
// Geçersiz UTF-8, sondaki artık veri ve NaN/Infinity reddedilir (Python bunları kabul edebilir; Go daha katıdır).
func decode(data []byte) (any, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("utf-8")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("artık veri")
	}
	return v, nil
}

func parseValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := newObj()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, ok := kt.(string)
				if !ok {
					return nil, errors.New("anahtar")
				}
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				o.set(k, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return o, nil
		case '[':
			l := []any{}
			for dec.More() {
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				l = append(l, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return l, nil
		}
		return nil, errors.New("ayraç")
	case json.Number:
		s := t.String()
		if strings.ContainsAny(s, ".eE") {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, err
			}
			return f, nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			f, ferr := strconv.ParseFloat(s, 64)
			if ferr != nil {
				return nil, ferr
			}
			return f, nil
		}
		return n, nil
	default:
		return tok, nil // string, bool, nil
	}
}
