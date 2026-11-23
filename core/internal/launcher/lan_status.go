package launcher

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
	"github.com/rsxdalv/TTS-WebUI/core/react_ui_backend"
)

// lanStatus is what GET /api/app/lan answers.
type lanStatus struct {
	LAN     bool     `json:"lan"`
	Warning string   `json:"warning,omitempty"`
	URLs    []string `json:"urls,omitempty"`
}

// handleAppLAN tells the launcher web UI whether the React UI is reachable from
// the network (TTS_GO_PROXY_LISTEN=lan), with the same warning, URLs and "no
// password" the launcher logs on stderr, so the user sees it on the page too
// (R2-8). With an invalid switch value it reports no LAN: the app start then
// fails with that error and starts nothing.
func (l *Launcher) handleAppLAN() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var st lanStatus
		if sw, err := switches.Parse(os.LookupEnv); err == nil && sw.ProxyLAN {
			st = lanStatus{
				LAN:     true,
				Warning: react_ui_backend.LANWarning(react_ui_backend.PublicPort),
				URLs:    react_ui_backend.LANURLs(react_ui_backend.PublicPort),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(st)
	})
}
