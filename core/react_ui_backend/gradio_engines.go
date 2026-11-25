package react_ui_backend

// P6: an api name that an own-venv engine owns goes to that engine's process,
// not to the Gradio host. The Vite page is unchanged: it still posts to
// /api/gradio/<name>. Go looks the name up in engineOwners, asks the launcher
// (control API /api/engines/<pkg>) whether that package is an installed
// own-venv engine, starts it if it is not running, waits until it answers, and
// then makes the same Gradio call against the engine's address at the router
// (http://127.0.0.1:<public port>/<pkg>/).
//
// A name that no engine owns, and a package that is not an own-venv engine
// (not "proxy": "native" in extensions.json), is served by the Gradio host as
// before. An engine that is native but not installed is an error: the host no
// longer has its tab either.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// engineOwners maps a Vite api name to the package whose Gradio tab defines
// it. Add the names of an engine here when it is converted.
var engineOwners = map[string]string{
	"kokoro_generate": "tts_webui_extension.kokoro",

	"mms": "tts_webui_extension.mms",

	"chatterbox_tts":        "tts_webui_extension.chatterbox",
	"get_chatterbox_voices": "tts_webui_extension.chatterbox",

	"vall_e_x_generate":                  "tts_webui_extension.vall_e_x",
	"vall_e_x_split_text_into_sentences": "tts_webui_extension.vall_e_x",
	"vall_e_x_tokenize":                  "tts_webui_extension.vall_e_x",
}

var (
	errNotEngine = errors.New("not an own-venv engine")
	// engineReadyWait is how long a call waits for a starting engine (loading
	// torch can take a while). Tests shorten it.
	engineReadyWait = 5 * time.Minute
	enginePoll      = 500 * time.Millisecond
)

type engineAnswer struct {
	State      string `json:"state"`
	Error      string `json:"error"`
	PublicPort int    `json:"public_port"`
	Path       string `json:"path"`
}

func engineGet(ctx context.Context, core *Core, method, pkg string) (engineAnswer, int, error) {
	path := "/api/engines/" + pkg
	body, ct := io.Reader(nil), ""
	if method == http.MethodPost {
		path += "/open"
		body, ct = strings.NewReader("{}"), "application/json"
	}
	resp, err := core.do(ctx, method, path, body, ct)
	if err != nil {
		return engineAnswer{}, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var a engineAnswer
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &a); err != nil {
			return a, resp.StatusCode, fmt.Errorf("engine %s: unreadable answer: %w", pkg, err)
		}
		return a, resp.StatusCode, nil
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(raw, &e)
	a.Error = e.Error
	return a, resp.StatusCode, nil
}

// engineBackend returns the Gradio address of an engine, trailing slash
// included. With start it opens the engine and waits for ready; without it
// the engine must already be running.
func engineBackend(ctx context.Context, core *Core, pkg string, start bool) (string, error) {
	method := http.MethodGet
	if start {
		method = http.MethodPost
	}
	deadline := time.Now().Add(engineReadyWait)
	for {
		a, code, err := engineGet(ctx, core, method, pkg)
		if err != nil {
			return "", err
		}
		switch {
		case code == http.StatusNotFound:
			return "", errNotEngine
		case code != http.StatusOK:
			return "", fmt.Errorf("engine %s: %s", pkg, a.Error)
		}
		switch a.State {
		case "ready":
			return fmt.Sprintf("http://127.0.0.1:%d/%s/", a.PublicPort, pkg), nil
		case "failed", "exited":
			return "", fmt.Errorf("engine %s did not start: %s", pkg, a.Error)
		case "stopped":
			if !start {
				return "", fmt.Errorf("engine %s is not running", pkg)
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("engine %s was not ready after %s", pkg, engineReadyWait)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(enginePoll):
		}
		method = http.MethodGet // opened once; poll from here
	}
}

// engineFilePrefix is where the file proxy path names an engine:
// /api/gradio-file-proxy/engine/<pkg>/<path in the engine>.
const engineFilePrefix = "engine/"

// engineOwnerPkgs is the set of packages in engineOwners.
func engineOwnerPkgs() map[string]bool {
	out := map[string]bool{}
	for _, p := range engineOwners {
		out[p] = true
	}
	return out
}

// logEngineTail writes the last lines of an engine's output to the launcher
// log after one of its calls failed: the browser only gets "Internal Server
// Error", and the engine's own error is otherwise only on the token-only
// control API.
func logEngineTail(ctx context.Context, core *Core, pkg string) {
	resp, err := core.do(ctx, http.MethodGet, "/api/extensions/"+pkg+"/logs?tail=40", nil, "")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var out struct {
		Lines []string `json:"lines"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if json.Unmarshal(raw, &out) != nil || len(out.Lines) == 0 {
		return
	}
	slog.Warn("engine output after a failed call", "pkg", pkg, "tail", strings.Join(out.Lines, "\n"))
}
