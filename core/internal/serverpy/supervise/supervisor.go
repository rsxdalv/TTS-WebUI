// Package supervise runs the TTS WebUI app process group (Gradio, React UI,
// extension manager) under supervision.
//
// It replaces internal/launcher's Supervisor and closes its gaps: per-process
// readiness (starting, ready, exited, failed), exit code and log tail in the
// status, a configurable public port, forwarded flags, a switchable gradio
// entry script and env, React pointed at the public port, and browser opening
// once each process is ready. There is no auto-restart.
package supervise

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

// Process names, as they appear in the status.
const (
	NameGradio   = "gradio"
	NameReact    = "react-ui"
	NameExtMgr   = "extension-manager"
	logTailLines = 50
)

// State is the lifecycle state of one process.
type State string

const (
	StateStopped  State = "stopped"  // never started, or stopped on request
	StateStarting State = "starting" // spawned, port not answering yet
	StateReady    State = "ready"    // port answers
	StateStopping State = "stopping" // Stop requested, process still alive
	StateExited   State = "exited"   // ended by itself
	StateFailed   State = "failed"   // could not start, or not ready in time
	StateDisabled State = "disabled" // not part of this run (NoReact)
)

// ErrAlreadyRunning is returned by Start while any process is alive. Its text
// matches what the launcher handlers compare against.
var ErrAlreadyRunning = errors.New("app already running")

// ProcessStatus is one entry of the status JSON. name and running are the
// legacy fields; the rest is additive.
type ProcessStatus struct {
	Name     string   `json:"name"`
	Running  bool     `json:"running"`
	State    State    `json:"state"`
	Port     int      `json:"port,omitempty"`
	PID      int      `json:"pid,omitempty"`
	ExitCode *int     `json:"exit_code,omitempty"`
	Error    string   `json:"error,omitempty"`
	LogTail  []string `json:"log_tail,omitempty"`
}

// runner is the part of *procs.Proc the supervisor uses. A runner that also
// has `Pid() int` (procs.Proc does) gets its pid reported.
type runner interface {
	Start(ctx context.Context, name string, args []string, cwd string, env []string) error
	Stop() error
	Done() <-chan struct{}
	ExitCode() int
	Lines() []string
}

// managed is one child. Mutable fields are guarded by Supervisor.mu.
type managed struct {
	name  string
	port  int
	probe probeFunc
	r     runner

	state    State
	errMsg   string
	alive    bool
	exited   bool
	exitCode int
	stopping bool

	ready    chan struct{} // closed on first ready
	gone     chan struct{} // closed when it can no longer become ready
	finished chan struct{} // closed when the exit has been recorded
}

func newManaged(name string, port int, probe probeFunc) *managed {
	return &managed{
		name: name, port: port, probe: probe, state: StateStarting,
		ready: make(chan struct{}), gone: make(chan struct{}), finished: make(chan struct{}),
	}
}

func (m *managed) markReady() { // mu held
	select {
	case <-m.ready:
	default:
		close(m.ready)
	}
}

func (m *managed) markGone() { // mu held
	select {
	case <-m.gone:
	default:
		close(m.gone)
	}
}

type statusSub struct{ ch chan string }

// Supervisor manages the app process group.
type Supervisor struct {
	cfg       Config
	newRunner func() runner

	mu  sync.Mutex
	cur []*managed // current generation, in start order; nil before first Start

	// supCtx/supCancel is supervisor-owned; it is never an HTTP request
	// context, which would kill the app when the handler returned.
	supCtx    context.Context
	supCancel context.CancelFunc

	subMu sync.Mutex // guards subs; held while fanning out (sends never block)
	subs  []*statusSub
}

// New validates cfg and returns a stopped Supervisor.
func New(cfg Config) (*Supervisor, error) {
	n, err := cfg.normalized()
	if err != nil {
		return nil, err
	}
	return &Supervisor{
		cfg:       n,
		newRunner: func() runner { return &procs.Proc{} },
	}, nil
}

// Config returns the normalized configuration.
func (s *Supervisor) Config() Config { return s.cfg }

type procDef struct {
	name  string
	port  int
	probe probeFunc
	args  []string
	env   []string
}

// defs builds the child command lines. extPort is the extension manager port.
func (s *Supervisor) defs(extPort int) []procDef {
	c := s.cfg
	base := c.baseEnv(extPort)
	reactEnv := []string{
		"REACT_UI_PORT=" + strconv.Itoa(c.ReactPort),
		fmt.Sprintf("GRADIO_BACKEND_AUTOMATIC=http://127.0.0.1:%d/", c.PublicPort),
	}

	gEnv := append([]string(nil), base...)
	if !c.NoReact {
		gEnv = append(gEnv, "REACT_UI_PORT="+strconv.Itoa(c.ReactPort))
	}
	gEnv = append(gEnv, c.GradioEnv...)

	defs := []procDef{{
		name: NameGradio, port: c.PublicPort, probe: httpProbe,
		args: append([]string{"python", c.GradioEntry}, c.gradioArgs()...),
		env:  gEnv,
	}}
	if !c.NoReact {
		defs = append(defs, procDef{
			name: NameReact, port: c.ReactPort, probe: tcpProbe,
			args: []string{"npm", "--prefix", filepath.Join(c.Root.Dir(), "react-ui"),
				"start", "--", "-p", strconv.Itoa(c.ReactPort)},
			env: append(append([]string(nil), base...), reactEnv...),
		})
	}
	defs = append(defs, procDef{
		name: NameExtMgr, port: extPort, probe: tcpProbe,
		args: []string{"python",
			filepath.Join(c.Root.InstallerScripts(), "tools", "extension_manager.py"),
			"--port", strconv.Itoa(extPort)},
		env: base,
	})
	return defs
}

