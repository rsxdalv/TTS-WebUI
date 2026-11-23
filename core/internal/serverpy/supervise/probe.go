package supervise

import (
	"context"
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
func httpProbe(ctx context.Context, port int) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+net.JoinHostPort("127.0.0.1", strconv.Itoa(port))+"/", nil)
	if err != nil {
		return false
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	return resp.StatusCode < 500
}

// publicURL is the URL of the user-facing Gradio entry.
func publicURL(port int) string { return fmt.Sprintf("http://127.0.0.1:%d", port) }
