package appenv

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
)

const (
	// TokenHeader carries the shared secret (same as appconfig, extcatalog, ...).
	TokenHeader = "X-TTS-Core-Token"
	// APIPrefix is the path prefix of the env API.
	APIPrefix = "/api/env/"

	maxBody = 64 << 10
)

// APIOptions configures an API.
type APIOptions struct {
	Root  string // project root, holds .env
	Token string // required; requests without it get 403
	Logf  func(format string, args ...any)
}

// API is the control API of the env files: the only way Python changes .env.
//
//	POST /api/env/save-model-locations
//	     {huggingface_hub_cache, hf_home, torch_home, xdg_cache_home} -> "saved"
//
// All four fields are required strings; unknown fields, other types and
// non-objects are refused. Values are never logged (they can be paths with
// user names or, for other variables later, secrets).
type API struct{ opts APIOptions }

// NewAPI returns the env API for one project root.
func NewAPI(o APIOptions) *API {
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	return &API{opts: o}
}

// Handler serves the API under APIPrefix.
func (a *API) Handler() http.Handler { return http.HandlerFunc(a.serve) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (a *API) authorized(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() || a.opts.Token == "" {
		return false
	}
	got := sha256.Sum256([]byte(r.Header.Get(TokenHeader)))
	want := sha256.Sum256([]byte(a.opts.Token))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func (a *API) serve(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	if strings.TrimPrefix(r.URL.Path, APIPrefix) == "model-locations" && r.Method == http.MethodGet {
		a.getModelLocations(w)
		return
	}
	if strings.TrimPrefix(r.URL.Path, APIPrefix) != "save-model-locations" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeErr(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	m, err := parseModelLocations(http.MaxBytesReader(w, r.Body, maxBody))
	if err == nil {
		err = SaveModelLocations(a.opts.Root, m)
	}
	if err != nil {
		code := http.StatusBadRequest
		if !errors.Is(err, errInvalid) {
			code = http.StatusInternalServerError
			a.opts.Logf("env: save-model-locations: %v", err)
		}
		writeErr(w, code, err.Error())
		return
	}
	a.opts.Logf("env: .env rewritten with the model locations")
	writeJSON(w, http.StatusOK, "saved")
}

var locationFields = []string{"huggingface_hub_cache", "hf_home", "torch_home", "xdg_cache_home"}

func parseModelLocations(body io.Reader) (ModelLocations, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(body)
	if err := dec.Decode(&raw); err != nil || raw == nil {
		return ModelLocations{}, invalidf("body must be a JSON object")
	}
	if dec.More() {
		return ModelLocations{}, invalidf("body must be one JSON object")
	}
	values := map[string]string{}
	for k, v := range raw {
		known := false
		for _, f := range locationFields {
			known = known || f == k
		}
		if !known {
			return ModelLocations{}, invalidf("unknown field %q", k)
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return ModelLocations{}, invalidf("%s must be a string", k)
		}
		values[k] = s
	}
	for _, f := range locationFields {
		if _, ok := values[f]; !ok {
			return ModelLocations{}, invalidf("missing field %q", f)
		}
	}
	return ModelLocations{
		HuggingFaceHubCache: values["huggingface_hub_cache"],
		HFHome:              values["hf_home"],
		TorchHome:           values["torch_home"],
		XDGCacheHome:        values["xdg_cache_home"],
	}, nil
}

// getModelLocations answers GET /api/env/model-locations with the four values
// as .env sets them (empty when the line is missing or commented out). The
// process environment is not read: this is what the file says, which is what
// the tab saves.
func (a *API) getModelLocations(w http.ResponseWriter) {
	env, err := LoadEnv(a.opts.Root, []string{})
	if err != nil {
		a.opts.Logf("env: model-locations: %v", err)
		writeErr(w, http.StatusInternalServerError, "could not read .env")
		return
	}
	get := func(name string) string {
		v, _ := lookupEnv(env, name, false)
		return v
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"huggingface_hub_cache": get("HUGGINGFACE_HUB_CACHE"),
		"hf_home":               get("HF_HOME"),
		"torch_home":            get("TORCH_HOME"),
		"xdg_cache_home":        get("XDG_CACHE_HOME"),
	})
}
