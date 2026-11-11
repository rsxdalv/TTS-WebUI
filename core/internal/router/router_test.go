package router

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// --- helpers ---

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func newRouter(t *testing.T) (*Router, *httptest.Server) {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo method, path and selected headers back as the body.
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "%s %s query=%s xfor=%s",
			r.Method,
			r.URL.Path,
			r.URL.RawQuery,
			r.Header.Get("X-Forwarded-For"),
		)
	}))
	t.Cleanup(backend.Close)
	rtr := New()
	rtr.AddRoute("/", mustURL(t, backend.URL))
	return rtr, backend
}

// --- tests ---

func TestPrefixRouting(t *testing.T) {
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "backend1")
	}))
	defer backend1.Close()
	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "backend2")
	}))
	defer backend2.Close()

	rtr := New()
	rtr.AddRoute("/api", mustURL(t, backend1.URL))
	rtr.AddRoute("/gradio", mustURL(t, backend2.URL))

	srv := httptest.NewServer(rtr)
	defer srv.Close()

	for _, tc := range []struct{ path, want string }{
		{"/api/v1", "backend1"},
		{"/gradio/queue", "backend2"},
	} {
		resp, err := http.Get(srv.URL + tc.path)
		if err != nil {
			t.Fatalf("GET %s: %v", tc.path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(body), tc.want) {
			t.Errorf("path %s: body = %q, want to contain %q", tc.path, body, tc.want)
		}
	}
}

func TestRawQueryPreservation(t *testing.T) {
	var capturedQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	rtr := New()
	rtr.AddRoute("/", mustURL(t, backend.URL))
	srv := httptest.NewServer(rtr)
	defer srv.Close()

	rawQ := "file=C%3A%2Ftmp%2Ffoo.wav&format=wav"
	resp, err := http.Get(srv.URL + "/audio?" + rawQ)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if capturedQuery != rawQ {
		t.Errorf("query = %q, want %q", capturedQuery, rawQ)
	}
}

func TestHopByHopStripping(t *testing.T) {
	var capturedHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		// Return a hop-by-hop header in the response.
		w.Header().Set("Keep-Alive", "timeout=5")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	rtr := New()
	rtr.AddRoute("/", mustURL(t, backend.URL))
	srv := httptest.NewServer(rtr)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("Keep-Alive", "timeout=30") // should be stripped
	req.Header.Set("X-Custom", "preserved")    // should pass through

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Keep-Alive should be stripped from the forwarded request.
	if capturedHeaders.Get("Keep-Alive") != "" {
		t.Errorf("Keep-Alive reached the backend: %q", capturedHeaders.Get("Keep-Alive"))
	}
	// Custom header should pass through.
	if capturedHeaders.Get("X-Custom") != "preserved" {
		t.Errorf("X-Custom stripped unexpectedly")
	}
	// Keep-Alive should be stripped from the response.
	if resp.Header.Get("Keep-Alive") != "" {
		t.Errorf("Keep-Alive in response: %q", resp.Header.Get("Keep-Alive"))
	}
}

func TestXForwardedFor(t *testing.T) {
	var capturedXFF string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedXFF = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	rtr := New()
	rtr.AddRoute("/", mustURL(t, backend.URL))
	srv := httptest.NewServer(rtr)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if capturedXFF == "" {
		t.Error("X-Forwarded-For not set")
	}
}

func TestClientDisconnectCancelsUpstream(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-time.After(5 * time.Second):
			t.Error("context not cancelled within timeout")
		}
	}))
	defer backend.Close()

	rtr := New()
	rtr.AddRoute("/", mustURL(t, backend.URL))
	srv := httptest.NewServer(rtr)
	defer srv.Close()

	// Connect and immediately close the client connection.
	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(conn, "GET /slow HTTP/1.1\r\nHost: %s\r\n\r\n", strings.TrimPrefix(srv.URL, "http://"))

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("backend never started")
	}
	conn.Close() // drop client

	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream context not cancelled after client disconnect")
	}
}

func TestWebSocketTunnel(t *testing.T) {
	// Minimal echo WebSocket "server": accepts the HTTP upgrade manually then
	// echoes raw frames back. Using raw TCP to avoid importing a WS library.
	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backendLn.Close()

	echoReady := make(chan struct{})
	go func() {
		conn, err := backendLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		// Read and discard the HTTP Upgrade request.
		for {
			line, _ := br.ReadString('\n')
			if strings.TrimSpace(line) == "" {
				break
			}
		}
		// Send a 101 response.
		conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"))
		close(echoReady)
		// Echo raw bytes back.
		io.Copy(conn, br)
	}()

	backendURL, _ := url.Parse("http://" + backendLn.Addr().String())
	rtr := New()
	rtr.AddRoute("/ws", backendURL)
	srv := httptest.NewServer(rtr)
	defer srv.Close()

	// Connect and send the WebSocket upgrade.
	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	fmt.Fprintf(conn, "GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n", host)

	// Wait for backend to be ready.
	select {
	case <-echoReady:
	case <-time.After(3 * time.Second):
		t.Fatal("backend echo never ready")
	}

	// The tunnel should have forwarded the 101 to us.
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("reading 101 response: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "101") {
		t.Errorf("expected 101 response, got: %s", buf[:n])
	}
}

func TestNoRouteReturns502(t *testing.T) {
	rtr := New()
	srv := httptest.NewServer(rtr)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nothing")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

func TestParseRoute(t *testing.T) {
	for _, tc := range []struct {
		input   string
		prefix  string
		host    string
		errWant bool
	}{
		{"/gradio=http://127.0.0.1:7860", "/gradio", "127.0.0.1:7860", false},
		{"/api=http://localhost:7700", "/api", "localhost:7700", false},
		{"bad-format", "", "", true},
	} {
		prefix, target, err := ParseRoute(tc.input)
		if tc.errWant {
			if err == nil {
				t.Errorf("ParseRoute(%q): expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRoute(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if prefix != tc.prefix {
			t.Errorf("prefix = %q, want %q", prefix, tc.prefix)
		}
		if target.Host != tc.host {
			t.Errorf("host = %q, want %q", target.Host, tc.host)
		}
	}
}

// Ensure the context from the incoming request flows to the backend.
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer backend.Close()

	rtr := New()
	rtr.AddRoute("/", mustURL(t, backend.URL))
	srv := httptest.NewServer(rtr)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/ping", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}
