// Package lifecycle manages engine process lifecycle: on-demand start,
// health-checking, idle stop, and cancel.
package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

// EngineStatus is a point-in-time snapshot of an engine's state.
type EngineStatus struct {
	Running bool `json:"running"`
	Port    int  `json:"port"`
	PID     int  `json:"pid"`
}

// entry holds the runtime state for one registered engine.
type entry struct {
	mu       sync.Mutex
	man      manifest.Manifest
	proc     *procs.Proc
	port     int
	pid      int
	running  bool
	cancelFn context.CancelFunc
	idleStop *time.Timer
}

// Manager manages on-demand engine processes.
type Manager struct {
	mu      sync.Mutex
	engines map[string]*entry
}

// New creates a Manager.
func New() *Manager {
	return &Manager{engines: make(map[string]*entry)}
}

// Register adds a manifest to the manager. Returns an error if the engine ID
// is already registered.
func (m *Manager) Register(man manifest.Manifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.engines[man.PackageName]; exists {
		return fmt.Errorf("engine %q already registered", man.PackageName)
	}
	m.engines[man.PackageName] = &entry{man: man}
	return nil
}

// Start assigns a free OS port, spawns the engine process, and waits for its
// health endpoint. Returns an error if the engine is already running or if the
// health check times out.
func (m *Manager) Start(ctx context.Context, id string) error {
	e := m.getEntry(id)
	if e == nil {
		return fmt.Errorf("engine %q not registered", id)
	}

	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine %q already running", id)
	}
	man := e.man
	e.mu.Unlock()

	port, err := freePort()
	if err != nil {
		return fmt.Errorf("finding free port for engine %q: %w", id, err)
	}

	engineCtx, cancel := context.WithCancel(ctx)

	proc := &procs.Proc{}
	args, env := buildLaunchArgs(man, port)
	if len(args) == 0 {
		cancel()
		return fmt.Errorf("engine %q has no launch command", id)
	}

	cwd := ""
	if man.Engine.Launch != nil {
		cwd = man.Engine.Launch.Cwd
	}

	if err := proc.Start(engineCtx, args[0], args[1:], cwd, env); err != nil {
		cancel()
		return fmt.Errorf("starting engine %q: %w", id, err)
	}

	if err := waitForHealth(engineCtx, man, port); err != nil {
		_ = proc.Stop()
		cancel()
		return fmt.Errorf("engine %q health check: %w", id, err)
	}

	e.mu.Lock()
	e.proc = proc
	e.port = port
	e.pid = proc.PID()
	e.cancelFn = cancel
	e.running = true

	// Start idle timer if keep_alive is configured.
	if man.Engine.KeepAlive > 0 {
		ka := time.Duration(man.Engine.KeepAlive) * time.Second
		e.idleStop = time.AfterFunc(ka, func() {
			slog.Info("engine idle timeout; stopping", "id", id)
			_ = m.Stop(id)
		})
	}
	e.mu.Unlock()

	// Watch for unexpected exit and clear running state.
	go func() {
		proc.Wait()
		e.mu.Lock()
		e.running = false
		if e.idleStop != nil {
			e.idleStop.Stop()
			e.idleStop = nil
		}
		e.mu.Unlock()
		cancel()
	}()

	return nil
}

// Stop sends SIGINT then tree-kills the engine.
func (m *Manager) Stop(id string) error {
	e := m.getEntry(id)
	if e == nil {
		return fmt.Errorf("engine %q not registered", id)
	}

	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return nil
	}
	proc := e.proc
	if e.idleStop != nil {
		e.idleStop.Stop()
		e.idleStop = nil
	}
	e.mu.Unlock()

	return proc.Stop()
}

// Cancel implements the manifest's cancel mode:
//   - "disconnect": cancels the engine's context
//   - "endpoint": POSTs to the manifest's cancel.path
func (m *Manager) Cancel(ctx context.Context, id string) error {
	e := m.getEntry(id)
	if e == nil {
		return fmt.Errorf("engine %q not registered", id)
	}

	e.mu.Lock()
	man := e.man
	port := e.port
	cancelFn := e.cancelFn
	e.mu.Unlock()

	if man.Engine.Cancel == nil {
		return fmt.Errorf("engine %q has no cancel configuration", id)
	}

	switch man.Engine.Cancel.Mode {
	case manifest.CancelModeDisconnect:
		if cancelFn != nil {
			cancelFn()
		}
		return nil
	case manifest.CancelModeEndpoint:
		url := fmt.Sprintf("http://127.0.0.1:%d%s", port, man.Engine.Cancel.Path)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return fmt.Errorf("building cancel request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("posting to cancel endpoint: %w", err)
		}
		resp.Body.Close()
		return nil
	default:
		return fmt.Errorf("unknown cancel mode %q for engine %q", man.Engine.Cancel.Mode, id)
	}
}

