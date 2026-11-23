package react_ui_backend

// Extension lists (the read-only part of extension_list_tab,
// extension_decorator_list_tab, extension_extension_catalog and
// custom_extensions_installer). Only the lists are served here.
//
// Install, uninstall, dependency checks and "add entries" are NOT served on
// port 3000: they start pip/uv and change the install, and CONTRACTS.md section 6
// says a claim must not do that (this port has no token; install-entry takes
// requirements that came from a pasted text). Those stay in the Gradio tabs and
// in the launcher's Extensions page on the token-protected port.

import (
	"net/http"
	"slices"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "core-extensions",
		Patterns: []string{"GET /api/go-data/extensions/{kind}"},
		Handler:  func(d Deps) http.Handler { return newExtensionList(d.Core) },
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
