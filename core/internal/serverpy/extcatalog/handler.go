package extcatalog

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
)

const (
	// TokenHeader carries the shared secret (same as appconfig, history, extprocs).
	TokenHeader = "X-TTS-Core-Token"
	// Prefix is the path prefix of the API.
	Prefix = "/api/extcatalog/"

	maxBody = 4 << 20
)

// Handler serves the API under Prefix.
func (s *Store) Handler() http.Handler { return http.HandlerFunc(s.serve) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(Marshal(v, ""))
	_, _ = io.WriteString(w, "\n")
}

func (s *Store) fail(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, errInvalid):
		code = http.StatusBadRequest
	case errors.Is(err, errNotFound):
		code = http.StatusNotFound
	case errors.Is(err, errBusy):
		code = http.StatusConflict
	}
	if code == http.StatusInternalServerError {
		s.opts.Logf("extcatalog: %v", err)
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

// body reads a JSON object body, keeping key order.
func body(w http.ResponseWriter, r *http.Request) (*Obj, error) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return nil, invalidf("Content-Type must be application/json")
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		return nil, invalidf("body: %v", err)
	}
	v, err := Parse(b)
	o, ok := v.(*Obj)
	if err != nil || !ok {
		return nil, invalidf("body must be a JSON object")
	}
	return o, nil
}

// only checks that the body has exactly the listed keys.
func only(o *Obj, keys ...string) error {
	for _, k := range o.Keys {
		found := false
		for _, want := range keys {
			found = found || k == want
		}
		if !found {
			return invalidf("unknown field %q", k)
		}
	}
	return nil
}

var ops = map[string]string{
	"list": http.MethodGet, "table": http.MethodGet, "picker": http.MethodGet,
	"resolve": http.MethodGet,
	"parse":   http.MethodPost, "add": http.MethodPost,
	"install": http.MethodPost, "install-entry": http.MethodPost,
	"uninstall": http.MethodPost,
}

func (s *Store) serve(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	op := strings.TrimPrefix(r.URL.Path, Prefix)
	want, ok := ops[op]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != want {
		w.Header().Set("Allow", want)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	q := r.URL.Query()
	switch op {
	case "list":
		list, err := s.List(q.Get("kind"))
		if err != nil {
			s.fail(w, err)
			return
		}
		out := make([]any, len(list))
		for i, e := range list {
			out[i] = e
		}
		writeJSON(w, 200, out)
	case "resolve":
		res, err := s.Resolve(q.Get("kind"), q.Get("extension_class"))
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, res)
	case "table":
		t, err := s.Table(q.Get("kind"))
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, mk("Markdown", t))
	case "picker":
		c, err := s.Choices(q.Get("kind"))
		if err != nil {
			s.fail(w, err)
			return
		}
		choices := make([]any, len(c))
		for i, x := range c {
			choices[i] = x
		}
		writeJSON(w, 200, map[string]any{"__gradio__": "Dropdown", "props": map[string]any{"label": q.Get("label"), "choices": choices}})
	case "parse":
		o, err := body(w, r)
		if err == nil {
			err = only(o, "text")
		}
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, ParseInput(o.Str("text")))
	case "add":
		o, err := body(w, r)
		if err == nil {
			err = only(o, "entries")
		}
		if err != nil {
			s.fail(w, err)
			return
		}
		arr, ok := o.Vals["entries"].([]any)
		if !ok {
			s.fail(w, invalidf("entries must be a list"))
			return
		}
		res, err := s.Add(arr)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, res)
	case "install", "uninstall":
		s.serveInstall(w, r, op)
	case "install-entry":
		s.serveInstallEntry(w, r)
	}
}
