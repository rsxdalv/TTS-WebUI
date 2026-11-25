package launcher

// Extension management that works with the app stopped. It replaces the
// extension_manager.py process (removed in step 29): the launcher mounts the
// same extcatalog (lists, pip/uv installs) and appconfig (enable/disable)
// stores the app's control API uses, behind the launcher's own token guard.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/detect"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/gradiohost"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/requires"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/uv"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/piplist"
)

// extOps are the extcatalog operations the Extensions page uses. The launcher
// does not pass the others (parse/add/install-entry take pasted text).
var extOps = map[string]bool{
	"install": true, "uninstall": true, "check-dependencies": true, "latest-version": true,
}

// extHost is the launcher's own set of stores. The app's control API has
// another set with the app's environment; both write the same files, each under
// its own lock, and an install while the app runs takes effect at its restart.
type extHost struct {
	root  paths.Root
	token string // internal: the adapter adds it after the launcher's guard
	cfg   *appconfig.Store
	cat   *extcatalog.Store
	pip   *piplist.Store
	// platform is what the stores run on.
	platform platform.Platform
}

func newExtHost(root paths.Root, systemPython bool, appRunning func() bool) (*extHost, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating the extension token: %w", err)
	}
	token := hex.EncodeToString(b)
	p := detect.Detect(detect.Options{Root: root, SystemPython: systemPython})
	logf := func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) }
	cfg := appconfig.New(appconfig.Options{Root: root.Dir(), Token: token, Logf: logf})
	pip := piplist.New(piplist.Options{
		Root: root.Dir(), Token: token,
		Platform: p, Logf: logf,
	})
	return &extHost{
		root: root, token: token, cfg: cfg,
		cat: extcatalog.New(extcatalog.Options{
			Root: root.Dir(), Token: token, Config: cfg, Pip: pip,
			Platform: p, UV: uv.New(uv.Options{Root: root}), Host: detect.Host(p), AppRunning: appRunning, Logf: logf,
			StopEngine: func(pkg string) bool { return stopRunningEngine(root.Dir(), pkg) },
		}),
		pip: pip, platform: p,
	}, nil
}

// extensions builds the stores on first use, so a launcher that never opens the
// Extensions page never touches them.
func (l *Launcher) extensions() (*extHost, error) {
	l.extOnce.Do(func() { l.ext, l.extErr = newExtHost(l.root, l.systemPython, l.sup.Running) })
	return l.ext, l.extErr
}

