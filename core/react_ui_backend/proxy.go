package react_ui_backend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
)

// PublicPort is the port the browser uses, the same one Next uses today.
const PublicPort = 3000

// claimsCookie tells pages which data claims are on (CONTRACTS.md section 7).
const claimsCookie = "tts-go-claims"

// Config describes one proxy.
type Config struct {
	// Root is the absolute install root.
	Root string
	// Port is the public port.
	Port int
	// NextAddr is the host:port where Next listens.
	NextAddr string
	// Routes says which claims are on.
	Routes switches.Set
}

// handler is the front door for every request on the public port.
type handler struct {
	port     string
	origins  map[string]bool
	next     http.Handler
	mux      *http.ServeMux
	prefixes map[string]bool
	cookie   string
}

// NewHandler returns the proxy's HTTP handler.
func NewHandler(cfg Config) (http.Handler, error) {
	if err := cfg.Routes.CheckRoutes(claimNames()); err != nil {
		return nil, err
	}
	next, err := newNextProxy(cfg)
	if err != nil {
		return nil, err
	}

	port := strconv.Itoa(cfg.Port)
	h := &handler{
		port: port,
		origins: map[string]bool{
			"http://127.0.0.1:" + port: true,
			"http://localhost:" + port: true,
			"http://[::1]:" + port:     true,
		},
		next:     next,
		mux:      http.NewServeMux(),
		prefixes: map[string]bool{},
	}

	deps := newDeps(cfg.Root, next)
	var dataClaims []string
	for _, c := range registeredClaims() {
		if !cfg.Routes.Route(c.Switch) {
			continue
		}
		route := c.Handler(deps)
		for _, pattern := range c.Patterns {
			prefix, err := claimPrefix(pattern)
			if err != nil {
				return nil, err
			}
			if prefix != "" {
				h.prefixes[prefix] = true
			}
			h.mux.Handle(pattern, route)
		}
		if strings.HasPrefix(c.Switch, "data-") {
			dataClaims = append(dataClaims, c.Switch)
		}
	}
	h.cookie = strings.Join(dataClaims, ".")
	return h, nil
}

// ServeHTTP applies the dispatch order in CONTRACTS.md section 5.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.validHost(r.Host) {
		http.Error(w, "bad request: invalid Host", http.StatusBadRequest)
		return
	}
	if originChecked(r) && !h.validOrigin(r.Header.Get("Origin"), r) {
		http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
		return
	}
	h.setClaimsCookie(w, r)

	if h.goAnswers(r) {
		if _, pattern := h.mux.Handler(r); pattern != "" {
			h.mux.ServeHTTP(w, r)
			return
		}
	}
	h.next.ServeHTTP(w, r)
}

// validHost is the DNS-rebinding guard.
func (h *handler) validHost(host string) bool {
	name, port, err := net.SplitHostPort(host)
	if err != nil || port != h.port {
		return false
	}
	return name == "127.0.0.1" || name == "::1" || strings.EqualFold(name, "localhost")
}

// originChecked reports whether a request must pass the Origin check.
func originChecked(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return true
	}
	// Next resolves dot segments and repeated slashes before routing, so check
	// the path it will route, not the one sent.
	p := path.Clean("/" + r.URL.Path)
	return p == "/api" || strings.HasPrefix(p, "/api/")
}

func (h *handler) validOrigin(origin string, r *http.Request) bool {
	if _, present := r.Header["Origin"]; !present {
		return true
	}
	return h.origins[origin]
}

// goAnswers reports whether a claim may answer this path. Anything else goes to
// Next, so Go's mux never produces a redirect of its own.
func (h *handler) goAnswers(r *http.Request) bool {
	raw := r.URL.EscapedPath()
	decoded := r.URL.Path
	if !cleanSegments(raw) || !cleanSegments(decoded) {
		return false
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
		return false
	}
	if strings.ContainsAny(decoded, "\\\x00") {
		return false
	}
	return !h.prefixes[decoded]
}

