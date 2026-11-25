package react_ui_backend

// The openai platform's routes (P5): named remote endpoints and the generate
// call. They forward to the control API (platform/openai), which holds the
// keys. A key goes in with PUT and never comes out: no answer carries one.
//
// Who may change an endpoint: the browser on this machine only. A request
// from another device (the UI open to the LAN, D20) can list endpoints and
// generate with them, but PUT, DELETE and the test call answer 403. "This
// machine" is a loopback peer address on a request with no forwarding header,
// so a reverse proxy on the same host does not make a remote user local.

import (
	"net"
	"net/http"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
)

func init() {
	RegisterClaim(Claim{
		Switch: "core-openai",
		Patterns: []string{
			"GET /api/go-data/openai/endpoints",
			"PUT /api/go-data/openai/endpoints/{name}",
			"DELETE /api/go-data/openai/endpoints/{name}",
			"POST /api/go-data/openai/endpoints/{name}/test",
			"POST /api/go-data/openai/speech",
		},
		Handler: func(d Deps) http.Handler { return newOpenAI(d.Core) },
	})
}

// thisMachine reports whether the request comes from the machine the app runs on.
func thisMachine(r *http.Request) bool {
	for _, h := range []string{"X-Forwarded-For", "Forwarded", "X-Real-Ip", "X-Forwarded-Host"} {
		if r.Header.Get(h) != "" {
			return false
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func newOpenAI(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/api/openai/speech"
		var body []byte
		if name := r.PathValue("name"); name != "" {
			if !openai.ValidName(name) {
				writeAPIError(w, http.StatusBadRequest, "invalid endpoint name")
				return
			}
			path = "/api/openai/endpoints/" + name
			if r.Method == http.MethodPost {
				path += "/test"
			}
		} else if r.Method == http.MethodGet {
			path = "/api/openai/endpoints"
		}
		if r.Method != http.MethodGet && r.URL.Path != "/api/go-data/openai/speech" && !thisMachine(r) {
			writeAPIError(w, http.StatusForbidden, "endpoints can be changed and tested only from the machine TTS WebUI runs on")
			return
		}
		contentType := ""
		if r.Method == http.MethodPut || r.URL.Path == "/api/go-data/openai/speech" {
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
