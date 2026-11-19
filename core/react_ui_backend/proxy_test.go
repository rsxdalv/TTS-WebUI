package react_ui_backend

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
)

const testPort = 31050

func init() {
	answer := func(name string) func(Deps) http.Handler {
		return func(Deps) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Answered-By", name)
				_, _ = io.WriteString(w, r.PathValue("path"))
			})
		}
	}
	RegisterClaim(Claim{Switch: "test-files", Patterns: []string{"GET /api/test-files/{path...}"}, Handler: answer("test-files")})
	RegisterClaim(Claim{Switch: "test-post", Patterns: []string{"POST /api/test-post"}, Handler: answer("test-post")})
	RegisterClaim(Claim{Switch: "data-test", Patterns: []string{"GET /api/go-data/test"}, Handler: answer("data-test")})
}

func routes(t *testing.T, names string) switches.Set {
	t.Helper()
	set, err := switches.Parse(func(k string) (string, bool) {
		switch k {
		case switches.EnvProxy:
			return "1", true
		case switches.EnvRoutes:
			if names == "" {
				return "none", true // no claim; unset now means every claim
			}
			return names, true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// frontOf starts the proxy in front of upstream and returns its URL. Requests
// carry the public Host the proxy expects.
func frontOf(t *testing.T, upstream http.Handler, names string) string {
	t.Helper()
	next := httptest.NewServer(upstream)
	t.Cleanup(next.Close)
	h, err := NewHandler(Config{
		Root:     t.TempDir(),
		Port:     testPort,
		NextAddr: strings.TrimPrefix(next.URL, "http://"),
		Routes:   routes(t, names),
	})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	return front.URL
}

func request(t *testing.T, method, url string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "127.0.0.1:" + strconv.Itoa(testPort)
	for k, v := range headers {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestForwardsTheRequestUnchanged(t *testing.T) {
	var got *http.Request
	var gotBody []byte
	front := frontOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("X-From-Next", "yes")
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "from next")
	}), "")

	resp := request(t, "POST", front+"/_next/data/abc/page.json?a=1;b=%zz&c=d%20e", strings.NewReader("payload"), map[string]string{
		"Origin":           "http://localhost:" + strconv.Itoa(testPort),
		"Host":             "localhost:" + strconv.Itoa(testPort),
		"X-Forwarded-Host": "spoofed.example",
		"X-Forwarded-Port": "1",
		"X-Forwarded-For":  "203.0.113.9",
	})
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusTeapot || string(body) != "from next" || resp.Header.Get("X-From-Next") != "yes" {
		t.Fatalf("response changed: %d %q %v", resp.StatusCode, body, resp.Header)
	}
	if got.Method != "POST" || got.URL.Path != "/_next/data/abc/page.json" || string(gotBody) != "payload" {
		t.Fatalf("request changed: %s %s %q", got.Method, got.URL.Path, gotBody)
	}
	if got.URL.RawQuery != "a=1;b=%zz&c=d%20e" {
		t.Fatalf("query changed: %q", got.URL.RawQuery)
	}
	wantHost := "localhost:" + strconv.Itoa(testPort)
	if got.Host != wantHost {
		t.Fatalf("Host = %q, want %q", got.Host, wantHost)
	}
	if got.Header.Get("Origin") != "http://"+wantHost {
		t.Fatalf("Origin = %q", got.Header.Get("Origin"))
	}
	if h := got.Header.Get("X-Forwarded-Host"); h != wantHost {
		t.Fatalf("X-Forwarded-Host = %q", h)
	}
	if p := got.Header.Get("X-Forwarded-Port"); p != strconv.Itoa(testPort) {
		t.Fatalf("X-Forwarded-Port = %q", p)
	}
	if p := got.Header.Get("X-Forwarded-Proto"); p != "http" {
		t.Fatalf("X-Forwarded-Proto = %q", p)
	}
	if f := got.Header.Get("X-Forwarded-For"); f != "127.0.0.1" {
		t.Fatalf("X-Forwarded-For = %q, the client's value must be replaced", f)
	}
}

func TestEncodedPathReachesNextAsSent(t *testing.T) {
	var raw string
	front := frontOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.EscapedPath()
	}), "")
	request(t, "GET", front+"/outputs/a%2Fb%20c", nil, nil)
	if raw != "/outputs/a%2Fb%20c" {
		t.Fatalf("path reached Next as %q", raw)
	}
}

