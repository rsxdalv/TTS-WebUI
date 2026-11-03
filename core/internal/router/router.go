// Package router implements a prefix-routing reverse proxy that replaces the
// Python gradio_proxy_tree. It uses only stdlib net/http; WebSocket tunnelling
// is implemented with http.Hijacker.
package router

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// hopByHop lists headers that must be stripped between proxy hops (RFC 7230 §6.1).
var hopByHop = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

// Router is a prefix-routing reverse proxy.
type Router struct {
	mu     sync.RWMutex
	routes []route
}

type route struct {
	prefix string
	target *url.URL
}

// New returns an empty Router.
func New() *Router { return &Router{} }

// AddRoute maps all requests whose URL path starts with prefix to the target.
// Longer prefixes take priority. Calling AddRoute with the same prefix again
// replaces the previous entry.
func (r *Router) AddRoute(prefix string, target *url.URL) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, rt := range r.routes {
		if rt.prefix == prefix {
			r.routes[i].target = target
			return
		}
	}
	r.routes = append(r.routes, route{prefix, target})
	// Sort longest-first so the most specific prefix wins.
	sort.Slice(r.routes, func(i, j int) bool {
		return len(r.routes[i].prefix) > len(r.routes[j].prefix)
	})
}

// ServeHTTP dispatches the request to the matching backend.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	target := r.match(req.URL.Path)
	if target == nil {
		http.Error(w, "no route found", http.StatusBadGateway)
		return
	}
	if isWebSocket(req) {
		r.handleWebSocket(w, req, target)
		return
	}
	r.proxy(target).ServeHTTP(w, req)
}

// match returns the target URL for the first route whose prefix matches path.
func (r *Router) match(path string) *url.URL {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rt := range r.routes {
		if strings.HasPrefix(path, rt.prefix) {
			return rt.target
		}
	}
	return nil
}

// proxy builds an httputil.ReverseProxy for the given target.
func (r *Router) proxy(target *url.URL) *httputil.ReverseProxy {
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Preserve the raw path and query (Gradio sends ?file=C:/...).
			rawQuery := req.URL.RawQuery
			rawPath := req.URL.RawPath
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			if rawPath != "" {
				req.URL.RawPath = rawPath
			}
			req.URL.RawQuery = rawQuery
			req.RequestURI = "" // must be cleared for outbound requests
			removeHopByHop(req.Header)
			setXForwarded(req)
		},
		FlushInterval:  -1, // flush immediately for SSE/streaming
		ModifyResponse: stripHopByHopResponse,
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			if req.Context().Err() != nil {
				// Client disconnected — not an upstream error.
				return
			}
			slog.Warn("proxy error", "err", err, "path", req.URL.Path)
			http.Error(w, "bad gateway: "+err.Error(), http.StatusBadGateway)
		},
		// No read/write deadline on proxy connections; Gradio generations can be long.
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			ResponseHeaderTimeout: 0, // no timeout waiting for engine response
		},
	}
	return rp
}

// isWebSocket reports whether req is an HTTP Upgrade request for WebSocket.
func isWebSocket(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
}

// handleWebSocket tunnels a WebSocket connection to the backend using Hijacker.
func (r *Router) handleWebSocket(w http.ResponseWriter, req *http.Request, target *url.URL) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket hijacking not supported", http.StatusInternalServerError)
		return
	}

	// Dial the backend before hijacking so we can still send an HTTP error.
	backendConn, err := net.DialTimeout("tcp", target.Host, 10*time.Second)
	if err != nil {
		http.Error(w, fmt.Sprintf("bad gateway: %v", err), http.StatusBadGateway)
		return
	}

	// Hijack the client connection — from here we own raw TCP on both sides.
	clientConn, clientBuf, err := hj.Hijack()
	if err != nil {
		backendConn.Close()
		slog.Warn("websocket hijack failed", "err", err)
		return
	}
	defer clientConn.Close()
	defer backendConn.Close()

	// Build and forward the upgrade request to the backend.
	outReq := req.Clone(context.Background())
	outReq.URL = &url.URL{
		Scheme:   "http",
		Host:     target.Host,
		Path:     req.URL.Path,
		RawPath:  req.URL.RawPath,
		RawQuery: req.URL.RawQuery,
	}
	outReq.Host = target.Host
	outReq.RequestURI = ""
	removeHopByHop(outReq.Header)
	setXForwarded(outReq)

	if err := outReq.Write(backendConn); err != nil {
		fmt.Fprintf(clientConn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}

	// Drain any bytes already buffered from the client side.
	if clientBuf != nil {
		if n := clientBuf.Reader.Buffered(); n > 0 {
			buf := make([]byte, n)
			if _, err := io.ReadFull(clientBuf.Reader, buf); err == nil {
				backendConn.Write(buf)
			}
		}
	}

	// Bidirectional tunnel: copy until one side closes.
	errc := make(chan error, 2)
	go func() { _, err := io.Copy(backendConn, clientConn); errc <- err }()
	go func() { _, err := io.Copy(clientConn, backendConn); errc <- err }()
	<-errc
}

// removeHopByHop deletes hop-by-hop headers from h.
func removeHopByHop(h http.Header) {
	// Also remove headers named in the Connection header.
	for _, name := range strings.Split(h.Get("Connection"), ",") {
		h.Del(strings.TrimSpace(name))
	}
	for _, name := range hopByHop {
		h.Del(name)
	}
}

// setXForwarded sets X-Forwarded-For, X-Forwarded-Host and X-Forwarded-Proto
// on the outbound request.
func setXForwarded(req *http.Request) {
	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		clientIP = req.RemoteAddr
	}
	if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
		req.Header.Set("X-Forwarded-For", prior+", "+clientIP)
	} else {
		req.Header.Set("X-Forwarded-For", clientIP)
	}
	req.Header.Set("X-Forwarded-Host", req.Host)
	proto := "http"
	if req.TLS != nil {
		proto = "https"
	}
	req.Header.Set("X-Forwarded-Proto", proto)
}

// stripHopByHopResponse removes hop-by-hop headers from the backend response.
func stripHopByHopResponse(resp *http.Response) error {
	removeHopByHop(resp.Header)
	return nil
}

// ParseRoute parses a "prefix=http://host:port" string into its components.
func ParseRoute(s string) (prefix string, target *url.URL, err error) {
	eq := strings.Index(s, "=")
	if eq < 0 {
		return "", nil, fmt.Errorf("route %q must be in the form /prefix=http://host:port", s)
	}
	prefix = s[:eq]
	target, err = url.Parse(s[eq+1:])
	if err != nil {
		return "", nil, fmt.Errorf("parsing target URL: %w", err)
	}
	return prefix, target, nil
}

// BufferedConn wraps a net.Conn with a bufio.Reader for the hijacked path.
type BufferedConn struct {
	net.Conn
	R *bufio.Reader
}
