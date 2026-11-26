package react_ui_backend

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGuardDial(t *testing.T) {
	ports := map[string]bool{"3000": true, "7770": true}
	local := func(ip net.IP) bool { return ip.String() == "192.168.1.5" }
	cases := []struct {
		addr string
		ok   bool
		why  string
	}{
		{"169.254.169.254:80", false, "cloud metadata"},
		{"[fe80::1]:3000", false, "link-local v6, even on an app port"},
		{"[::ffff:169.254.169.254]:80", false, "v4-mapped metadata"},
		{"0.0.0.0:3000", false, "unspecified"},
		{"224.0.0.1:80", false, "multicast"},
		{"127.0.0.1:3000", true, "own UI port on loopback"},
		{"127.0.0.1:7770", true, "gradio port on loopback"},
		{"[::1]:3000", true, "own UI port on ::1"},
		{"127.0.0.1:5432", false, "another service on loopback"},
		{"127.0.0.1:11434", false, "another service on loopback"},
		{"192.168.1.5:3000", true, "own LAN address on the UI port"},
		{"192.168.1.5:22", false, "own LAN address, other port"},
		{"192.168.1.9:3000", false, "another LAN host on the UI port"},
		{"10.0.0.1:80", false, "private host"},
		{"[fd00::1]:3000", false, "unique local, not ours"},
		{"93.184.216.34:443", true, "public address"},
		{"[2606:2800:220:1::1]:80", true, "public v6"},
	}
	for _, c := range cases {
		err := guardDial(c.addr, ports, local)
		if (err == nil) != c.ok {
			t.Errorf("%s (%s): allowed=%v, want %v (err %v)", c.addr, c.why, err == nil, c.ok, err)
		}
	}
	if guardDial("not-an-address", ports, local) == nil {
		t.Error("an address that is not ip:port was allowed")
	}
}

func TestAppPorts(t *testing.T) {
	got := appPorts("localhost:3000", "http://127.0.0.1:7770")
	if !got["3000"] || !got["7770"] || len(got) != 2 {
		t.Errorf("ports = %v, want 3000 and 7770", got)
	}
	got = appPorts("ui.example", "https://gradio.example")
	if !got["80"] || !got["443"] {
		t.Errorf("a Host without a port should allow 80 and 443, got %v", got)
	}
	if len(appPorts("", "")) != 0 {
		t.Error("no host and no backend should allow no port")
	}
}

func fetchCall(selfHost string) *gradioCall {
	return &gradioCall{route: &gradioRoute{backend: "http://127.0.0.1:1"}, ctx: context.Background(), selfHost: selfHost}
}

func hostOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func TestReadFromURLFetchesTheAppsOwnOrigin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("wav")) }))
	defer srv.Close()
	got, err := fetchCall(hostOf(t, srv)).readFromURL(srv.URL + "/api/webui-generations/a.wav")
	if err != nil {
		t.Fatalf("own origin refused: %v", err)
	}
	if b, ok := got.(jsBlob); !ok || string(b.data) != "wav" {
		t.Errorf("got %#v", got)
	}
}

func TestReadFromURLRefusesOtherLoopbackServices(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer other.Close()
	// The request came in on another port: the service on `other` is not ours.
	_, err := fetchCall("localhost:3000").readFromURL(other.URL + "/")
	if err == nil || !strings.Contains(err.Error(), "internal address") {
		t.Fatalf("a loopback service outside the app was fetched: %v", err)
	}
}

func TestReadFromURLRefusesMetadataAddress(t *testing.T) {
	_, err := fetchCall("localhost:3000").readFromURL("http://169.254.169.254/latest/meta-data/")
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("the metadata address was not refused: %v", err)
	}
}

func TestReadFromURLRefusesARedirectToAnInternalService(t *testing.T) {
	secret := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer secret.Close()
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, secret.URL+"/", http.StatusFound)
	}))
	defer front.Close()
	// `front` is the app's own origin; it redirects to a port that is not.
	_, err := fetchCall(hostOf(t, front)).readFromURL(front.URL + "/a.wav")
	if err == nil || !strings.Contains(err.Error(), "internal address") {
		t.Fatalf("a redirect to an internal service was followed: %v", err)
	}
}

func TestReadFromURLStillRefusesOtherSchemes(t *testing.T) {
	for _, raw := range []string{"http://", "httpfoo://x/", "file:///etc/passwd", "ftp://x/"} {
		if _, err := fetchCall("localhost:3000").readFromURL(raw); err == nil {
			t.Errorf("%q was fetched", raw)
		}
	}
}
