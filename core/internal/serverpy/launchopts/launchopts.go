// Package launchopts computes the keyword arguments of Gradio's launch() for
// the Python child, so Python only calls demo.queue().launch(**options). It
// replaces upgrade_gradio_options, the host/port overrides of
// core_router_bridge, the fixed launch arguments of core_server.py and the
// printing of print_gradio_options.py.
package launchopts

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
)

const (
	// TokenHeader carries the shared secret (same as appconfig, extcatalog, ...).
	TokenHeader = "X-TTS-Core-Token"
	// Path is the single path of the API (GET).
	Path = "/api/launch/options"

	// FaviconPath is relative to the project root, which is the child's cwd.
	faviconPath = "./react-ui/public/favicon.ico"
)

// droppedOptions are keys of old config.json files that launch() no longer
// takes (upgrade_gradio_options removed them), plus the two that are fixed here.
var droppedOptions = []string{"file_directories", "favicon_path", "show_tips", "enable_queue", "prevent_thread_lock"}

// Loader reads config.json (appconfig.Store).
type Loader interface{ Load() ([]byte, error) }

// Options configures a Server.
type Options struct {
	Config Loader
	Token  string // required; requests without it get 403
	// GradioHost and GradioPort are where Gradio itself binds: private, behind the
	// router. They replace server_name and server_port of the config, which
	// describe the public front door.
	GradioHost string
	GradioPort int
	Flags      appenv.Flags
	Logf       func(format string, args ...any)
}

// Result is the answer: the launch() arguments and the lines Python prints.
// Options holds secrets (auth, ssl_keyfile_password); Lines never does.
type Result struct {
	Options map[string]any `json:"options"`
	Lines   []string       `json:"lines"`
}

// Server serves GET Path.
type Server struct{ opts Options }

func New(o Options) *Server {
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	return &Server{opts: o}
}

// Compute is the final set of launch options, from config.json now.
//
//   - auth is null or a list of [user, password] pairs (a string "user:password"
//     is split on ":", as upgrade_gradio_options did; Python turns the pairs
//     into tuples, which JSON cannot carry);
//   - the legacy keys are dropped; server_name and server_port are the private
//     Gradio address; inbrowser is false (the launcher opens the browser);
//   - --share forces share; allowed_paths, favicon_path and prevent_thread_lock
//     are fixed.
func (s *Server) Compute() (Result, error) {
	data, err := s.opts.Config.Load()
	if err != nil {
		return Result{}, err
	}
	// The same checks the launcher made before it started the app.
	cfg, err := appenv.ParseConfig(data)
	if err != nil {
		return Result{}, err
	}
	var doc struct {
		Options map[string]any `json:"gradio_interface_options"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return Result{}, err
	}

	opts := doc.Options
	for _, k := range droppedOptions {
		delete(opts, k)
	}
	// Gradio is private, behind the router, and speaks plain HTTP there: the
	// router serves the certificate (ssl_keyfile, ssl_certfile and
	// ssl_keyfile_password are the router's, see launcher.startServerPyOn).
	for _, k := range []string{"ssl_keyfile", "ssl_certfile", "ssl_keyfile_password"} {
		delete(opts, k)
	}
	opts["auth"] = authPairs(cfg.Auth)
	opts["server_name"] = s.opts.GradioHost
	opts["server_port"] = s.opts.GradioPort
	opts["inbrowser"] = false
	if s.opts.Flags.Share {
		opts["share"] = true
	}
	opts["allowed_paths"] = []any{"."}
	opts["favicon_path"] = faviconPath
	opts["prevent_thread_lock"] = true

	var lines []string
	if cfg.Auth.Enabled() {
		lines = append(lines, "Gradio server authentication enabled")
	}
	if s.opts.Flags.Share {
		lines = append(lines, "Gradio share mode enabled")
	}
	if cfg.SSL.Enabled() {
		lines = append(lines, "HTTPS enabled on the public port (certificate "+cfg.SSL.CertFile+")")
	}
	return Result{Options: opts, Lines: append(lines, describe(opts)...)}, nil
}

func authPairs(a appenv.Auth) any {
	if !a.Enabled() {
		return nil
	}
	pairs := make([]any, len(a.Credentials))
	for i, c := range a.Credentials {
		pairs[i] = []any{c.User, c.Password}
	}
	return pairs
}

// Handler serves GET Path.
func (s *Server) Handler() http.Handler { return http.HandlerFunc(s.serve) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) authorized(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() || s.opts.Token == "" {
		return false
	}
	got := sha256.Sum256([]byte(r.Header.Get(TokenHeader)))
	want := sha256.Sum256([]byte(s.opts.Token))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case !s.authorized(r):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case r.URL.Path != Path:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case r.Method != http.MethodGet:
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	default:
		res, err := s.Compute()
		if err != nil {
			// The message names keys, never values: ParseConfig errors do not carry them.
			s.opts.Logf("launch options: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
