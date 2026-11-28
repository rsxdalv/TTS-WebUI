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

// In development the app's pages and modules come from Vite; uploads stay live
// files and the /api routes stay the server's (an unknown /api path is a 404,
// never forwarded to Vite).
func TestDevProxyForwardsPagesAndKeepsUploadsAndAPI(t *testing.T) {
	var gotHost, gotOrigin string
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotOrigin = r.Host, r.Header.Get("Origin")
		io.WriteString(w, "vite "+r.URL.Path)
	}))
	defer vite.Close()

	root := exportFixture(t)
	up := filepath.Join(root, "frontend", "public", "file-input-cache", "up.wav")
	if err := os.WriteFile(up, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(Config{Root: root, Port: testPort, Routes: routes(t, ""), DevURL: vite.URL})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	defer front.Close()

	cases := []struct{ path, want string }{
		{"/", "vite /"},
		{"/tools/gpu_info", "vite /tools/gpu_info"},
		{"/src/main.tsx", "vite /src/main.tsx"},
		{"/@vite/client", "vite /@vite/client"},
		{"/node_modules/.vite/deps/react.js", "vite /node_modules/.vite/deps/react.js"}, // a dot path, as Vite's pre-bundled modules
		{"/file-input-cache/up.wav", "RIFF"},                                            // a live upload, not Vite's
		{"/api/no-such-route", notFoundBody},                                            // never forwarded
	}
	for _, c := range cases {
		resp := request(t, "GET", front.URL+c.path, nil, nil)
		if got := body(t, resp); got != c.want {
			t.Errorf("%s: %q, want %q", c.path, got, c.want)
		}
	}
	if want := strings.TrimPrefix(vite.URL, "http://"); gotHost != want {
		t.Errorf("Vite saw Host %q, want its own %q", gotHost, want)
	}
	_ = gotOrigin
}

func TestDevProxyRejectsABadURL(t *testing.T) {
	if _, err := NewHandler(Config{Root: exportFixture(t), Port: testPort, Routes: routes(t, ""), DevURL: "not a url"}); err == nil {
		t.Fatal("a bad dev URL was accepted")
	}
}

func TestGradioPrefix(t *testing.T) {
	for in, want := range map[string]string{
		"http://127.0.0.1:7770/gradio-legacy/": "/gradio-legacy",
		"http://127.0.0.1:7770/gradio-legacy":  "/gradio-legacy",
		"http://127.0.0.1:7770/":               "",
		"http://127.0.0.1:7770":                "",
		"":                                     "",
	} {
		if got := gradioPrefix(in); got != want {
			t.Errorf("gradioPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}
