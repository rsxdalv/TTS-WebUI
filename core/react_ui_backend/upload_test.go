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

// F3: temporary files left by a killed run are removed when the claim starts;
// other files stay.
func TestUploadRemovesStaleTempFilesAtStart(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "react-ui", "public", "file-input-cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".upload-123", ".upload-456", ".gitkeep", "a.wav", ".uploadx-1"} {
		if err := os.WriteFile(filepath.Join(cache, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(cache, ".upload-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	newUpload(Deps{Root: root})
	for name, want := range map[string]bool{".upload-123": false, ".upload-456": false, ".gitkeep": true, "a.wav": true, ".uploadx-1": true, ".upload-dir": true} {
		_, err := os.Stat(filepath.Join(cache, name))
		if (err == nil) != want {
			t.Errorf("%s: exists=%v, want %v", name, err == nil, want)
		}
	}
	// A missing cache directory is not an error here (the upload itself is a 500).
	newUpload(Deps{Root: t.TempDir()})
}

// dirSize is the bytes in a directory.
func dirSize(dir string) int64 {
	var n int64
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		// os.Stat opens the file; the size in a directory listing can lag on Windows.
		if info, err := os.Stat(filepath.Join(dir, e.Name())); err == nil {
			n += info.Size()
		}
	}
	return n
}

// F4: an upload over the limit never puts more than the limit on disk, is
// answered 413 as before, and leaves nothing.
func TestUploadOverLimitStopsWritingAtTheLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("streams 300 MiB")
	}
	cache, srv := uploadFixture(t)
	var maxOnDisk int64
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		w, _ := mw.CreateFormFile("file", "big.wav")
		chunk := make([]byte, 1<<20)
		for i := 0; i < 300; i++ { // 300 MiB, sent chunked: no Content-Length
			if _, err := w.Write(chunk); err != nil {
				return
			}
			if n := dirSize(cache); n > maxOnDisk {
				maxOnDisk = n
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
	if maxOnDisk > uploadMaxBytes+(1<<20) {
		t.Fatalf("%d bytes were on disk, the limit is %d", maxOnDisk, int64(uploadMaxBytes))
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Fatalf("left files: %v", entries)
	}
}

type failingBody struct{ t *testing.T }

func (f failingBody) Read([]byte) (int, error) {
	f.t.Error("the body was read although Content-Length is over the limit")
	return 0, io.EOF
}

// F4: a Content-Length over the limit is answered 413 without reading the body.
func TestUploadRejectsOversizedContentLengthEarly(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "react-ui", "public", "file-input-cache")
	_ = os.MkdirAll(cache, 0o755)
	h := newUpload(Deps{Root: root})
	req := httptest.NewRequest("POST", "/api/upload", failingBody{t})
	req.ContentLength = uploadMaxBytes + 10<<20
	req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 413 || rec.Body.String() != `{"success":false,"error":"File too large"}` || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	// Just under the framing slack is still read and parsed.
	req = httptest.NewRequest("POST", "/api/upload", strings.NewReader(""))
	req.ContentLength = uploadMaxBytes + 1<<19
	req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == 413 {
		t.Fatal("a Content-Length within the framing slack must not be rejected early")
	}
}

// A file that is rejected for its type is read but never written.
func TestUploadRejectedTypeIsNotWritten(t *testing.T) {
	cache, srv := uploadFixture(t)
	if status, _ := postFile(t, srv, "file", "bad.exe", make([]byte, 5<<20)); status != 400 {
		t.Fatalf("status %d", status)
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Fatalf("left files: %v", entries)
	}
}
