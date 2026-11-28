package supervise

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

// When the router serves TLS the public port answers HTTPS, and the https probe
// (which skips certificate verification on 127.0.0.1) sees it ready.
func TestHTTPSProbeReadsATLSPublicPort(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	if !httpsProbe(context.Background(), port) {
		t.Error("httpsProbe: the TLS server is not seen as ready")
	}
	if got := publicURLScheme(port, true); got != "https://127.0.0.1:"+u.Port() {
		t.Errorf("publicURLScheme = %q", got)
	}
	if !(publicProbe(true, "") != nil && publicProbe(false, "") != nil) {
		t.Error("publicProbe returned nil")
	}
}