// aliveLocked reports whether any child process is alive.
func (s *Supervisor) aliveLocked() bool {
	for _, m := range s.cur {
		if m.alive {
			return true
		}
	}
	return false
}

// Start launches the app processes. setupCtx covers setup work only; the
// children run under a supervisor-owned context. On a partial start failure
// everything already started is stopped and the error names the failed child.
func (s *Supervisor) Start(setupCtx context.Context) error {
	if err := setupCtx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	running := s.aliveLocked()
	s.mu.Unlock()
	if running {
		return ErrAlreadyRunning
	}

	if !s.cfg.SkipGPUCheck {
		if _, err := readGPULabel(s.cfg); err != nil {
			return fmt.Errorf("determining GPU: %w", err)
		}
	}

	extPort := s.cfg.ExtensionManagerPort
	if extPort == 0 {
		p, err := freePort()
		if err != nil {
			return fmt.Errorf("allocating extension manager port: %w", err)
		}
		extPort = p
	}

	s.mu.Lock()
	if s.aliveLocked() {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	if s.supCancel != nil {
		s.supCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.supCtx, s.supCancel = ctx, cancel

	cwd := s.cfg.Root.Dir()
	prefix := s.cfg.CommandPrefix
	gen := make([]*managed, 0, 3)
	s.cur = gen
	var startErr error
	for _, d := range s.defs(extPort) {
		m := newManaged(d.name, d.port, d.probe)
		gen = append(gen, m)
		s.cur = gen
		r := s.newRunner()
		args := append(append([]string(nil), prefix[1:]...), d.args...)
		if err := r.Start(ctx, prefix[0], args, cwd, d.env); err != nil {
			m.state, m.errMsg = StateFailed, err.Error()
			m.markGone()
			startErr = fmt.Errorf("starting %s: %w", d.name, err)
			break
		}
		m.r, m.alive = r, true
		go s.watch(m)
		go s.probeLoop(ctx, m)
	}
	if startErr != nil {
		s.mu.Unlock()
		s.stopGeneration(gen, cancel)
		return startErr
	}
	if s.cfg.OpenBrowser {
		go s.openBrowsers(ctx, gen)
	}
	s.mu.Unlock()
	s.publish()
	return nil
}

// Stop stops all children and cancels the supervisor context. Children that
// exited or failed on their own keep their status until the next Start.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	cancel := s.supCancel
	s.supCancel, s.supCtx = nil, nil
	gen := s.cur
	s.mu.Unlock()
	s.stopGeneration(gen, cancel)
}

func (s *Supervisor) stopGeneration(gen []*managed, cancel context.CancelFunc) {
	var toStop []*managed
	s.mu.Lock()
	for _, m := range gen {
		if m.alive && !m.stopping {
			m.stopping, m.state, m.errMsg = true, StateStopping, ""
			toStop = append(toStop, m)
		}
		m.markGone()
	}
	s.mu.Unlock()
	s.publish()

	var wg sync.WaitGroup
	for _, m := range toStop {
		wg.Add(1)
		go func(m *managed) { defer wg.Done(); _ = m.r.Stop() }(m)
	}
	wg.Wait()
	if cancel != nil {
		cancel() // catches anything that missed the graceful stop
	}
	for _, m := range toStop {
		select {
		case <-m.finished:
		case <-time.After(5 * time.Second):
			s.mu.Lock()
			m.alive, m.state = false, StateStopped
			s.mu.Unlock()
		}
	}
	s.publish()
}

// Restart stops then starts; the new children get a fresh supervisor context.
func (s *Supervisor) Restart(setupCtx context.Context) error {
	s.Stop()
	return s.Start(setupCtx)
}

// watch records the exit of m.
func (s *Supervisor) watch(m *managed) {
	<-m.r.Done()
	code := m.r.ExitCode()
	s.mu.Lock()
	m.alive, m.exited, m.exitCode = false, true, code
	switch {
	case m.stopping:
		m.state = StateStopped
	case m.state != StateFailed:
		m.state = StateExited
	}
	m.markGone()
	close(m.finished)
	s.mu.Unlock()
	s.publish()
}

// probeLoop polls m's port until ready, timeout, exit or cancel.
func (s *Supervisor) probeLoop(ctx context.Context, m *managed) {
	timeout := time.NewTimer(s.cfg.ReadyTimeout)
	defer timeout.Stop()
	tick := time.NewTicker(s.cfg.ProbeInterval)
	defer tick.Stop()
	for {
		if m.probe(ctx, m.port) {
			s.mu.Lock()
			changed := m.state == StateStarting
			if changed {
				m.state = StateReady
				m.markReady()
			}
			s.mu.Unlock()
			if changed {
				s.publish()
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-m.r.Done():
			return
		case <-timeout.C:
			s.mu.Lock()
			changed := m.state == StateStarting
			if changed {
				m.state = StateFailed
				m.errMsg = fmt.Sprintf("not ready on 127.0.0.1:%d after %s", m.port, s.cfg.ReadyTimeout)
				m.markGone()
			}
			s.mu.Unlock()
			if changed {
				s.publish()
			}
			return
		case <-tick.C:
		}
	}
}

// openBrowsers opens the public URL when Gradio is ready, then the React URL
// when React is ready. React's tab is skipped if Gradio never became ready.
func (s *Supervisor) openBrowsers(ctx context.Context, gen []*managed) {
	wait := func(m *managed) bool {
		select {
		case <-m.ready:
			return true
		default:
		}
		select {
		case <-m.ready:
			return true
		case <-m.gone:
			select {
			case <-m.ready:
				return true
			default:
				return false
			}
		case <-ctx.Done():
			return false
		}
	}
	open := func(url string) {
		if err := s.cfg.Opener(url); err != nil {
			slog.Warn("could not open browser", "url", url, "err", err)
		}
	}
	for _, m := range gen {
		switch m.name {
		case NameGradio:
			if !wait(m) {
				return
			}
			open(publicURL(s.cfg.PublicPort))
		case NameReact:
			if !wait(m) {
				return
			}
			open(fmt.Sprintf("http://127.0.0.1:%d", s.cfg.ReactPort))
		}
	}
}

// Statuses returns the current status of every process.
func (s *Supervisor) Statuses() []ProcessStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusesLocked()
}

func (s *Supervisor) statusesLocked() []ProcessStatus {
	byName := map[string]*managed{}
	for _, m := range s.cur {
		byName[m.name] = m
	}
	out := make([]ProcessStatus, 0, 3)
	for _, name := range []string{NameGradio, NameReact, NameExtMgr} {
		m := byName[name]
		if m == nil {
			st := ProcessStatus{Name: name, State: StateStopped}
			switch name {
			case NameGradio:
				st.Port = s.cfg.PublicPort
			case NameReact:
				st.Port = s.cfg.ReactPort
				if s.cfg.NoReact {
					st.State, st.Port = StateDisabled, 0
				}
			default:
				st.Port = s.cfg.ExtensionManagerPort
			}
			out = append(out, st)
			continue
		}
		st := ProcessStatus{Name: m.name, Running: m.alive, State: m.state, Port: m.port}
		if pp, ok := m.r.(interface{ Pid() int }); ok && m.alive {
			st.PID = pp.Pid()
		}
		if m.exited && !m.stopping {
			code := m.exitCode
			st.ExitCode = &code
		}
		if m.state == StateFailed {
			st.Error = m.errMsg
		}
		if (m.state == StateExited || m.state == StateFailed) && m.r != nil {
			st.LogTail = tail(m.r.Lines(), logTailLines)
		}
		out = append(out, st)
	}
	return out
}

func tail(lines []string, n int) []string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return append([]string(nil), lines...)
}

