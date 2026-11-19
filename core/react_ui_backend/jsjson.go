package react_ui_backend

// An ordered JSON model that parses like JSON.parse and prints like
// JSON.stringify (compact), so Go data endpoints return the bytes the Next
// getStaticProps pages would put in their props.
//
// What it reproduces: key order (integer-like keys first in ascending order,
// the rest in insertion order, a repeated key keeps its first position and
// takes the last value), number formatting (1.0 -> 1, 1e21 -> 1e+21, -0 -> 0,
// Infinity -> null), and string escaping (no HTML escaping, U+2028 and U+2029
// raw, lone control characters as \u00xx).
// What it does not: lone surrogate escapes such as "\ud800" become U+FFFD.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// jsObject is a JavaScript object: keys in insertion order.
type jsObject struct {
	keys []string
	vals map[string]any
}

func newJSObject() *jsObject { return &jsObject{vals: map[string]any{}} }

// set assigns a property: an existing key keeps its position.
func (o *jsObject) set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *jsObject) get(k string) (any, bool) {
	v, ok := o.vals[k]
	return v, ok
}

// orderedKeys is the order JSON.stringify visits the keys in.
func (o *jsObject) orderedKeys() []string {
	var idx, rest []string
	for _, k := range o.keys {
		if isArrayIndex(k) {
			idx = append(idx, k)
		} else {
			rest = append(rest, k)
		}
	}
	if len(idx) == 0 {
		return o.keys
	}
	sort.Slice(idx, func(i, j int) bool {
		a, _ := strconv.ParseUint(idx[i], 10, 64)
		b, _ := strconv.ParseUint(idx[j], 10, 64)
		return a < b
	})
	return append(idx, rest...)
}

func isArrayIndex(k string) bool {
	if k == "" || len(k) > 10 || (len(k) > 1 && k[0] == '0') {
		return false
	}
	for i := 0; i < len(k); i++ {
		if k[i] < '0' || k[i] > '9' {
			return false
		}
	}
	n, err := strconv.ParseUint(k, 10, 64)
	return err == nil && n < 4294967295
}

// parseJS is JSON.parse. Values are *jsObject, []any, string, float64, bool, nil.
func parseJS(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the JSON value")
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
			obj := newJSObject()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				obj.set(kt.(string), v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return obj, nil
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil
		}
		return nil, errors.New("unexpected delimiter")
	case json.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil && !math.IsInf(f, 0) {
			return nil, err
		}
		return f, nil
	default: // string, bool, nil
		return tok, nil
	}
}

// stringifyJS is JSON.stringify(v) without indentation.
func stringifyJS(v any) string {
	var b strings.Builder
	writeJS(&b, v)
	return b.String()
}

func writeJS(b *strings.Builder, v any) {
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(t))
	case float64:
		if math.IsInf(t, 0) || math.IsNaN(t) {
			b.WriteString("null")
		} else {
			b.WriteString(jsNumber(t))
		}
	case int:
		b.WriteString(strconv.Itoa(t))
	case string:
		writeJSString(b, t)
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJS(b, e)
		}
		b.WriteByte(']')
	case []*jsObject:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJS(b, e)
		}
		b.WriteByte(']')
	case *jsObject:
		b.WriteByte('{')
		first := true
		for _, k := range t.orderedKeys() {
			if !first {
				b.WriteByte(',')
			}
			first = false
			writeJSString(b, k)
			b.WriteByte(':')
			writeJS(b, t.vals[k])
		}
		b.WriteByte('}')
	default:
		panic("writeJS: unsupported type")
	}
}

const hexDigits = "0123456789abcdef"

func writeJSString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '"':
				b.WriteString(`\"`)
			case c == '\\':
				b.WriteString(`\\`)
			case c == '\b':
				b.WriteString(`\b`)
			case c == '\f':
				b.WriteString(`\f`)
			case c == '\n':
				b.WriteString(`\n`)
			case c == '\r':
				b.WriteString(`\r`)
			case c == '\t':
				b.WriteString(`\t`)
			case c < 0x20:
				b.WriteString(`\u00`)
				b.WriteByte(hexDigits[c>>4])
				b.WriteByte(hexDigits[c&0xf])
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteString("�")
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
}

// jsNumber is Number.prototype.toString for a finite number.
func jsNumber(f float64) string {
	if f == 0 {
		return "0"
	}
	neg := f < 0
	if neg {
		f = -f
	}
	// Shortest digits that round-trip: d.ddde±xx
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mant, expStr, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	exp, _ := strconv.Atoi(expStr)
	k, n := len(digits), exp+1
	var out string
	switch {
	case k <= n && n <= 21:
		out = digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		out = digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		out = "0." + strings.Repeat("0", -n) + digits
	default:
		sign := "+"
		if n-1 < 0 {
			sign = "-"
		}
		abs := n - 1
		if abs < 0 {
			abs = -abs
		}
		if k == 1 {
			out = digits + "e" + sign + strconv.Itoa(abs)
		} else {
			out = digits[:1] + "." + digits[1:] + "e" + sign + strconv.Itoa(abs)
		}
	}
	if neg {
		return "-" + out
	}
	return out
}
