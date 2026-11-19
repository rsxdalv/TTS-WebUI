package react_ui_backend

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func uploadFixture(t *testing.T) (cache string, srv *httptest.Server) {
	t.Helper()
	root := t.TempDir()
	cache = filepath.Join(root, "react-ui", "public", "file-input-cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("POST /api/upload", newUpload(Deps{Root: root}))
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return cache, srv
}

func postFile(t *testing.T, srv *httptest.Server, field, filename string, content []byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	w, _ := mw.CreateFormFile(field, "x")
	_, _ = w.Write(content)
	_ = mw.Close()
	// CreateFormFile escapes the name, so write the raw filename here.
	raw := strings.Replace(body.String(), `filename="x"`, `filename="`+filename+`"`, 1)
	resp, err := http.Post(srv.URL+"/api/upload", mw.FormDataContentType(), strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestUploadWritesAndSanitises(t *testing.T) {
	cache, srv := uploadFixture(t)
	for _, tc := range []struct{ filename, want string }{
		{"a.wav", "a.wav"},
		{"B.MP3", "B.MP3"},
		{"../../evil.wav", "evil.wav"},
		{"/etc/abs.wav", "abs.wav"},
		{"...dots.wav", "dots.wav"},
	} {
		status, body := postFile(t, srv, "file", tc.filename, []byte("DATA"))
		if status != 200 || body != `{"success":true,"name":"`+tc.want+`"}` {
			t.Fatalf("%q: %d %s", tc.filename, status, body)
		}
		if b, err := os.ReadFile(filepath.Join(cache, tc.want)); err != nil || string(b) != "DATA" {
			t.Fatalf("%q: stored %q %v", tc.filename, b, err)
		}
	}
	// Nothing was written outside the cache directory, and no temp file is left.
	entries, _ := os.ReadDir(cache)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".upload-") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(cache, "..", "evil.wav")); err == nil {
		t.Fatal("traversal wrote outside the cache directory")
	}
}

func TestUploadRejects(t *testing.T) {
	cache, srv := uploadFixture(t)
	for _, tc := range []struct {
		field, filename string
		status          int
		body            string
	}{
		{"file", "bad.exe", 400, `{"success":false,"error":"Unsupported file type"}`},
		{"file", "noext", 400, `{"success":false,"error":"Unsupported file type"}`},
		{"file", "....wav", 400, `{"success":false,"error":"Unsupported file type"}`},
		{"file", "", 400, `{"success":false,"error":"Unsupported file type"}`},
		{"other", "a.wav", 400, `{"success":false}`},
	} {
		if status, body := postFile(t, srv, tc.field, tc.filename, []byte("x")); status != tc.status || body != tc.body {
			t.Errorf("%q: %d %s", tc.filename, status, body)
		}
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Errorf("rejected uploads left files: %v", entries)
	}
}

func TestUploadBrokenBodyIs500AndLeavesNothing(t *testing.T) {
	cache, srv := uploadFixture(t)
	body := "--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"t.wav\"\r\n\r\nDATA" // no closing boundary
	resp, err := http.Post(srv.URL+"/api/upload", "multipart/form-data; boundary=b", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Fatalf("left files: %v", entries)
	}
}

// A body over the limit is answered 413 and is streamed, not held in memory.
func TestUploadOverLimitIsStreamed(t *testing.T) {
	if testing.Short() {
		t.Skip("streams 256 MiB")
	}
	cache, srv := uploadFixture(t)
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		w, _ := mw.CreateFormFile("file", "big.wav")
		chunk := make([]byte, 1<<20)
		for i := 0; i < 256+1; i++ { // 257 MiB
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
		_ = mw.Close()
		_ = pw.Close()
	}()
	resp, err := http.Post(srv.URL+"/api/upload", mw.FormDataContentType(), pr)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 413 || string(b) != `{"success":false,"error":"File too large"}` {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.HeapSys > 128<<20 {
		t.Fatalf("heap grew to %d MiB while streaming 257 MiB", m.HeapSys>>20)
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Fatalf("left files: %v", entries)
	}
}

func TestSafeUploadName(t *testing.T) {
	for in, want := range map[string]string{
		"a.wav": "a.wav", "../a.wav": "a.wav", "x/y/a.wav": "a.wav", "...a.wav": "a.wav", ".": "", "..": "", "a.wav/": "a.wav", "": "",
	} {
		if got := safeUploadName(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
