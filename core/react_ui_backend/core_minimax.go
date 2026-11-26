package react_ui_backend

// MiniMax Cloud TTS routes: the settings, a test call and the generate call.
// They forward to the control API (internal/cloud/minimax), which holds the key.
// A key goes in with PUT and never comes out: no answer carries one.
//
// Who may change the settings: the browser on this machine only (thisMachine, as
// for the openai endpoints). A request from another device (the UI open to the
// LAN, D20) can read the settings (no key) and generate, but PUT and the test
// call answer 403.

import "net/http"

func init() {
	RegisterClaim(Claim{
		Switch: "core-minimax",
		Patterns: []string{
			"GET /api/go-data/minimax/settings",
			"PUT /api/go-data/minimax/settings",
			"POST /api/go-data/minimax/test",
			"POST /api/go-data/minimax/speech",
		},
		Handler: func(d Deps) http.Handler { return newMiniMax(d.Core) },
	})
}

func newMiniMax(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/api/minimax/" + r.URL.Path[len("/api/go-data/minimax/"):]
		if r.URL.Path != "/api/go-data/minimax/speech" && r.Method != http.MethodGet && !thisMachine(r) {
			writeAPIError(w, http.StatusForbidden, "the MiniMax settings can be changed and tested only from the machine TTS WebUI runs on")
			return
		}
		var body []byte
		contentType := ""
		if r.Method == http.MethodPut || r.URL.Path == "/api/go-data/minimax/speech" {
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
