package openai

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"strings"
)

const (
	// TokenHeader carries the shared secret of the control API.
	TokenHeader = "X-TTS-Core-Token"
	// Prefix is where the handler is mounted on the control API.
	Prefix = "/api/openai/"

	maxBody = 1 << 20
)

// Handler is the control API of the platform (loopback, token). The browser
// reaches it only through port 3000's routes, which add the rules about who
// may change an endpoint.
//
//	GET    /api/openai/endpoints              the endpoints (no keys)
//	PUT    /api/openai/endpoints/{name}       create or replace: {url, model, voice, key?}
//	DELETE /api/openai/endpoints/{name}
//	POST   /api/openai/endpoints/{name}/test  one call to the endpoint
//	POST   /api/openai/speech                 generate and save: SpeechRequest
//
// "key" is the only way a key enters; absent it keeps the stored one, "" removes it.
type Handler struct {
	store  *Store
	client *Client
	token  string
}

// NewHandler returns the handler.
func NewHandler(c *Client, token string) *Handler {
	return &Handler{store: c.o.Store, client: c, token: token}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (h *Handler) authorized(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() || h.token == "" {
		return false
	}
	got := sha256.Sum256([]byte(r.Header.Get(TokenHeader)))
	want := sha256.Sum256([]byte(h.token))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// fail answers an error: the caller's input is 400, a missing endpoint 404, a
// problem on the endpoint's side 502 (its text is already safe to show).
func (h *Handler) fail(w http.ResponseWriter, err error) {
	var ue *UpstreamError
	switch {
	case errors.Is(err, ErrInvalid):
		writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "invalid: "))
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, strings.TrimPrefix(err.Error(), "not found: "))
	case errors.As(err, &ue):
		writeErr(w, http.StatusBadGateway, ue.Msg)
	default:
		h.client.o.Logf("openai: %v", err)
		writeErr(w, http.StatusInternalServerError, "the openai platform failed; see the log")
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return invalidf("Content-Type must be application/json")
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return invalidf("bad JSON body")
	}
	return nil
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, Prefix)
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	parts := strings.Split(rest, "/")
	method := func(want string) bool {
		if r.Method == want {
			return true
		}
		w.Header().Set("Allow", want)
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	switch {
	case rest == "endpoints":
		if !method(http.MethodGet) {
			return
		}
		list, err := h.store.List()
		if err != nil {
			h.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	case len(parts) == 2 && parts[0] == "endpoints" && r.Method == http.MethodDelete:
		if err := h.store.Delete(parts[1]); err != nil {
			h.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case len(parts) == 2 && parts[0] == "endpoints":
		if !method(http.MethodPut) {
			return
		}
		var in struct {
			URL   string  `json:"url"`
			Model string  `json:"model"`
			Voice string  `json:"voice"`
			Key   *string `json:"key"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, err)
			return
		}
		e, err := h.store.Put(Endpoint{Name: parts[1], URL: in.URL, Model: in.Model, Voice: in.Voice}, in.Key)
		if err != nil {
			h.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, e)
	case len(parts) == 3 && parts[0] == "endpoints" && parts[2] == "test":
		if !method(http.MethodPost) {
			return
		}
		res, err := h.client.Test(r.Context(), parts[1])
		if err != nil {
			h.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	case rest == "speech":
		if !method(http.MethodPost) {
			return
		}
		var in SpeechRequest
		if err := decode(w, r, &in); err != nil {
			h.fail(w, err)
			return
		}
		res, err := h.client.Speak(r.Context(), in)
		if err != nil {
			h.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	default:
		writeErr(w, http.StatusNotFound, "not found")
	}
}
