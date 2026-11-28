package react_ui_backend

// Extension lists (the read-only part of extension_list_tab,
// extension_decorator_list_tab, extension_extension_catalog and
// custom_extensions_installer). Only the lists are served here.
//
// Install and uninstall by package name are in core_extensions_install.go (a
// named exception). Dependency checks, "add entries" and install-entry are NOT
// served on port 3000: install-entry takes requirements that came from a pasted
// text. Those stay in the Gradio tabs and in the launcher's Extensions page on
// the token-protected port.

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "core-extensions",
		Patterns: []string{"GET /api/go-data/extensions/{kind}", "GET /api/go-data/extensions/{kind}/state"},
		Handler: func(d Deps) http.Handler {
			list, state := newExtensionList(d.Core), newExtensionState(d.Core)
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/state") {
					state.ServeHTTP(w, r)
					return
				}
				list.ServeHTTP(w, r)
			})
		},
	})
}

// extensionState is what the list page shows next to an entry.
type extensionState struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Enabled   bool   `json:"enabled"`
	Proxy     string `json:"proxy,omitempty"`
}

// newExtensionState answers {package_name: state} for one kind: the control
// API's resolve answer (installed, version, enabled) of every class in the list.
func newExtensionState(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		if !slices.Contains([]string{"interface", "decorator"}, kind) {
			http.NotFound(w, r)
			return
		}
		var list []struct {
			Class string `json:"extension_class"`
		}
		if err := core.getJSON(r.Context(), "/api/extcatalog/list?kind="+kind, &list); err != nil {
			coreError(w, err)
			return
		}
		var classes []string
		for _, e := range list {
			if e.Class != "" && !slices.Contains(classes, e.Class) {
				classes = append(classes, e.Class)
			}
		}
		out := map[string]extensionState{}
		for _, class := range classes {
			var res struct {
				Extensions []struct {
					PackageName string `json:"package_name"`
					Enabled     bool   `json:"enabled"`
					Installed   bool   `json:"installed"`
					Version     string `json:"version"`
					Proxy       string `json:"proxy"`
				} `json:"extensions"`
			}
			if err := core.getJSON(r.Context(), "/api/extcatalog/resolve?kind="+kind+"&extension_class="+url.QueryEscape(class), &res); err != nil {
				coreError(w, err)
				return
			}
			for _, x := range res.Extensions {
				out[x.PackageName] = extensionState{Installed: x.Installed, Version: x.Version, Enabled: x.Enabled, Proxy: x.Proxy}
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(out)
	})
}

func newExtensionList(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		if !slices.Contains([]string{"interface", "decorator"}, kind) {
			http.NotFound(w, r)
			return
		}
		resp, err := core.do(r.Context(), http.MethodGet, "/api/extcatalog/list?kind="+kind, nil, "")
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}
