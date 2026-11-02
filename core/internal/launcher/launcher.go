// Package launcher implements the step-by-step installation HTTP API used
// by the Go launcher (tts-webui launch).
//
// It exposes typed step endpoints on the internal/server.Server and
// implements each step doing what the legacy installer path does today
// (legacy-compat mode, C3.1). The Supervisor (C3.2) manages the running
// app processes after installation.
package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

// StepStatus is the JSON shape returned by GET /api/steps/status.
type StepStatus struct {
	Step    string `json:"step"`
	Done    bool   `json:"done"`
	Running bool   `json:"running,omitempty"`
}

// Launcher holds shared mutable state for the step handlers.
type Launcher struct {
	root         paths.Root
	envMode      string
	mu           sync.Mutex
	sprocs       map[string]*procs.Proc // subprocess steps: mamba, install, update, run
	slogs        map[string]*Log        // log-only steps: download
	sup          *Supervisor            // C3.2: app process supervisor
	updateOnce   sync.Once
	updateChecked bool // true = already up to date (cached after first check)
}

// LauncherOptions holds optional configuration for the Launcher.
type LauncherOptions struct {
	// EnvMode selects the Python environment: "conda" (default) or "uv".
	EnvMode string
	// ProxyMode selects how proxy extensions are started: "python" (default) or "core".
	// In "core" mode the lifecycle manager starts server.py and native extensions.
	ProxyMode string
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
	srv.Handle("POST /api/steps/preflight/fix/long-paths", l.handlePreflightFixLongPaths())
	srv.Handle("POST /api/steps/preflight/fix/msvc", l.handlePreflightFixMSVC())

	srv.Handle("POST /api/steps/mamba", l.handleMamba())
	srv.Handle("GET /api/steps/mamba/stream", l.handleProcStream("mamba"))

	srv.Handle("POST /api/steps/gpu", l.handleGPU())

	srv.Handle("POST /api/steps/install", l.handleInstall())
	srv.Handle("GET /api/steps/install/stream", l.handleProcStream("install"))

	srv.Handle("POST /api/steps/update", l.handleUpdate())
	srv.Handle("GET /api/steps/update/stream", l.handleProcStream("update"))

	// C3.2: app supervisor endpoints (start/stop/restart/status/stream).
	// These replace the legacy /poll endpoint with SSE.
	srv.Handle("POST /api/app/start", l.handleAppStart())
	srv.Handle("POST /api/app/stop", l.handleAppStop())
	srv.Handle("POST /api/app/restart", l.handleAppRestart())
	srv.Handle("GET /api/app/status", l.handleAppStatus())
	srv.Handle("GET /api/app/status/stream", l.handleAppStatusStream())
	srv.Handle("GET /api/app/link", l.handleAppLink())
}

// handleStatus returns [{step, done, running}] for all steps.
func (l *Launcher) handleStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For the update step: run isGitUpdated once and cache the result.
		updateDone := false
		if installDone(l.root) {
			l.updateOnce.Do(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				updated, err := isGitUpdated(ctx, l.root)
				l.updateChecked = (err == nil && updated)
			})
			updateDone = l.updateChecked
		}

		statuses := []StepStatus{
			{Step: "download", Done: downloadDone(l.root)},
			{Step: "preflight", Done: preflightDone(l.root)},
			{Step: "mamba", Done: mambaDone(l.root), Running: l.isStepRunning("mamba")},
			{Step: "gpu", Done: gpuDone(l.root)},
			{Step: "install", Done: installDone(l.root), Running: l.isStepRunning("install")},
			{Step: "update", Done: updateDone, Running: l.isStepRunning("update")},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statuses)
	})
}

// isStepRunning returns true if the subprocess for the given step is alive.
func (l *Launcher) isStepRunning(step string) bool {
	l.mu.Lock()
	p := l.sprocs[step]
	l.mu.Unlock()
	if p == nil {
		return false
	}
	return p.Running()
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
// Headers are flushed immediately so EventSource does not hang.
// Buffered history is replayed for reconnecting clients.
// Ends with "event: done" carrying the exit code when the process exits.
func (l *Launcher) handleProcStream(step string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := l.getProc(step)
		if p == nil {
			http.Error(w, "no active process for "+step, http.StatusNotFound)
			return
		}

		flusher, ok := server.SSEStart(w)
		if !ok {
			return
		}

		// Subscribe before reading history to avoid a race on new lines.
		live, unsub := p.Subscribe()
		defer unsub()

		// Replay buffered history.
		for _, line := range p.Lines() {
			if r.Context().Err() != nil {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
		}
		flusher.Flush()

		// Stream live lines; also watch for process exit.
		procDone := p.Done()
		for {
			select {
			case <-r.Context().Done():
				return
			case line, ok := <-live:
				if !ok {
					server.WriteSSEDone(w, flusher, p.ExitCode())
					return
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			case <-procDone:
				// Drain any lines that arrived just before the channel closed.
				for {
					select {
					case line, ok := <-live:
						if !ok {
							goto sendDone
						}
						_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
					default:
						goto sendDone
					}
				}
			sendDone:
				flusher.Flush()
				server.WriteSSEDone(w, flusher, p.ExitCode())
				return
			}
		}
	})
}

// --- C3.2: app supervisor HTTP handlers ---

func (l *Launcher) handleAppStart() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := l.sup.Start(context.Background()); err != nil {
			if err.Error() == "app already running" {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
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
		if err := l.sup.Restart(context.Background()); err != nil {
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

// handleAppLink returns the Gradio URL when reachable, or an empty url field.
func (l *Launcher) handleAppLink() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := l.sup.AppLink()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			URL string `json:"url"`
		}{URL: url})
	})
}

func (l *Launcher) handleLogStream(step string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lg := l.getLog(step)
		if lg == nil {
			http.Error(w, "no active download", http.StatusNotFound)
			return
		}

		flusher, ok := server.SSEStart(w)
		if !ok {
			return
		}

		live, unsub := lg.Subscribe()
		defer unsub()

		// Replay buffered history.
		for _, line := range lg.Lines() {
			if r.Context().Err() != nil {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
		}
		flusher.Flush()

		logDone := lg.Done()
		for {
			select {
			case <-r.Context().Done():
				return
			case line, ok := <-live:
				if !ok {
					server.WriteSSEDone(w, flusher, 0)
					return
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			case <-logDone:
				// Drain remaining buffered lines.
				for {
					select {
					case line, ok := <-live:
						if !ok {
							goto logSendDone
						}
						_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
					default:
						goto logSendDone
					}
				}
			logSendDone:
				flusher.Flush()
				server.WriteSSEDone(w, flusher, 0)
				return
			}
		}
	})
}
