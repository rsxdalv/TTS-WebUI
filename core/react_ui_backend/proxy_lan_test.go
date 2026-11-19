package react_ui_backend

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// guardFront starts a proxy handler with fake interface addresses.
func guardFront(t *testing.T, lan bool, allowed []string, ips ...string) (string, *handler) {
	t.Helper()
	next := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(next.Close)
	hh, err := NewHandler(Config{Root: t.TempDir(), Port: testPort, NextAddr: strings.TrimPrefix(next.URL, "http://"),
		Routes: routes(t, ""), LAN: lan, AllowedHosts: allowed})
	if err != nil {
		t.Fatal(err)
	}
	h := hh.(*handler)
	if h.lan != nil {
		h.lan.lookup = func() ([]net.IP, error) {
			var out []net.IP
			for _, s := range ips {
				out = append(out, net.ParseIP(s))
			}
			return out, nil
		}
	}
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	return front.URL, h
}

var lanIPs = []string{"127.0.0.1", "::1", "192.168.1.66", "fe80::1c2d", "2001:db8::5"}

func TestHostGuardDefaultModeIsUnchanged(t *testing.T) {
	front, _ := guardFront(t, false, nil)
	port := strconv.Itoa(testPort)
	for host, want := range map[string]int{
		"127.0.0.1:" + port:     200,
		"localhost:" + port:     200,
		"[::1]:" + port:         200,
		"192.168.1.66:" + port:  400,
		"evil.example:" + port:  400,
		"[2001:db8::5]:" + port: 400,
	} {
		if got := request(t, "GET", front+"/", nil, map[string]string{"Host": host}).StatusCode; got != want {
			t.Errorf("Host %q: %d, want %d", host, got, want)
		}
	}
	lanOrigin := map[string]string{"Origin": "http://192.168.1.66:" + port}
	if got := request(t, "POST", front+"/api/upload", nil, lanOrigin).StatusCode; got != 403 {
		t.Errorf("LAN origin in default mode: %d, want 403", got)
	}
}

func TestHostGuardLANMode(t *testing.T) {
	front, _ := guardFront(t, true, []string{"mypc.local", "10.9.8.7"}, lanIPs...)
	port := strconv.Itoa(testPort)
	cases := map[string]int{
		"127.0.0.1:" + port:           200,
		"localhost:" + port:           200,
		"[::1]:" + port:               200,
		"192.168.1.66:" + port:        200,
		"[2001:db8::5]:" + port:       200,
		"[2001:0db8::0005]:" + port:   200, // same address, other spelling
		"[fe80::1c2d]:" + port:        200,
		"mypc.local:" + port:          200,
		"MyPC.Local:" + port:          200,
		"10.9.8.7:" + port:            200,
		"evil.example:" + port:        400, // DNS rebinding
		"192.168.1.66.nip.io:" + port: 400,
		"192.168.1.67:" + port:        400, // not this machine
		"192.168.1.66:3001":           400, // other port
		"192.168.1.66":                400, // no port
		"mypc.local.:" + port:         400,
		"0.0.0.0:" + port:             400,
		"":                            400,
	}
	for host, want := range cases {
		if got := request(t, "GET", front+"/", nil, map[string]string{"Host": host}).StatusCode; got != want {
			t.Errorf("Host %q: %d, want %d", host, got, want)
		}
	}
}

