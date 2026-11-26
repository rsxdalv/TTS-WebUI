package minimax

import (
	"errors"
	"net/http"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
)

const (
	// Prefix is where the handler is mounted on the control API.
	Prefix = "/api/minimax/"
)

// Handler is the control API (loopback, token). The browser reaches it only
// through port 3000's routes, which add the rule about who may change a setting.
//
//	GET  /api/minimax/settings   the settings (no key)
//	PUT  /api/minimax/settings   {base_url, model, voice, key?}
//	POST /api/minimax/test       one call to MiniMax
//	POST /api/minimax/speech     {text, model?, voice_id?}: generate and save
//
// "key" is the only way a key enters; absent it keeps the stored one, "" removes it.
type Handler struct {
	client *Client
	token  string
}

// NewHandler returns the handler.
func NewHandler(c *Client, token string) *Handler { return &Handler{client: c, token: token} }

func (h *Handler) fail(w http.ResponseWriter, err error) {
	var ue *openai.UpstreamError
	switch {
	case errors.Is(err, openai.ErrInvalid):
		openai.WriteErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "invalid: "))
	case errors.As(err, &ue):
		openai.WriteErr(w, http.StatusBadGateway, ue.Msg)
	default:
		h.client.o.Logf("minimax: %v", err)
		openai.WriteErr(w, http.StatusInternalServerError, "MiniMax Cloud TTS failed; see the log")
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !openai.Authorized(r, h.token) {
		openai.WriteErr(w, http.StatusForbidden, "forbidden")
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, Prefix)
	if !ok {
		openai.WriteErr(w, http.StatusNotFound, "not found")
		return
	}
	method := func(want string) bool {
		if r.Method == want {
			return true
		}
		w.Header().Set("Allow", want)
		openai.WriteErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	switch rest {
	case "settings":
		switch r.Method {
		case http.MethodGet:
			set, err := h.client.o.Store.Get()
			if err != nil {
				h.fail(w, err)
				return
			}
			openai.WriteJSON(w, http.StatusOK, set)
		case http.MethodPut:
			var in SettingsIn
			if err := openai.DecodeJSON(w, r, &in); err != nil {
				h.fail(w, err)
				return
			}
			set, err := h.client.o.Store.Put(in)
			if err != nil {
				h.fail(w, err)
				return
			}
			openai.WriteJSON(w, http.StatusOK, set)
		default:
			w.Header().Set("Allow", "GET, PUT")
			openai.WriteErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case "test":
		if !method(http.MethodPost) {
			return
		}
		res, err := h.client.Test(r.Context())
		if err != nil {
			h.fail(w, err)
			return
		}
		openai.WriteJSON(w, http.StatusOK, res)
	case "speech":
		if !method(http.MethodPost) {
			return
		}
		var in SpeechRequest
		if err := openai.DecodeJSON(w, r, &in); err != nil {
			h.fail(w, err)
			return
		}
		res, err := h.client.Speak(r.Context(), in)
		if err != nil {
			h.fail(w, err)
			return
		}
		openai.WriteJSON(w, http.StatusOK, res)
	default:
		openai.WriteErr(w, http.StatusNotFound, "not found")
	}
}
