package extcatalog

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The Extension Catalog tab shows the JSON error the user gets, and until the
// port it was CPython's: "Expecting value: line 1 column 1 (char 0)". Go's
// encoding/json says something else ("invalid character 'o' looking for
// beginning of object key string"), so the pasted-JSON path uses the scanner
// below, which reports the same messages and positions as json.loads.
//
// The file reading path (Parse) keeps encoding/json: there the message only
// goes to the log, and the file parser is the frozen one.
//
// json.loads also accepts NaN and Infinity; this scanner does not (see
// INTEGRATION.md: Go refuses them, and the file writer cannot write them).

// pyJSONError is a JSON error with CPython's message and position.
type pyJSONError struct {
	msg string
	doc []rune
	pos int // rune offset
}

func (e *pyJSONError) Error() string {
	// json.decoder.JSONDecodeError: "%s: line %d column %d (char %d)".
	lineno, colno := 1, e.pos+1
	for i := 0; i < e.pos && i < len(e.doc); i++ {
		if e.doc[i] == '\n' {
			lineno++
			colno = e.pos - i
		}
	}
	return fmt.Sprintf("%s: line %d column %d (char %d)", e.msg, lineno, colno, e.pos)
}

type pyScanner struct {
	s []rune
	i int
}

func (p *pyScanner) err(msg string, pos int) *pyJSONError {
	return &pyJSONError{msg: msg, doc: p.s, pos: pos}
}

// pyParse parses JSON like json.loads does, with CPython's errors.
func pyParse(text string) (any, *pyJSONError) {
	p := &pyScanner{s: []rune(text)}
	v, err := p.scanValue()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.i < len(p.s) {
		return nil, p.err("Extra data", p.i)
	}
	return v, nil
}

// skipSpace skips the JSON whitespace: space, tab, newline, carriage return.
func (p *pyScanner) skipSpace() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func isDigit(c rune) bool { return c >= '0' && c <= '9' }

// scanValue is CPython's scan_once.
func (p *pyScanner) scanValue() (any, *pyJSONError) {
	p.skipSpace()
	if p.i >= len(p.s) {
		return nil, p.err("Expecting value", p.i)
	}
	switch c := p.s[p.i]; {
	case c == '"':
		return p.scanString()
	case c == '{':
		return p.scanObject()
	case c == '[':
		return p.scanArray()
	case c == '}' || c == ']' || c == ',':
		return nil, p.err("Expecting value", p.i)
	case p.has("true"):
		p.i += 4
		return true, nil
	case p.has("false"):
		p.i += 5
		return false, nil
	case p.has("null"):
		p.i += 4
		return nil, nil
	default:
		return p.scanNumber()
	}
}

// has reports whether the text at the cursor starts with word.
func (p *pyScanner) has(word string) bool {
	w := []rune(word)
	if p.i+len(w) > len(p.s) {
		return false
	}
	for k, c := range w {
		if p.s[p.i+k] != c {
			return false
		}
	}
	return true
}

// scanString is CPython's scanstring.
func (p *pyScanner) scanString() (string, *pyJSONError) {
	start := p.i
	p.i++ // the opening quote
	var b strings.Builder
	for {
		if p.i >= len(p.s) {
			return "", p.err("Unterminated string starting at", start)
		}
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			return b.String(), nil
		case c == '\\':
			esc := p.i
			p.i++
			if p.i >= len(p.s) {
				return "", p.err("Unterminated string starting at", start)
			}
			switch p.s[p.i] {
			case '"':
				b.WriteRune('"')
			case '\\':
				b.WriteRune('\\')
			case '/':
				b.WriteRune('/')
			case 'b':
				b.WriteRune('\b')
			case 'f':
				b.WriteRune('\f')
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			case 't':
				b.WriteRune('\t')
			case 'u':
				r, ok := p.readHex4()
				if !ok {
					// CPython reports the position of the "u".
					return "", p.err(`Invalid \uXXXX escape`, p.i)
				}
				p.i++ // past the last hex digit of the escape
				// A surrogate pair is one character, like CPython's.
				if r >= 0xd800 && r <= 0xdbff && p.has("\\u") {
					save := p.i
					p.i = save + 1 // the "u" of the second escape
					r2, ok2 := p.readHex4()
					if ok2 && r2 >= 0xdc00 && r2 <= 0xdfff {
						p.i++ // past the last hex digit
						b.WriteRune(0x10000 + (r-0xd800)<<10 + (r2 - 0xdc00))
						continue
					}
					p.i = save
				}
				b.WriteRune(r)
				continue
			default:
				return "", p.err(`Invalid \escape`, esc)
			}
			p.i++
		case c < 0x20:
			return "", p.err("Invalid control character at", p.i)
		default:
			b.WriteRune(c)
			p.i++
		}
	}
}

