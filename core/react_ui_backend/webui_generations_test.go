package react_ui_backend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generationsFixture(t *testing.T) (string, *httptest.Server) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("outputs/a.wav", "RIFFwavdata0123456789")
	write("outputs/sub/b.txt", "text")
	write("outputs/.hidden", "secret")
	write("outputs/.dir/x.txt", "secret")
	write("config.json", "TOP SECRET")
	write("other/o.txt", "not served")
	write("voices-tortoise/v.json", "{}")

	mux := http.NewServeMux()
	mux.Handle("GET /api/webui-generations/{path...}", newWebuiGenerations(Deps{Root: root}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return root, srv
}

func get(t *testing.T, srv *httptest.Server, path string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", srv.URL+path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Transport: &http.Transport{DisableCompression: true}}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestWebuiGenerationsServesAllowedFiles(t *testing.T) {
	_, srv := generationsFixture(t)
	resp, body := get(t, srv, "/api/webui-generations/outputs/a.wav", nil)
	if resp.StatusCode != 200 || body != "RIFFwavdata0123456789" || resp.Header.Get("Content-Type") != "audio/wav" ||
		resp.Header.Get("Accept-Ranges") != "bytes" || resp.Header.Get("Cache-Control") != "public, max-age=0" ||
		!strings.HasPrefix(resp.Header.Get("ETag"), `W/"`) || resp.Header.Get("Last-Modified") == "" {
		t.Fatalf("%d %q %v", resp.StatusCode, body, resp.Header)
	}
	if resp, body = get(t, srv, "/api/webui-generations/OUTPUTS/sub/b.txt", nil); resp.StatusCode != 200 || body != "text" ||
		resp.Header.Get("Content-Type") != "text/plain; charset=UTF-8" {
		t.Fatalf("case-insensitive directory: %d %q %v", resp.StatusCode, body, resp.Header)
	}
	if resp, _ = get(t, srv, "/api/webui-generations/voices-tortoise/v.json", nil); resp.StatusCode != 200 ||
		resp.Header.Get("Content-Type") != "application/json; charset=UTF-8" {
		t.Fatalf("json: %d %v", resp.StatusCode, resp.Header)
	}
}

// Everything that is not a served file is an empty 404, and no secret leaks.
func TestWebuiGenerationsDenies(t *testing.T) {
	_, srv := generationsFixture(t)
	for _, p := range []string{
		"/api/webui-generations/outputs/.hidden",
		"/api/webui-generations/outputs/.dir/x.txt",
		"/api/webui-generations/outputs/%2ehidden",
		"/api/webui-generations/outputs/sub",
		"/api/webui-generations/outputs",
		"/api/webui-generations/config.json",
		"/api/webui-generations/other/o.txt",
		"/api/webui-generations/outputs/missing.wav",
		// Paths the proxy never lets through, and which the handler refuses on its own.
		"/api/webui-generations/outputs/../config.json",
		"/api/webui-generations/outputs/%2e%2e/config.json",
		"/api/webui-generations/outputs/.%2e/config.json",
		"/api/webui-generations/outputs/..%5cconfig.json",
		"/api/webui-generations/outputs%2f..%2fconfig.json",
		"/api/webui-generations/outputs/%252e%252e/config.json",
	} {
		resp, body := get(t, srv, p, nil)
		if resp.StatusCode == 200 || strings.Contains(body, "SECRET") || strings.Contains(body, "secret") || strings.Contains(body, "not served") {
			t.Errorf("%s: %d %q", p, resp.StatusCode, body)
		}
	}
}

func TestGenerationFileRejectsEscapes(t *testing.T) {
	for _, p := range []string{"outputs/../x", "outputs/./x", "outputs//x", "outputs/..", "outputs/.x", "outputs/a/.b/c", "outputs/a\\b", "outputs/a\x00b", "outputs", "", "nope/x", "../outputs/x"} {
		if f, ok := generationFile("/root", p); ok {
			t.Errorf("%q was accepted as %q", p, f)
		}
	}
	if f, ok := generationFile("/root", "Outputs/a/b.wav"); !ok || filepath.ToSlash(f) != "/root/outputs/a/b.wav" {
		t.Errorf("got %q %v", f, ok)
	}
}

func TestWebuiGenerationsRangesAndValidators(t *testing.T) {
	_, srv := generationsFixture(t)
	const p = "/api/webui-generations/outputs/a.wav"
	resp, body := get(t, srv, p, map[string]string{"Range": "bytes=0-3"})
	if resp.StatusCode != 206 || body != "RIFF" || resp.Header.Get("Content-Range") != "bytes 0-3/21" || resp.Header.Get("Content-Length") != "4" {
		t.Fatalf("range: %d %q %v", resp.StatusCode, body, resp.Header)
	}
	if resp, body = get(t, srv, p, map[string]string{"Range": "bytes=-4"}); resp.StatusCode != 206 || body != "6789" {
		t.Fatalf("suffix range: %d %q", resp.StatusCode, body)
	}
	if resp, body = get(t, srv, p, map[string]string{"Range": "bytes=0-1,5-6"}); resp.StatusCode != 200 || len(body) != 21 {
		t.Fatalf("multi range is a plain 200: %d %q", resp.StatusCode, body)
	}
	if resp, body = get(t, srv, p, map[string]string{"Range": "bytes=50-60"}); resp.StatusCode != 416 ||
		resp.Header.Get("Content-Range") != "bytes */21" || !strings.Contains(body, "<pre>Range Not Satisfiable</pre>") {
		t.Fatalf("unsatisfiable: %d %q %v", resp.StatusCode, body, resp.Header)
	}
	first, _ := get(t, srv, p, nil)
	etag := first.Header.Get("ETag")
	if resp, body = get(t, srv, p, map[string]string{"If-None-Match": etag}); resp.StatusCode != 304 || body != "" || resp.Header.Get("Content-Type") != "" {
		t.Fatalf("304: %d %q %v", resp.StatusCode, body, resp.Header)
	}
	if resp, _ = get(t, srv, p, map[string]string{"If-Match": `"nope"`}); resp.StatusCode != 412 {
		t.Fatalf("412: %d", resp.StatusCode)
	}
	if resp, body = get(t, srv, p, map[string]string{"Range": "bytes=0-3", "If-Range": `"other"`}); resp.StatusCode != 200 || len(body) != 21 {
		t.Fatalf("stale If-Range: %d %q", resp.StatusCode, body)
	}
}

func TestParseByteRanges(t *testing.T) {
	cases := []struct {
		h     string
		want  [][2]int64
		state rangeState
	}{
		{"bytes=0-9", [][2]int64{{0, 9}}, rangeOK},
		{"bytes=5-", [][2]int64{{5, 19}}, rangeOK},
		{"bytes=-5", [][2]int64{{15, 19}}, rangeOK},
		{"bytes=0-99", [][2]int64{{0, 19}}, rangeOK},
		{"bytes=0-5,3-8", [][2]int64{{0, 8}}, rangeOK},
		{"bytes=0-1,5-6", [][2]int64{{0, 1}, {5, 6}}, rangeOK},
		{"bytes=30-40", nil, rangeUnsatisfiable},
		{"bytes=5-2", nil, rangeUnsatisfiable},
		{"bytes=abc", nil, rangeUnsatisfiable},
		{"nothing", nil, rangeSyntax},
	}
	for _, c := range cases {
		got, state := parseByteRanges(20, c.h)
		if state != c.state || len(got) != len(c.want) {
			t.Errorf("%q: %v %v", c.h, got, state)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: %v", c.h, got)
			}
		}
	}
}