// Status returns the current state of the engine.
func (m *Manager) Status(id string) EngineStatus {
	e := m.getEntry(id)
	if e == nil {
		return EngineStatus{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return EngineStatus{
		Running: e.running,
		Port:    e.port,
		PID:     e.pid,
	}
}

// LogStream returns a channel of output lines from the engine and a cancel func.
// Returns a closed channel if the engine is not registered or not running.
func (m *Manager) LogStream(id string) (<-chan string, func()) {
	e := m.getEntry(id)
	if e == nil {
		return closedChan(), func() {}
	}
	e.mu.Lock()
	proc := e.proc
	e.mu.Unlock()
	if proc == nil {
		return closedChan(), func() {}
	}
	return proc.Subscribe()
}

// Touch resets the idle stop timer for an engine.
// The router calls this on every proxied request.
func (m *Manager) Touch(id string) {
	e := m.getEntry(id)
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.idleStop != nil && e.man.Engine.KeepAlive > 0 {
		ka := time.Duration(e.man.Engine.KeepAlive) * time.Second
		e.idleStop.Reset(ka)
	}
}

// IDs returns the IDs of all registered engines.
func (m *Manager) IDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.engines))
	for id := range m.engines {
		ids = append(ids, id)
	}
	return ids
}

// UpdateManifest replaces the registered manifest for a stopped engine.
// Returns an error if the engine is not registered or is currently running.
func (m *Manager) UpdateManifest(id string, man manifest.Manifest) error {
	e := m.getEntry(id)
	if e == nil {
		return fmt.Errorf("engine %q not registered", id)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return fmt.Errorf("engine %q is running; stop before updating manifest", id)
	}
	e.man = man
	return nil
}

// Manifest returns the registered manifest for id.
func (m *Manager) Manifest(id string) (manifest.Manifest, bool) {
	e := m.getEntry(id)
	if e == nil {
		return manifest.Manifest{}, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.man, true
}

// getEntry returns the entry for id, or nil if not registered.
func (m *Manager) getEntry(id string) *entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.engines[id]
}

// closedChan returns an already-closed string channel.
func closedChan() <-chan string {
	ch := make(chan string)
	close(ch)
	return ch
}

// freePort asks the OS for a free TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// buildLaunchArgs constructs the command-line args and environment from the
// manifest, substituting {port} and appending PortArg if configured.
func buildLaunchArgs(man manifest.Manifest, port int) ([]string, []string) {
	if man.Engine.Launch == nil {
		return nil, nil
	}
	portStr := fmt.Sprintf("%d", port)

	args := make([]string, len(man.Engine.Launch.Cmd))
	for i, a := range man.Engine.Launch.Cmd {
		args[i] = strings.ReplaceAll(a, "{port}", portStr)
	}
	if man.Engine.Launch.PortArg != "" {
		args = append(args, man.Engine.Launch.PortArg, portStr)
	}

	// Merge OS environment with manifest overrides.
	base := os.Environ()
	env := make([]string, 0, len(base)+len(man.Engine.Launch.Env))
	env = append(env, base...)
	for k, v := range man.Engine.Launch.Env {
		env = append(env, k+"="+v)
	}
	return args, env
}

// waitForHealth probes the engine's health endpoint until it responds or times out.
func waitForHealth(ctx context.Context, man manifest.Manifest, port int) error {
	h := man.Engine.Health
	if h == nil {
		return nil // no health check configured
	}

	path := h.Path
	if path == "" {
		path = "/health"
	}
	intervalS := h.IntervalS
	if intervalS <= 0 {
		intervalS = 2
	}
	timeoutS := h.TimeoutS
	if timeoutS <= 0 {
		timeoutS = 10
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	client := &http.Client{Timeout: time.Duration(intervalS) * time.Second}

	deadline := time.Now().Add(time.Duration(timeoutS) * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled while waiting for health: %w", err)
		}
		resp, err := client.Get(url) //nolint:noctx — short-lived probe, context checked above
		if err == nil && resp.StatusCode < 500 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timed out after %ds", timeoutS)
		}
		time.Sleep(time.Duration(intervalS) * time.Second)
	}
}
