package extcatalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Obj is a JSON object that keeps its key order, like a Python dict: the
// extension files are edited by hand and read by Python, so a rewrite must
// not shuffle them.
type Obj struct {
	Keys []string
	Vals map[string]any
}

func newObj() *Obj { return &Obj{Vals: map[string]any{}} }

// Get returns the value of k.
func (o *Obj) Get(k string) (any, bool) {
	v, ok := o.Vals[k]
	return v, ok
}

// Str returns k as a string ("" when missing or not a string).
func (o *Obj) Str(k string) string {
	s, _ := o.Vals[k].(string)
	return s
}

// Set replaces or appends k.
func (o *Obj) Set(k string, v any) {
	if _, ok := o.Vals[k]; !ok {
		o.Keys = append(o.Keys, k)
	}
	o.Vals[k] = v
}

// MarshalJSON writes the object compactly with keys in order, so an Obj sent
// to Python keeps its order through a round trip.
func (o *Obj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	write(&b, o, "", 0, false)
	return b.Bytes(), nil
}

// Parse decodes JSON keeping object key order (objects become *Obj, arrays
// []any, numbers json.Number).
func Parse(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after the JSON value")
	}
	return v, nil
}

func parseValue(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := t.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := newObj()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k := kt.(string)
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				o.Set(k, v) // a repeated key keeps the last value, like Python
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return o, nil
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
	default:
		return t, nil
	}
	return nil, fmt.Errorf("unexpected token")
}

// Marshal writes v like json.dump(v, f, indent=indent, ensure_ascii=False):
// ordered keys, UTF-8 left as is, "key": value. indent "" is compact.
func Marshal(v any, indent string) []byte {
	var b bytes.Buffer
	write(&b, v, indent, 0, indent != "")
	return b.Bytes()
}

func write(b *bytes.Buffer, v any, indent string, depth int, pretty bool) {
	nl := func(d int) {
		if pretty {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(indent, d))
		}
	}
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case json.Number:
		b.WriteString(x.String())
	case float64:
		// Python's json.dumps writes a float with repr(): 1000.0, not 1000.
		// Only pyParse (the pasted text of the Extension Catalog tab) makes
		// float64 values; the file reader keeps the numbers as they were
		// written (json.Number).
		b.WriteString(formatFloat(x))
	case string:
		writeString(b, x)
	case *Obj:
		if len(x.Keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, k := range x.Keys {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(depth + 1)
			writeString(b, k)
			b.WriteString(":")
			if pretty {
				b.WriteByte(' ')
			}
			write(b, x.Vals[k], indent, depth+1, pretty)
		}
		nl(depth)
		b.WriteByte('}')
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(depth + 1)
			write(b, e, indent, depth+1, pretty)
		}
		nl(depth)
		b.WriteByte(']')
	default:
		raw, err := json.Marshal(x) // plain Go values (tests, responses)
		if err != nil {
			b.WriteString("null")
			return
		}
		b.Write(raw)
	}
}

// formatFloat writes a float like Python's repr(): the shortest text that
// gives the float back, always with a point or an exponent.
func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	// Go writes 1e+100, Python writes 1e+100 too; but Go writes 1e-07 where
	// Python writes 1e-07: both use at least two digits.
	return s
}

func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
