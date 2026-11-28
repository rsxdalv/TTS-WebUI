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
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/supervise"
)

// StepStatus is the JSON shape returned by GET /api/steps/status.
type StepStatus struct {
	Step    string `json:"step"`
	Done    bool   `json:"done"`
	Running bool   `json:"running,omitempty"`
	// Detail is a line to show with the step: an update waiting, or why it could
	// not be checked.
	Detail string `json:"detail,omitempty"`
	// Checking is true while the answer is still being worked out (the update
	// check asks the network); the page asks again shortly.
	Checking bool `json:"checking,omitempty"`
}

// Launcher holds shared mutable state for the step handlers.
type Launcher struct {
	root   paths.Root
	mu     sync.Mutex
	sprocs map[string]*procs.Proc // subprocess steps: mamba, install, update, run
	slogs  map[string]*Log        // log-only steps: download
	stepMu sync.Mutex             // held while a step request checks and starts (R2-3)
	sup    appSupervisor          // C3.2: app process supervisor

	updMu    sync.Mutex // guards updState and updAt (the update step's cached answer)
	updState updateState
	updAt    time.Time
	updBusy  bool // a background check is running

	systemPython bool      // the system platform (WithSystemPython)
	extOnce      sync.Once // the Extensions page's stores, built on first use
	ext          *extHost
	extErr       error
}

// New creates a Launcher for the given install root.
func New(root paths.Root, opts ...Option) *Launcher {
	migrateFrontendDir(root.Dir())
	launcherPorts.Store(root.Dir(), buildOptions(opts).launcherPort)
	return &Launcher{
		root:         root,
		sprocs:       make(map[string]*procs.Proc),
		slogs:        make(map[string]*Log),
		sup:          newAppSupervisor(root, opts...),
		systemPython: buildOptions(opts).systemPython,
	}
}

// StartApp starts the app, as the Start button does. It returns once the
// children are started, not when they are ready. With echo set (a headless
// start: Docker logs, a Colab cell) the output of the Gradio and React children
// is copied to it; the copy ends with the children, so a Restart is not echoed.
func (l *Launcher) StartApp(ctx context.Context, echo io.Writer) error {
	if err := l.sup.Start(ctx); err != nil {
		return err
	}
	if echo == nil {
		return nil
	}
	for _, name := range []string{supervise.NameGradio, supervise.NameReact} {
		lines, cancel, ok := l.sup.SubscribeLogs(name)
		if !ok {
			continue
		}
		go func(name string) {
			defer cancel()
			for line := range lines {
				if name == supervise.NameGradio {
					fmt.Fprintln(echo, line)
				} else {
					fmt.Fprintf(echo, "[%s] %s\n", name, line)
				}
			}
		}(name)
	}
	return nil
}

// Shutdown stops the app and any running install step, for a launcher that
// is exiting (Ctrl+C, SIGTERM, SIGHUP). It returns when they are gone.
func (l *Launcher) Shutdown() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		l.sup.Stop()
	}()
	l.mu.Lock()
	var steps []*procs.Proc
	for _, p := range l.sprocs {
		if p != nil && p.Running() {
			steps = append(steps, p)
		}
	}
	l.mu.Unlock()
	for _, p := range steps {
		wg.Add(1)
		go func(p *procs.Proc) {
			defer wg.Done()
			_ = p.Stop()
		}(p)
	}
	wg.Wait()
}

// CleanupOrphans stops what a killed launcher left running for this install
// (Linux and macOS; Windows job objects leave nothing). It returns how many
// processes it stopped.
func (l *Launcher) CleanupOrphans() int { return sweepOrphans(l.root) }

// Register adds all /api/steps/* and /api/app/* endpoints to srv.
// The server's existing token guard covers these because they are registered
// under /api/.
func (l *Launcher) Register(srv *server.Server) {
	srv.Handle("GET /api/steps/status", l.handleStatus())
	srv.Handle("GET /api/app/notice", l.handleNotice())

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
	srv.Handle("GET /api/steps/update/stream", l.handleUpdateStream())

	// C3.2: app supervisor endpoints (start/stop/restart/status/stream).
	// These replace the legacy /poll endpoint with SSE.
	srv.Handle("POST /api/app/start", l.handleAppStart())
	srv.Handle("POST /api/app/stop", l.handleAppStop())
	srv.Handle("POST /api/app/restart", l.handleAppRestart())
	srv.Handle("GET /api/app/status", l.handleAppStatus())
	srv.Handle("GET /api/app/status/stream", l.handleAppStatusStream())
	srv.Handle("GET /api/app/link", l.handleAppLink())
	srv.Handle("GET /api/app/lan", l.handleAppLAN())

	// Extensions page: list, install, uninstall, enable and disable, with the app stopped.
	l.registerExtensions(srv)
	l.registerStorage(srv)

	// Diagnostic: runs installer_scripts/tools/diagnostic.{bat,sh}.
	srv.Handle("POST /api/app/diagnostic", l.handleAppDiagnostic())
	srv.Handle("GET /api/app/diagnostic/stream", l.handleProcStream("diagnostic"))
}

// handleStatus returns [{step, done, running, detail}] for all steps.
func (l *Launcher) handleStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The update step: an install with the old env asks git, any other folder asks
		// the archive's published description. The answer is cached for a minute.
		upd, updChecking := l.updateStatus()
		// Also done when the legacy update proc ran and exited cleanly.
		if !upd.Done {
			if p := l.getProc("update"); p != nil && !p.Running() && p.ExitCode() == 0 {
				upd = updateState{Done: true}
			}
		}
		updRunning := l.isStepRunning("update") || l.updateRunning()

		// A running step is never reported done, so a reload during it shows it
		// running and keeps the later steps unavailable (R2-3).
		dlRunning := l.downloadRunning()
		mambaRunning := l.isStepRunning("mamba")
		installRunning := l.isStepRunning("install")
		statuses := []StepStatus{
			{Step: "download", Done: downloadDone(l.root) && !dlRunning, Running: dlRunning},
			{Step: "preflight", Done: preflightDone(l.root)},
		}
		// The steps that set up the old env belong to an install that has one.
		// A new folder shows download, pre-flight and update (P4).
		if l.legacy() {
			statuses = append(statuses,
				StepStatus{Step: "mamba", Done: mambaDone(l.root) && !mambaRunning, Running: mambaRunning},
				StepStatus{Step: "gpu", Done: gpuDone(l.root)},
				StepStatus{Step: "install", Done: installDone(l.root) && !installRunning, Running: installRunning},
			)
		}
		statuses = append(statuses, StepStatus{Step: "update", Done: upd.Done && !updRunning, Running: updRunning, Detail: upd.Detail, Checking: updChecking})
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
					server.WriteSSEDone(w, flusher, lg.ExitCode())
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
				server.WriteSSEDone(w, flusher, lg.ExitCode())
				return
			}
		}
	})
}

// handleUpdateStream follows the archive update (a log) when there is one, the
// old env install's update process otherwise.
func (l *Launcher) handleUpdateStream() http.Handler {
	logStream, procStream := l.handleLogStream("update"), l.handleProcStream("update")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l.getLog("update") != nil {
			logStream.ServeHTTP(w, r)
			return
		}
		procStream.ServeHTTP(w, r)
	})
}