// extItem is one row of the Extensions page.
type extItem struct {
	PackageName string `json:"package_name"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Class       string `json:"extension_class"`
	Kind        string `json:"kind"` // interface or decorator
	Proxy       string `json:"proxy"`
	// Validation is "full" when an own-venv engine generated real audio in P6,
	// "unvalidated" when it only passed the install and start checks, "" otherwise.
	Validation    string `json:"validation"`
	Builtin       bool   `json:"builtin"`
	Installed     bool   `json:"installed"`
	Disabled      bool   `json:"disabled"`
	CanBeDisabled bool   `json:"can_be_disabled"`
	// Requires is what the entry needs first (its own list or the derived one),
	// and RequirementsState says for each name whether it is there now.
	Requires          []string       `json:"requires"`
	RequirementsState []requires.Req `json:"requirements_state"`
}

type extListAnswer struct {
	Extensions []extItem `json:"extensions"`
	AppRunning bool      `json:"app_running"`
	Warning    string    `json:"warning,omitempty"`
	// Platform is the runtime this install runs on ("legacy-env", "system"), and
	// PlatformReady whether its Python can start; PlatformDetail says why not.
	Platform       string `json:"platform"`
	PlatformReady  bool   `json:"platform_ready"`
	PlatformDetail string `json:"platform_detail,omitempty"`
	// RequiresError is set when the catalog's requires cannot be read (a bad
	// name); the rows then carry no requirements.
	RequiresError string `json:"requires_error,omitempty"`
}

// list answers the Extensions page: interface and decorator extensions with
// their installed and disabled state.
func (h *extHost) list(ctx context.Context, appRunning bool) (extListAnswer, error) {
	ans := extListAnswer{AppRunning: appRunning, Extensions: []extItem{}}
	lists, err := h.cfg.ExtensionLists()
	if err != nil {
		return ans, err
	}
	st := h.platform.Status(ctx)
	ans.Platform, ans.PlatformReady, ans.PlatformDetail = h.platform.Name(), st.Ready, st.Detail
	graph, gerr := h.cat.RequiresGraph(ctx)
	if gerr != nil {
		ans.RequiresError = gerr.Error()
	}
	installs := h.cat.Installs(ctx)
	if installs.Err != nil {
		// Without pip's answer, say so and show everything as not installed
		// except what needs no pip.
		ans.Warning = "Could not read the installed packages: " + installs.Err.Error()
	}
	disabled := map[string]bool{}
	for _, n := range lists.Disabled {
		disabled[n] = true
	}
	if h.cat.HostActive() && gerr == nil {
		// The Gradio host is an entry of its own: it is not in extensions.json.
		ans.Extensions = append(ans.Extensions, extItem{
			PackageName: gradiohost.Name, Name: gradiohost.Title,
			Description: "The Gradio web interface and the tabs that run inside it. Optional: the Go pages, the engines and the OpenAI-style endpoints work without it.",
			Class:       "host", Kind: "host", Installed: graph.Entries[gradiohost.Name].Installed,
			Requires: graph.Entries[gradiohost.Name].Requires, RequirementsState: graph.State(gradiohost.Name),
		})
	}
	for _, kind := range []string{"interface", "decorator"} {
		entries, err := h.cat.List(kind)
		if err != nil {
			return ans, err
		}
		for _, e := range entries {
			pkg := e.Str("package_name")
			if pkg == "" {
				continue
			}
			proxy := e.Str("proxy")
			installed, _ := installs.Of(pkg, proxy)
			var req []string
			var reqState []requires.Req
			if gerr == nil {
				req, reqState = graph.Entries[pkg].Requires, graph.State(pkg)
			}
			if req == nil {
				req, reqState = []string{}, []requires.Req{}
			}
			ans.Extensions = append(ans.Extensions, extItem{
				PackageName: pkg, Name: nonEmpty(e.Str("name"), pkg),
				Description: e.Str("description"), Website: e.Str("website"),
				Class: e.Str("extension_class"), Kind: kind, Proxy: proxy, Validation: e.Str("validation"),
				Builtin:       strings.Contains(pkg, "builtin"),
				Installed:     installed,
				Disabled:      kind == "interface" && disabled[pkg],
				CanBeDisabled: kind == "interface",
				Requires:      req, RequirementsState: reqState,
			})
		}
	}
	return ans, nil
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func writeExtJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (l *Launcher) handleExtList() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, err := l.extensions()
		if err != nil {
			writeExtJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
		defer cancel()
		ans, err := h.list(ctx, l.sup.Running())
		if err != nil {
			writeExtJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeExtJSON(w, http.StatusOK, ans)
	})
}

type extEnabledRequest struct {
	PackageName string `json:"package_name"`
	Enabled     bool   `json:"enabled"`
}

// handleExtEnabled enables or disables an interface extension in config.json.
func (l *Launcher) handleExtEnabled() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, err := l.extensions()
		if err != nil {
			writeExtJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		var req extEnabledRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || req.PackageName == "" {
			writeExtJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be {package_name, enabled}"})
			return
		}
		if _, err := h.cfg.SetExtensionDisabled(req.PackageName, !req.Enabled); err != nil {
			code := http.StatusInternalServerError
			if appconfig.IsInvalid(err) {
				code = http.StatusBadRequest
			}
			writeExtJSON(w, code, map[string]string{"error": err.Error()})
			return
		}
		writeExtJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
}

// handleExtOp passes install, uninstall and the dry runs to extcatalog's own
// handler. The launcher's token guard has already run, so the adapter adds the
// store's internal token and marks the request loopback, which the store checks.
func (l *Launcher) handleExtOp() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := r.PathValue("op")
		if !extOps[op] {
			writeExtJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		h, err := l.extensions()
		if err != nil {
			writeExtJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = extcatalog.Prefix + op
		r2.Header.Set(extcatalog.TokenHeader, h.token)
		r2.RemoteAddr = "127.0.0.1:0"
		h.cat.Handler().ServeHTTP(w, r2)
	})
}

// registerExtensions mounts the Extensions page's API.
func (l *Launcher) registerExtensions(srv *server.Server) {
	srv.Handle("GET /api/ext/list", l.handleExtList())
	srv.Handle("POST /api/ext/enabled", l.handleExtEnabled())
	srv.Handle("POST /api/ext/op/{op}", l.handleExtOp())
}
