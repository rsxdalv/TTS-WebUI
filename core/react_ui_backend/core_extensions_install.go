package react_ui_backend

// Install, uninstall and "add custom entries" for the React UI's Extension
// Lists and Custom Extensions Installer pages (the Gradio tabs are retired).
//
// CONTRACTS.md section 6 says a claim must not change the install. These are
// named exceptions (accepted by the human on 2026-11-28, LAB-59 and LAB-62; see
// CONTRACTS.md section 6):
//
//   - install and uninstall take a package_name, which the control API looks up
//     in its own catalog (it refuses built-ins, takes no requirements from the
//     caller and runs one install at a time);
//   - parse is a pure check of pasted JSON; add takes the same pasted text, which
//     Go parses and validates itself before it writes the valid entries to
//     extensions.external.json. The browser never sends entries, so it cannot
//     skip the validation. An added entry can then be installed by name, which is
//     what the Custom Extensions Installer page does, as the Gradio tab did.
//
// Not served: install-entry, check-dependencies, a GPU choice, and anything for
// audio.cpp or the Gradio host. The request must be JSON (a cross-site form
// cannot send that), and it must be addressed to a loopback name, so LAN mode
// (no password) cannot reach it.

import (
	"bytes"
	"encoding/json"
	"io"
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

// strictBody decodes a request body that has exactly the one string field.
func strictBody(raw []byte, field string) (string, bool) {
	var m map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil || len(m) != 1 {
		return "", false
	}
	var s string
	if json.Unmarshal(m[field], &s) != nil {
		return "", false
	}
	return s, true
}

func newExtensionInstall(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := r.PathValue("op")
		if op != "install" && op != "uninstall" && op != "parse" && op != "add" {
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
		switch op {
		case "install", "uninstall":
			name, ok := strictBody(raw, "package_name")
			if !ok || name == "" {
				writeAPIError(w, http.StatusBadRequest, `body must be {"package_name": "<package>"}`)
				return
			}
			// Re-encoded, so nothing but the package name reaches the control API.
			body, _ := json.Marshal(map[string]string{"package_name": name})
			resp, err := core.do(r.Context(), http.MethodPost, "/api/extcatalog/"+op, bytesReader(body), "application/json")
			if err != nil {
				coreError(w, err)
				return
			}
			relay(w, resp)
		case "parse", "add":
			text, ok := strictBody(raw, "text")
			if !ok {
				writeAPIError(w, http.StatusBadRequest, `body must be {"text": "<pasted JSON>"}`)
				return
			}
			parseBody, _ := json.Marshal(map[string]string{"text": text})
			resp, err := core.do(r.Context(), http.MethodPost, "/api/extcatalog/parse", bytesReader(parseBody), "application/json")
			if err != nil {
				coreError(w, err)
				return
			}
			if op == "parse" {
				relay(w, resp)
				return
			}
			parsed, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			if err != nil || resp.StatusCode != http.StatusOK {
				writeAPIError(w, http.StatusBadGateway, "the core API did not answer")
				return
			}
			var res struct {
				Entries []json.RawMessage `json:"entries"`
				Info    string            `json:"info"`
			}
			if json.Unmarshal(parsed, &res) != nil {
				writeAPIError(w, http.StatusBadGateway, "the core API answered something unexpected")
				return
			}
			if len(res.Entries) == 0 {
				writeAPIError(w, http.StatusBadRequest, "nothing to add: "+res.Info)
				return
			}
			addBody, _ := json.Marshal(map[string][]json.RawMessage{"entries": res.Entries})
			resp, err = core.do(r.Context(), http.MethodPost, "/api/extcatalog/add", bytesReader(addBody), "application/json")
			if err != nil {
				coreError(w, err)
				return
			}
			relay(w, resp)
		}
	})
}
