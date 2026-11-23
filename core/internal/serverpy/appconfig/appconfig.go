// Package appconfig makes Go the only reader and writer of config.json when
// the server.py port is on (TTS_GO_SERVERPY=1). Python loads and saves the
// config through the control API served by [Store.Handler].
package appconfig

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
)

const (
	// TokenHeader carries the shared secret (same as router and extprocs).
	TokenHeader = "X-TTS-Core-Token"
	// Path is the single path of the API.
	Path = "/api/config"

	maxBody = 8 << 20
)

// defaultDoc is default_config in tts_webui/config/load_config.py, key for key.
const defaultDoc = `{
  "gradio_interface_options": {
    "inline": false,
    "inbrowser": true,
    "share": false,
    "debug": false,
    "max_threads": 40,
    "auth": null,
    "auth_message": null,
    "prevent_thread_lock": false,
    "show_error": false,
    "server_name": "127.0.0.1",
    "server_port": 7770,
    "height": 500,
    "width": "100%",
    "favicon_path": null,
    "ssl_keyfile": null,
    "ssl_certfile": null,
    "ssl_keyfile_password": null,
    "ssl_verify": true,
    "quiet": true,
    "show_api": true,
    "_frontend": true
  },
  "extensions": {
    "disabled": [],
    "disabled_decorators": []
  }
}`

// Options configures a Store.
type Options struct {
	Root  string // directory holding config.json
	Token string // required; requests without it get 403
	Logf  func(format string, args ...any)
}

// Store owns root/config.json.
type Store struct {
	opts Options
	path string
	mu   sync.Mutex
}

func New(o Options) *Store {
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	return &Store{opts: o, path: filepath.Join(o.Root, "config.json")}
}

// Handler serves GET and PUT on /api/config.
func (s *Store) Handler() http.Handler { return http.HandlerFunc(s.serve) }

// Load returns the current document, creating the default file when missing
// (load_config.py). The bytes are exactly the file's.
func (s *Store) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() ([]byte, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		v, _ := parse([]byte(defaultDoc))
		data = marshal(v)
		s.opts.Logf("config: config.json not found, creating the default config")
		if err := s.writeLocked(data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return data, err
}

// validate checks what Save checks about a whole document.
func validate(v any) (*object, error) {
	obj, ok := v.(*object)
	if !ok {
		return nil, &invalidError{"config must be a JSON object"}
	}
	if g, ok := obj.get("gradio_interface_options"); ok {
		if gobj, ok := g.(*object); ok {
			if p, ok := gobj.get("server_port"); ok {
				n, isNum := p.(json.Number)
				if !isNum {
					return nil, &invalidError{"gradio_interface_options.server_port must be an integer"}
				}
				if _, err := n.Int64(); err != nil {
					return nil, &invalidError{"gradio_interface_options.server_port must be an integer"}
				}
			}
		}
	}
	return obj, nil
}

// Save validates and writes doc (a JSON object) as json.dump(indent=2) would.
func (s *Store) Save(doc []byte) error {
	v, err := parse(doc)
	if err != nil {
		return &invalidError{fmt.Sprintf("invalid JSON: %v", err)}
	}
	obj, err := validate(v)
	if err != nil {
		return err
	}
	out := marshal(obj)

	s.mu.Lock()
	defer s.mu.Unlock()
	old, _ := os.ReadFile(s.path)
	if err := s.writeLocked(out); err != nil {
		return err
	}
	s.logChanges(old, out)
	return nil
}

// update is the one read-modify-write: under the mutex it loads the document
// (creating the default when missing), lets fn change it, validates, and
// writes it only when fn reports a change. Every operation in ops.go goes
// through here, so concurrent changes cannot lose each other.
func (s *Store) update(fn func(doc *object) (changed bool, err error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.loadLocked()
	if err != nil {
		return err
	}
	v, err := parse(old)
	if err != nil {
		return fmt.Errorf("config.json is not valid JSON: %w", err)
	}
	doc, ok := v.(*object)
	if !ok {
		return &invalidError{"config.json is not a JSON object"}
	}
	changed, err := fn(doc)
	if err != nil || !changed {
		return err
	}
	if _, err := validate(doc); err != nil {
		return err
	}
	out := marshal(doc)
	if err := s.writeLocked(out); err != nil {
		return err
	}
	s.logChanges(old, out)
	return nil
}

type invalidError struct{ msg string }

func (e *invalidError) Error() string { return e.msg }

// IsInvalid reports whether err is a refusal of bad input (as opposed to an I/O failure).
func IsInvalid(err error) bool {
	var inv *invalidError
	return errors.As(err, &inv)
}

func (s *Store) writeLocked(data []byte) error {
	tmp, err := os.CreateTemp(s.opts.Root, ".config.json.*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, s.path)
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}

// launchOpts returns the launch-time options of a document as JSON text.
func launchOpts(b []byte) map[string]string {
	m := map[string]string{}
	v, err := parse(b)
	if err != nil {
		return m
	}
	o, _ := v.(*object)
	if o == nil {
		return m
	}
	x, _ := o.get("gradio_interface_options")
	g, _ := x.(*object)
	if g == nil {
		return m
	}
	for _, it := range g.Items {
		if launchKey(it.Key) {
			m[it.Key] = string(marshal(it.Val))
		}
	}
	return m
}

// logChanges logs, at INFO, which launch-time keys changed. Values are never
// logged (auth and ssl_keyfile_password are secrets).
func (s *Store) logChanges(old, cur []byte) {
	a, b := launchOpts(old), launchOpts(cur)
	var changed []string
	for k, v := range b {
		if av, ok := a[k]; !ok || av != v {
			changed = append(changed, k)
		}
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			changed = append(changed, k)
		}
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		s.opts.Logf("config: launch-time settings changed (%s); they take effect after a restart", strings.Join(changed, ", "))
	}
	if _, err := appenv.ParseConfig(cur); err != nil {
		s.opts.Logf("config: warning: server.py would not start with this config: %v", err)
	}
}

func launchKey(k string) bool {
	switch k {
	case "server_port", "server_name", "auth", "share":
		return true
	}
	return strings.HasPrefix(k, "ssl_")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
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
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.URL.Path != Path {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		data, err := s.Load()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	case http.MethodPut:
		if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
			writeErr(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		if err != nil {
			writeErr(w, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		if err := s.Save(body); err != nil {
			code := http.StatusInternalServerError
			var inv *invalidError
			if errors.As(err, &inv) {
				code = http.StatusBadRequest
			}
			writeErr(w, code, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