func TestStreamsBeforeTheUpstreamFinishes(t *testing.T) {
	release := make(chan struct{})
	front := frontOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "data: second\n\n")
	}), "")

	resp := request(t, "GET", front+"/stream", nil, nil)
	first := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := resp.Body.Read(buf)
		first <- string(buf[:n])
	}()
	select {
	case got := <-first:
		if got != "data: first\n\n" {
			t.Fatalf("first chunk = %q", got)
		}
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the first chunk was held back until the upstream finished")
	}
	close(release)
	rest, _ := io.ReadAll(resp.Body)
	if string(rest) != "data: second\n\n" {
		t.Fatalf("rest = %q", rest)
	}
}

func TestLargeUploadArrivesIntact(t *testing.T) {
	const size = 300 << 20
	var gotSum [32]byte
	var gotLen int64
	front := frontOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := sha256.New()
		gotLen, _ = io.Copy(h, r.Body)
		h.Sum(gotSum[:0])
	}), "")

	block := bytes.Repeat([]byte("0123456789abcdef"), 64*1024)
	want := sha256.New()
	body := io.TeeReader(io.LimitReader(repeatReader{block}, size), want)
	resp := request(t, "POST", front+"/upload-target", body, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if gotLen != size {
		t.Fatalf("received %d bytes, want %d", gotLen, size)
	}
	if !bytes.Equal(gotSum[:], want.Sum(nil)) {
		t.Fatal("the upload was changed on the way")
	}
}

type repeatReader struct{ block []byte }

func (r repeatReader) Read(p []byte) (int, error) { return copy(p, r.block), nil }

func TestHostCheck(t *testing.T) {
	front := frontOf(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "")
	port := strconv.Itoa(testPort)
	cases := map[string]int{
		"127.0.0.1:" + port:        200,
		"localhost:" + port:        200,
		"LOCALHOST:" + port:        200,
		"[::1]:" + port:            200,
		"evil.example:" + port:     400,
		"127.0.0.1":                400,
		"localhost:1":              400,
		"127.0.0.1.nip.io:" + port: 400,
	}
	for host, want := range cases {
		if got := request(t, "GET", front+"/", nil, map[string]string{"Host": host}).StatusCode; got != want {
			t.Errorf("Host %q: status %d, want %d", host, got, want)
		}
	}
}

func TestOriginCheck(t *testing.T) {
	front := frontOf(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "")
	good := "http://localhost:" + strconv.Itoa(testPort)
	cases := []struct {
		method, path, origin string
		want                 int
	}{
		{"GET", "/", "http://evil.example", 200}, // a page load is not checked
		{"GET", "/_next/static/a.js", "http://evil.example", 200},
		{"GET", "/api/gradio/x", "http://evil.example", 403},
		{"GET", "/%61pi/gradio/x", "http://evil.example", 403},
		{"GET", "/x/../api/gradio/x", "http://evil.example", 403},
		{"GET", "/x/%2e%2e/api/gradio/x", "http://evil.example", 403},
		{"GET", "//api/gradio/x", "http://evil.example", 403},
		{"GET", "/apix/y", "http://evil.example", 200},
		{"GET", "/api/gradio/x", "null", 403},
		{"GET", "/api/gradio/x", good, 200},
		{"GET", "/api/gradio/x", "", 200},
		{"POST", "/outputs/x", "http://evil.example", 403},
		{"POST", "/api/upload", "http://localhost:7770", 403},
		{"POST", "/api/upload", good, 200},
		{"POST", "/api/upload", "http://127.0.0.1:" + strconv.Itoa(testPort), 200},
		{"POST", "/api/upload", "http://[::1]:" + strconv.Itoa(testPort), 200},
		{"POST", "/api/upload", "", 200},
	}
	for _, c := range cases {
		headers := map[string]string{}
		if c.origin != "" {
			headers["Origin"] = c.origin
		}
		if got := request(t, c.method, front+c.path, nil, headers).StatusCode; got != c.want {
			t.Errorf("%s %s Origin %q: status %d, want %d", c.method, c.path, c.origin, got, c.want)
		}
	}
}

