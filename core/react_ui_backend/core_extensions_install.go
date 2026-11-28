package react_ui_backend

// Install and uninstall of a catalog extension from the React UI's Extension
// Lists page (the Gradio list tabs are retired).
//
// CONTRACTS.md section 6 says a claim must not change the install. This is a
// named exception (accepted by the human on 2026-11-28, LAB-59; see CONTRACTS.md
// section 6): only these two operations,
// only for a package_name, which the control API looks up in its own catalog
// (it refuses built-ins, takes no requirements from the caller and runs one
// install at a time). Not served: install-entry, add, check-dependencies, a
// GPU choice, and anything for audio.cpp or the Gradio host. The request must
// be JSON (a cross-site form cannot send that), and it must be addressed to a
// loopback name, so LAN mode (no password) cannot reach it.

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "core-extensions-install",
		Patterns: []string{"POST /api/go-data/extensions/{op}"},
		Handler:  func(d Deps) http.Handler { return newExtensionInstall(d.Core) },
	})
}

func loopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func newExtensionInstall(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := r.PathValue("op")
		if op != "install" && op != "uninstall" {
			http.NotFound(w, r)
			return
		}
		if !loopbackHost(r.Host) {
			writeAPIError(w, http.StatusForbidden, "installing is only available on this computer, not over the network")
			return
		}
		if !requireJSON(w, r) {
			return
		}
		raw, ok := readBody(w, r)
		if !ok {
			return
		}
		var in struct {
			PackageName string `json:"package_name"`
		}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil || in.PackageName == "" {
			writeAPIError(w, http.StatusBadRequest, `body must be {"package_name": "<package>"}`)
			return
		}
		// Re-encoded, so nothing but the package name reaches the control API.
		body, _ := json.Marshal(map[string]string{"package_name": in.PackageName})
		resp, err := core.do(r.Context(), http.MethodPost, "/api/extcatalog/"+op, bytesReader(body), "application/json")
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}
