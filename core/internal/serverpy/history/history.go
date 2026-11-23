// Package history is the Go side of the Gallery History tab. Python's Gradio
// handlers (extensions/builtin/extension_gallery_history) call it through
// tts_webui/core_bridge (bridge.history) when TTS_CORE_URL is set; Go does all the
// filesystem work so the path checks live in one place.
package history

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	// TokenHeader carries the shared secret (same as appconfig, router, extprocs).
	TokenHeader = "X-TTS-Core-Token"
	// Prefix is the path prefix of the API.
	Prefix = "/api/history/"

	maxBody = 1 << 20
)

// Options configures a Store.
type Options struct {
	Root  string // project root: holds outputs/, favorites/, collections/
	Token string // required; requests without it get 403
	Logf  func(format string, args ...any)
}

// Store serves the history API for one project root.
type Store struct {
	opts Options
	mu   sync.Mutex // serialises writes
}

func New(o Options) *Store {
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	return &Store{opts: o}
}

// Collections lists "outputs", "favorites", then collections/<name> sorted,
// like get_collections() in tts_webui/history_tab/collections_directories_atom.py.
func (s *Store) Collections() ([]string, error) {
	out := []string{"outputs", "favorites"}
	ents, err := os.ReadDir(filepath.Join(s.opts.Root, "collections"))
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() && validSegment(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, "collections/"+n)
	}
	return out, nil
}

// Items lists the .png files under a collection that have a sibling .wav, as
// slash-separated paths relative to the project root, sorted. Hidden entries
// and symlinks are skipped.
func (s *Store) Items(collection string) ([]string, error) {
	abs, canon, err := s.resolveCollection(collection)
	if err != nil {
		return nil, err
	}
	items := []string{}
	err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable subtree: skip, like glob
		}
		if p != abs && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.EqualFold(filepath.Ext(p), ".png") {
			return nil
		}
		wav := strings.TrimSuffix(p, filepath.Ext(p)) + ".wav"
		if fi, err := os.Lstat(wav); err != nil || !fi.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(abs, p)
		if err != nil {
			return nil
		}
		items = append(items, canon+"/"+filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(items)
	return items, nil
}

// ItemJSON returns the metadata file next to an item, given its .wav (or .png)
// path: the same stem with .json.
//
// A generation without metadata (no file, a file that is not JSON, a file that
// is too big) has no metadata: the answer is nil and the tab shows an empty
// component. The old Python tab did the same. A path that leaves the
// collection is still an error.
func (s *Store) ItemJSON(path string) ([]byte, error) {
	p := strings.ReplaceAll(path, "\\", "/")
	i := strings.LastIndexByte(p, '.')
	if i < 0 {
		return nil, invalidf("expected a .wav or .png path")
	}
	switch strings.ToLower(p[i:]) {
	case ".wav", ".png":
	default:
		return nil, invalidf("expected a .wav or .png path")
	}
	abs, _, _, err := s.resolve(p[:i] + ".json")
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil, nil
		}
		return nil, err
	}
	fi, err := os.Lstat(abs)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, nil
	}
	if fi.Size() > 32<<20 {
		return nil, nil
	}
	b, err := os.ReadFile(abs)
	if err != nil || !json.Valid(b) {
		return nil, nil
	}
	return b, nil
}

// CreateCollection makes collections/<name>. It fails if it exists.
func (s *Store) CreateCollection(name string) error {
	if !validSegment(name) {
		return invalidf("bad collection name %q", name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rootReal, err := filepath.EvalSymlinks(s.opts.Root)
	if err != nil {
		return err
	}
	parent := filepath.Join(rootReal, "collections")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(parent, name), 0o755); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: collection %q", errExists, name)
		}
		return err
	}
	return nil
}

// Delete removes one generation directory.
func (s *Store) Delete(dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	abs, _, err := s.resolveGeneration(dir)
	if err != nil {
		return err
	}
	s.opts.Logf("history: delete %s", dir)
	return os.RemoveAll(abs)
}

