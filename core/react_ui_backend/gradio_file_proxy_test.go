package react_ui_backend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fileProxyServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	gradio := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		switch {
		case strings.HasSuffix(r.URL.Path, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, "<script>alert(1)</script>")
		case strings.HasSuffix(r.URL.Path, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = io.WriteString(w, `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`)
		case strings.HasSuffix(r.URL.Path, ".wav"):
			w.Header().Set("Content-Type", "audio/wav")
			w.Header().Set("Content-Disposition", `attachment; filename="a.wav"`)
			w.Header().Set("X-Other", "dropped")
			_, _ = io.WriteString(w, "RIFFdata")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(gradio.Close)

	mux := http.NewServeMux()
	mux.Handle("/api/gradio-file-proxy/{path...}", newGradioFileProxy(Deps{GradioBackend: gradio.URL + "/"}))
	front := httptest.NewServer(mux)
	t.Cleanup(front.Close)
	return front, &seen
}

func TestGradioFileProxy(t *testing.T) {
	front, seen := fileProxyServer(t)

	resp, err := http.Get(front.URL + "/api/gradio-file-proxy/gradio_api/file=C:/Temp/gradio/a.wav")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "RIFFdata" {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
	if resp.Header.Get("Content-Type") != "audio/wav" || resp.Header.Get("Content-Length") != "8" ||
		resp.Header.Get("Content-Disposition") != `attachment; filename="a.wav"` {
		t.Fatalf("headers: %v", resp.Header)
	}
	if resp.Header.Get("X-Other") != "" {
		t.Fatal("only Content-Type, Content-Length and Content-Disposition are passed on")
	}
	if len(*seen) != 1 || (*seen)[0] != "GET /gradio_api/file=C:/Temp/gradio/a.wav" {
		t.Fatalf("upstream saw %v", *seen)
	}
}

func TestGradioFileProxyErrors(t *testing.T) {
	front, _ := fileProxyServer(t)

	resp, err := http.Get(front.URL + "/api/gradio-file-proxy/missing.bin")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 404 || string(body) != `{"error":"Failed to fetch file from Gradio backend: Not Found"}` ||
		resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("404: %d %q %v", resp.StatusCode, body, resp.Header)
	}

	// An unreachable Gradio is a 500, as in the original's catch block.
	mux := http.NewServeMux()
	mux.Handle("/api/gradio-file-proxy/{path...}", newGradioFileProxy(Deps{GradioBackend: "http://127.0.0.1:1/"}))
	down := httptest.NewServer(mux)
	defer down.Close()
	resp, err = http.Get(down.URL + "/api/gradio-file-proxy/a.wav")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 500 || string(body) != `{"error":"Failed to proxy file"}` {
		t.Fatalf("500: %d %q", resp.StatusCode, body)
	}
}

// The upstream is always fetched with GET, whatever the method of the request.
func TestGradioFileProxyUsesGet(t *testing.T) {
	front, seen := fileProxyServer(t)
	resp, err := http.Post(front.URL+"/api/gradio-file-proxy/a.wav", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(*seen) != 1 || !strings.HasPrefix((*seen)[0], "GET ") {
		t.Fatalf("upstream saw %v", *seen)
	}
}

func TestProxyContentType(t *testing.T) {
	const octet = "application/octet-stream"
	for _, c := range []struct{ in, want string }{
		{"audio/wav", "audio/wav"},
		{"audio/mpeg", "audio/mpeg"},
		{"audio/x-wav", "audio/x-wav"},
		{"video/mp4", "video/mp4"},
		{"image/png", "image/png"},
		{"image/webp", "image/webp"},
		{"application/json", "application/json"},
		{"application/json; charset=utf-8", "application/json; charset=utf-8"},
		{"text/plain; charset=utf-8", "text/plain; charset=utf-8"},
		{"application/ogg", "application/ogg"},
		{"AUDIO/WAV", "AUDIO/WAV"}, // media types are case-insensitive
		{"", octet},
		{"not a type", octet},
		{"text/html", octet},
		{"text/html; charset=utf-8", octet},
		{"application/xhtml+xml", octet},
		{"image/svg+xml", octet},
		{"IMAGE/SVG+XML", octet},
		{"image/svg+xml; charset=utf-8", octet},
		{"application/javascript", octet},
		{"text/javascript", octet},
		{"text/xml", octet},
		{"application/pdf", octet},
		{"multipart/x-mixed-replace", octet},
	} {
		if got := proxyContentType(c.in); got != c.want {
			t.Errorf("proxyContentType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGradioFileProxyDoesNotServeUpstreamDocumentsAsScript(t *testing.T) {
	front, _ := fileProxyServer(t)
	for _, name := range []string{"x.html", "x.svg"} {
		resp, err := http.Get(front.URL + "/api/gradio-file-proxy/gradio_api/file=outputs/" + name)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("%s served as %q", name, ct)
		}
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Content-Security-Policy") != "sandbox" {
			t.Errorf("%s: safety headers missing: %v", name, resp.Header)
		}
	}
}

func TestGradioFileProxyKeepsPreviewsAndAddsSafetyHeaders(t *testing.T) {
	front, _ := fileProxyServer(t)
	resp, err := http.Get(front.URL + "/api/gradio-file-proxy/gradio_api/file=outputs/a.wav")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Header.Get("Content-Type") != "audio/wav" {
		t.Errorf("a preview lost its type: %v", resp.Header)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("nosniff missing: %v", resp.Header)
	}
}
