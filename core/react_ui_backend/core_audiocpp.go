package react_ui_backend

// The audiocpp platform's routes (P7): what is installed, generate and cancel.
// They forward to the control API (platform/audiocpp). Installing and removing
// the platform and its models is not here: it is the Extensions page of the
// launcher (D30). Cancel is open to whoever can generate (they can already tie
// the server up); stopping the server is not offered to the browser at all.

import (
	"net/http"
)

func init() {
	RegisterClaim(Claim{
		Switch: "core-audiocpp",
		Patterns: []string{
			"GET /api/go-data/audiocpp/status",
			"POST /api/go-data/audiocpp/speech",
			"POST /api/go-data/audiocpp/cancel",
		},
		Handler: func(d Deps) http.Handler { return newAudioCpp(d.Core) },
	})
}

func newAudioCpp(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var path string
		switch r.URL.Path {
		case "/api/go-data/audiocpp/status":
			path = "/api/audiocpp/status"
		case "/api/go-data/audiocpp/speech":
			path = "/api/audiocpp/speech"
		default:
			path = "/api/audiocpp/cancel"
		}
		var body []byte
		contentType := ""
		if r.Method == http.MethodPost {
			if !requireJSON(w, r) {
				return
			}
			b, ok := readBody(w, r)
			if !ok {
				return
			}
			body, contentType = b, "application/json"
		}
		resp, err := core.do(r.Context(), r.Method, path, bytesReader(body), contentType)
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}
