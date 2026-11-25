// Package extprocs runs TTS-WebUI proxy-extension processes on behalf of
// server.py. Python asks Go (through the control API in api.go) to start and
// stop an extension; Go spawns it through procs (kill tree, ring-buffer logs),
// probes readiness and registers the /<pkg> route with the router.
package extprocs

import (
	"context"
	"errors"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"net"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

// RouteRegistrar is implemented by the router (stage 3's *router.Router).
// AddRoute is idempotent: re-adding a prefix replaces its target.
type RouteRegistrar interface {
	AddRoute(prefix, target string) error
	RemoveRoute(prefix string)
}

// State is the lifecycle state of one extension process.
type State string

const (
	StateStarting State = "starting"
	StateReady    State = "ready"
	StateExited   State = "exited"
	StateFailed   State = "failed"
)

var (
	// ErrNotFound means no extension with that package name was ever started.
	ErrNotFound = errors.New("extension not found")
	// ErrInvalidPackage means the package name failed validation.
	ErrInvalidPackage = errors.New("invalid package name")
)

var pkgRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// ValidatePackage checks a package name (no separators, no "..").
func ValidatePackage(pkg string) error {
	if !pkgRe.MatchString(pkg) || strings.Contains(pkg, "..") {
		return fmt.Errorf("%w: %q", ErrInvalidPackage, pkg)
	}
	return nil
}

var harnessArgs = []string{"-m", "tts_webui.extensions_loader.proxy_harness"}

// Options configure a Manager.
type Options struct {
	Root paths.Root
	// Platform resolves the program of every extension: its own venv's python
	// when .venvs/<pkg> has one, else the main env's. Its command environment is
	// the base environment of the children.
	Platform platform.Platform
	// ExtraEnv, when set, is asked at each start for KEY=VALUE variables that
	// Go computes for the extension (the resolved decorators). They win over
	// BaseEnv, so Python cannot override them.
	ExtraEnv func() ([]string, error)
	// PackageEnv, when set, is asked at each start for the KEY=VALUE variables
	// the package's own catalog entry wants (its "env" object). They win over
	// ExtraEnv and BaseEnv.
	PackageEnv func(pkg string) ([]string, error)
	// Routes receives AddRoute/RemoveRoute calls. May be nil.
	Routes RouteRegistrar
	// Token is the shared secret for the control API. Empty rejects all calls.
	Token string
	// ReadyTimeout is how long to probe before marking an extension failed
	// (default 120s). The process is left running and probing continues.
	ReadyTimeout time.Duration
	// Logf receives diagnostic lines. May be nil.
	Logf func(format string, args ...any)
	// IdleTimeout stops a ready engine that has had no request for this long
	// and has none in flight (D31). Zero turns idle stop off, which is the
	// default: most people do not want it.
	IdleTimeout time.Duration
	// MaxRunning caps the engines that run at once (D31). When it is reached a
	// start stops the engine that was used longest ago and is not busy; if all
	// are busy the start fails with ErrCapReached. Zero means no cap.
	MaxRunning int
}

// StartRequest is the body of POST /api/extensions/{pkg}/start.
type StartRequest struct {
	Port            int    `json:"port,omitempty"`
	OpenAIProxyHost string `json:"openai_proxy_host,omitempty"`
}

// StartResult is returned by Start.
type StartResult struct {
	Port           int  `json:"port"`
	Pid            int  `json:"pid"`
	AlreadyRunning bool `json:"already_running,omitempty"`
}

// Status describes one extension.
type Status struct {
	Package  string `json:"package"`
	Port     int    `json:"port"`
	Pid      int    `json:"pid"`
	State    State  `json:"state"`
	Error    string `json:"error,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

type entry struct {
	pkg      string
	port     int
	pid      int
	proc     *procs.Proc
	cancel   context.CancelFunc
	state    State
	errMsg   string
	exitCode *int
	stopping bool
	inflight int           // requests being served through the router
	lastUsed time.Time     // last request start or end, else the start time
	dead     bool          // process has exited (set by watcher)
	exited   chan struct{} // closed after watcher updated state
}

// Manager owns the extension processes.
type Manager struct {
	opts Options
	mu   sync.Mutex // also serialises route calls; the registrar must be fast
	ents map[string]*entry
}

// New returns a Manager.
func New(o Options) *Manager {
	if o.ReadyTimeout <= 0 {
		o.ReadyTimeout = 120 * time.Second
	}
	return &Manager{opts: o, ents: map[string]*entry{}}
}

func (m *Manager) logf(f string, a ...any) {
	if m.opts.Logf != nil {
		m.opts.Logf(f, a...)
	}
}

// command is the python command of an extension: its own venv's when it has
// one, otherwise the platform's main env.
func (m *Manager) command(pkg string) (platform.Command, error) {
	return m.opts.Platform.Command(&platform.Entry{Package: pkg, Own: true}, []string{"python"})
}

func portUsable(p int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Start spawns the extension and returns right after the spawn. If the
// package is already running it returns the current port and pid.
// ctx is only checked before spawning: the process outlives it.
func (m *Manager) Start(ctx context.Context, pkg string, req StartRequest) (StartResult, error) {
	if err := ValidatePackage(pkg); err != nil {
		return StartResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return StartResult{}, err
	}
	if strings.Contains(req.OpenAIProxyHost, "\x00") {
		return StartResult{}, errors.New("invalid openai_proxy_host")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if old := m.ents[pkg]; old != nil && !old.dead {
		old.lastUsed = time.Now()
		return StartResult{Port: old.port, Pid: old.pid, AlreadyRunning: true}, nil
	}
	if err := m.makeRoomLocked(pkg); err != nil {
		return StartResult{}, err
	}

	pc, err := m.command(pkg)
	if err != nil {
		return StartResult{}, fmt.Errorf("no python for %s: %w", pkg, err)
	}

	port := req.Port
	if port < 1024 || port > 65535 || m.portTaken(port) || !portUsable(port) {
		var err error
		if port, err = freePort(); err != nil {
			return StartResult{}, fmt.Errorf("allocating port: %w", err)
		}
	}

	env, err := m.buildEnv(pkg, port, req, pc.Env)
	if err != nil {
		return StartResult{}, fmt.Errorf("environment of %s: %w", pkg, err)
	}
	cctx, cancel := context.WithCancel(context.Background())
	e := &entry{pkg: pkg, port: port, state: StateStarting, lastUsed: time.Now(), cancel: cancel, proc: &procs.Proc{}, exited: make(chan struct{})}
	args := append(append([]string{}, pc.Args...), harnessArgs...)
	if err := e.proc.Start(cctx, pc.Program, args, m.opts.Root.Dir(), env); err != nil {
		cancel()
		e.state, e.errMsg, e.dead = StateFailed, "spawn: "+err.Error(), true
		close(e.exited)
		m.ents[pkg] = e
		return StartResult{}, fmt.Errorf("spawning %s: %w", pkg, err)
	}
	e.pid = e.proc.Pid()
	m.ents[pkg] = e
	m.logf("extprocs: started %s pid=%d port=%d", pkg, e.pid, port)
	go m.watch(e)
	go m.probe(e)
	return StartResult{Port: port, Pid: e.pid}, nil
}

func (m *Manager) portTaken(p int) bool {
	for _, e := range m.ents {
		if !e.dead && e.port == p {
			return true
		}
	}
	return false
}

func envKey(kv string) string {
	k, _, _ := strings.Cut(kv, "=")
	if runtime.GOOS == "windows" {
		k = strings.ToUpper(k)
	}
	return k
}

func dropEnv(env []string, k string) []string {
	want := envKey(k + "=")
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if envKey(kv) != want {
			out = append(out, kv)
		}
	}
	return out
}

func setEnv(env []string, k, v string) []string {
	return append(dropEnv(env, k), k+"="+v)
}

func (m *Manager) buildEnv(pkg string, port int, req StartRequest, base []string) ([]string, error) {
	env := append([]string{}, base...)
	if m.opts.ExtraEnv != nil {
		extra, err := m.opts.ExtraEnv()
		if err != nil {
			return nil, err
		}
		for _, kv := range extra {
			k, v, _ := strings.Cut(kv, "=")
			env = setEnv(env, k, v)
		}
	}
	if m.opts.PackageEnv != nil {
		own, err := m.opts.PackageEnv(pkg)
		if err != nil {
			return nil, err
		}
		for _, kv := range own {
			k, v, _ := strings.Cut(kv, "=")
			env = setEnv(env, k, v)
		}
	}
	// The control token must never reach third-party extension code.
	env = dropEnv(env, "TTS_CORE_TOKEN")
	env = dropEnv(env, "TTS_CORE_URL")
	env = setEnv(env, "GRADIO_SERVER_PORT", fmt.Sprint(port))
	env = setEnv(env, "GRADIO_ROOT_PATH", "/"+pkg)
	env = setEnv(env, "TTS_WEBUI_EXTENSION_PACKAGE", pkg)
	if req.OpenAIProxyHost != "" {
		env = setEnv(env, "OPENAI_PROXY_HOST", req.OpenAIProxyHost)
	}
	return env, nil
}

// removeRoute must be called with m.mu held.
func (m *Manager) removeRoute(pkg string) {
	if m.opts.Routes != nil {
		m.opts.Routes.RemoveRoute("/" + pkg)
	}
}

func (m *Manager) watch(e *entry) {
	e.proc.Wait()
	code := e.proc.ExitCode()
	m.mu.Lock()
	e.dead = true
	e.exitCode = &code
	if m.ents[e.pkg] == e {
		m.removeRoute(e.pkg)
	}
	switch {
	case e.stopping || e.state == StateReady:
		e.state = StateExited
	default:
		e.state = StateFailed
		if e.errMsg == "" {
			e.errMsg = fmt.Sprintf("exited before ready (code %d)", code)
		}
	}
	m.mu.Unlock()
	e.cancel()
	close(e.exited)
	m.logf("extprocs: %s exited code=%d state=%s", e.pkg, code, e.state)
}

func (m *Manager) probe(e *entry) {
	done := e.proc.Done()
	deadline := time.Now().Add(m.opts.ReadyTimeout)
	timedOut := false
	addr := fmt.Sprintf("127.0.0.1:%d", e.port)
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		if c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond); err == nil {
			_ = c.Close()
			m.markReady(e)
			return
		}
		if !timedOut && time.Now().After(deadline) {
			timedOut = true
			m.mu.Lock()
			if !e.dead && !e.stopping && e.state == StateStarting {
				e.state = StateFailed
				e.errMsg = fmt.Sprintf("not ready within %s (process still running)", m.opts.ReadyTimeout)
			}
			m.mu.Unlock()
		}
		select {
		case <-done:
			return
		case <-tick.C:
		}
	}
}

func (m *Manager) markReady(e *entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.dead || e.stopping || m.ents[e.pkg] != e {
		return
	}
	e.state, e.errMsg = StateReady, ""
	if m.opts.Routes != nil {
		if err := m.opts.Routes.AddRoute("/"+e.pkg, fmt.Sprintf("http://127.0.0.1:%d", e.port)); err != nil {
			e.state, e.errMsg = StateFailed, "route registration: "+err.Error()
			m.logf("extprocs: %s: %v", e.pkg, err)
		}
	}
}

// Stop removes the route and stops the process tree. Stopping an extension
// that already exited is a no-op; an unknown package returns ErrNotFound.
func (m *Manager) Stop(pkg string) error {
	if err := ValidatePackage(pkg); err != nil {
		return err
	}
	m.mu.Lock()
	e := m.ents[pkg]
	if e == nil {
		m.mu.Unlock()
		return ErrNotFound
	}
	e.stopping = true
	m.removeRoute(pkg)
	m.mu.Unlock()

	_ = e.proc.Stop()
	select {
	case <-e.exited:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("extension %s did not exit", pkg)
	}
}

// StopAll stops every extension. The Manager stays usable afterwards.
func (m *Manager) StopAll() {
	m.mu.Lock()
	pkgs := make([]string, 0, len(m.ents))
	for p, e := range m.ents {
		if !e.dead {
			pkgs = append(pkgs, p)
		}
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, p := range pkgs {
		wg.Add(1)
		go func(p string) { defer wg.Done(); _ = m.Stop(p) }(p)
	}
	wg.Wait()
}

// List returns all known extensions sorted by package name.
func (m *Manager) List() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(m.ents))
	for _, e := range m.ents {
		out = append(out, Status{Package: e.pkg, Port: e.port, Pid: e.pid, State: e.state, Error: e.errMsg, ExitCode: e.exitCode})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Package < out[j].Package })
	return out
}

func (m *Manager) get(pkg string) (*entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ents[pkg]; e != nil {
		return e, nil
	}
	return nil, ErrNotFound
}

// Lines returns the buffered output lines of the extension.
func (m *Manager) Lines(pkg string) ([]string, error) {
	e, err := m.get(pkg)
	if err != nil {
		return nil, err
	}
	return e.proc.Lines(), nil
}

// Subscribe streams future output lines of the extension.
func (m *Manager) Subscribe(pkg string) (<-chan string, func(), error) {
	e, err := m.get(pkg)
	if err != nil {
		return nil, nil, err
	}
	ch, cancel := e.proc.Subscribe()
	return ch, cancel, nil
}

// StopIfRunning stops the process of pkg when it is starting or ready, and
// reports whether it did. An engine that never started or already exited is
// left alone.
func (m *Manager) StopIfRunning(pkg string) bool {
	for _, st := range m.List() {
		if st.Package == pkg && (st.State == StateStarting || st.State == StateReady) {
			return m.Stop(pkg) == nil
		}
	}
	return false
}