// SubscribeStatus returns a channel of JSON-encoded []ProcessStatus snapshots,
// sent on every state change, and a cancel function. The channel is buffered;
// a slow reader misses snapshots.
func (s *Supervisor) SubscribeStatus() (<-chan string, func()) {
	sub := &statusSub{ch: make(chan string, 16)}
	s.subMu.Lock()
	s.subs = append(s.subs, sub)
	s.subMu.Unlock()
	return sub.ch, func() {
		s.subMu.Lock()
		defer s.subMu.Unlock()
		for i, x := range s.subs {
			if x == sub {
				s.subs = append(s.subs[:i], s.subs[i+1:]...)
				close(sub.ch)
				return
			}
		}
	}
}

// publish fans the current status out. It must not be called with s.mu held.
func (s *Supervisor) publish() {
	b, _ := json.Marshal(s.Statuses())
	line := string(b)
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for _, sub := range s.subs {
		select {
		case sub.ch <- line:
		default:
		}
	}
}

// AppLink returns the public URL when it answers, else "".
func (s *Supervisor) AppLink() string {
	url := publicURL(s.cfg.PublicPort)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Head(url)
	if err != nil {
		return ""
	}
	_ = resp.Body.Close()
	return url
}

// readGPULabel mirrors the launcher: the .gpu marker written by the GPU step
// must exist and parse; there is no auto-detect fallback.
func readGPULabel(c Config) (string, error) {
	data, err := os.ReadFile(c.Root.MarkerFile(".gpu"))
	if err != nil {
		return "", fmt.Errorf("GPU label not set: complete the GPU step first (%w)", err)
	}
	label, err := gpu.Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("invalid GPU label in .gpu file: %w", err)
	}
	return string(label), nil
}
