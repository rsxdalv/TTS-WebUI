package react_ui_backend

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

// npyChars is what numpy writes for np.array(list(text)): dtype <U1, one
// UTF-32 value per character, header padded to a multiple of 64 bytes.
func npyChars(text string, shapeLen int) []byte {
	header := fmt.Sprintf("{'descr': '<U1', 'fortran_order': False, 'shape': (%d,), }", shapeLen)
	for (10+len(header)+1)%64 != 0 {
		header += " "
	}
	header += "\n"
	var b bytes.Buffer
	b.WriteString("\x93NUMPY\x01\x00")
	_ = binary.Write(&b, binary.LittleEndian, uint16(len(header)))
	b.WriteString(header)
	for _, r := range text {
		_ = binary.Write(&b, binary.LittleEndian, uint32(r))
	}
	return b.Bytes()
}

func makeNpz(t *testing.T, method uint16, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }

func TestBarkVoices(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "voices")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	put := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	meta := `{"prompt":"hi","b":2,"2":"x","filename":"old","a":[1,2.50]}`
	put("a-stored.npz", makeNpz(t, zip.Store, map[string][]byte{"metadata.npy": npyChars(meta, utf16Len(meta)), "tokens.npy": {1, 2}}))
	put("b-deflate.npz", makeNpz(t, zip.Deflate, map[string][]byte{"sub/metadata.npy": npyChars(`{"n":1}`, 7)}))
	put("c-nometa.npz", makeNpz(t, zip.Store, map[string][]byte{"tokens.npy": {1, 2}}))
	put("d-notzip.npz", []byte("not a zip"))
	put("e-wronglen.npz", makeNpz(t, zip.Store, map[string][]byte{"metadata.npy": npyChars(`{"n":1}`, 9)}))
	put("f-badjson.npz", makeNpz(t, zip.Store, map[string][]byte{"metadata.npy": npyChars(`{no`, 3)}))
	put("g-array.npz", makeNpz(t, zip.Store, map[string][]byte{"metadata.npy": npyChars(`["x",5]`, 7)}))
	put("h-emoji.npz", makeNpz(t, zip.Store, map[string][]byte{"metadata.npy": npyChars("\"\U0001F600\"", 3)})) // code points 3, UTF-16 units 4
	put("UPPER.NPZ", makeNpz(t, zip.Store, nil))
	put("notes.txt", []byte("x"))

	mux := http.NewServeMux()
	mux.Handle("GET /api/go-data/bark-voices", newDataBarkVoices(Deps{Root: root}))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, body := get(t, srv, "/api/go-data/bark-voices", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	fu := func(n string) string {
		return `"filename":"voices/` + n + `","url":"api/webui-generations/voices/` + n + `"`
	}
	want := "[" + strings.Join([]string{
		`{"2":"x","prompt":"hi","b":2,"filename":"voices/a-stored.npz","a":[1,2.5],"url":"api/webui-generations/voices/a-stored.npz"}`,
		`{"n":1,` + fu("b-deflate.npz") + `}`,
		`{` + fu("c-nometa.npz") + `}`,
		`{` + fu("d-notzip.npz") + `}`,
		`{` + fu("e-wronglen.npz") + `}`,
		`{` + fu("f-badjson.npz") + `}`,
		`{"0":"x","1":5,` + fu("g-array.npz") + `}`,
		`{` + fu("h-emoji.npz") + `}`,
	}, ",") + "]"
	if body != want {
		t.Fatalf("\n got %s\nwant %s", body, want)
	}
}

func TestBarkVoicesMissingDirAndUnreadableEntry(t *testing.T) {
	root := t.TempDir()
	mux := http.NewServeMux()
	mux.Handle("GET /api/go-data/bark-voices", newDataBarkVoices(Deps{Root: root}))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if resp, body := get(t, srv, "/api/go-data/bark-voices", nil); resp.StatusCode != 200 || body != "[]" {
		t.Fatalf("missing dir: %d %q", resp.StatusCode, body)
	}
	if err := os.MkdirAll(filepath.Join(root, "voices", "dir.npz"), 0o755); err != nil {
		t.Fatal(err)
	}
	if resp, _ := get(t, srv, "/api/go-data/bark-voices", nil); resp.StatusCode != 500 {
		t.Fatalf("a directory named x.npz makes the original throw: %d", resp.StatusCode)
	}
}
