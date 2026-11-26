// Package minimax is MiniMax Cloud TTS in Go: one HTTPS call to MiniMax's
// synchronous text-to-speech API (POST <base>/v1/t2a_v2), the clip saved in
// outputs/ in the layout the History page reads. It replaces what the Python
// extension extensions/builtin/extension_minimax_cloud_tts does for the Vite
// page, so the page works without the Gradio host; the Gradio tab is untouched.
//
// It reuses the rules of platform/openai (P5): the key is kept by Go in a file
// the browser routes never read, no answer carries it, an error text has it
// removed, nothing logs it, and settings change from this machine only.
//
// Settings live in platforms/minimax/settings.json, the key in
// platforms/minimax/key.json (mode 0600). The key can also come from the
// MINIMAX_API_KEY environment variable, as in the Python extension; a stored
// key wins.
package minimax

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
)

// Name is the folder name under platforms/ and the route name.
const Name = "minimax"

// Defaults are the values of the Python extension (main.py).
const (
	DefaultBaseURL = "https://api.minimax.io"
	DefaultModel   = "speech-2.8-hd"
	DefaultVoice   = "English_Graceful_Lady"
	// EnvKey is the environment variable the Python extension also reads.
	EnvKey = "MINIMAX_API_KEY"
)

// Settings is what the browser may see: never the key.
type Settings struct {
	// BaseURL is the API host; the call goes to <BaseURL>/v1/t2a_v2.
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	Voice   string `json:"voice"`
	// HasKey says a key is available; KeySource says where from: "stored",
	// "env" (MINIMAX_API_KEY) or "none". The value is never sent.
	HasKey    bool   `json:"has_key"`
	KeySource string `json:"key_source"`
}

// SettingsIn is a change. Empty BaseURL, Model or Voice go back to the default.
// Key nil keeps the stored key, "" removes it.
type SettingsIn struct {
	BaseURL string  `json:"base_url"`
	Model   string  `json:"model"`
	Voice   string  `json:"voice"`
	Key     *string `json:"key"`
}

type settingsFile struct {
	BaseURL string `json:"base_url,omitempty"`
	Model   string `json:"model,omitempty"`
	Voice   string `json:"voice,omitempty"`
}

type keyFile struct {
	Key string `json:"key,omitempty"`
}

// Store keeps the settings and the key for one install folder.
type Store struct {
	mu     sync.Mutex
	dir    string
	getenv func(string) string
}

// NewStore returns the store of the install folder root. getenv nil reads the
// process environment.
func NewStore(root string, getenv func(string) string) *Store {
	if getenv == nil {
		getenv = os.Getenv
	}
	return &Store{dir: filepath.Join(root, "platforms", Name), getenv: getenv}
}

func (s *Store) settingsPath() string { return filepath.Join(s.dir, "settings.json") }
func (s *Store) keyPath() string      { return filepath.Join(s.dir, "key.json") }

// load returns the settings with defaults filled and the effective key.
func (s *Store) load() (Settings, string, error) {
	var sf settingsFile
	if err := openai.ReadJSON(s.settingsPath(), &sf); err != nil {
		return Settings{}, "", err
	}
	var kf keyFile
	if err := openai.ReadJSON(s.keyPath(), &kf); err != nil {
		return Settings{}, "", err
	}
	set := Settings{BaseURL: first(sf.BaseURL, DefaultBaseURL), Model: first(sf.Model, DefaultModel), Voice: first(sf.Voice, DefaultVoice)}
	key := kf.Key
	switch {
	case key != "":
		set.KeySource = "stored"
	case strings.TrimSpace(s.getenv(EnvKey)) != "":
		key, set.KeySource = strings.TrimSpace(s.getenv(EnvKey)), "env"
	default:
		set.KeySource = "none"
	}
	set.HasKey = key != ""
	return set, key, nil
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// Get returns the settings, with no key.
func (s *Store) Get() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, _, err := s.load()
	return set, err
}

// credentials returns the settings and the key to call with.
func (s *Store) credentials() (Settings, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// Put replaces the settings and, when in.Key is set, the stored key.
func (s *Store) Put(in SettingsIn) (Settings, error) {
	in.Model, in.Voice = strings.TrimSpace(in.Model), strings.TrimSpace(in.Voice)
	if len(in.Model) > 200 || len(in.Voice) > 200 {
		return Settings{}, openai.Invalidf("model and voice are at most 200 characters")
	}
	if strings.ContainsAny(in.Model+in.Voice, "\r\n\x00") {
		return Settings{}, openai.Invalidf("model and voice are single lines")
	}
	base := ""
	if strings.TrimSpace(in.BaseURL) != "" {
		u, err := CleanBaseURL(in.BaseURL)
		if err != nil {
			return Settings{}, err
		}
		base = u
	}
	if in.Key != nil && (len(*in.Key) > 4096 || strings.ContainsAny(*in.Key, "\r\n\x00")) {
		return Settings{}, openai.Invalidf("the key is not a single line of at most 4096 characters")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var kf keyFile
	if err := openai.ReadJSON(s.keyPath(), &kf); err != nil {
		return Settings{}, err
	}
	if in.Key != nil {
		kf.Key = strings.TrimSpace(*in.Key)
	}
	// A key sent to MiniMax must not travel in clear text over the internet; the
	// MINIMAX_API_KEY key is checked too, since it is sent with every call.
	hasKey := kf.Key != "" || strings.TrimSpace(s.getenv(EnvKey)) != ""
	if err := openai.CheckTransport(first(base, DefaultBaseURL), hasKey); err != nil {
		return Settings{}, err
	}
	key := kf.Key
	if err := openai.WriteFileAtomic(s.keyPath(), keyFile{Key: key}); err != nil {
		return Settings{}, err
	}
	if err := openai.WriteFileAtomic(s.settingsPath(), settingsFile{BaseURL: base, Model: in.Model, Voice: in.Voice}); err != nil {
		return Settings{}, err
	}
	set, _, err := s.load()
	return set, err
}

// CleanBaseURL checks a host URL and returns it without a trailing slash. A
// path is allowed (a gateway in front of the API); a query or user is not.
func CleanBaseURL(raw string) (string, error) {
	u, err := openai.CleanURL(raw)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(u, "/v1") {
		return "", openai.Invalidf("the URL is the host (https://api.minimax.io): the call adds /v1/t2a_v2 itself")
	}
	return u, nil
}

// Host is the host part of a base URL, for messages.
func Host(base string) string {
	if u, err := url.Parse(base); err == nil && u.Host != "" {
		return u.Host
	}
	return base
}

const noKeyMsg = "no MiniMax API key: save one on this page, or set " + EnvKey + " before TTS WebUI starts"

func noKeyError() error { return openai.Invalidf("%s", noKeyMsg) }
