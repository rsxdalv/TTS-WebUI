// Package openai is the openai platform (P5, D27): named remote endpoints that
// speak the OpenAI audio API (POST <base>/audio/speech). Nothing is installed
// and no process runs; Go calls the endpoint and saves the clip in outputs/ in
// the layout the History page reads.
//
// Endpoints live in platforms/openai/endpoints.json. The keys are in a second
// file, platforms/openai/keys.json, which only this package reads and writes:
// no answer of this package carries a key, an error text has it removed, and
// nothing logs it.
package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Name is the platform's name.
const Name = "openai"

// Endpoint is what the browser may see of an endpoint: never the key.
type Endpoint struct {
	Name string `json:"name"`
	// URL is the base URL, normally ending in /v1: the speech route is
	// <URL>/audio/speech.
	URL   string `json:"url"`
	Model string `json:"model,omitempty"`
	Voice string `json:"voice,omitempty"`
	// HasKey says a key is stored; its value is never sent.
	HasKey bool `json:"has_key"`
}

// ErrInvalid is wrapped by every error that is the caller's input.
var ErrInvalid = errors.New("invalid")

// ErrNotFound is wrapped when a named endpoint does not exist.
var ErrNotFound = errors.New("not found")

func invalidf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$`)

// Store keeps the endpoints and their keys for one install folder.
type Store struct {
	mu  sync.Mutex
	dir string
}

// NewStore returns the store of the install folder root.
func NewStore(root string) *Store {
	return &Store{dir: filepath.Join(root, "platforms", Name)}
}

// Dir is platforms/openai.
func (s *Store) Dir() string { return s.dir }

type endpointFile struct {
	Endpoints []stored `json:"endpoints"`
}

type stored struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Model string `json:"model,omitempty"`
	Voice string `json:"voice,omitempty"`
}

func (s *Store) endpointsPath() string { return filepath.Join(s.dir, "endpoints.json") }
func (s *Store) keysPath() string      { return filepath.Join(s.dir, "keys.json") }

func (s *Store) load() ([]stored, map[string]string, error) {
	var ef endpointFile
	if err := readJSON(s.endpointsPath(), &ef); err != nil {
		return nil, nil, err
	}
	keys := map[string]string{}
	if err := readJSON(s.keysPath(), &keys); err != nil {
		return nil, nil, err
	}
	return ef.Endpoints, keys, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", filepath.Base(path), err)
	}
	return nil
}

// writeFile replaces path atomically with mode 0600.
func writeFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Store) save(eps []stored, keys map[string]string) error {
	sort.Slice(eps, func(i, j int) bool { return eps[i].Name < eps[j].Name })
	// Keys first: a crash between the two files leaves a key without an
	// endpoint (unused), never an endpoint that is missing its key.
	if err := writeFile(s.keysPath(), keys); err != nil {
		return err
	}
	return writeFile(s.endpointsPath(), endpointFile{Endpoints: eps})
}

// List returns the endpoints sorted by name, with no keys.
func (s *Store) List() ([]Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	eps, keys, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]Endpoint, 0, len(eps))
	for _, e := range eps {
		out = append(out, Endpoint{Name: e.Name, URL: e.URL, Model: e.Model, Voice: e.Voice, HasKey: keys[e.Name] != ""})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// get returns an endpoint and its key.
func (s *Store) get(name string) (Endpoint, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	eps, keys, err := s.load()
	if err != nil {
		return Endpoint{}, "", err
	}
	for _, e := range eps {
		if e.Name == name {
			k := keys[name]
			return Endpoint{Name: e.Name, URL: e.URL, Model: e.Model, Voice: e.Voice, HasKey: k != ""}, k, nil
		}
	}
	return Endpoint{}, "", fmt.Errorf("%w: no endpoint named %q", ErrNotFound, name)
}

// Put creates or replaces an endpoint. key nil keeps the stored key, an empty
// key removes it. The returned Endpoint is what List shows.
func (s *Store) Put(e Endpoint, key *string) (Endpoint, error) {
	if !nameRe.MatchString(e.Name) {
		return Endpoint{}, invalidf("the name must be 1 to 40 letters, digits, - or _")
	}
	u, err := CleanURL(e.URL)
	if err != nil {
		return Endpoint{}, err
	}
	e.URL = u
	if len(e.Model) > 200 || len(e.Voice) > 200 {
		return Endpoint{}, invalidf("model and voice are at most 200 characters")
	}
	if key != nil && (len(*key) > 4096 || strings.ContainsAny(*key, "\r\n\x00")) {
		return Endpoint{}, invalidf("the key is not a single line of at most 4096 characters")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	eps, keys, err := s.load()
	if err != nil {
		return Endpoint{}, err
	}
	if key != nil {
		if *key == "" {
			delete(keys, e.Name)
		} else {
			keys[e.Name] = *key
		}
	}
	if err := checkTransport(u, keys[e.Name] != ""); err != nil {
		return Endpoint{}, err
	}
	rec := stored{Name: e.Name, URL: e.URL, Model: strings.TrimSpace(e.Model), Voice: strings.TrimSpace(e.Voice)}
	replaced := false
	for i := range eps {
		if eps[i].Name == e.Name {
			eps[i], replaced = rec, true
		}
	}
	if !replaced {
		eps = append(eps, rec)
	}
	if err := s.save(eps, keys); err != nil {
		return Endpoint{}, err
	}
	rec2 := Endpoint{Name: rec.Name, URL: rec.URL, Model: rec.Model, Voice: rec.Voice, HasKey: keys[e.Name] != ""}
	return rec2, nil
}

// Delete removes an endpoint and its key.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	eps, keys, err := s.load()
	if err != nil {
		return err
	}
	kept := eps[:0:0]
	found := false
	for _, e := range eps {
		if e.Name == name {
			found = true
			continue
		}
		kept = append(kept, e)
	}
	if !found {
		return fmt.Errorf("%w: no endpoint named %q", ErrNotFound, name)
	}
	delete(keys, name)
	return s.save(kept, keys)
}

// CleanURL checks a base URL and returns it without a trailing slash.
func CleanURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", invalidf("the URL must start with http:// or https:// and name a host")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", invalidf("the URL must not have a user name, query or fragment (the key is a separate field)")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u.String(), nil
}

// checkTransport refuses to send a key in clear text over the internet: plain
// http is accepted for this machine and for private addresses, https always.
func checkTransport(rawURL string, hasKey bool) error {
	if !hasKey {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return invalidf("bad URL")
	}
	if u.Scheme == "https" || privateHost(u.Hostname()) {
		return nil
	}
	return invalidf("a key is sent with every request: use https:// for %s (plain http is accepted only for this machine and private network addresses)", u.Hostname())
}

func privateHost(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

// ValidName reports whether s is acceptable as an endpoint name.
func ValidName(s string) bool { return nameRe.MatchString(s) }
