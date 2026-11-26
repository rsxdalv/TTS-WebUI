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
	"os"
	"path/filepath"

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
	// Root is the install root, which is also Gradio's working directory.
	// blocked_paths must be absolute (see blockedPaths), so it is needed.
	Root string
	// Blocked are more paths Gradio must not serve, which the caller names
	// because it owns them (the installer's folder, from the paths package).
	Blocked []string
	Flags   appenv.Flags
	Logf    func(format string, args ...any)
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
//     are fixed;
//   - blocked_paths is blockedPaths plus what the file lists, as absolute paths.
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
	opts["blocked_paths"] = s.blocked(opts["blocked_paths"])
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

// blockedPaths are what Gradio's file route must never serve. allowed_paths is
// "." (an extension may serve a file from any folder of the install), and the
// install root also holds the credentials: config.json has the Gradio auth
// password, .env* and env_store.json the provider keys, platforms/openai/
// keys.json the endpoint keys, data/sqlite the database, .git the history and
// the remote's credentials. The installer's folder (its profile holds tokens)
// is named by the caller, in Options.Blocked. Gradio checks blocked_paths before allowed_paths, so
// listing them here keeps "." from reaching them.
//
// They are made absolute (blocked): Gradio lowercases both sides before it
// compares, so a relative entry is resolved against the real-case working
// directory and never matches the lowercased request path when that directory
// has an upper-case letter in it (/home/me/TTS-WebUI). Tried with Gradio
// 5.49.1: relative entries blocked nothing, absolute ones do.
var blockedPaths = []string{
	"config.json",
	".env", ".env.user", ".env.local", ".env.production", ".env.production.local",
	"env_store.json",
	"platforms",
	"installer_scripts/output.log",
	"data/sqlite",
	".git",
}

// blocked is blockedPaths and Options.Blocked followed by the entries the file adds, all absolute:
// the file may block more, never less.
func (s *Server) blocked(fromFile any) []any {
	root := s.opts.Root
	if root == "" {
		root, _ = os.Getwd()
	}
	var out []any
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, filepath.FromSlash(p))
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range blockedPaths {
		add(p)
	}
	for _, p := range s.opts.Blocked {
		add(p)
	}
	if list, ok := fromFile.([]any); ok {
		for _, v := range list {
			if p, ok := v.(string); ok {
				add(p)
			}
		}
	}
	return out
}
