// Package engines is the control API behind the Vite engine page: opening the
// page starts an installed own-venv engine and shows its own Gradio page (D27,
// D31). Opening is the one process start that a browser on port 3000 can
// cause, so it is limited to engines the catalog names and that are installed
// (extcatalog.Engine), and it only starts: the stop, the logs and the install
// stay on the token-only control API.
//
//	POST /api/engines/{pkg}/open  start (or find) the engine, answer its state
//	GET  /api/engines/{pkg}       the state, for polling until "ready"
//
// The engine's page is served by the router at /{pkg}/ on the public port; the
// answer carries that port so the page can build the iframe address.
package engines

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extprocs"
)

const (
	// Prefix is where the handler is mounted on the control API.
	Prefix      = "/api/engines/"
	tokenHeader = "X-TTS-Core-Token"
)

// Catalog is what the handler asks the extension catalog.
type Catalog interface {
	Engine(pkg string) (name string, err error)
	Engines() ([]extcatalog.InstalledEngine, error)
}

// Procs is what the handler asks the extension process manager.
type Procs interface {
	Start(ctx context.Context, pkg string, req extprocs.StartRequest) (extprocs.StartResult, error)
	List() []extprocs.Status
}

// Options configure a Handler.
type Options struct {
	Cat   Catalog
	Procs Procs
	Token string
	// PublicPort is the port the router listens on, where /{pkg}/ is served.
	PublicPort int
}

// Answer is the JSON body of both routes.
type Answer struct {
	Package string `json:"package"`
	Name    string `json:"name"`
	// State is "stopped" (never started or exited cleanly), "starting",
	// "ready", "failed" or "exited".
	State      extprocs.State `json:"state"`
	Error      string         `json:"error,omitempty"`
	PublicPort int            `json:"public_port"`
	// Path is where the router serves the engine's page, with a trailing slash.
	Path string `json:"path"`
}

// Handler serves the two routes. Mount it at Prefix.
type Handler struct{ o Options }

// New returns a Handler.
func New(o Options) *Handler { return &Handler{o: o} }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (h *Handler) authorized(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() || h.o.Token == "" {
		return false
	}
	got := sha256.Sum256([]byte(r.Header.Get(tokenHeader)))
	want := sha256.Sum256([]byte(h.o.Token))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, Prefix)
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	pkg, action, _ := strings.Cut(rest, "/")
	var want string
	switch action {
	case "":
		want = http.MethodGet
	case "open":
		want = http.MethodPost
	default:
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != want {
		w.Header().Set("Allow", want)
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := extprocs.ValidatePackage(pkg); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := h.o.Cat.Engine(pkg)
	if err != nil {
		code := http.StatusInternalServerError
		switch {
		case errors.Is(err, extcatalog.ErrNotEngine):
			code = http.StatusNotFound
		case errors.Is(err, extcatalog.ErrNotInstalled):
			code = http.StatusConflict
		}
		writeErr(w, code, err.Error())
		return
	}
	if action == "open" {
		if _, err := h.o.Procs.Start(r.Context(), pkg, extprocs.StartRequest{}); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, h.answer(pkg, name))
}

func (h *Handler) answer(pkg, name string) Answer {
	a := Answer{Package: pkg, Name: name, State: "stopped", PublicPort: h.o.PublicPort, Path: "/" + pkg + "/"}
	for _, st := range h.o.Procs.List() {
		if st.Package == pkg {
			a.State, a.Error = st.State, st.Error
		}
	}
	return a
}
