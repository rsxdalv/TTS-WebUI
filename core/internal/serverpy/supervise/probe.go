package supervise

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// probeFunc reports whether something answers on 127.0.0.1:port.
type probeFunc func(ctx context.Context, port int) bool

// tcpProbe succeeds when a TCP connection to 127.0.0.1:port is accepted.
func tcpProbe(ctx context.Context, port int) bool {
	d := net.Dialer{Timeout: 400 * time.Millisecond}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// probeClient never follows redirects: a 3xx is an answer.
var probeClient = &http.Client{
	Timeout: 1500 * time.Millisecond,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// httpProbe succeeds when GET / on 127.0.0.1:port answers with a status below
// 500. The Python proxy tree accepts TCP connections long before Gradio is
// up and answers 502 until then, so a plain connect would report ready far too
// early for the public port.
func httpProbe(ctx context.Context, port int) bool { return probeScheme(ctx, port, false, "/") }

// httpsProbe is httpProbe for a public port the router serves over TLS.
func httpsProbe(ctx context.Context, port int) bool { return probeScheme(ctx, port, true, "/") }

// pathProbe is httpProbe/httpsProbe for Gradio living under a prefix of the
// public port (the router's "/" is then another service, so "/" proves nothing).
func pathProbe(useTLS bool, prefix string) probeFunc {
	path := prefix + "/"
	return func(ctx context.Context, port int) bool { return probeScheme(ctx, port, useTLS, path) }
}

// tlsProbeClient skips certificate verification: it only asks our own router,
// on 127.0.0.1, whether it answers, and the certificate is usually issued for a
// host name the probe does not use.
var tlsProbeClient = &http.Client{
	Timeout:       probeClient.Timeout,
	CheckRedirect: probeClient.CheckRedirect,
	Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, DisableKeepAlives: true},
}

func probeScheme(ctx context.Context, port int, useTLS bool, path string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		publicURLScheme(port, useTLS)+path, nil)
	if err != nil {
		return false
	}
	client := probeClient
	if useTLS {
		client = tlsProbeClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	return resp.StatusCode < 500
}

// publicURL is the URL of the user-facing Gradio entry.
func publicURL(port int) string { return publicURLScheme(port, false) }

// publicURLScheme is publicURL, https when the router serves TLS.
func publicURLScheme(port int, useTLS bool) string {
	if useTLS {
		return fmt.Sprintf("https://127.0.0.1:%d", port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