// Copy copies a generation directory into a collection root ("favorites",
// "outputs" or "collections/<name>"). It never overwrites and refuses
// symlinks. It returns the new path.
func (s *Store) Copy(dir, to string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, segs, err := s.resolveGeneration(dir)
	if err != nil {
		return "", err
	}
	// server.py creates outputs/ and favorites/ at startup; do the same here
	// so a fresh checkout can still save its first favorite.
	if t := strings.Trim(strings.ReplaceAll(to, "\\", "/"), "/"); t == "outputs" || t == "favorites" {
		if err := os.MkdirAll(filepath.Join(s.opts.Root, t), 0o755); err != nil {
			return "", err
		}
	}
	dstRoot, canon, err := s.resolveCollection(to)
	if err != nil {
		return "", err
	}
	base := segs[len(segs)-1]
	dst := filepath.Join(dstRoot, base)
	if _, err := os.Lstat(dst); err == nil {
		return "", fmt.Errorf("%w: %s/%s already exists", errExists, canon, base)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	tmp, err := os.MkdirTemp(dstRoot, ".copy-*")
	if err != nil {
		return "", err
	}
	if err := copyTree(src, filepath.Join(tmp, base)); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	if err := os.Rename(filepath.Join(tmp, base), dst); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	_ = os.Remove(tmp)
	return canon + "/" + base, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			return copyFile(p, target)
		default:
			return invalidf("refusing to copy %q: not a regular file or directory", rel)
		}
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// Handler serves the API under Prefix.
func (s *Store) Handler() http.Handler { return http.HandlerFunc(s.serve) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Store) fail(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, errInvalid):
		code = http.StatusBadRequest
	case errors.Is(err, errNotFound):
		code = http.StatusNotFound
	case errors.Is(err, errExists):
		code = http.StatusConflict
	}
	if code == http.StatusInternalServerError {
		s.opts.Logf("history: %v", err)
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
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

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return invalidf("Content-Type must be application/json")
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return invalidf("bad JSON body: %v", err)
	}
	return nil
}

func (s *Store) serve(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	op := strings.TrimPrefix(r.URL.Path, Prefix)
	want := http.MethodPost
	switch op {
	case "collections", "items", "select", "rows":
		want = http.MethodGet
	case "create-collection", "delete", "copy", "save-to-voices":
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != want {
		w.Header().Set("Allow", want)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	switch op {
	case "collections":
		c, err := s.Collections()
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, c)
	case "items":
		it, err := s.Items(r.URL.Query().Get("collection"))
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, it)
	case "select":
		ui, err := s.Select(r.URL.Query().Get("path"))
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, ui)
	case "rows":
		rows, err := s.Rows(r.URL.Query().Get("directory"))
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, rows)
	case "create-collection":
		var in struct {
			Name string `json:"name"`
		}
		if err := decode(w, r, &in); err != nil {
			s.fail(w, err)
			return
		}
		if err := s.CreateCollection(in.Name); err != nil {
			s.fail(w, err)
			return
		}
		c, err := s.Collections()
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"collections": c, "ui": mk("Button", map[string]any{"value": "Created"})})
	case "delete":
		var in struct {
			Directory string `json:"directory"`
		}
		if err := decode(w, r, &in); err != nil {
			s.fail(w, err)
			return
		}
		if err := s.Delete(in.Directory); err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	case "copy":
		var in struct {
			Directory string `json:"directory"`
			To        string `json:"to"`
		}
		if err := decode(w, r, &in); err != nil {
			s.fail(w, err)
			return
		}
		p, err := s.Copy(in.Directory, in.To)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, savedUI(p))
	case "save-to-voices":
		var in struct {
			Npz string `json:"npz"`
		}
		if err := decode(w, r, &in); err != nil {
			s.fail(w, err)
			return
		}
		if err := s.SaveToVoices(in.Npz); err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, mk("Button", map[string]any{"value": "Saved"}))
	}
}

// savedUI is the Gradio update the tab shows after a copy: the "Saved" Button
// for favorites, the "Saved" Dropdown for any other collection. It is a
// marker the Python bridge turns into the component (see
// tts_webui/core_bridge/gradio_resolve.py).
func savedUI(newPath string) map[string]any {
	kind := "Dropdown"
	if strings.HasPrefix(newPath, "favorites/") {
		kind = "Button"
	}
	return map[string]any{"__gradio__": kind, "props": map[string]any{"value": "Saved"}}
}