func TestDispatch(t *testing.T) {
	front := frontOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Answered-By", "next")
	}), "test-files,test-post")

	cases := []struct {
		method, path, want string
	}{
		{"GET", "/api/test-files/a/b.ogg", "test-files"},
		{"HEAD", "/api/test-files/a.ogg", "test-files"},
		{"POST", "/api/test-post", "test-post"},

		{"POST", "/api/test-files/a.ogg", "next"}, // other method
		{"GET", "/api/test-post", "next"},
		{"GET", "/api/test-files", "next"},  // {path...} prefix without its slash
		{"GET", "/api/test-files/", "next"}, // trailing slash
		{"GET", "/api/test-files/a/", "next"},
		{"POST", "/api/test-post/", "next"},
		{"GET", "/api/test-files//a", "next"},
		{"GET", "/api/test-files/./a", "next"},
		{"GET", "/api/test-files/../a", "next"},
		{"GET", "/api/test-files/%2e%2e/a", "next"},
		{"GET", "/api/test-files/%2E%2e/a", "next"},
		{"GET", "/api/test-files/.%2e/a", "next"},
		{"GET", "/api/test-files/a%2fb", "next"},
		{"GET", "/api/test-files/a%2Fb", "next"},
		{"GET", "/api/test-files/a%5cb", "next"},
		{"GET", "/api/test-files/a%5Cb", "next"},
		{"GET", "/api/test-files/a%00b", "next"},
		{"GET", "/", "next"},
		{"GET", "/_next/static/a.js", "next"},
		{"GET", "/_next/data/abc/page.json", "next"},
		{"GET", "/api/gradio/anything", "next"},
		{"GET", "/api/go-data/test", "next"}, // registered, but its switch is off
	}
	for _, c := range cases {
		resp := request(t, c.method, front+c.path, nil, nil)
		if got := resp.Header.Get("X-Answered-By"); got != c.want {
			t.Errorf("%s %s answered by %q (status %d), want %q", c.method, c.path, got, resp.StatusCode, c.want)
		}
	}
}

func TestNoClaimRunsWithRoutesOff(t *testing.T) {
	front := frontOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Answered-By", "next")
	}), "")
	for _, path := range []string{"/api/test-files/a.ogg", "/api/go-data/test"} {
		if got := request(t, "GET", front+path, nil, nil).Header.Get("X-Answered-By"); got != "next" {
			t.Errorf("%s answered by %q", path, got)
		}
	}
}

func TestUnknownRouteNameIsRefused(t *testing.T) {
	_, err := NewHandler(Config{Root: t.TempDir(), Port: testPort, NextAddr: "127.0.0.1:1", Routes: routes(t, "no-such-route")})
	if err == nil || !strings.Contains(err.Error(), "no-such-route") {
		t.Fatalf("err = %v", err)
	}
}