// cleanSegments reports whether path has no empty, "." or ".." segment. A
// trailing slash counts as an empty segment.
func cleanSegments(path string) bool {
	rest, ok := strings.CutPrefix(path, "/")
	if !ok {
		return false
	}
	for _, segment := range strings.Split(rest, "/") {
		switch strings.ToLower(segment) {
		case "", ".", "..", "%2e", ".%2e", "%2e.", "%2e%2e":
			return false
		}
	}
	return true
}

// setClaimsCookie tells a loaded page which data claims Go answers.
func (h *handler) setClaimsCookie(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Sec-Fetch-Dest") != "document" && !strings.Contains(r.Header.Get("Accept"), "text/html") {
		return
	}
	if h.cookie != "" {
		http.SetCookie(w, &http.Cookie{Name: claimsCookie, Value: h.cookie, Path: "/", SameSite: http.SameSiteStrictMode})
		return
	}
	if _, err := r.Cookie(claimsCookie); err == nil {
		http.SetCookie(w, &http.Cookie{Name: claimsCookie, Path: "/", MaxAge: -1, SameSite: http.SameSiteStrictMode})
	}
}

// newNextProxy builds the reverse proxy to Next. Nothing here limits how long a
// request runs or how large its body is: a generation can take many minutes.
func newNextProxy(cfg Config) (http.Handler, error) {
	target, err := url.Parse("http://" + cfg.NextAddr)
	if err != nil {
		return nil, fmt.Errorf("next address %q: %w", cfg.NextAddr, err)
	}
	port := strconv.Itoa(cfg.Port)
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			// Keep what the browser sent: SetURL replaces Host, and the proxy
			// drops query strings it cannot parse.
			pr.Out.Host = pr.In.Host
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
			pr.SetXForwarded()
			pr.Out.Header.Set("X-Forwarded-Port", port)
		},
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			DisableCompression:  true,
			MaxIdleConns:        64,
			MaxIdleConnsPerHost: 64,
			IdleConnTimeout:     90 * time.Second,
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, context.Canceled) {
				return // the browser went away
			}
			slog.Warn("react-ui proxy", "path", r.URL.Path, "err", err)
			http.Error(w, "React UI is not reachable", http.StatusBadGateway)
		},
	}, nil
}

// Server is a running proxy.
type Server struct {
	srv       *http.Server
	listeners []net.Listener
	closeOnce sync.Once
}

// Start binds the loopback addresses and serves until Close.
func Start(cfg Config) (*Server, error) {
	h, err := NewHandler(cfg)
	if err != nil {
		return nil, err
	}
	listeners, err := listenLoopback(cfg.Port)
	if err != nil {
		return nil, err
	}
	s := &Server{srv: &http.Server{Handler: h}, listeners: listeners}
	for _, l := range listeners {
		go func(l net.Listener) { _ = s.srv.Serve(l) }(l)
	}
	return s, nil
}

// Addrs returns the addresses the server listens on.
func (s *Server) Addrs() []string {
	var addrs []string
	for _, l := range s.listeners {
		addrs = append(addrs, l.Addr().String())
	}
	return addrs
}

// Close stops listening and ends all connections. It is safe to call twice.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		_ = s.srv.Close()
		for _, l := range s.listeners {
			_ = l.Close()
		}
	})
}

// listenLoopback binds 127.0.0.1 and, where IPv6 exists, [::1]. It never binds
// all interfaces.
func listenLoopback(port int) ([]net.Listener, error) {
	v4, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("listening on port %d: %w", port, err)
	}
	listeners := []net.Listener{v4}

	probe, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return listeners, nil // this machine has no IPv6 loopback
	}
	_ = probe.Close()

	// With port 0 the v6 side must share the port the OS picked for v4.
	v6Port := v4.Addr().(*net.TCPAddr).Port
	v6, err := net.Listen("tcp6", net.JoinHostPort("::1", strconv.Itoa(v6Port)))
	if err != nil {
		_ = v4.Close()
		return nil, fmt.Errorf("listening on [::1]:%d: %w", v6Port, err)
	}
	return append(listeners, v6), nil
}
