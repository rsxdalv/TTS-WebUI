package react_ui_backend

// The Vite menu asks whether the Gradio host is installed (P3): the pages that
// call /api/gradio are shown only when it is, otherwise the menu links to the
// launcher's Extensions page. Read only; the answer comes from the control API.

import "net/http"

func init() {
	RegisterClaim(Claim{
		Switch:   "core-host",
		Patterns: []string{"GET /api/go-data/host-status"},
		Handler:  func(d Deps) http.Handler { return newHostStatus(d.Core) },
	})
}

func newHostStatus(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := core.do(r.Context(), http.MethodGet, "/api/host/status", nil, "")
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}
