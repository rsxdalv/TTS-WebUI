package router

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// routeRegistrar is a local copy of the shared contract between the router
// and the extension supervisor.
type routeRegistrar interface {
	AddRoute(prefix, target string) error
	RemoveRoute(prefix string)
}

var _ routeRegistrar = (*Router)(nil)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// newFront returns a router with the given routes and an httptest server in
// front of it.
func newFront(t *testing.T, routes map[string]string) (*Router, *httptest.Server) {
	t.Helper()
	rt := New(Options{Token: "tok", Logger: quiet()})
	for p, target := range routes {
		if err := rt.AddRoute(p, target); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	return rt, srv
}

// echoUpstream answers with a JSON dump of what it received.
func echoUpstream(t *testing.T, name string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":    name,
			"method":  r.Method,
			"uri":     r.RequestURI,
			"host":    r.Host,
			"header":  r.Header,
			"body":    string(body),
			"hasbody": len(body) > 0,
		})
	}))
	t.Cleanup(s.Close)
	return s
}

type echoed struct {
	Name   string
	Method string
	URI    string
	Host   string
	Header http.Header
	Body   string
}

func get(t *testing.T, c *http.Client, req *http.Request) (*http.Response, echoed) {
	t.Helper()
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var e echoed
	_ = json.Unmarshal(b, &e)
	return resp, e
}

func noRedirect() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func TestLongestPrefixAndStripping(t *testing.T) {
	root, a, ab := echoUpstream(t, "root"), echoUpstream(t, "a"), echoUpstream(t, "ab")
	_, front := newFront(t, map[string]string{"/": root.URL, "/a": a.URL, "/a/b/": ab.URL})

	cases := []struct{ path, name, uri string }{
		{"/", "root", "/"},
		{"/x/y", "root", "/x/y"},
		{"/ab", "root", "/ab"}, // "/ab" does not start with "/a/"
		{"/a", "a", "/"},
		{"/a/", "a", "/"},
		{"/a/c?q=1&r=%20x", "a", "/c?q=1&r=%20x"},
		{"/a/bc", "a", "/bc"}, // "/a/b" is not a prefix of "/a/bc"
		{"/a/b", "ab", "/"},
		{"/a/b/c/d", "ab", "/c/d"},
		{"/a/b/file=C:/x%2Fy.wav?z=1", "ab", "/file=C:/x%2Fy.wav?z=1"}, // raw path kept
		{"//double//slash", "root", "//double//slash"},
		{"/a/x?semi=1;two=2", "a", "/x?semi=1;two=2"}, // unparsable query kept verbatim
	}
	for _, c := range cases {
		req, _ := http.NewRequest("GET", front.URL+c.path, nil)
		resp, e := get(t, noRedirect(), req)
		if resp.StatusCode != 200 || e.Name != c.name || e.URI != c.uri {
			t.Errorf("%s: status %d upstream %q uri %q, want %q %q", c.path, resp.StatusCode, e.Name, e.URI, c.name, c.uri)
		}
	}
}

