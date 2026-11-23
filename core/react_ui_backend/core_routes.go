package react_ui_backend

// Routes for pip list, logs and model locations (the Settings tabs
// extension_pip_list, extension_log_viewer, extension_model_location). Each one
// forwards to a loopback service with the token; see core_client.go.

import (
	"net/http"
	"net/url"
	"slices"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "core-pip",
		Patterns: []string{"GET /api/go-data/pip-list"},
		Handler:  func(d Deps) http.Handler { return newPipList(d.Core) },
	})
	RegisterClaim(Claim{
		Switch:   "core-logs",
		Patterns: []string{"GET /api/go-data/logs/{op}", "POST /api/go-data/logs/{op}"},
		Handler:  func(d Deps) http.Handler { return newLogs(d.Core) },
	})
	RegisterClaim(Claim{
		Switch:   "core-model-locations",
		Patterns: []string{"GET /api/go-data/model-locations", "PUT /api/go-data/model-locations"},
		Handler:  func(d Deps) http.Handler { return newModelLocations(d.Core) },
	})
}

func newPipList(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := core.do(r.Context(), http.MethodGet, "/api/pip/list", nil, "")
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}

var (
	logGetOps  = []string{"files", "pip-logs", "stats", "summary", "info", "analyze", "read"}
	logPostOps = []string{"delete", "delete-all"}
)

// newLogs forwards the Log Viewer operations. The upstream validates the file
// name (one element, ends in .log) and the other parameters; this layer only
// restricts the operation names, so no other /api/logs path is reachable.
func newLogs(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := r.PathValue("op")
		if r.Method == http.MethodPost {
			if !slices.Contains(logPostOps, op) {
				http.NotFound(w, r)
				return
			}
			if !requireJSON(w, r) {
				return
			}
			body, ok := readBody(w, r)
			if !ok {
				return
			}
			resp, err := core.do(r.Context(), http.MethodPost, "/api/logs/"+op, bytesReader(body), "application/json")
			if err != nil {
				coreError(w, err)
				return
			}
			relay(w, resp)
			return
		}
		if !slices.Contains(logGetOps, op) {
			http.NotFound(w, r)
			return
		}
		path := "/api/logs/" + op
		if q := r.URL.RawQuery; q != "" {
			if _, err := url.ParseQuery(q); err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad query")
				return
			}
			path += "?" + q
		}
		resp, err := core.do(r.Context(), http.MethodGet, path, nil, "")
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}

func newModelLocations(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			if !requireJSON(w, r) {
				return
			}
			body, ok := readBody(w, r)
			if !ok {
				return
			}
			resp, err := core.do(r.Context(), http.MethodPost, "/api/env/save-model-locations", bytesReader(body), "application/json")
			if err != nil {
				coreError(w, err)
				return
			}
			relay(w, resp)
			return
		}
		resp, err := core.do(r.Context(), http.MethodGet, "/api/env/model-locations", nil, "")
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}
