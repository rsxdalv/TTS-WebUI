package react_ui_backend

// The Manage Extension panel of the extension pages: stop a running own-venv
// engine, and change an extension's enabled and autostart settings.
//
// Like the install route (core_extensions_install.go, D32) these are named
// exceptions to "a claim must not change the install", and as narrow: only a
// package name (and one boolean for autostart) reaches the control API, which
// is re-encoded here, the request must be JSON (a cross-site form cannot send
// that) and must be addressed to a loopback name, so LAN mode (no password)
// cannot reach them. Stop only stops a process the engine route started;
// enable, disable and autostart are single settings in config.json, applied the
// next time the Gradio UI starts.

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extprocs"
)

func init() {
	RegisterClaim(Claim{
		Switch: "core-extension-manage",
		Patterns: []string{
			"POST /api/go-data/engines/{pkg}/stop",
			"POST /api/go-data/extension-config/{op}",
		},
		Handler: func(d Deps) http.Handler { return newExtensionManage(d.Core) },
	})
}

func newExtensionManage(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			writeAPIError(w, http.StatusForbidden, "managing extensions is only available on this computer, not over the network")
			return
		}
		if !requireJSON(w, r) {
			return
		}
		if pkg := r.PathValue("pkg"); pkg != "" {
			if extprocs.ValidatePackage(pkg) != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid package name")
				return
			}
			resp, err := core.do(r.Context(), http.MethodPost, "/api/extensions/"+pkg+"/stop", strings.NewReader("{}"), "application/json")
			if err != nil {
				coreError(w, err)
				return
			}
			relay(w, resp)
			return
		}

		op := r.PathValue("op")
		if op != "enable" && op != "disable" && op != "autostart" {
			http.NotFound(w, r)
			return
		}
		raw, ok := readBody(w, r)
		if !ok {
			return
		}
		var in struct {
			PackageName string `json:"package_name"`
			Enabled     *bool  `json:"enabled"`
		}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil || extprocs.ValidatePackage(in.PackageName) != nil {
			writeAPIError(w, http.StatusBadRequest, `body must be {"package_name": "<package>"} (autostart also takes "enabled")`)
			return
		}
		out := map[string]any{"package_name": in.PackageName}
		if op == "autostart" {
			if in.Enabled == nil {
				writeAPIError(w, http.StatusBadRequest, `autostart needs "enabled": true or false`)
				return
			}
			out["enabled"] = *in.Enabled
		} else if in.Enabled != nil {
			writeAPIError(w, http.StatusBadRequest, `"enabled" belongs to autostart only`)
			return
		}
		body, _ := json.Marshal(out)
		resp, err := core.do(r.Context(), http.MethodPost, "/api/config/extension-"+op, bytesReader(body), "application/json")
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}
