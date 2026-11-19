package react_ui_backend

// JavaScript-compatible JSON values for the gradio route. The Next route parses
// JSON with JSON.parse and answers with JSON.stringify, which keep object key
// order, put integer-like keys first, print numbers the JavaScript way and do
// not escape non-ASCII text. Go's encoding/json does none of these, so the
// route works on these values instead.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A jsValue is one of: nil (null), undefined, bool, float64, string, []any,
// *gradioObject, jsBlob or jsBuffer.
type undefinedType struct{}

// undefined is JavaScript's undefined: dropped from objects, null in arrays.
var undefined = undefinedType{}

// jsBlob is a Blob from fetch(...).blob(). JSON.stringify prints it as {}.
type jsBlob struct{ data []byte }

// jsBuffer is a Node Buffer from fs.readFile.
type jsBuffer struct{ data []byte }

// gradioObject is a plain object with insertion-ordered keys.
type gradioObject struct {
	keys []string
	vals map[string]any
}

func newObject() *gradioObject { return &gradioObject{vals: map[string]any{}} }

// get returns the value for key, or undefined.
func (o *gradioObject) get(key string) any {
	if o == nil {
		return undefined
	}
	if v, ok := o.vals[key]; ok {
		return v
	}
	return undefined
}

func (o *gradioObject) has(key string) bool {
	_, ok := o.vals[key]
	return ok
}

// set assigns key, keeping the position of an existing key.
func (o *gradioObject) set(key string, v any) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = v
}

func (o *gradioObject) delete(key string) {
	if _, ok := o.vals[key]; !ok {
		return
	}
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// ownKeys returns keys in JavaScript's order: array indexes ascending, then the
// rest in insertion order.
func (o *gradioObject) ownKeys() []string {
	var indexes, rest []string
	for _, k := range o.keys {
		if isGradioArrayIndex(k) {
			indexes = append(indexes, k)
		} else {
			rest = append(rest, k)
		}
	}
	sort.Slice(indexes, func(i, j int) bool {
		a, _ := strconv.ParseUint(indexes[i], 10, 32)
		b, _ := strconv.ParseUint(indexes[j], 10, 32)
		return a < b
	})
	return append(indexes, rest...)
}

// clone copies the top level of o, like {...o}.
func (o *gradioObject) clone() *gradioObject {
	c := newObject()
	for _, k := range o.ownKeys() {
		c.set(k, o.vals[k])
	}
	return c
}

func isGradioArrayIndex(k string) bool {
	if k == "" || (len(k) > 1 && k[0] == '0') {
		return false
	}
	n, err := strconv.ParseUint(k, 10, 32)
	return err == nil && n < math.MaxUint32
}

// obj builds an object from alternating keys and values.
func obj(kv ...any) *gradioObject {
	o := newObject()
	for i := 0; i+1 < len(kv); i += 2 {
		o.set(kv[i].(string), kv[i+1])
	}
	return o
}

// --- JSON.parse ---

var errInvalidJSON = errors.New("invalid JSON")

// parseJSON parses data like JSON.parse.
func parseJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, errInvalidJSON
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errInvalidJSON
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := newObject()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := kt.(string)
				if !ok {
					return nil, errInvalidJSON
				}
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				o.set(key, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return o, nil
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
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
		return nil, errInvalidJSON
	case json.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return nil, err
		}
		return f, nil
	case string, bool, nil:
		return t, nil
	}
	return nil, errInvalidJSON
}

// --- JSON.stringify ---

// stringifyJSON returns JSON.stringify(v). ok is false when the result is
// undefined (v is undefined).
func stringifyJSON(v any) (string, bool) {
	var b strings.Builder
	if !writeJSON(&b, v) {
		return "", false
	}
	return b.String(), true
}

func writeJSON(b *strings.Builder, v any) bool {
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case undefinedType:
		return false
	case bool:
		b.WriteString(strconv.FormatBool(t))
	case float64:
		b.WriteString(jsNumberJSON(t))
	case int:
		b.WriteString(strconv.Itoa(t))
	case string:
		writeJSONString(b, t)
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			if !writeJSON(b, e) {
				b.WriteString("null")
			}
		}
		b.WriteByte(']')
	case *gradioObject:
		b.WriteByte('{')
		first := true
		for _, k := range t.ownKeys() {
			var part strings.Builder
			if !writeJSON(&part, t.vals[k]) {
				continue
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			writeJSONString(b, k)
			b.WriteByte(':')
			b.WriteString(part.String())
		}
		b.WriteByte('}')
	case jsBlob:
		b.WriteString("{}")
	case jsBuffer:
		b.WriteString(`{"type":"Buffer","data":[`)
		for i, c := range t.data {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.Itoa(int(c)))
		}
		b.WriteString("]}")
	default:
		panic(fmt.Sprintf("gradio_json: unexpected value %T", v))
	}
	return true
}

