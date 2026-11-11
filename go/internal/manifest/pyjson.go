package manifest

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// dumps, Python json.dumps(v, ensure_ascii=False, ...) karşılığıdır. indent<0: tek satır
// (compact ise ayraçlar "," ve ":", değilse ", " ve ": "); indent>=0: girintili.
func dumps(v any, indent int, sortKeys, compact bool) string {
	var b strings.Builder
	enc(&b, v, indent, sortKeys, compact, 0)
	return b.String()
}

func enc(b *strings.Builder, v any, indent int, sortKeys, compact bool, level int) {
	itemSep, keySep := ", ", ": "
	if compact {
		itemSep, keySep = ",", ":"
	}
	nl := func(l int) string { return "\n" + strings.Repeat(" ", indent*l) }
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
	case int:
		b.WriteString(strconv.Itoa(t))
	case float64:
		b.WriteString(pyFloat(t))
	case string:
		b.WriteString(jsonString(t))
	case []string:
		l := make([]any, len(t))
		for i, s := range t {
			l[i] = s
		}
		enc(b, l, indent, sortKeys, compact, level)
	case []any:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[")
		for i, e := range t {
			if i > 0 {
				if indent >= 0 {
					b.WriteString(",")
				} else {
					b.WriteString(itemSep)
				}
			}
			if indent >= 0 {
				b.WriteString(nl(level + 1))
			}
			enc(b, e, indent, sortKeys, compact, level+1)
		}
		if indent >= 0 {
			b.WriteString(nl(level))
		}
		b.WriteString("]")
	case *obj:
		keys := append([]string(nil), t.keys...)
		if sortKeys {
			sort.Strings(keys)
		}
		encMap(b, keys, func(k string) any { return t.m[k] }, indent, sortKeys, compact, level, itemSep, keySep)
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys) // Go haritasının sırası yok; sıralı yazılır
		encMap(b, keys, func(k string) any { return t[k] }, indent, sortKeys, compact, level, itemSep, keySep)
	default:
		b.WriteString(jsonString(fmt.Sprint(v)))
	}
}

func encMap(b *strings.Builder, keys []string, get func(string) any, indent int, sortKeys, compact bool, level int, itemSep, keySep string) {
	if len(keys) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			if indent >= 0 {
				b.WriteString(",")
			} else {
				b.WriteString(itemSep)
			}
		}
		if indent >= 0 {
			b.WriteString("\n" + strings.Repeat(" ", indent*(level+1)))
		}
		b.WriteString(jsonString(k))
		b.WriteString(keySep)
		enc(b, get(k), indent, sortKeys, compact, level+1)
	}
	if indent >= 0 {
		b.WriteString("\n" + strings.Repeat(" ", indent*level))
	}
	b.WriteString("}")
}

// jsonString: Python ensure_ascii=False kaçışı (yalnızca ", \, \b \f \n \r \t ve diğer <0x20).
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else if r == utf8.RuneError {
				b.WriteRune(r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// pyFloat: Python repr(float) ile uyumlu (üstel gösterim: üs < -4 ya da >= 16).
func pyFloat(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "null"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e16 {
		return strconv.FormatFloat(f, 'f', 1, 64)
	}
	a := math.Abs(f)
	if a != 0 && (a < 1e-4 || a >= 1e16) {
		return strconv.FormatFloat(f, 'e', -1, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
