package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
	"github.com/rsxdalv/TTS-WebUI/core/react_ui_backend"
)

// ProcessStatus describes one supervised process.
type ProcessStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
}

// appProcess is one managed app process.
type appProcess struct {
	name string
	proc *procs.Proc
	// front is the Go proxy in front of this process, if any.
	front *react_ui_backend.Server
}

// reactUIArgs returns the micromamba arguments that start Next. With the proxy
// off they are the arguments the app has always used, so Next owns port 3000.
func reactUIArgs(envPath, rootDir string, port int, proxied bool) []string {
	args := []string{"-p", envPath, "run", "npm", "--prefix", filepath.Join(rootDir, "react-ui"), "start", "--", "-p", strconv.Itoa(port)}
	if proxied {
		args = append(args, "-H", "127.0.0.1")
	}
	return args
}

// Supervisor manages the full app process group (Gradio, React UI, extension manager).
// It is separate from the step-by-step installer flow so C3.2 can be developed
// and tested independently.
type Supervisor struct {
	root  paths.Root
	mu    sync.Mutex
	procs []*appProcess

	// supCtx/supCancel is a supervisor-owned context used for all child
	// processes. It is never the HTTP request context; that would kill the
	// app as soon as the handler returned.
	supCtx    context.Context
	supCancel context.CancelFunc

	// Status fan-out: Supervisor publishes a JSON ProcessStatus slice whenever
	// a process starts or stops. Subscribers receive those snapshots.
	statusSubs []*logSub

	// runTool runs a node tool in react-ui; nil means runReactUITool. Tests
	// replace it.
	runTool func(ctx context.Context, name string, args ...string) error
}

// newSupervisor creates a Supervisor for root.
func newSupervisor(root paths.Root) *Supervisor {
	return &Supervisor{root: root}
}

// Start launches all app processes under supervision.
// Gradio is started first; React UI and extension manager run in parallel.
// setupCtx is used only for setup work. Processes run under a
// supervisor-owned context that is not tied to any HTTP request lifetime.
func (s *Supervisor) Start(setupCtx context.Context) error {
	s.mu.Lock()
	if len(s.procs) > 0 {
		s.mu.Unlock()
		return fmt.Errorf("app already running")
	}
	// Replace any previous supervisor context with a fresh one.
	if s.supCancel != nil {
		s.supCancel()
	}
	supCtx, supCancel := context.WithCancel(context.Background())
	s.supCtx = supCtx
	s.supCancel = supCancel
	s.mu.Unlock()

	// GPU label is required before any process can start.
	if _, err := readGPULabel(setupCtx, s.root); err != nil {
		return fmt.Errorf("determining GPU: %w", err)
	}

	// Get an OS-assigned port for the extension manager to avoid the legacy
	// port 7771 collision.
	extPort, err := freePort()
	if err != nil {
		return fmt.Errorf("allocating extension manager port: %w", err)
	}

	// With the proxy on (the default, D18), Go takes port 3000 and either
	// serves the static export or proxies Next on a private port.
	sw, err := switches.FromEnv()
	if err != nil {
		return err
	}
	plan, err := s.prepareReactUI(setupCtx, sw)
	if err != nil {
		return err
	}
	var front *react_ui_backend.Server
	nextPort := react_ui_backend.PublicPort
	if sw.Proxy {
		if plan.mode == modeNextProxied {
			if nextPort, err = freePort(); err != nil {
				return fmt.Errorf("allocating React UI port: %w", err)
			}
		}
		front, err = react_ui_backend.Start(react_ui_backend.Config{
			Root:         s.root.Dir(),
			Port:         react_ui_backend.PublicPort,
			NextAddr:     fmt.Sprintf("127.0.0.1:%d", nextPort),
			Static:       plan.mode == modeStatic,
			Routes:       sw,
			LAN:          sw.ProxyLAN,
			AllowedHosts: sw.ProxyAllowedHosts,
		})
		if err != nil {
			return fmt.Errorf("starting the React UI proxy: %w", err)
		}
		if sw.ProxyLAN {
			slog.Warn(react_ui_backend.LANWarning(react_ui_backend.PublicPort))
		}
	}

	cwd := s.root.Dir()
	extMgrScript := filepath.Join(s.root.InstallerScripts(), "tools", "extension_manager.py")

	// Base env: inherit + extension manager port + mamba root.
	mambaRoot := "MAMBA_ROOT_PREFIX=" + filepath.Join(s.root.InstallerFiles(), "mamba")
	baseEnv := append(os.Environ(), fmt.Sprintf("TTS_EXTENSION_MANAGER_PORT=%d", extPort), mambaRoot)

	// Processes run via micromamba so they inherit the activated conda environment.
	type procDef struct {
		name string
		cmd  string
		args []string
		env  []string
	}
	mamba := micromambaPath(s.root)
	envPath := installEnvPath(s.root)
	defs := []procDef{
		{name: "gradio", cmd: mamba, args: []string{"-p", envPath, "run", "python", "server.py", "--no-react"}, env: baseEnv},
		{name: "react-ui", cmd: mamba, args: reactUIArgs(envPath, s.root.Dir(), nextPort, sw.Proxy), env: baseEnv},
		{name: "extension-manager", cmd: mamba, args: []string{"-p", envPath, "run", "python", extMgrScript, "--port", fmt.Sprintf("%d", extPort)}, env: baseEnv},
	}
	if plan.mode == modeStatic {
		defs = append(defs[:1], defs[2:]...) // no Next process: Go serves the UI
	}

	s.mu.Lock()

	for _, d := range defs {
		p := &procs.Proc{}
		if err := p.Start(supCtx, d.cmd, d.args, cwd, d.env); err != nil {
			// Best-effort stop what's already started.
			for _, running := range s.procs {
				if running.proc != nil {
					_ = running.proc.Stop()
				}
			}
			if front != nil {
				front.Close()
			}
			s.procs = nil
			supCancel()
			s.supCtx = nil
			s.supCancel = nil
			s.mu.Unlock()
			return fmt.Errorf("starting %s: %w", d.name, err)
		}
		ap := &appProcess{name: d.name, proc: p}
		if d.name == "react-ui" {
			ap.front = front
		}
		s.procs = append(s.procs, ap)
		go s.watchExit(ap)
	}
	if plan.mode == modeStatic {
		// The UI is the Go server itself; it shows as react-ui until Stop.
		s.procs = append(s.procs, &appProcess{name: "react-ui", front: front})
	}
	s.mu.Unlock()
	s.publishStatus() // must be called without mu held (publishStatus acquires mu itself)
	return nil
}