func TestOriginGuardLANMode(t *testing.T) {
	front, _ := guardFront(t, true, []string{"mypc.local"}, lanIPs...)
	port := strconv.Itoa(testPort)
	lanHost := map[string]string{"Host": "192.168.1.66:" + port}
	cases := []struct {
		method, path, origin string
		want                 int
	}{
		{"POST", "/api/upload", "http://192.168.1.66:" + port, 200},
		{"POST", "/api/upload", "http://mypc.local:" + port, 200},
		{"POST", "/api/upload", "http://[2001:db8::5]:" + port, 200},
		{"POST", "/api/upload", "http://localhost:" + port, 200},
		{"POST", "/api/upload", "http://evil.example:" + port, 403},
		{"POST", "/api/upload", "https://192.168.1.66:" + port, 403},
		{"POST", "/api/upload", "http://192.168.1.66:3001", 403},
		{"POST", "/api/upload", "http://192.168.1.66", 403},
		{"POST", "/api/upload", "http://192.168.1.66:" + port + "/", 403},
		{"POST", "/api/upload", "http://user@192.168.1.66:" + port, 403},
		{"POST", "/api/upload", "http://192.168.1.67:" + port, 403},
		{"POST", "/api/upload", "null", 403},
		{"GET", "/api/gradio/x", "http://evil.example:" + port, 403},
		{"GET", "/x/../api/gradio/x", "http://evil.example:" + port, 403},
		{"GET", "/x/%2e%2e/api/gradio/x", "http://evil.example:" + port, 403},
		{"GET", "//api/gradio/x", "http://evil.example:" + port, 403},
		{"GET", "/", "http://evil.example:" + port, 200}, // page loads are not checked
	}
	for _, c := range cases {
		headers := map[string]string{"Host": lanHost["Host"], "Origin": c.origin}
		if got := request(t, c.method, front+c.path, nil, headers).StatusCode; got != c.want {
			t.Errorf("%s %s Origin %q: %d, want %d", c.method, c.path, c.origin, got, c.want)
		}
	}
}

func TestLANAddressesAreReread(t *testing.T) {
	_, h := guardFront(t, true, nil)
	var calls atomic.Int32
	ips := []string{"192.168.1.66"}
	h.lan.lookup = func() ([]net.IP, error) {
		calls.Add(1)
		var out []net.IP
		for _, s := range ips {
			out = append(out, net.ParseIP(s))
		}
		return out, nil
	}
	if !h.lan.accepts("192.168.1.66") {
		t.Fatal("own address refused")
	}
	// Go's HTTP client drops IPv6 zones from Host, so check zoned names here.
	if h.lan.accepts("fe80::1c2d%eth0") || h.lan.accepts("fe80::1c2d%25eth0") {
		t.Fatal("zoned IPv6 address accepted")
	}
	for i := 0; i < 5; i++ {
		if h.lan.accepts("10.0.0.9") {
			t.Fatal("unknown address accepted")
		}
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("interfaces read %d times within the refresh window, want 1", n)
	}

	ips = append(ips, "10.0.0.9") // DHCP gave a new address
	h.lan.mu.Lock()
	h.lan.refreshed = time.Now().Add(-2 * refreshEvery)
	h.lan.mu.Unlock()
	if !h.lan.accepts("10.0.0.9") {
		t.Fatal("new address not picked up after the refresh window")
	}
}

func TestLANServerListensOnAllInterfaces(t *testing.T) {
	next := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") }))
	defer next.Close()
	port := freeTestPort(t)
	s, err := Start(Config{Root: t.TempDir(), Port: port, NextAddr: strings.TrimPrefix(next.URL, "http://"), Routes: routes(t, ""), LAN: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, addr := range s.Addrs() {
		host, _, _ := net.SplitHostPort(addr)
		if ip := net.ParseIP(host); ip == nil || !ip.IsUnspecified() {
			t.Fatalf("LAN mode listens on %s, want all interfaces", addr)
		}
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("body %q", body)
	}
	s.Close()
	if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("still listening after Close")
	}
}

func TestLANWarningSaysWhereAndNoPassword(t *testing.T) {
	w := LANWarning(3000)
	for _, want := range []string{"reachable from the network", "no password", "TTS_GO_PROXY_LISTEN"} {
		if !strings.Contains(w, want) {
			t.Errorf("warning %q lacks %q", w, want)
		}
	}
	for _, u := range LANURLs(3000) {
		if !strings.Contains(w, u) || !strings.HasSuffix(u, ":3000") {
			t.Errorf("warning %q lacks URL %q", w, u)
		}
	}
}
