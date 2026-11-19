package appenv

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file is a port of python-dotenv 1.0.0 parser.py / variables.py.
// Python's \s is Unicode-aware (str.isspace), Go's is ASCII only, so every
// regex below is built from an explicit class.

const pySpaceClass = `\x09-\x0d\x1c-\x20\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// inlineSpaceClass is [^\S\r\n]: whitespace except CR and LF.
const inlineSpaceClass = `\x09\x0b\x0c\x1c-\x20\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

func anch(s string) *regexp.Regexp { return regexp.MustCompile(`^(?:` + s + `)`) }

var (
	reMultiWS    = anch(`[` + pySpaceClass + `]*`)
	reWS         = anch(`[` + inlineSpaceClass + `]*`)
	reExport     = anch(`(?:export[` + inlineSpaceClass + `]+)?`)
	reSQKey      = anch(`'([^']+)'`)
	reUnqKey     = anch(`([^=#` + pySpaceClass + `]+)`)
	reEqual      = anch(`=[` + inlineSpaceClass + `]*`)
	reSQValue    = anch(`'((?:\\'|[^'])*)'`)
	reDQValue    = anch(`"((?:\\"|[^"])*)"`)
	reUnqValue   = anch(`([^\r\n]*)`)
	reComment    = anch(`(?:[` + inlineSpaceClass + `]*#[^\r\n]*)?`)
	reEOL        = anch(`[` + inlineSpaceClass + `]*(?:\r\n|\n|\r|$)`)
	reRestOfLine = anch(`[^\r\n]*(?:\r|\n|\r\n)?`)
	reDQEscapes  = regexp.MustCompile(`\\[\\'"abfnrtv]`)
	reSQEscapes  = regexp.MustCompile(`\\[\\']`)
	reInlineCmt  = regexp.MustCompile(`[` + pySpaceClass + `]+#.*`)
	rePosixVar   = regexp.MustCompile(`\$\{([^}:]*)(?::-([^}]*))?\}`)
)

// binding is one parsed statement. key=="" means no key (comment/blank/error).
type binding struct {
	key      string
	value    string
	hasValue bool
	isErr    bool
	line     int
}

type reader struct {
	s    string
	pos  int // byte offset
	line int
}

func (r *reader) match(re *regexp.Regexp) ([]string, bool) {
	m := re.FindStringSubmatch(r.s[r.pos:])
	if m == nil {
		return nil, false
	}
	r.pos += len(m[0])
	r.line += strings.Count(strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(m[0]), "\n")
	return m, true
}

func (r *reader) peek() string {
	if r.pos >= len(r.s) {
		return ""
	}
	_, n := utf8.DecodeRuneInString(r.s[r.pos:])
	return r.s[r.pos : r.pos+n]
}

func pyIsSpace(c rune) bool { return unicode.IsSpace(c) || (c >= 0x1c && c <= 0x1f) }

var dqEscapes = map[string]string{`\\`: `\`, `\'`: `'`, `\"`: `"`, `\a`: "\a", `\b`: "\b", `\f`: "\f", `\n`: "\n", `\r`: "\r", `\t`: "\t", `\v`: "\v"}

func decodeEscapes(re *regexp.Regexp, s string) string {
	return re.ReplaceAllStringFunc(s, func(m string) string { return dqEscapes[m] })
}

func parseBinding(r *reader) binding {
	startLine := r.line
	fail := func() binding {
		r.match(reRestOfLine)
		return binding{isErr: true, line: startLine}
	}
	r.match(reMultiWS)
	if r.pos >= len(r.s) {
		return binding{line: startLine}
	}
	r.match(reExport)
	var key string
	switch r.peek() {
	case "#":
		key = ""
	case "'":
		m, ok := r.match(reSQKey)
		if !ok {
			return fail()
		}
		key = m[1]
	default:
		m, ok := r.match(reUnqKey)
		if !ok {
			return fail()
		}
		key = m[1]
	}
	r.match(reWS)
	var value string
	hasValue := false
	if r.peek() == "=" {
		r.match(reEqual)
		hasValue = true
		switch c := r.peek(); c {
		case "'":
			m, ok := r.match(reSQValue)
			if !ok {
				return fail()
			}
			value = decodeEscapes(reSQEscapes, m[1])
		case `"`:
			m, ok := r.match(reDQValue)
			if !ok {
				return fail()
			}
			value = decodeEscapes(reDQEscapes, m[1])
		case "", "\n", "\r":
			value = ""
		default:
			m, _ := r.match(reUnqValue)
			value = strings.TrimRightFunc(reInlineCmt.ReplaceAllString(m[1], ""), pyIsSpace)
		}
	}
	r.match(reComment)
	if _, ok := r.match(reEOL); !ok {
		return fail()
	}
	return binding{key: key, value: value, hasValue: hasValue, line: startLine}
}

// parseDotenv parses text (already newline-normalised) into bindings with a key.
// errLines lists the start line of every statement python-dotenv cannot parse.
func parseDotenv(text string) (out []binding, errLines []int) {
	r := &reader{s: text, line: 1}
	for r.pos < len(r.s) {
		b := parseBinding(r)
		if b.isErr {
			errLines = append(errLines, b.line)
		} else if b.key != "" {
			out = append(out, b)
		}
	}
	return
}

// resolveVars expands ${NAME} and ${NAME:-default} in value against env.
// A name present in env with a nil value resolves to "" (not the default).
func resolveVars(value string, env map[string]*string) string {
	var sb strings.Builder
	cur := 0
	for _, m := range rePosixVar.FindAllStringSubmatchIndex(value, -1) {
		sb.WriteString(value[cur:m[0]])
		name := value[m[2]:m[3]]
		def := ""
		if m[4] >= 0 {
			def = value[m[4]:m[5]]
		}
		if v, ok := env[name]; ok {
			if v != nil {
				sb.WriteString(*v)
			}
		} else {
			sb.WriteString(def)
		}
		cur = m[1]
	}
	sb.WriteString(value[cur:])
	return sb.String()
}
