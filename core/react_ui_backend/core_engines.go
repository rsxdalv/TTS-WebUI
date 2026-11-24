package react_ui_backend

// The engine page (D27, D31): opening it starts an installed own-venv engine and
// shows the engine's own Gradio page. This is the one process start that port 3000
// reaches, so the limits are in the control API (engines package): only a
// catalog engine that is installed starts, nothing else is reachable, and the
// route only starts, never stops, installs or reads logs. The front door's Origin
// check and the JSON content type below keep another site from calling it.

import (
	"net/http"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extprocs"
)

func init() {
	RegisterClaim(Claim{
		Switch: "core-engines",
		Patterns: []string{
			"GET /api/go-data/engines/{pkg}",
			"POST /api/go-data/engines/{pkg}/open",
		},
		Handler: func(d Deps) http.Handler { return newEngines(d.Core) },
	})
}

func newEngines(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pkg := r.PathValue("pkg")
		if extprocs.ValidatePackage(pkg) != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid package name")
			return
		}
		method, path := http.MethodGet, "/api/engines/"+pkg
		if r.Method == http.MethodPost {
			if !requireJSON(w, r) {
				return
			}
			method, path = http.MethodPost, path+"/open"
		}
		resp, err := core.do(r.Context(), method, path, nil, "")
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}
