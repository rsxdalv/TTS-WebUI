package react_ui_backend

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exportIndex is the app's entry page. It is larger than 1 KB so that the
// compression test has something to compress.
var exportIndex = "<html>home" + strings.Repeat("<p>filler</p>", 200) + "</html>"

// exportFixture writes a small single-page export and public folder.
func exportFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"frontend/out/index.html":                  exportIndex,
		"frontend/out/assets/a.js":                 "console.log(1)",
		"frontend/out/favicon.ico":                 "ICO",
		"frontend/out/_tts-export.json":            `{"commit":"C1"}`,
		"frontend/out/.hidden":                     "hidden",
		"frontend/public/robots.txt":               "robots",
		"frontend/public/file-input-cache/.secret": "secret",
		"frontend/public/.env":                     "secret",
	}
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func staticFront(t *testing.T, root string) string {
	t.Helper()
	h, err := NewHandler(Config{Root: root, Port: testPort, Routes: routes(t, "")})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	return front.URL
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// notFoundBody is what http.NotFound writes.
const notFoundBody = "404 page not found\n"

func TestHasExport(t *testing.T) {
	root := exportFixture(t)
	if !HasExport(root) {
		t.Fatal("complete export not found")
	}
	if err := os.Remove(filepath.Join(ExportDir(root), ExportMarker)); err != nil {
		t.Fatal(err)
	}
	if HasExport(root) {
		t.Fatal("an export without its marker counts as complete")
	}
}

func TestStaticExportMapping(t *testing.T) {
	root := exportFixture(t)
	front := staticFront(t, root)

	// Uploaded after the server started: served at once (R1's O2).
	upload := filepath.Join(root, "frontend", "public", "file-input-cache", "up.wav")
	if err := os.WriteFile(upload, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}

	const html = "text/html; charset=utf-8"
	cases := []struct {
		path, want   string
		status       int
		cacheControl string
		contentType  string
	}{
		// Routes of the app: every one gets the entry page, so a reload works.
		{"/", exportIndex, 200, cachePage, html},
		{"/index.html", exportIndex, 200, cachePage, html},
		{"/tools", exportIndex, 200, cachePage, html},
		{"/tools/gpu_info", exportIndex, 200, cachePage, html},
		{"/outputs/favorites", exportIndex, 200, cachePage, html},
		{"/extensions/my_new_extension", exportIndex, 200, cachePage, html},
		{"/extensions/a/b", exportIndex, 200, cachePage, html},
		{"/nope", exportIndex, 200, cachePage, html},
		// Files of the export.
		{"/assets/a.js", "console.log(1)", 200, cacheImmutable, "application/javascript; charset=UTF-8"},
		{"/favicon.ico", "ICO", 200, "public, max-age=0", "image/x-icon"},
		{"/file-input-cache/up.wav", "RIFF", 200, "public, max-age=0", "audio/wav"},
		{"/robots.txt", "robots", 200, "public, max-age=0", ""},
		// A missing file (a path with an extension) is a 404, not the entry page.
		{"/missing.js", notFoundBody, 404, cacheNotFound, ""},
		{"/assets/missing.js", notFoundBody, 404, cacheNotFound, ""},
		{"/file-input-cache/missing.wav", notFoundBody, 404, cacheNotFound, ""},
		// Rule 1: dotfiles and odd paths never reach a file.
		{"/.hidden", notFoundBody, 404, cacheNotFound, ""},
		{"/.env", notFoundBody, 404, cacheNotFound, ""},
		{"/file-input-cache/.secret", notFoundBody, 404, cacheNotFound, ""},
		{"/%2ehidden", notFoundBody, 404, cacheNotFound, ""},
		{"/x/..%2f.hidden", notFoundBody, 404, cacheNotFound, ""},
		{"/tools%5cgpu_info", notFoundBody, 404, cacheNotFound, ""},
		{"/a%00b", notFoundBody, 404, cacheNotFound, ""},
	}
	for _, c := range cases {
		resp := request(t, "GET", front+c.path, nil, nil)
		got := body(t, resp)
		if resp.StatusCode != c.status || got != c.want {
			t.Errorf("%s: %d %q, want %d %q", c.path, resp.StatusCode, got, c.status, c.want)
			continue
		}
		if c.cacheControl != "" && resp.Header.Get("Cache-Control") != c.cacheControl {
			t.Errorf("%s: Cache-Control %q, want %q", c.path, resp.Header.Get("Cache-Control"), c.cacheControl)
		}
		if c.contentType != "" && resp.Header.Get("Content-Type") != c.contentType {
			t.Errorf("%s: Content-Type %q, want %q", c.path, resp.Header.Get("Content-Type"), c.contentType)
		}
	}
}

func TestStaticExportRedirectsAndMethods(t *testing.T) {
	front := staticFront(t, exportFixture(t))

	resp := request(t, "GET", front+"/tools/?x=1", nil, nil)
	if resp.StatusCode != http.StatusPermanentRedirect || resp.Header.Get("Location") != "/tools?x=1" {
		t.Fatalf("trailing slash: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	// An empty segment is refused, so this can never become an open redirect.
	resp = request(t, "GET", front+"//evil.example/", nil, nil)
	if resp.StatusCode != 404 || resp.Header.Get("Location") != "" {
		t.Fatalf("//evil.example/: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp := request(t, "POST", front+"/", nil, nil); resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /: %d", resp.StatusCode)
	}
	for _, method := range []string{"GET", "POST"} {
		resp = request(t, method, front+"/api/unknown", nil, nil)
		if got := body(t, resp); resp.StatusCode != 404 || strings.Contains(got, "<html>") {
			t.Fatalf("%s /api/unknown: %d %q; an API path must get a plain 404", method, resp.StatusCode, got)
		}
	}
	resp = request(t, "HEAD", front+"/tools", nil, nil)
	if got := body(t, resp); resp.StatusCode != 200 || got != "" {
		t.Fatalf("HEAD: %d %q", resp.StatusCode, got)
	}
}

func TestStaticExportETagAndCompression(t *testing.T) {
	front := staticFront(t, exportFixture(t))

	resp := request(t, "GET", front+"/tools", nil, nil)
	etag := resp.Header.Get("ETag")
	if etag != nextETag(exportIndex) {
		t.Fatalf("ETag %q", etag)
	}
	if resp := request(t, "GET", front+"/tools", nil, map[string]string{"If-None-Match": etag}); resp.StatusCode != http.StatusNotModified {
		t.Fatalf("If-None-Match: %d", resp.StatusCode)
	}

	resp = request(t, "GET", front+"/assets/a.js", nil, nil)
	weak := resp.Header.Get("ETag")
	resp = request(t, "GET", front+"/assets/a.js", nil, map[string]string{"If-None-Match": weak})
	if resp.StatusCode != http.StatusNotModified || resp.Header.Get("Cache-Control") != cacheImmutable {
		t.Fatalf("static 304: %d %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}

	req, _ := http.NewRequest("GET", front+"/tools", nil)
	req.Host = "127.0.0.1:31050"
	req.Header.Set("Accept-Encoding", "gzip")
	gz, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Body.Close()
	if gz.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding %q", gz.Header.Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(gz.Body)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(zr)
	if !bytes.Equal(plain, []byte(exportIndex)) {
		t.Fatalf("decompressed %q", plain[:20])
	}
}

func TestStaticModeKeepsClaimsAndGuards(t *testing.T) {
	root := exportFixture(t)
	h, err := NewHandler(Config{Root: root, Port: testPort, Routes: routes(t, "test-files")})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	defer front.Close()
	if resp := request(t, "GET", front.URL+"/api/test-files/a.ogg", nil, nil); resp.Header.Get("X-Answered-By") != "test-files" {
		t.Fatalf("claim not answered in static mode: %d", resp.StatusCode)
	}
	if resp := request(t, "GET", front.URL+"/", nil, map[string]string{"Host": "evil.example:31050"}); resp.StatusCode != 400 {
		t.Fatalf("Host guard: %d", resp.StatusCode)
	}
	if resp := request(t, "POST", front.URL+"/api/upload", nil, map[string]string{"Origin": "http://evil.example"}); resp.StatusCode != 403 {
		t.Fatalf("Origin guard: %d", resp.StatusCode)
	}
}
