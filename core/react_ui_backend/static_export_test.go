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

// exportFixture writes a small export and public folder.
func exportFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	big := strings.Repeat("<p>filler</p>", 200)
	files := map[string]string{
		"react-ui/out/index.html":                                "<html>home</html>",
		"react-ui/out/tools.html":                                "<html>tools</html>",
		"react-ui/out/tools/gpu_info.html":                       "<html>gpu</html>",
		"react-ui/out/text-to-speech/mms.html":                   "<html>mms" + big + "</html>",
		"react-ui/out/404.html":                                  "<html>not found</html>",
		"react-ui/out/favicon.ico":                               "ICO",
		"react-ui/out/_tts-export.json":                          `{"buildId":"B1"}`,
		"react-ui/out/_next/static/chunks/a.js":                  "console.log(1)",
		"react-ui/out/_next/data/B1/outputs/outputs.json":        `{"pageProps":{},"__N_SSG":true}`,
		"react-ui/out/extensions/_shell.html":                    "<html>shell</html>",
		"react-ui/out/extensions/huggingface_cache_manager.html": "<html>hf</html>",
		"react-ui/out/_next/data/B1/extensions/_shell.json":      `{"pageProps":{"name":"_shell"}}`,
		"react-ui/out/.hidden":                                   "hidden",
		"react-ui/public/robots.txt":                             "robots",
		"react-ui/public/file-input-cache/.secret":               "secret",
		"react-ui/public/.env":                                   "secret",
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
	h, err := NewHandler(Config{Root: root, Port: testPort, Static: true, Routes: routes(t, "")})
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
	upload := filepath.Join(root, "react-ui", "public", "file-input-cache", "up.wav")
	if err := os.WriteFile(upload, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path, want   string
		status       int
		cacheControl string
		contentType  string
	}{
		{"/", "<html>home</html>", 200, cachePage, "text/html; charset=utf-8"},
		{"/tools", "<html>tools</html>", 200, cachePage, "text/html; charset=utf-8"},
		{"/tools/gpu_info", "<html>gpu</html>", 200, cachePage, "text/html; charset=utf-8"},
		{"/_next/static/chunks/a.js", "console.log(1)", 200, cacheImmutable, "application/javascript; charset=UTF-8"},
		{"/_next/data/B1/outputs/outputs.json", `{"pageProps":{},"__N_SSG":true}`, 200, cachePage, "application/json; charset=utf-8"},
		{"/favicon.ico", "ICO", 200, "public, max-age=0", "image/x-icon"},
		{"/extensions/my_new_extension", "<html>shell</html>", 200, cachePage, ""},
		{"/extensions/huggingface_cache_manager", "<html>hf</html>", 200, cachePage, ""},
		{"/_next/data/B1/extensions/my_new_extension.json", `{"pageProps":{"name":"_shell"}}`, 200, cachePage, ""},
		{"/file-input-cache/up.wav", "RIFF", 200, "public, max-age=0", "audio/wav"},
		{"/robots.txt", "robots", 200, "public, max-age=0", ""},
		{"/nope", "<html>not found</html>", 404, cacheNotFound, "text/html; charset=utf-8"},
		{"/outputs/other", "<html>not found</html>", 404, cacheNotFound, ""},
		{"/extensions/a/b", "<html>not found</html>", 404, "", ""},
		{"/file-input-cache/missing.wav", "<html>not found</html>", 404, "", ""},
		// Rule 1: dotfiles and odd paths never reach a file.
		{"/.hidden", "<html>not found</html>", 404, "", ""},
		{"/.env", "<html>not found</html>", 404, "", ""},
		{"/file-input-cache/.secret", "<html>not found</html>", 404, "", ""},
		{"/%2ehidden", "<html>not found</html>", 404, "", ""},
		{"/x/..%2f.hidden", "<html>not found</html>", 404, "", ""},
		{"/tools%5cgpu_info", "<html>not found</html>", 404, "", ""},
		{"/a%00b", "<html>not found</html>", 404, "", ""},
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
	resp = request(t, "GET", front+"/api/unknown", nil, nil)
	if got := body(t, resp); resp.StatusCode != 404 || strings.Contains(got, "<html>") {
		t.Fatalf("/api/unknown: %d %q; an API path must not get a page", resp.StatusCode, got)
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
	if etag != nextETag("<html>tools</html>") {
		t.Fatalf("ETag %q", etag)
	}
	if resp := request(t, "GET", front+"/tools", nil, map[string]string{"If-None-Match": etag}); resp.StatusCode != http.StatusNotModified {
		t.Fatalf("If-None-Match: %d", resp.StatusCode)
	}

	resp = request(t, "GET", front+"/_next/static/chunks/a.js", nil, nil)
	weak := resp.Header.Get("ETag")
	resp = request(t, "GET", front+"/_next/static/chunks/a.js", nil, map[string]string{"If-None-Match": weak})
	if resp.StatusCode != http.StatusNotModified || resp.Header.Get("Cache-Control") != cacheImmutable {
		t.Fatalf("static 304: %d %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}

	req, _ := http.NewRequest("GET", front+"/text-to-speech/mms", nil)
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
	if !bytes.HasPrefix(plain, []byte("<html>mms")) {
		t.Fatalf("decompressed %q", plain[:20])
	}
}

func TestStaticModeKeepsClaimsAndGuards(t *testing.T) {
	root := exportFixture(t)
	h, err := NewHandler(Config{Root: root, Port: testPort, Static: true, Routes: routes(t, "test-files")})
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