func TestRoutesAPIAndNormalisation(t *testing.T) {
	rt := New(Options{Logger: quiet()})
	for _, p := range []string{"pkg", "/pkg/", "/pkg"} {
		if err := rt.AddRoute(p, "http://127.0.0.1:1/"); err != nil {
			t.Fatal(err)
		}
	}
	if err := rt.AddRoute("/", "http://127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}
	got := rt.Routes()
	if len(got) != 2 || got["/pkg"] != "http://127.0.0.1:1" || got["/"] != "http://127.0.0.1:2" {
		t.Fatalf("routes = %v", got)
	}
	// Re-adding replaces the target.
	_ = rt.AddRoute("/pkg", "http://127.0.0.1:3")
	if rt.Routes()["/pkg"] != "http://127.0.0.1:3" {
		t.Fatal("AddRoute did not replace")
	}
	rt.RemoveRoute("/pkg/")
	rt.RemoveRoute("/missing")
	if _, ok := rt.Routes()["/pkg"]; ok {
		t.Fatal("RemoveRoute did not remove")
	}
	for _, bad := range []string{"/a b", "/a?x", "/a/../b", "/a//b", "/a%2Fb"} {
		if rt.AddRoute(bad, "http://127.0.0.1:1") == nil {
			t.Errorf("prefix %q accepted", bad)
		}
	}
	for _, bad := range []string{"", "127.0.0.1:1", "ftp://x:1", "http://x:1/path", "http://u@x:1", "http://"} {
		if rt.AddRoute("/ok", bad) == nil {
			t.Errorf("target %q accepted", bad)
		}
	}
}

func TestHeaderHandling(t *testing.T) {
	up := echoUpstream(t, "up")
	_, front := newFront(t, map[string]string{"/": up.URL})

	req, _ := http.NewRequest("POST", front.URL+"/p", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom", "keep")
	req.Header.Set("Cookie", "s=1")
	req.Header.Set("Connection", "X-Drop-Me")
	req.Header.Set("X-Drop-Me", "1")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("Proxy-Authorization", "Basic eA==")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	resp, e := get(t, noRedirect(), req)
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
	host := strings.TrimPrefix(front.URL, "http://")
	h := e.Header
	if e.Host != host {
		t.Errorf("Host = %q, want the client's %q", e.Host, host)
	}
	if h.Get("X-Forwarded-Host") != host || h.Get("X-Forwarded-Proto") != "http" {
		t.Errorf("forwarded host/proto = %q %q", h.Get("X-Forwarded-Host"), h.Get("X-Forwarded-Proto"))
	}
	if h.Get("X-Forwarded-For") != "203.0.113.9" {
		t.Errorf("X-Forwarded-For = %q, want the client's value untouched", h.Get("X-Forwarded-For"))
	}
	if h.Get("X-Custom") != "keep" || h.Get("Cookie") != "s=1" || e.Body != `{"a":1}` {
		t.Errorf("custom header/cookie/body lost: %v %q", h, e.Body)
	}
	for _, k := range []string{"X-Drop-Me", "Keep-Alive", "Proxy-Authorization"} {
		if h.Get(k) != "" {
			t.Errorf("hop-by-hop %s forwarded", k)
		}
	}

	// No X-Forwarded-For is invented.
	req, _ = http.NewRequest("GET", front.URL+"/", nil)
	_, e = get(t, noRedirect(), req)
	if _, ok := e.Header["X-Forwarded-For"]; ok {
		t.Error("X-Forwarded-For was added")
	}
}

func TestResponseHeadersAndRedirects(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Proxy-Authenticate", "Basic")
		w.Header().Set("Keep-Alive", "timeout=5")
		w.Header().Set("X-Custom", "v")
		w.Header().Add("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(302)
	}))
	defer up.Close()
	_, front := newFront(t, map[string]string{"/": up.URL})
	resp, err := noRedirect().Get(front.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 302 || resp.Header.Get("Location") != "/elsewhere" {
		t.Errorf("redirect not passed through: %d %v", resp.StatusCode, resp.Header)
	}
	if resp.Header.Get("Proxy-Authenticate") != "" || resp.Header.Get("Keep-Alive") != "" {
		t.Errorf("hop-by-hop response headers leaked: %v", resp.Header)
	}
	if resp.Header.Get("X-Custom") != "v" || len(resp.Header.Values("Set-Cookie")) != 2 {
		t.Errorf("headers lost: %v", resp.Header)
	}
}

func TestNotFoundAndBadGateway(t *testing.T) {
	rt, front := newFront(t, nil)

	resp, err := http.Get(front.URL + "/nope/x")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 404 || resp.Header.Get("Content-Type") != "application/json" ||
		string(b) != `{"detail":"Not Found (proxy server - no route matches /nope/x)","proxy":true}` {
		t.Errorf("404: %d %q %s", resp.StatusCode, resp.Header.Get("Content-Type"), b)
	}

	// Management endpoints exist only while nothing routes "/".
	resp, _ = http.Get(front.URL + "/routes")
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(b) != `{"routes":{}}` {
		t.Errorf("/routes: %d %s", resp.StatusCode, b)
	}
	resp, _ = http.Get(front.URL + "/healthz")
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(b) != `{"status":"ok","routes":{}}` {
		t.Errorf("/healthz: %d %s", resp.StatusCode, b)
	}

	// A refused upstream is a 502 with the Python body.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	dead := "http://" + ln.Addr().String()
	ln.Close()
	_ = rt.AddRoute("/", dead)
	resp, _ = http.Get(front.URL + "/healthz")
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 502 || string(b) != "Proxy error: upstream unavailable" {
		t.Errorf("502: %d %s", resp.StatusCode, b)
	}

	// An upstream that dies mid-request is a 500.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _, _ := w.(http.Hijacker).Hijack()
		c.Close()
	}))
	defer bad.Close()
	_ = rt.AddRoute("/", bad.URL)
	resp, _ = http.Get(front.URL + "/")
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 500 || string(b) != "Proxy error: internal error" {
		t.Errorf("500: %d %s", resp.StatusCode, b)
	}
}

func TestSSEArrivesIncrementally(t *testing.T) {
	for _, ct := range []string{"text/event-stream", "application/octet-stream"} {
		t.Run(ct, func(t *testing.T) {
			release := make(chan struct{})
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", ct)
				w.(http.Flusher).Flush()
				io.WriteString(w, "data: one\n\n")
				w.(http.Flusher).Flush()
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				io.WriteString(w, "data: two\n\n")
			}))
			defer up.Close()
			_, front := newFront(t, map[string]string{"/": up.URL})

			resp, err := http.Get(front.URL + "/queue/data")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			rd := bufio.NewReader(resp.Body)
			readEvent := func() string {
				done := make(chan string, 1)
				go func() {
					l1, _ := rd.ReadString('\n')
					_, _ = rd.ReadString('\n')
					done <- l1
				}()
				select {
				case l := <-done:
					return strings.TrimSpace(l)
				case <-time.After(5 * time.Second):
					t.Fatal("event not delivered: the proxy is buffering")
					return ""
				}
			}
			if got := readEvent(); got != "data: one" {
				t.Fatalf("first event %q", got)
			}
			close(release) // only now may the upstream send the second event
			if got := readEvent(); got != "data: two" {
				t.Fatalf("second event %q", got)
			}
		})
	}
}

