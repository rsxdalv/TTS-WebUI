package react_ui_backend

// Data endpoint for frontend/src/pages/text-to-speech/bark/voices.tsx (task
// B5b): the `outputs` array that getStaticProps builds with
// getNpzDataSimpleVoices() in frontend/src/data/getVoicesData.tsx. Behind the
// claim switch data-bark-voices.
//
// For every file of <root>/voices that ends in ".npz" (case-sensitive) the
// original reads the zip, parses the entry named metadata.npy as an .npy array
// of UTF-32 characters (frontend/src/data/npz-parsing.ts, npyToUtf8.ts), parses
// that text as JSON and spreads it into
//   {...metadata, filename: "voices/<file>", url: "api/webui-generations/voices/<file>"}.
// Anything that cannot be parsed (not a zip, no metadata entry, another dtype,
// wrong length, invalid JSON) gives just filename and url. A missing voices
// directory gives []; a ".npz" entry that cannot be read at all (a directory)
// makes the original throw, which is a 500 here.
//
// Known gaps: only whole 32-bit text is decoded for other dtypes; a metadata
// string that is itself a JSON string with lone surrogates is not reproduced.

import (
	"archive/zip"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "data-bark-voices",
		Patterns: []string{"GET /api/go-data/bark-voices"},
		Handler:  newDataBarkVoices,
	})
}

func newDataBarkVoices(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		outputs, err := npzVoices(deps.Root)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError) // getStaticProps throws
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, stringifyJS(outputs))
	})
}

// npzVoices is getNpzDataSimpleVoices().
func npzVoices(root string) ([]any, error) {
	dir := filepath.Join(root, "voices")
	out := []any{}
	for _, name := range readdirNames(dir) {
		if !strings.HasSuffix(name, ".npz") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		item := newJSObject()
		spreadInto(item, parseNpzMetadata(raw))
		item.set("filename", "voices/"+name)
		item.set("url", "api/webui-generations/voices/"+name)
		out = append(out, item)
	}
	return out, nil
}

// spreadInto is {...v}: an object copies its keys, an array its indexes, a
// string its UTF-16 units; anything else adds nothing.
func spreadInto(dst *jsObject, v any) {
	switch t := v.(type) {
	case *jsObject:
		for _, k := range t.keys {
			dst.set(k, t.vals[k])
		}
	case []any:
		for i, e := range t {
			dst.set(itoa(i), e)
		}
	case string:
		for i, u := range utf16.Encode([]rune(t)) {
			dst.set(itoa(i), string(rune(u)))
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

// parseNpzMetadata is parseToNpzData: nil when anything goes wrong.
func parseNpzMetadata(data []byte) any {
	zr, err := zip.NewReader(bytesReaderAt(data), int64(len(data)))
	if err != nil {
		return nil
	}
	var first any
	found := false
	for _, f := range zr.File {
		name := f.Name
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if name != "metadata.npy" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		npy, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil
		}
		text, err := npyToText(npy)
		if err != nil {
			return nil
		}
		v, err := parseJS([]byte(text))
		if err != nil {
			return nil
		}
		if !found {
			first, found = v, true
		}
	}
	return first
}

type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// elementSize is the size of the typed array the original builds per dtype.
var npyElementSize = map[string]int{
	"<u1": 1, "|u1": 1, "<u2": 2, "|i1": 1, "<i2": 2, "<u4": 4, "<i4": 4,
	"<u8": 8, "<i8": 8, "<f4": 4, "<f8": 8, "<U1": 4,
}

var (
	npyTrue    = regexp.MustCompile(`True`)
	npyFalse   = regexp.MustCompile(`False`)
	npyParens  = regexp.MustCompile(`,*\),*`)
	errNpyBad  = errors.New("bad npy")
	errNpyText = errors.New("unexpected string length")
)

// npyToText is npyToUtf8(parseNpy(buffer)).
func npyToText(buf []byte) (string, error) {
	if len(buf) < 10 {
		return "", errNpyBad // DataView(slice(8, 10)).getUint8 throws
	}
	headerLength := int(buf[8])
	offset := 10 + headerLength
	end := min(offset, len(buf))
	header := strings.ToValidUTF8(string(buf[10:end]), "�")
	header = npyTrue.ReplaceAllString(header, "true")
	header = npyFalse.ReplaceAllString(header, "false")
	header = strings.ReplaceAll(header, "'", `"`)
	header = strings.Replace(header, "(", "[", 1)
	header = npyParens.ReplaceAllString(header, "]")
	parsed, err := parseJS([]byte(header))
	if err != nil {
		return "", err
	}
	obj, ok := parsed.(*jsObject)
	if !ok {
		return "", errNpyBad
	}
	descr, _ := obj.get("descr")
	size, known := npyElementSize[asString(descr)]
	if !known {
		return "", errNpyBad // Unknown dtype
	}
	if offset > len(buf) || offset%size != 0 || (len(buf)-offset)%size != 0 {
		return "", errNpyBad // new TypedArray(buffer, offset) throws a RangeError
	}
	body := buf[offset:]
	if len(body)%4 != 0 {
		return "", errNpyBad
	}
	var sb strings.Builder
	units := 0
	for i := 0; i < len(body); i += 4 {
		cp := uint32(body[i]) | uint32(body[i+1])<<8 | uint32(body[i+2])<<16 | uint32(body[i+3])<<24
		r := rune(cp)
		if cp > utf8.MaxRune || (cp >= 0xD800 && cp <= 0xDFFF) {
			r = utf8.RuneError
		}
		sb.WriteRune(r)
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
	}
	shape, _ := obj.get("shape")
	arr, isArr := shape.([]any)
	if !isArr || len(arr) == 0 {
		return "", errNpyText
	}
	if n, isNum := arr[0].(float64); !isNum || int(n) != units || float64(units) != n {
		return "", errNpyText
	}
	return sb.String(), nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
