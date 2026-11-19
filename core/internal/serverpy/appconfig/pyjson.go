package appconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

// Ordered JSON, just enough to read a document and write it back exactly like
// Python's json.dump(data, f, indent=2): object key order is kept, numbers keep
// their literal, strings are ASCII-escaped.

type kv struct {
	Key string
	Val any
}

// object is a JSON object that remembers key order.
type object struct{ Items []kv }

func (o *object) get(k string) (any, bool) {
	for _, it := range o.Items {
		if it.Key == k {
			return it.Val, true
		}
	}
	return nil, false
}

func (o *object) set(k string, v any) {
	for i := range o.Items {
		if o.Items[i].Key == k {
			o.Items[i].Val = v
			return
		}
	}
	o.Items = append(o.Items, kv{k, v})
}

// Values: *object, []any, string, json.Number, bool, nil.

func parse(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after the JSON document")
	}
	return v, nil
}

func parseValue(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, ok := t.(json.Delim)
	if !ok {
		return t, nil // string, json.Number, bool, nil
	}
	if d == '[' {
		arr := []any{}
		for dec.More() {
			v, err := parseValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		_, err := dec.Token()
		return arr, err
	}
	obj := &object{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		v, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		obj.set(kt.(string), v) // Python: the last duplicate wins, at the first position
	}
	_, err = dec.Token()
	return obj, err
}

// marshal is json.dumps(v, indent=2) (ensure_ascii, no trailing newline).
func marshal(v any) []byte {
	var b bytes.Buffer
	write(&b, v, 0)
	return b.Bytes()
}

func write(b *bytes.Buffer, v any, depth int) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case json.Number:
		b.WriteString(string(x))
	case string:
		writeString(b, x)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[")
		for i, e := range x {
			if i > 0 {
				b.WriteString(",")
			}
			indent(b, depth+1)
			write(b, e, depth+1)
		}
		indent(b, depth)
		b.WriteString("]")
	case *object:
		if len(x.Items) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{")
		for i, it := range x.Items {
			if i > 0 {
				b.WriteString(",")
			}
			indent(b, depth+1)
			writeString(b, it.Key)
			b.WriteString(": ")
			write(b, it.Val, depth+1)
		}
		indent(b, depth)
		b.WriteString("}")
	default:
		panic(fmt.Sprintf("appconfig: unsupported JSON value %T", v))
	}
}

func indent(b *bytes.Buffer, depth int) {
	b.WriteString("\n")
	b.WriteString(strings.Repeat("  ", depth))
}

const hexd = "0123456789abcdef"

// writeString is Python's py_encode_basestring_ascii.
func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r >= ' ' && r <= '~':
			b.WriteRune(r)
		case r >= 0x10000:
			hi, lo := utf16.EncodeRune(r)
			u(b, hi)
			u(b, lo)
		default:
			u(b, r)
		}
	}
	b.WriteByte('"')
}

func u(b *bytes.Buffer, r rune) {
	b.WriteString(`\u`)
	b.WriteByte(hexd[r>>12&15])
	b.WriteByte(hexd[r>>8&15])
	b.WriteByte(hexd[r>>4&15])
	b.WriteByte(hexd[r&15])
}
