// Package piplist is the Go side of Settings > Installed Packages
// (extensions/builtin/extension_pip_list). Python's Gradio handlers call it
// through tts_webui/core_bridge (bridge.pip) when TTS_CORE_URL is set. Go runs
// `python -m pip list --format=json` in the main env and answers the list.
package piplist

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"net"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// TokenHeader carries the shared secret (same as the other control APIs).
	TokenHeader = "X-TTS-Core-Token"
	// Path is the single endpoint: GET returns [{name, version}].
	Path = "/api/pip/list"

	maxOutput = 16 << 20
)

// Options configures a Store.
type Options struct {
	Root  string // working directory of the command
	Token string // required; requests without it get 403
	// Platform runs pip: the main env's python.
	Platform platform.Platform
	Timeout  time.Duration // default 2 minutes
	Logf     func(format string, args ...any)
}

// Package is one installed distribution.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// cacheTTL is how long Cached keeps a pip answer. Installs through extcatalog
// call Invalidate, so this only bounds changes made outside the app.
const cacheTTL = 30 * time.Second

// Store serves the API for one project root.
type Store struct {
	opts Options
	mu   sync.Mutex // one pip at a time

	cacheMu sync.Mutex
	cache   []Package
	cacheAt time.Time
}

func New(o Options) *Store {
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	if o.Timeout == 0 {
		o.Timeout = 2 * time.Minute
	}
	return &Store{opts: o}
}

// command is the pip invocation, resolved by the platform.
func (s *Store) command() (platform.Command, error) {
	return s.opts.Platform.Command(nil, []string{"python", "-m", "pip", "list", "--format=json", "--disable-pip-version-check"})
}

// List runs pip and returns the packages sorted case-insensitively by name
// (then version). A failing pip is an error carrying the first lines of its
// stderr; nothing is cached.
func (s *Store) List(ctx context.Context) ([]Package, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, s.opts.Timeout)
	defer cancel()
	if !s.opts.Platform.Status(ctx).Ready {
		return nil, ErrNoPython
	}
	pc, err := s.command()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, pc.Program, pc.Args...)
	cmd.Dir = s.opts.Root
	cmd.Env = pc.Env
	var stdout, stderr limited
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("pip list timed out after %s", s.opts.Timeout)
		}
		return nil, fmt.Errorf("pip list failed: %v: %s", err, firstLines(stderr.String(), 3))
	}
	if stdout.over {
		return nil, errors.New("pip list: output too large")
	}
	return parse(stdout.Bytes())
}

// Cached is List with a short memory, for callers that ask many times in a row
// (the extension lists ask once per extension class at startup, and a pip list
// takes seconds). The slice is shared: callers must not change it. Invalidate
// drops it, and a failed List is never remembered.
func (s *Store) Cached(ctx context.Context) ([]Package, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.cache != nil && time.Since(s.cacheAt) < cacheTTL {
		return s.cache, nil
	}
	pkgs, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	s.cache, s.cacheAt = pkgs, time.Now()
	return pkgs, nil
}

// Invalidate forgets the cached list, after something installed or removed a package.
func (s *Store) Invalidate() {
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheMu.Unlock()
}

// parse reads pip's JSON (`[{"name": ..., "version": ...}, ...]`).
func parse(b []byte) ([]Package, error) {
	var raw []Package
	// Some setups print a warning before the JSON on stdout: start at '['.
	if i := bytes.IndexByte(b, '['); i > 0 {
		b = b[i:]
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("pip list: unreadable output: %v", err)
	}
	out := make([]Package, 0, len(raw))
	for _, p := range raw {
		if p.Name != "" {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Version < out[j].Version
	})
	return out, nil
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	out := strings.Join(lines, " | ")
	if len(out) > 300 {
		out = out[:300]
	}
	if strings.TrimSpace(out) == "" {
		return "(no stderr)"
	}
	return out
}

// limited is a buffer that stops growing at maxOutput.
type limited struct {
	bytes.Buffer
	over bool
}

func (l *limited) Write(p []byte) (int, error) {
	if l.Len()+len(p) > maxOutput {
		l.over = true
		return len(p), nil
	}
	return l.Buffer.Write(p)
}

// Handler serves GET Path.
func (s *Store) Handler() http.Handler { return http.HandlerFunc(s.serve) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Store) authorized(r *http.Request) bool {
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

func (s *Store) serve(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if r.URL.Path != Path {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	list, err := s.List(r.Context())
	if err != nil {
		if errors.Is(err, ErrNoPython) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "code": "no_python"})
			return
		}
		s.opts.Logf("piplist: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// ErrNoPython is the answer when there is no Python to ask (no environment
// installed yet, or no python on PATH). It is a state, not a failure: the
// core runs without Python.
var ErrNoPython = errors.New("no Python environment")