// --- minimal WebSocket (RFC 6455) helpers, no dependencies -------------------

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func wsAccept(key string) string {
	h := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h[:])
}

func wsWrite(w io.Writer, op byte, payload []byte, mask bool) error {
	hdr := []byte{0x80 | op}
	mbit := byte(0)
	if mask {
		mbit = 0x80
	}
	switch n := len(payload); {
	case n < 126:
		hdr = append(hdr, mbit|byte(n))
	case n < 65536:
		hdr = append(hdr, mbit|126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, mbit|127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	body := payload
	if mask {
		key := []byte{1, 2, 3, 4}
		hdr = append(hdr, key...)
		body = make([]byte, len(payload))
		for i := range payload {
			body[i] = payload[i] ^ key[i%4]
		}
	}
	_, err := w.Write(append(hdr, body...))
	return err
}

func wsRead(r io.Reader) (byte, []byte, error) {
	var h [2]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var b [2]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, nil, err
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	var key [4]byte
	masked := h[1]&0x80 != 0
	if masked {
		if _, err := io.ReadFull(r, key[:]); err != nil {
			return 0, nil, err
		}
	}
	p := make([]byte, n)
	if _, err := io.ReadFull(r, p); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range p {
			p[i] ^= key[i%4]
		}
	}
	return h[0] & 0x0f, p, nil
}

func TestWebSocketUpgradeAndEcho(t *testing.T) {
	seen := make(chan *http.Request, 1)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "no upgrade", 400)
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		io.WriteString(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: "+
			wsAccept(r.Header.Get("Sec-WebSocket-Key"))+"\r\n\r\n")
		rw.Flush()
		for {
			op, p, err := wsRead(rw)
			if err != nil || op == 8 {
				_ = wsWrite(conn, 8, nil, false)
				return
			}
			_ = wsWrite(conn, op, p, false)
		}
	}))
	defer up.Close()
	_, front := newFront(t, map[string]string{"/": up.URL + "/", "/ext": up.URL})

	conn, err := net.Dial("tcp", strings.TrimPrefix(front.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	io.WriteString(conn, "GET /ext/queue/join?x=1 HTTP/1.1\r\nHost: browser.test:7770\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Key: "+key+"\r\nSec-WebSocket-Version: 13\r\nOrigin: http://browser.test:7770\r\nCookie: s=1\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: "GET"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 101 || resp.Header.Get("Sec-WebSocket-Accept") != wsAccept(key) {
		t.Fatalf("handshake: %d %v", resp.StatusCode, resp.Header)
	}

	r := <-seen
	if r.URL.RequestURI() != "/queue/join?x=1" || r.Host != "browser.test:7770" ||
		r.Header.Get("X-Forwarded-Host") != "browser.test:7770" || r.Header.Get("X-Forwarded-Proto") != "http" ||
		r.Header.Get("Origin") != "http://browser.test:7770" || r.Header.Get("Cookie") != "s=1" {
		t.Errorf("upstream saw %s host=%s hdr=%v", r.URL.RequestURI(), r.Host, r.Header)
	}

	big := make([]byte, 70000)
	for i := range big {
		big[i] = byte(i)
	}
	for _, msg := range [][]byte{[]byte("hello"), big} {
		if err := wsWrite(conn, 1, msg, true); err != nil {
			t.Fatal(err)
		}
		op, p, err := wsRead(br)
		if err != nil || op != 1 || string(p) != string(msg) {
			t.Fatalf("echo of %d bytes: op=%d len=%d err=%v", len(msg), op, len(p), err)
		}
	}
	_ = wsWrite(conn, 8, nil, true)
	if op, _, _ := wsRead(br); op != 8 {
		t.Errorf("close frame not relayed (op %d)", op)
	}
}

func TestServeShutdownEndsTunnels(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer up.Close()
	rt := New(Options{Logger: quiet()})
	_ = rt.AddRoute("/", up.URL)
	ln, err := Listen("", 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- rt.ServeListener(ctx, ln) }()

	go func() { _, _ = http.Get("http://" + ln.Addr().String() + "/hang") }()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}

func TestBindHost(t *testing.T) {
	if BindHost("") != "127.0.0.1" || BindHost("0.0.0.0") != "0.0.0.0" || BindHost("127.0.0.1") != "127.0.0.1" {
		t.Fatal("BindHost")
	}
	ln, err := Listen("", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if !ln.Addr().(*net.TCPAddr).IP.IsLoopback() {
		t.Fatalf("empty host bound %v, want loopback", ln.Addr())
	}
}
