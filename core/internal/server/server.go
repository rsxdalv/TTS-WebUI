// Package server implements the local HTTP server for TTS WebUI core.
//
// Security model (D9):
//   - Binds to 127.0.0.1 only.
//   - Injects a per-run random token into index.html; all /api/* endpoints
//     (except /api/health) require the X-TTS-Token header.
//   - Rejects requests whose Host header is not 127.0.0.1:<port> or localhost:<port>
//     (DNS-rebinding guard).
//   - Rejects cross-origin Origin headers.
package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/webui"
)

// Options holds optional configuration for the server.
type Options struct {
	// UIDev, when non-empty, is a URL (e.g. "http://localhost:5173") to which
	// all non-/api requests are reverse-proxied instead of serving the embedded
	// UI. Pass this with the --ui-dev flag during development.
	UIDev string
}

// Server is the local HTTP server.
type Server struct {
	port  int
	token string
	opts  Options
	mux   *http.ServeMux
}

// New creates a Server that will listen on port (0 = OS-assigned).
// A fresh 32-byte random token is generated for every call.
func New(port int) (*Server, error) {
	return NewWithOptions(port, Options{})
}

// NewWithOptions creates a Server with the given options.
func NewWithOptions(port int, opts Options) (*Server, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}
	s := &Server{
		port:  port,
		token: hex.EncodeToString(raw),
		opts:  opts,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

// Token returns the per-run token that clients must send as X-TTS-Token.
func (s *Server) Token() string { return s.token }

// Addr returns the TCP address string "127.0.0.1:<port>".
func (s *Server) Addr() string { return fmt.Sprintf("127.0.0.1:%d", s.port) }

// Handle registers an additional handler pattern on the mux.
// Patterns starting with /api/ (excluding /api/health) are automatically
// wrapped in the token guard so callers cannot accidentally skip auth.
func (s *Server) Handle(pattern string, h http.Handler) {
	if strings.Contains(pattern, "/api/") && !strings.HasSuffix(pattern, "/api/health") {
		s.mux.Handle(pattern, s.tokenGuard(h))
	} else {
		s.mux.Handle(pattern, h)
	}
}

// Listen creates and returns a bound TCP listener on Addr().
// If s.port was 0, it is updated to the actual OS-assigned port.
func (s *Server) Listen() (net.Listener, error) {
	l, err := net.Listen("tcp", s.Addr())
	if err != nil {
		return nil, err
	}
	// Update port in case the OS picked one.
	s.port = l.Addr().(*net.TCPAddr).Port
	return l, nil
}

// Serve accepts connections on l until l is closed.
func (s *Server) Serve(l net.Listener) error {
	// Capture the actual port in case Listen() was not called by the caller.
	if addr, ok := l.Addr().(*net.TCPAddr); ok {
		s.port = addr.Port
	}
	srv := &http.Server{Handler: s.Handler()}
	return srv.Serve(l)
}

// Handler returns an http.Handler that enforces Host and Origin guards,
// then dispatches to the registered routes.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.validHost(r.Host) {
			http.Error(w, "bad request: invalid Host", http.StatusBadRequest)
			return
		}
		if !s.validOrigin(r) {
			http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	uiFS, err := fs.Sub(webui.FS(), ".")
	if err != nil {
		slog.Error("sub webui fs", "err", err)
		uiFS = webui.FS()
	}

	// Health check — no token required.
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":  true,
			"os":  runtime.GOOS,
			"cwd": cwd,
		})
	})

	// All other /api/* require the token.
	s.mux.HandleFunc("/api/", s.tokenGuard(http.NotFoundHandler()))

	// UI: in dev mode proxy to the Vite dev server; otherwise serve embedded.
	if s.opts.UIDev != "" {
		devTarget, err := url.Parse(s.opts.UIDev)
		if err == nil {
			rp := httputil.NewSingleHostReverseProxy(devTarget)
			s.mux.Handle("/", rp)
			return
		}
		slog.Error("invalid --ui-dev URL, falling back to embedded", "url", s.opts.UIDev, "err", err)
	}
	s.mux.HandleFunc("/", s.uiHandler(uiFS))
}

// tokenGuard wraps a handler to require either the X-TTS-Token header or the
// tts-token cookie (set by serveIndexWithToken for EventSource and media requests).
func (s *Server) tokenGuard(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-TTS-Token")), []byte(s.token)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		if c, err := r.Cookie("tts-token"); err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "forbidden: missing or invalid token", http.StatusForbidden)
	}
}

func (s *Server) uiHandler(files fs.FS) http.HandlerFunc {
	fileServer := http.FileServerFS(files)
	return func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/" || p == "/index.html" || p == "" {
			serveIndexWithToken(w, files, s.token)
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}

// serveIndexWithToken reads index.html, injects the token meta tag, and sets
// a SameSite=Strict HttpOnly cookie so EventSource and <audio src> can authenticate.
func serveIndexWithToken(w http.ResponseWriter, files fs.FS, token string) {
	data, err := fs.ReadFile(files, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	meta := `<meta name="tts-webui-token" content="` + token + `">`
	injected := strings.Replace(string(data), "<head>", "<head>\n\t"+meta, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.SetCookie(w, &http.Cookie{
		Name:     "tts-token",
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
	})
	_, _ = w.Write([]byte(injected))
}

// validHost returns true if the Host header is 127.0.0.1:<port> or localhost:<port>.
func (s *Server) validHost(host string) bool {
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		return false
	}
	portStr := fmt.Sprintf("%d", s.port)
	if port != portStr {
		return false
	}
	return hostname == "127.0.0.1" || hostname == "localhost"
}

// validOrigin rejects cross-origin requests. An absent Origin is allowed
// (direct browser navigation, curl, etc.).
func (s *Server) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	portStr := fmt.Sprintf("%d", s.port)
	return origin == "http://127.0.0.1:"+portStr ||
		origin == "http://localhost:"+portStr
}
