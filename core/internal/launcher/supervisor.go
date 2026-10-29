package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/env"
	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/tools"
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
}

// Supervisor manages the full app process group (Gradio, React UI, extension manager).
// It is separate from the step-by-step installer flow so C3.2 can be developed
// and tested independently.
type Supervisor struct {
	root    paths.Root
	envMode string // "conda" (default) or "uv"
	mu      sync.Mutex
	procs   []*appProcess

	// Status fan-out: Supervisor publishes a JSON ProcessStatus slice whenever
	// a process starts or stops. Subscribers receive those snapshots.
	statusSubs []*logSub
}

// newSupervisor creates a Supervisor for root with the given env mode.
func newSupervisor(root paths.Root, envMode string) *Supervisor {
	return &Supervisor{root: root, envMode: envMode}
}

// Start launches all app processes under supervision.
// Gradio is started first; React UI and extension manager run in parallel.
// All processes share a parent context so cancelling ctx stops them all.
// In uv mode (--env=uv) it ensures tools and the main venv are present first.
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	if len(s.procs) > 0 {
		s.mu.Unlock()
		return fmt.Errorf("app already running")
	}
	s.mu.Unlock()

	// uv mode: ensure pinned tools and main venv, then use uv Python.
	var python string
	var baseEnvOverride []string
	if s.envMode == "uv" {
		if err := tools.EnsureAll(ctx, s.root); err != nil {
			return fmt.Errorf("ensuring tools (uv mode): %w", err)
		}
		gpuLabel, err := readGPULabel(ctx, s.root)
		if err != nil {
			return fmt.Errorf("determining GPU: %w", err)
		}
		mainVenv := filepath.Join(s.root.Venvs(), "main")
		if _, err := os.Stat(mainVenv); err != nil {
			if err := env.CreateMain(ctx, s.root, gpuLabel); err != nil {
				return fmt.Errorf("creating main uv venv: %w", err)
			}
		}
		python = uvPythonPath(s.root)
		baseEnvOverride = buildUVEnv(s.root)
	} else {
		python = appPython(s.root)
	}

	// Get an OS-assigned port for the extension manager to avoid the legacy
	// port 7771 collision.
	extPort, err := freePort()
	if err != nil {
		return fmt.Errorf("allocating extension manager port: %w", err)
	}

	serverPy := filepath.Join(s.root.Dir(), "server.py")
	cwd := s.root.Dir()

	// Base env: inherit (+ tool paths if uv mode) + extension manager port.
	var baseEnv []string
	if len(baseEnvOverride) > 0 {
		baseEnv = append(baseEnvOverride, fmt.Sprintf("TTS_EXTENSION_MANAGER_PORT=%d", extPort))
	} else {
		baseEnv = append(os.Environ(), fmt.Sprintf("TTS_EXTENSION_MANAGER_PORT=%d", extPort))
	}

	defs := []struct {
		name string
		cmd  string
		args []string
		env  []string
	}{
		{
			name: "gradio",
			cmd:  python,
			args: []string{serverPy, "--no-react"},
			env:  baseEnv,
		},
		{
			name: "react-ui",
			cmd:  npmCmd(),
			args: npmReactArgs(s.root),
			env:  baseEnv,
		},
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, d := range defs {
		p := &procs.Proc{}
		if err := p.Start(ctx, d.cmd, d.args, cwd, d.env); err != nil {
			// Best-effort stop what's already started.
			for _, running := range s.procs {
				_ = running.proc.Stop()
			}
			s.procs = nil
			return fmt.Errorf("starting %s: %w", d.name, err)
		}
		ap := &appProcess{name: d.name, proc: p}
		s.procs = append(s.procs, ap)
		go s.watchExit(ap)
	}
	s.publishStatus()
	return nil
}

// Stop stops all supervised processes.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	ps := make([]*appProcess, len(s.procs))
	copy(ps, s.procs)
	s.procs = nil
	s.mu.Unlock()

	for _, ap := range ps {
		_ = ap.proc.Stop()
	}
	s.publishStatus()
}

// Restart stops then starts all processes.
func (s *Supervisor) Restart(ctx context.Context) error {
	s.Stop()
	return s.Start(ctx)
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

// npmCmd returns the platform-appropriate npm executable.
func npmCmd() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

// npmReactArgs returns npm args to start the React UI.
func npmReactArgs(root paths.Root) []string {
	return []string{"--prefix", filepath.Join(root.Dir(), "react-ui"), "start", "--", "-p", "3000"}
}

// buildUVEnv returns an inherited environment with tool binary directories
// prepended to PATH for uv-mode processes.
func buildUVEnv(root paths.Root) []string {
	toolsBase := filepath.Join(root.InstallerFiles(), "tools")
	extraDirs := []string{
		filepath.Join(toolsBase, "uv"),
		filepath.Join(toolsBase, "ffmpeg"),
	}
	if runtime.GOOS == "windows" {
		extraDirs = append(extraDirs, filepath.Join(toolsBase, "mingit", "cmd"))
	}

	sep := string(os.PathListSeparator)
	prefix := strings.Join(extraDirs, sep)

	out := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			out = append(out, "PATH="+prefix+sep+e[5:])
		} else {
			out = append(out, e)
		}
	}
	return out
}

// readGPULabel reads the GPU label from the installer-written .gpu marker file.
// If the file doesn't exist, it falls back to gpu.Detect.
func readGPULabel(ctx context.Context, root paths.Root) (string, error) {
	data, err := os.ReadFile(root.MarkerFile(".gpu"))
	if err == nil {
		label, parseErr := gpu.Parse(string(data))
		if parseErr == nil {
			return string(label), nil
		}
	}
	result, detectErr := gpu.Detect(ctx)
	if detectErr != nil {
		return "", detectErr
	}
	return string(result.Label), nil
}
