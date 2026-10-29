package server_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

// newTestServer binds a free port, creates a Server with that port, and
// returns both. The test server is closed via t.Cleanup.
func newTestServer(t *testing.T) (*server.Server, *httptest.Server) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port

	srv, err := server.New(port)
	if err != nil {
		l.Close()
		t.Fatalf("server.New: %v", err)
	}

	ts := &httptest.Server{
		Listener: l,
		Config:   &http.Server{Handler: srv.Handler()},
	}
	ts.Start()
	t.Cleanup(ts.Close)
	return srv, ts
}

func get(t *testing.T, ts *httptest.Server, path string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", ts.URL+path, nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func TestHealth_NoToken(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "/api/health", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/health status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"ok":true`) {
		t.Errorf("body = %q, want contains ok:true", body)
	}
}

func TestAPIRequiresToken(t *testing.T) {
	srv, ts := newTestServer(t)

	// Without token → 403
	resp := get(t, ts, "/api/anything", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("no token: status = %d, want 403", resp.StatusCode)
	}

	// With wrong token → 403
	resp = get(t, ts, "/api/anything", map[string]string{"X-TTS-Token": "wrong"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("wrong token: status = %d, want 403", resp.StatusCode)
	}

	// With correct token → not 403 (404 because route doesn't exist)
	resp = get(t, ts, "/api/anything", map[string]string{"X-TTS-Token": srv.Token()})
	resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		t.Error("correct token: got 403, want any other status")
	}
}

func TestHostRejection(t *testing.T) {
	_, ts := newTestServer(t)

	req, _ := http.NewRequest("GET", ts.URL+"/api/health", nil)
	req.Host = "evil.com:1234"
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad host: status = %d, want 400", resp.StatusCode)
	}
}

func TestOriginRejection(t *testing.T) {
	_, ts := newTestServer(t)

	resp := get(t, ts, "/api/health", map[string]string{
		"Origin": "http://evil.com",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("bad origin: status = %d, want 403", resp.StatusCode)
	}
}

func TestOriginSameHost_Allowed(t *testing.T) {
	_, ts := newTestServer(t)

	// Extract port from ts.URL
	host := strings.TrimPrefix(ts.URL, "http://")
	origin := "http://" + host

	resp := get(t, ts, "/api/health", map[string]string{
		"Origin": origin,
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("same-origin: status = %d, want 200", resp.StatusCode)
	}
}

func TestIndexHasToken(t *testing.T) {
	srv, ts := newTestServer(t)

	resp := get(t, ts, "/", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	expected := fmt.Sprintf(`content="%s"`, srv.Token())
	if !strings.Contains(string(body), expected) {
		t.Errorf("index.html missing token meta tag\nbody:\n%s", body)
	}
}

func TestSSE(t *testing.T) {
	events := make(chan string, 2)
	events <- "hello"
	events <- "world"
	close(events)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/", nil)
	server.SSE(w, r, events)

	body := w.Body.String()
	if !strings.Contains(body, "data: hello") {
		t.Errorf("SSE output missing 'data: hello': %q", body)
	}
	if !strings.Contains(body, "data: world") {
		t.Errorf("SSE output missing 'data: world': %q", body)
	}
}