// Stop stops all supervised processes and cancels the supervisor context.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	cancel := s.supCancel
	s.supCancel = nil
	s.supCtx = nil
	ps := make([]*appProcess, len(s.procs))
	copy(ps, s.procs)
	s.procs = nil
	s.mu.Unlock()

	for _, ap := range ps {
		if ap.front != nil {
			ap.front.Close()
		}
		if ap.proc != nil {
			_ = ap.proc.Stop()
		}
	}
	if cancel != nil {
		cancel() // signal any processes that missed graceful stop
	}
	s.publishStatus()
}

// Restart stops then starts all processes. setupCtx is used only for setup
// work; the new processes are bound to a fresh supervisor-owned context.
func (s *Supervisor) Restart(setupCtx context.Context) error {
	s.Stop()
	return s.Start(setupCtx)
}

// Statuses returns the current process statuses.
func (s *Supervisor) Statuses() []ProcessStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statuses()
}

// statuses must be called with s.mu held.
func (s *Supervisor) statuses() []ProcessStatus {
	out := []ProcessStatus{
		{Name: "gradio", Running: false},
		{Name: "react-ui", Running: false},
		{Name: "extension-manager", Running: false},
	}
	for _, ap := range s.procs {
		for i := range out {
			if out[i].Name == ap.name {
				out[i].Running = true
			}
		}
	}
	return out
}

// SubscribeStatus returns a channel that receives JSON-encoded ProcessStatus
// slices and a cancel function. The channel is buffered.
func (s *Supervisor) SubscribeStatus() (<-chan string, func()) {
	sub := &logSub{ch: make(chan string, 16)}
	s.mu.Lock()
	s.statusSubs = append(s.statusSubs, sub)
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, ss := range s.statusSubs {
			if ss == sub {
				s.statusSubs = append(s.statusSubs[:i], s.statusSubs[i+1:]...)
				close(sub.ch)
				return
			}
		}
	}
	return sub.ch, cancel
}

// publishStatus fans out the current status JSON to all subscribers.
// Must NOT be called with s.mu held.
func (s *Supervisor) publishStatus() {
	s.mu.Lock()
	statuses := s.statuses()
	subs := make([]*logSub, len(s.statusSubs))
	copy(subs, s.statusSubs)
	s.mu.Unlock()

	b, _ := json.Marshal(statuses)
	line := string(b)
	for _, sub := range subs {
		select {
		case sub.ch <- line:
		default:
		}
	}
}

// watchExit removes a process from the tracked list and publishes status when it exits.
func (s *Supervisor) watchExit(ap *appProcess) {
	ap.proc.Wait()
	// Without Next there is nothing to proxy, so port 3000 closes with it.
	if ap.front != nil {
		ap.front.Close()
	}
	s.mu.Lock()
	for i, p := range s.procs {
		if p == ap {
			s.procs = append(s.procs[:i], s.procs[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	s.publishStatus()
}

// AppLink returns the Gradio URL if the server is reachable, else empty string.
func (s *Supervisor) AppLink() string {
	const gradioURL = "http://127.0.0.1:7770"
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Head(gradioURL)
	if err != nil {
		return ""
	}
	_ = resp.Body.Close()
	return gradioURL
}

// freePort asks the OS for a free TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	// Brief pause so the OS can reclaim the ephemeral port before the child claims it.
	time.Sleep(10 * time.Millisecond)
	return port, nil
}

// readGPULabel reads the GPU label from the installer-written .gpu marker file.
// A missing file or any read error is returned as-is: there is no auto-detect
// fallback. The user's choice (written by the GPU step) is the contract; legacy
// refuses to auto-install AMD and Intel (initializeApp.js:114).
func readGPULabel(_ context.Context, root paths.Root) (string, error) {
	data, err := os.ReadFile(root.MarkerFile(".gpu"))
	if err != nil {
		return "", fmt.Errorf("GPU label not set: complete the GPU step first (%w)", err)
	}
	label, parseErr := gpu.Parse(string(data))
	if parseErr != nil {
		return "", fmt.Errorf("invalid GPU label in .gpu file: %w", parseErr)
	}
	return string(label), nil
}