func TestClaimsCookie(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "from-next", Value: "1"})
	})
	document := map[string]string{"Sec-Fetch-Dest": "document"}

	on := frontOf(t, upstream, "data-test,test-post")
	cookies := request(t, "GET", on+"/outputs/outputs", nil, document).Header.Values("Set-Cookie")
	if len(cookies) != 2 || !strings.HasPrefix(cookies[0], "tts-go-claims=data-test;") || !strings.Contains(cookies[0], "SameSite=Strict") {
		t.Fatalf("cookies = %q", cookies)
	}
	if c := request(t, "GET", on+"/_next/static/a.js", nil, map[string]string{"Accept": "*/*"}).Header.Values("Set-Cookie"); len(cookies) == 0 || strings.Contains(strings.Join(c, ";"), claimsCookie) {
		t.Fatalf("cookie set on a non-document request: %q", c)
	}

	off := frontOf(t, upstream, "test-post")
	if c := strings.Join(request(t, "GET", off+"/", nil, document).Header.Values("Set-Cookie"), ";"); strings.Contains(c, claimsCookie) {
		t.Fatalf("cookie set with no data claim on: %q", c)
	}
	stale := map[string]string{"Accept": "text/html", "Cookie": claimsCookie + "=data-test"}
	if c := strings.Join(request(t, "GET", off+"/", nil, stale).Header.Values("Set-Cookie"), ";"); !strings.Contains(c, claimsCookie+"=;") || !strings.Contains(c, "Max-Age=0") {
		t.Fatalf("stale cookie not expired: %q", c)
	}
}

func TestNextDownIsABadGateway(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	h, err := NewHandler(Config{Root: t.TempDir(), Port: testPort, NextAddr: addr, Routes: routes(t, "")})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	defer front.Close()
	if got := request(t, "GET", front.URL+"/", nil, nil).StatusCode; got != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", got)
	}
}

func TestServerBindsLoopbackOnlyAndCloses(t *testing.T) {
	next := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer next.Close()

	port := freeTestPort(t)
	s, err := Start(Config{Root: t.TempDir(), Port: port, NextAddr: strings.TrimPrefix(next.URL, "http://"), Routes: routes(t, "")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, addr := range s.Addrs() {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatal(err)
		}
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			t.Fatalf("listening on %s, which is not loopback", addr)
		}
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}

	s.Close()
	s.Close() // a second Close must be harmless
	if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("the port still accepts connections after Close")
	}
}

func TestStartFailsWhenThePortIsTaken(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	if _, err := Start(Config{Root: t.TempDir(), Port: port, NextAddr: "127.0.0.1:1", Routes: routes(t, "")}); err == nil {
		t.Fatal("Start succeeded on a taken port")
	}
}

func freeTestPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestGradioSettingsFollowNextsOrder(t *testing.T) {
	root := t.TempDir()
	ui := filepath.Join(root, "react-ui")
	if err := os.MkdirAll(ui, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ui, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"GRADIO_BACKEND", "GRADIO_BACKEND_AUTOMATIC", "GRADIO_AUTH"} {
		if v, ok := os.LookupEnv(name); ok {
			t.Setenv(name, v) // restore after the test
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}

	if got := newDeps(root, nil).GradioBackend; got != "http://127.0.0.1:7770/" {
		t.Fatalf("default = %q", got)
	}

	write(".env", "# comment\nGRADIO_BACKEND=http://from-env/\nGRADIO_AUTH=\"user:pass\"\n")
	write(".env.production", "export GRADIO_BACKEND=http://from-production/ # note\n")
	deps := newDeps(root, nil)
	if deps.GradioBackend != "http://from-production/" || deps.GradioAuth != "user:pass" {
		t.Fatalf("deps = %+v", deps)
	}

	write(".env.local", "GRADIO_BACKEND=http://from-local/\n")
	if got := newDeps(root, nil).GradioBackend; got != "http://from-local/" {
		t.Fatalf(".env.local ignored: %q", got)
	}

	t.Setenv("GRADIO_BACKEND_AUTOMATIC", "http://automatic/")
	if got := newDeps(root, nil).GradioBackend; got != "http://from-local/" {
		t.Fatalf("GRADIO_BACKEND must win over the automatic value: %q", got)
	}
	t.Setenv("GRADIO_BACKEND", "")
	if got := newDeps(root, nil).GradioBackend; got != "http://automatic/" {
		t.Fatalf("an empty GRADIO_BACKEND must fall through: %q", got)
	}
	t.Setenv("GRADIO_BACKEND", "http://process/")
	if got := newDeps(root, nil).GradioBackend; got != "http://process/" {
		t.Fatalf("the process environment must win: %q", got)
	}
}
