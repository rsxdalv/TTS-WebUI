package launcher

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/detect"
)

// hostStatusPath is the control API route the Vite menu asks (through
// /api/go-data/host-status): is the Gradio host there, so the pages that call
// Gradio can be shown.
const hostStatusPath = "/api/host/status"

// launcherPorts maps an install folder to the port its launcher listens on.
var launcherPorts sync.Map // string -> int

// HostStatus is the answer of hostStatusPath.
type HostStatus struct {
	// Installed: the Gradio pages have a backend. True on legacy-env and system
	// (the env is adopted) and on the host once it is installed.
	Installed bool `json:"installed"`
	// Installable: the host can be installed from the launcher (a new-style
	// folder). False on legacy-env and system, where Installed is the env.
	Installable bool `json:"installable"`
	// Detail says what to do when it is not installed.
	Detail string `json:"detail,omitempty"`
	// LauncherPort is where the Extensions page is, 0 when not known.
	LauncherPort int `json:"launcherPort,omitempty"`
}

// hostStatusHandler answers GET hostStatusPath. It is asked on each call: the
// host may be installed while the app runs (it starts after the next restart),
// so "installed" means the venv is complete, not that Gradio is running.
func hostStatusHandler(p platform.Platform, token, root string) http.Handler {
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := sha256.Sum256([]byte(r.Header.Get("X-TTS-Core-Token")))
		if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		st := p.Status(r.Context())
		out := HostStatus{Installed: st.Ready, Installable: detect.Host(p) != nil}
		if !st.Ready {
			out.Detail = st.Detail
		}
		if v, ok := launcherPorts.Load(root); ok {
			out.LauncherPort, _ = v.(int)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(out)
	})
}