// readHex4 reads the four hex digits of a \u escape.
func (p *pyScanner) readHex4() (rune, bool) {
	if p.i+4 >= len(p.s) {
		return 0, false
	}
	var v rune
	for k := 1; k <= 4; k++ {
		c := p.s[p.i+k]
		var d rune
		switch {
		case c >= '0' && c <= '9':
			d = c - '0'
		case c >= 'a' && c <= 'f':
			d = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			d = c - 'A' + 10
		default:
			return 0, false
		}
		v = v*16 + d
	}
	p.i += 4
	return v, true
}

// scanObject is CPython's JSONObject.
func (p *pyScanner) scanObject() (any, *pyJSONError) {
	p.i++ // '{'
	o := newObj()
	p.skipSpace()
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		return o, nil
	}
	for {
		p.skipSpace()
		if p.i >= len(p.s) || p.s[p.i] != '"' {
			return nil, p.err("Expecting property name enclosed in double quotes", p.i)
		}
		key, err := p.scanString()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			return nil, p.err("Expecting ':' delimiter", p.i)
		}
		p.i++
		v, err := p.scanValue()
		if err != nil {
			return nil, err
		}
		o.Set(key, v)
		p.skipSpace()
		if p.i >= len(p.s) {
			return nil, p.err("Expecting ',' delimiter", p.i)
		}
		if p.s[p.i] == '}' {
			p.i++
			return o, nil
		}
		if p.s[p.i] != ',' {
			return nil, p.err("Expecting ',' delimiter", p.i)
		}
		p.i++
	}
}

// scanArray is CPython's JSONArray.
func (p *pyScanner) scanArray() (any, *pyJSONError) {
	p.i++ // '['
	arr := []any{}
	p.skipSpace()
	if p.i < len(p.s) && p.s[p.i] == ']' {
		p.i++
		return arr, nil
	}
	for {
		v, err := p.scanValue()
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
		p.skipSpace()
		if p.i >= len(p.s) {
			return nil, p.err("Expecting ',' delimiter", p.i)
		}
		if p.s[p.i] == ']' {
			p.i++
			return arr, nil
		}
		if p.s[p.i] != ',' {
			return nil, p.err("Expecting ',' delimiter", p.i)
		}
		p.i++
	}
}

// scanNumber is CPython's NUMBER_RE: -?(0|[1-9]\d*)(\.\d+)?([eE][-+]?\d+)?.
//
// The value is a float when it has a fraction or an exponent (CPython makes
// it one), and an int otherwise. The int keeps its text, so a big number
// stays as it was written.
func (p *pyScanner) scanNumber() (any, *pyJSONError) {
	start := p.i
	i := p.i
	if i < len(p.s) && p.s[i] == '-' {
		i++
	}
	if i >= len(p.s) {
		return nil, p.err("Expecting value", start)
	}
	isFloat := false
	switch {
	case p.s[i] == '0':
		i++
	case p.s[i] >= '1' && p.s[i] <= '9':
		for i < len(p.s) && isDigit(p.s[i]) {
			i++
		}
	default:
		return nil, p.err("Expecting value", start)
	}
	if i < len(p.s) && p.s[i] == '.' {
		j := i + 1
		for j < len(p.s) && isDigit(p.s[j]) {
			j++
		}
		if j > i+1 {
			i = j
			isFloat = true
		}
	}
	if i < len(p.s) && (p.s[i] == 'e' || p.s[i] == 'E') {
		j := i + 1
		if j < len(p.s) && (p.s[j] == '+' || p.s[j] == '-') {
			j++
		}
		k := j
		for k < len(p.s) && isDigit(p.s[k]) {
			k++
		}
		if k > j {
			i = k
			isFloat = true
		}
	}
	p.i = i
	text := string(p.s[start:i])
	if isFloat {
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, p.err("Expecting value", start)
		}
		return f, nil
	}
	return json.Number(text), nil
}
