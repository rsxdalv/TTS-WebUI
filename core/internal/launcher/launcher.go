// Package launcher implements the step-by-step installation HTTP API used
// by the Go launcher (tts-webui launch).
//
// It exposes typed step endpoints on the internal/server.Server and
// implements each step doing what the legacy installer path does today
// (legacy-compat mode, C3.1). The Supervisor (C3.2) manages the running
// app processes after installation.
package launcher

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

// StepStatus is the JSON shape returned by GET /api/steps/status.
type StepStatus struct {
	Step string `json:"step"`
	Done bool   `json:"done"`
}

// Launcher holds shared mutable state for the step handlers.
type Launcher struct {
	root    paths.Root
	envMode string
	mu      sync.Mutex
	sprocs  map[string]*procs.Proc // subprocess steps: mamba, install, update, run
	slogs   map[string]*Log        // log-only steps: download
	sup     *Supervisor            // C3.2: app process supervisor
}

// LauncherOptions holds optional configuration for the Launcher.
type LauncherOptions struct {
	// EnvMode selects the Python environment: "conda" (default) or "uv".
	EnvMode string
}

// New creates a Launcher for the given install root with default options.
func New(root paths.Root) *Launcher {
	return NewWithOptions(root, LauncherOptions{})
}

// NewWithOptions creates a Launcher with explicit options.
func NewWithOptions(root paths.Root, opts LauncherOptions) *Launcher {
	mode := opts.EnvMode
	if mode == "" {
		mode = "conda"
	}
	return &Launcher{
		root:    root,
		envMode: mode,
		sprocs:  make(map[string]*procs.Proc),
		slogs:   make(map[string]*Log),
		sup:     newSupervisor(root, mode),
	}
}

// Register adds all /api/steps/* and /api/app/* endpoints to srv.
// The server's existing token guard covers these because they are registered
// under /api/.
func (l *Launcher) Register(srv *server.Server) {
	srv.Handle("GET /api/steps/status", l.handleStatus())

	srv.Handle("POST /api/steps/download", l.handleDownload())
	srv.Handle("GET /api/steps/download/stream", l.handleLogStream("download"))

	srv.Handle("POST /api/steps/preflight", l.handlePreflight())

	srv.Handle("POST /api/steps/mamba", l.handleMamba())
	srv.Handle("GET /api/steps/mamba/stream", l.handleProcStream("mamba"))

	srv.Handle("POST /api/steps/gpu", l.handleGPU())

	srv.Handle("POST /api/steps/install", l.handleInstall())
	srv.Handle("GET /api/steps/install/stream", l.handleProcStream("install"))

	srv.Handle("POST /api/steps/update", l.handleUpdate())
	srv.Handle("GET /api/steps/update/stream", l.handleProcStream("update"))

	srv.Handle("POST /api/steps/run", l.handleRun())
	srv.Handle("GET /api/steps/run/stream", l.handleProcStream("run"))

	// C3.2: app supervisor endpoints (start/stop/restart/status/stream).
	// These replace the legacy /poll endpoint with SSE.
	srv.Handle("POST /api/app/start", l.handleAppStart())
	srv.Handle("POST /api/app/stop", l.handleAppStop())
	srv.Handle("POST /api/app/restart", l.handleAppRestart())
	srv.Handle("GET /api/app/status", l.handleAppStatus())
	srv.Handle("GET /api/app/status/stream", l.handleAppStatusStream())
}

// handleStatus returns [{step, done}] for all 7 steps.
func (l *Launcher) handleStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statuses := []StepStatus{
			{Step: "download", Done: downloadDone(l.root)},
			{Step: "preflight", Done: preflightDone(l.root)},
			{Step: "mamba", Done: mambaDone(l.root)},
			{Step: "gpu", Done: gpuDone(l.root)},
			{Step: "install", Done: installDone(l.root)},
			{Step: "update", Done: installDone(l.root)}, // proxy for update check
			{Step: "run", Done: false},                  // never persisted
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statuses)
	})
}

// --- state helpers ---

func (l *Launcher) setProc(step string, p *procs.Proc) {
	l.mu.Lock()
	l.sprocs[step] = p
	l.mu.Unlock()
}

func (l *Launcher) getProc(step string) *procs.Proc {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sprocs[step]
}

func (l *Launcher) setLog(step string, lg *Log) {
	l.mu.Lock()
	l.slogs[step] = lg
	l.mu.Unlock()
}

func (l *Launcher) getLog(step string) *Log {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.slogs[step]
}

// --- stream handlers ---

// handleProcStream is an SSE handler for subprocess-backed steps.
// It replays buffered history then streams live output.
func (l *Launcher) handleProcStream(step string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := l.getProc(step)
		if p == nil {
			http.Error(w, "no active process for "+step, http.StatusNotFound)
			return
		}
		// Subscribe before reading history to avoid missing lines.
		live, unsub := p.Subscribe()
		defer unsub()

		events := make(chan string, 512)
		go func() {
			defer close(events)
			for _, line := range p.Lines() {
				select {
				case events <- line:
				case <-r.Context().Done():
					return
				}
			}
			for {
				select {
				case line, ok := <-live:
					if !ok {
						return
					}
					select {
					case events <- line:
					case <-r.Context().Done():
						return
					}
				case <-r.Context().Done():
					return
				}
			}
		}()
		server.SSE(w, r, events)
	})
}

// --- C3.2: app supervisor HTTP handlers ---

func (l *Launcher) handleAppStart() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := l.sup.Start(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

func (l *Launcher) handleAppStop() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.sup.Stop()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

func (l *Launcher) handleAppRestart() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := l.sup.Restart(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

func (l *Launcher) handleAppStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(l.sup.Statuses())
	})
}

// handleAppStatusStream replaces the legacy /poll endpoint with an SSE stream.
// Clients receive a JSON ProcessStatus slice whenever a process starts or stops.
func (l *Launcher) handleAppStatusStream() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ch, cancel := l.sup.SubscribeStatus()
		defer cancel()
		server.SSE(w, r, ch)
	})
}
func (l *Launcher) handleLogStream(step string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lg := l.getLog(step)
		if lg == nil {
			http.Error(w, "no active download", http.StatusNotFound)
			return
		}
		live, unsub := lg.Subscribe()
		defer unsub()

		events := make(chan string, 512)
		go func() {
			defer close(events)
			for _, line := range lg.Lines() {
				select {
				case events <- line:
				case <-r.Context().Done():
					return
				}
			}
			for {
				select {
				case line, ok := <-live:
					if !ok {
						return
					}
					select {
					case events <- line:
					case <-r.Context().Done():
						return
					}
				case <-r.Context().Done():
					return
				}
			}
		}()
		server.SSE(w, r, events)
	})
}
