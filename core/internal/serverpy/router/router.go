// Package router is the Go replacement for the Python "Gradio proxy tree"
// (tts_webui/gradio_proxy_tree). It is a path-prefix reverse proxy: the public
// port serves Gradio at "/" and every proxied extension at "/<pkg>", each one
// forwarded to a local upstream with the prefix stripped.
//
// Behaviour follows proxy_tree.py + handlers.py:
//
//   - longest-prefix match; a path matches a prefix when it equals the prefix
//     or starts with prefix+"/"; the "/" route is the catch-all
//   - the prefix is stripped before forwarding (extensions run with
//     GRADIO_ROOT_PATH=/<pkg>); the raw (still escaped) path and the query
//     string are preserved byte for byte
//   - Host is passed through, X-Forwarded-Host and X-Forwarded-Proto are set,
//     client supplied Forwarded / X-Forwarded-For pass through unchanged
//     (nothing is added, as in Python), hop-by-hop headers are dropped
//   - WebSocket upgrades are tunnelled, responses are never buffered (Gradio's
//     queue is Server-Sent Events)
//   - nothing routed -> JSON 404, upstream unreachable -> 502, any other
//     upstream failure -> 500, with the same bodies as the Python proxy
//
// The control API ([Router.ControlHandler]) lets other processes (the Python
// bridge, the extension supervisor) add and remove routes.
package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Options configures a Router.
type Options struct {
	// Token is the shared secret for the control API (header X-TTS-Core-Token).
	// An empty token disables the control API: every request is refused.
	Token string
	// Logger receives route changes and proxy errors. Nil means slog.Default().
	Logger *slog.Logger
	// Home answers GET / when no route owns it (there is no "/" upstream, as
	// while the Gradio host is not installed). Nil means the JSON 404.
	Home http.Handler
}

// route is one prefix -> upstream mapping. The ReverseProxy is built once per
// route; replacing a route swaps the whole struct, in-flight requests finish on
// the old one.
type route struct {
	prefix string       // normalized: "" is the catch-all, otherwise "/pkg"
	target string       // "http://127.0.0.1:27771", or "(in process)" for a mounted handler
	proxy  http.Handler // the reverse proxy, or the handler of AddHandler
}

// Router is the reverse proxy. The zero value is not usable; call [New].
type Router struct {
	token     string
	log       *slog.Logger
	transport *http.Transport
	home      http.Handler

	mu     sync.RWMutex
	routes map[string]*route

	// activity, when set, is called for each proxied request with the route's
	// prefix; the function it returns runs when the response is finished. The
	// engine manager uses it to know which engine is in use (idle stop, D31).
	activity func(prefix string) func()
}

// SetActivity installs the per-request activity hook. Call it before serving.
func (rt *Router) SetActivity(f func(prefix string) func()) {
	rt.mu.Lock()
	rt.activity = f
	rt.mu.Unlock()
}

// New returns an empty Router: until routes are added every request is a 404.
func New(opts Options) *Router {
	lg := opts.Logger
	if lg == nil {
		lg = slog.Default()
	}
	return &Router{
		token:     opts.Token,
		home:      opts.Home,
		log:       lg,
		transport: newTransport(),
		routes:    make(map[string]*route),
	}
}

// newTransport builds the upstream transport shared by all routes.
func newTransport() *http.Transport {
	return &http.Transport{
		// Upstreams are local processes; never go through an environment proxy.
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// The Python proxy used httpx.Timeout(300): no response headers for
		// five minutes means the upstream is stuck.
		ResponseHeaderTimeout: 300 * time.Second,
		// Gradio's server (uvicorn) closes idle keep-alive connections after
		// 5 s. Retiring ours earlier avoids reusing a connection the upstream
		// is just closing, which would fail a POST that cannot be replayed.
		IdleConnTimeout:     4 * time.Second,
		MaxIdleConnsPerHost: 32,
		// Pass bodies through exactly as the upstream sent them; the client's
		// own Accept-Encoding decides about compression.
		DisableCompression: true,
	}
}