// jsNumberJSON prints a number as JSON.stringify does: non-finite is null.
func jsNumberJSON(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	return jsNumberString(f)
}

// jsNumberString is JavaScript's Number.prototype.toString().
func jsNumberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	abs := math.Abs(f)
	if abs >= 1e-6 && abs < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	s := strconv.FormatFloat(f, 'e', -1, 64) // like 1e+21 or 1.5e-07
	mantissa, exp, _ := strings.Cut(s, "e")
	sign := exp[0]
	exp = strings.TrimLeft(exp[1:], "0")
	return mantissa + "e" + string(sign) + exp
}

// writeJSONString quotes s as JSON.stringify does: only ", \, control
// characters and lone surrogates are escaped.
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20:
			fmt.Fprintf(b, `\u%04x`, r)
		case r == utf8.RuneError && size == 1:
			// Invalid UTF-8 cannot come from JSON.parse; keep the replacement.
			b.WriteRune(utf8.RuneError)
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
}

// --- JavaScript property access ---

// errTypeError stands for a TypeError thrown by property access on null or
// undefined, or by calling something that is not a function.
type errTypeError struct{ msg string }

func (e errTypeError) Error() string { return "TypeError: " + e.msg }

func typeError(format string, args ...any) error {
	return errTypeError{fmt.Sprintf(format, args...)}
}

// prop is v[key]. It fails like JavaScript on null and undefined.
func prop(v any, key string) (any, error) {
	switch t := v.(type) {
	case nil, undefinedType:
		return nil, typeError("Cannot read properties of %s (reading '%s')", jsTypeName(v), key)
	case *gradioObject:
		return t.get(key), nil
	case []any:
		if key == "length" {
			return float64(len(t)), nil
		}
		if isGradioArrayIndex(key) {
			i, _ := strconv.Atoi(key)
			if i < len(t) {
				return t[i], nil
			}
		}
		return undefined, nil
	case string:
		if key == "length" {
			return float64(len([]rune(t))), nil
		}
		if isGradioArrayIndex(key) {
			units := utf16Units(t)
			i, _ := strconv.Atoi(key)
			if i < len(units) {
				return units[i], nil
			}
		}
		return undefined, nil
	}
	return undefined, nil
}

// index is v[i].
func index(v any, i int) (any, error) { return prop(v, strconv.Itoa(i)) }

// optProp is v?.[key]: undefined instead of a TypeError for null and undefined.
func optProp(v any, key string) any {
	if v == nil || v == undefined {
		return undefined
	}
	r, _ := prop(v, key)
	return r
}

func jsTypeName(v any) string {
	if v == nil {
		return "null"
	}
	return "undefined"
}

// utf16Units splits s into JavaScript string elements (UTF-16 code units).
func utf16Units(s string) []string {
	var out []string
	for _, r := range s {
		if r >= 0x10000 {
			// Surrogate halves cannot be represented as valid UTF-8 on their
			// own; keep the pair as one element.
			out = append(out, string(r), "")
			continue
		}
		out = append(out, string(r))
	}
	return out
}

// objectView returns what Object.entries / {...v} / destructuring see for v.
// Null and undefined throw.
func objectView(v any) (*gradioObject, error) {
	switch t := v.(type) {
	case nil, undefinedType:
		return nil, typeError("Cannot convert undefined or null to object")
	case *gradioObject:
		return t, nil
	case []any:
		o := newObject()
		for i, e := range t {
			o.set(strconv.Itoa(i), e)
		}
		return o, nil
	case string:
		o := newObject()
		for i, u := range utf16Units(t) {
			o.set(strconv.Itoa(i), u)
		}
		return o, nil
	}
	return newObject(), nil
}

// isTruthy is JavaScript's ToBoolean.
func isTruthy(v any) bool {
	switch t := v.(type) {
	case nil, undefinedType:
		return false
	case bool:
		return t
	case float64:
		return t != 0 && !math.IsNaN(t)
	case string:
		return t != ""
	}
	return true
}
