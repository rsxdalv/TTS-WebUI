package launcher

import (
	"encoding/json"
	"net/http"
)

// migrationURL is the guide an install on the old env is pointed to.
const migrationURL = "https://github.com/rsxdalv/TTS-WebUI/blob/main/documentation/core-rewrite/MIGRATION.md"

// Notice is the answer of GET /api/app/notice.
type Notice struct {
	// Legacy: the install runs on the old env (the folder has the old installer's files or one
	// of its markers). Text and URL are set then.
	Legacy bool   `json:"legacy"`
	Text   string `json:"text,omitempty"`
	URL    string `json:"url,omitempty"`
}

func legacyNotice() Notice {
	return Notice{
		Legacy: true,
		URL:    migrationURL,
		Text: "This install still runs on the old shared Python environment. " +
			"It keeps working and nothing was changed. New installs use uv, one venv per engine and an optional Gradio host; " +
			"moving over is your choice, and the guide says how (documentation/core-rewrite/MIGRATION.md).",
	}
}

func (l *Launcher) handleNotice() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var n Notice
		if l.legacy() {
			n = legacyNotice()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(n)
	})
}