// NormalizePrefix canonicalises a route prefix the way proxy_tree.py does: a
// leading "/" is added, trailing "/" are removed, so "/" (and "") become the
// catch-all, returned as "". It rejects characters that cannot be part of a
// literal URL path prefix.
func NormalizePrefix(prefix string) (string, error) {
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return "", nil
	}
	for _, r := range prefix {
		if r <= 0x20 || r == 0x7f || strings.ContainsRune(`?#%\`, r) {
			return "", fmt.Errorf("router: invalid character %q in prefix %q", r, prefix)
		}
	}
	for _, seg := range strings.Split(prefix[1:], "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", fmt.Errorf("router: invalid path segment %q in prefix %q", seg, prefix)
		}
	}
	return prefix, nil
}

// parseTarget validates an upstream base URL and returns it without a
// trailing slash.
func parseTarget(target string) (*url.URL, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("router: bad target %q: %w", target, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Opaque != "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("router: target %q must look like http://host:port", target)
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}

// AddRoute makes requests under prefix go to target (a full base URL such as
// "http://127.0.0.1:27771"). It is idempotent: adding an existing prefix
// replaces its target. The prefix "/" is the catch-all route.
func (rt *Router) AddRoute(prefix, target string) error {
	p, err := NormalizePrefix(prefix)
	if err != nil {
		return err
	}
	u, err := parseTarget(target)
	if err != nil {
		return err
	}
	r := &route{prefix: p, target: u.String(), proxy: rt.newProxy(p, u)}
	rt.mu.Lock()
	rt.routes[p] = r
	rt.mu.Unlock()
	rt.log.Info("route added", "prefix", displayPrefix(p), "target", r.target)
	return nil
}

// AddHandler makes requests under prefix go to h, which runs inside this
// process (no upstream port). The prefix is NOT stripped: h sees the path the
// browser used, as the "/" route of a proxy does. Like AddRoute it replaces an
// existing prefix. The prefix "/" is the catch-all route.
func (rt *Router) AddHandler(prefix string, h http.Handler) error {
	p, err := NormalizePrefix(prefix)
	if err != nil {
		return err
	}
	if h == nil {
		return errors.New("router: nil handler")
	}
	r := &route{prefix: p, target: "(in process)", proxy: h}
	rt.mu.Lock()
	rt.routes[p] = r
	rt.mu.Unlock()
	rt.log.Info("handler mounted", "prefix", displayPrefix(p))
	return nil
}

// RemoveRoute deletes a route. Removing an unknown prefix is a no-op.
func (rt *Router) RemoveRoute(prefix string) {
	rt.removeRoute(prefix)
}

func (rt *Router) removeRoute(prefix string) bool {
	p, err := NormalizePrefix(prefix)
	if err != nil {
		return false
	}
	rt.mu.Lock()
	_, ok := rt.routes[p]
	delete(rt.routes, p)
	rt.mu.Unlock()
	if ok {
		rt.log.Info("route removed", "prefix", displayPrefix(p))
	}
	return ok
}

// Routes returns a copy of the routing table: prefix -> target URL. The
// catch-all route is listed under "/".
func (rt *Router) Routes() map[string]string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	out := make(map[string]string, len(rt.routes))
	for p, r := range rt.routes {
		out[displayPrefix(p)] = r.target
	}
	return out
}

func displayPrefix(p string) string {
	if p == "" {
		return "/"
	}
	return p
}

// match returns the route with the longest prefix that owns path.
func (rt *Router) match(path string) *route {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	var best *route
	for p, r := range rt.routes {
		if path == p || strings.HasPrefix(path, p+"/") {
			if best == nil || len(p) > len(best.prefix) {
				best = r
			}
		}
	}
	return best
}

// ServeHTTP is the reverse proxy.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rte := rt.match(r.URL.Path)
	if rte == nil {
		rt.serveUnmatched(w, r)
		return
	}
	if rt.log.Enabled(r.Context(), slog.LevelDebug) {
		rt.log.Debug("proxy", "method", r.Method, "path", r.URL.EscapedPath(), "target", rte.target)
	}
	rt.mu.RLock()
	act := rt.activity
	rt.mu.RUnlock()
	if act != nil && rte.prefix != "" {
		defer act(rte.prefix)()
	}
	rte.proxy.ServeHTTP(w, r)
}

// serveUnmatched answers requests no route owns, like the FastAPI app behind
// the Python dispatcher: GET /healthz, GET /routes, otherwise a JSON 404.
// With a "/" route in place none of this is reachable, as in Python.
func (rt *Router) serveUnmatched(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if rt.home != nil && r.URL.Path == "/" {
			rt.home.ServeHTTP(w, r)
			return
		}
		switch r.URL.Path {
		case "/healthz":
			writeJSON(w, http.StatusOK, struct {
				Status string            `json:"status"`
				Routes map[string]string `json:"routes"`
			}{"ok", rt.Routes()})
			return
		case "/routes":
			writeJSON(w, http.StatusOK, struct {
				Routes map[string]string `json:"routes"`
			}{rt.Routes()})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, struct {
		Detail string `json:"detail"`
		Proxy  bool   `json:"proxy"`
	}{fmt.Sprintf("Not Found (proxy server - no route matches /%s)", strings.TrimPrefix(r.URL.Path, "/")), true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// newProxy builds the ReverseProxy for one route.
func (rt *Router) newProxy(prefix string, target *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) { rewriteRequest(pr, prefix, target) },
		// -1 flushes after every write: Gradio's SSE queue must reach the
		// browser as it is produced.
		FlushInterval: -1,
		Transport:     rt.transport,
		ErrorHandler:  rt.proxyError,
		ErrorLog:      slog.NewLogLogger(rt.log.Handler(), slog.LevelWarn),
	}
}

// rewriteRequest turns the inbound request into the upstream one.
func rewriteRequest(pr *httputil.ProxyRequest, prefix string, target *url.URL) {
	in, out := pr.In, pr.Out

	out.URL.Scheme = target.Scheme
	out.URL.Host = target.Host
	out.URL.Path, out.URL.RawPath = stripPrefix(in.URL.Path, in.URL.RawPath, prefix)
	// ReverseProxy drops query parameters it cannot parse (";"); the Python
	// proxy forwarded the query verbatim, so do the same.
	out.URL.RawQuery = in.URL.RawQuery

	// Like nginx's proxy_set_header Host $host: the upstream sees the Host the
	// browser used, which is what makes Gradio build correct absolute URLs.
	host := in.Host
	if host == "" {
		host = "localhost"
	}
	out.Host = host

	// ReverseProxy strips every forwarding header before calling Rewrite.
	// Python forwarded the client's own Forwarded / X-Forwarded-For untouched
	// and only overwrote the two below.
	for _, k := range []string{"Forwarded", "X-Forwarded-For"} {
		if v, ok := in.Header[k]; ok {
			out.Header[k] = append([]string(nil), v...)
		}
	}
	scheme := "http"
	if in.TLS != nil {
		scheme = "https"
	}
	out.Header.Set("X-Forwarded-Host", host)
	out.Header.Set("X-Forwarded-Proto", scheme)
}

// stripPrefix removes prefix from the decoded path and, when the request used a
// non-default escaping, from the raw path too. The returned rawPath is empty
// when Go should derive it from path.
func stripPrefix(path, rawPath, prefix string) (string, string) {
	if prefix == "" {
		return path, rawPath
	}
	p := strings.TrimPrefix(path, prefix)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if rawPath == "" {
		return p, ""
	}
	rest := strings.TrimPrefix(rawPath, prefix)
	if rest == rawPath {
		// The prefix was written with escapes (e.g. %70kg); re-encode from p.
		return p, ""
	}
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}
	return p, rest
}

// proxyError mirrors handlers.py: an unreachable upstream is a 502 "upstream
// unavailable", anything else a 500 "internal error".
func (rt *Router) proxyError(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil {
		// The browser went away (or we are shutting down); nobody to answer.
		rt.log.Debug("proxy request cancelled", "path", r.URL.Path, "err", err)
		return
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		rt.log.Error("upstream unavailable", "path", r.URL.Path, "err", err)
		textError(w, http.StatusBadGateway, "Proxy error: upstream unavailable")
		return
	}
	rt.log.Error("proxy error", "path", r.URL.Path, "err", err)
	textError(w, http.StatusInternalServerError, "Proxy error: internal error")
}

func textError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(msg))
}

// closeIdle drops pooled upstream connections.
func (rt *Router) closeIdle() { rt.transport.CloseIdleConnections() }
