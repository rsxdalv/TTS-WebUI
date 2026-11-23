package logview

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
)

const (
	// TokenHeader carries the shared secret (same as appconfig, history, extcatalog).
	TokenHeader = "X-TTS-Core-Token"
	// Prefix is the path prefix of the API.
	Prefix = "/api/logs/"

	maxBody = 1 << 16
)

// Handler serves the API under Prefix.
//
//	GET  files                        -> [name, ...] newest first
//	GET  pip-logs?kind=               -> [{name, file}, ...]  (kind: install|uninstall)
//	GET  stats                        -> Markdown string
//	GET  summary?kind=                -> Markdown string
//	GET  info?file=                   -> Markdown string
//	GET  analyze?file=                -> Markdown string
//	GET  read?file=&search=&max_lines= -> text (messages in-band)
//	POST delete {file}                -> {status, files}
//	POST delete-all {}                -> {status, files}
func (s *Store) Handler() http.Handler { return http.HandlerFunc(s.serve) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Store) fail(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	if errors.Is(err, errInvalid) {
		code = http.StatusBadRequest
	} else {
		s.opts.Logf("logview: %v", err)
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

var ops = map[string]string{
	"files": http.MethodGet, "pip-logs": http.MethodGet, "stats": http.MethodGet,
	"summary": http.MethodGet, "info": http.MethodGet, "analyze": http.MethodGet,
	"read": http.MethodGet, "delete": http.MethodPost, "delete-all": http.MethodPost,
}

// allowed checks the query has only the listed keys.
func allowed(r *http.Request, keys ...string) error {
	for k := range r.URL.Query() {
		ok := false
		for _, w := range keys {
			ok = ok || k == w
		}
		if !ok {
			return invalidf("unknown parameter %q", k)
		}
	}
	return nil
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
	var (
		res any
		err error
	)
	switch op {
	case "files":
		if err = allowed(r); err == nil {
			res, err = s.Files()
		}
	case "pip-logs":
		if err = allowed(r, "kind"); err == nil {
			res, err = s.PipLogs(q.Get("kind"))
		}
	case "stats":
		if err = allowed(r); err == nil {
			res, err = s.Stats()
		}
	case "summary":
		if err = allowed(r, "kind"); err == nil {
			res, err = s.Summary(q.Get("kind"))
		}
	case "info":
		if err = allowed(r, "file"); err == nil {
			res, err = s.Info(q.Get("file"))
		}
	case "analyze":
		if err = allowed(r, "file"); err == nil {
			res, err = s.Analyze(q.Get("file"))
		}
	case "read":
		if err = allowed(r, "file", "search", "max_lines"); err != nil {
			break
		}
		n, perr := strconv.Atoi(q.Get("max_lines"))
		if perr != nil {
			err = invalidf("max_lines must be an integer")
			break
		}
		res, err = s.Read(q.Get("file"), q.Get("search"), n)
	case "delete", "delete-all":
		var o map[string]any
		if o, err = jsonBody(w, r); err != nil {
			break
		}
		if op == "delete-all" {
			if len(o) != 0 {
				err = invalidf("delete-all takes no fields")
				break
			}
			res, err = s.DeleteAll()
			break
		}
		f, isStr := o["file"].(string)
		if len(o) != 1 || !isStr {
			err = invalidf("body must be {\"file\": string}")
			break
		}
		res, err = s.Delete(f)
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func jsonBody(w http.ResponseWriter, r *http.Request) (map[string]any, error) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return nil, invalidf("Content-Type must be application/json")
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		return nil, invalidf("body: %v", err)
	}
	var o map[string]any
	if err := json.Unmarshal(b, &o); err != nil || o == nil {
		return nil, invalidf("body must be a JSON object")
	}
	return o, nil
}
