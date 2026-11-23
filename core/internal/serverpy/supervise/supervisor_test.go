package supervise

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

var fakeBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "supervise-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdirtemp:", err)
		os.Exit(1)
	}
	bin := filepath.Join(dir, "fakechild")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/fakechild").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building fakechild: %v\n%s\n", err, out)
		os.Exit(1)
	}
	fakeBin = bin
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// --- helpers ---

type opened struct {
	mu   sync.Mutex
	urls []string
}

func (o *opened) open(u string) error {
	o.mu.Lock()
	o.urls = append(o.urls, u)
	o.mu.Unlock()
	return nil
}

func (o *opened) get() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.urls...)
}

type env struct {
	root       paths.Root
	dump       string
	gradioPort int
	reactPort  int
	op         *opened
}

func tmpPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}

// newSup builds a Supervisor on a temp root with the fake child. extraEnv
// configures the fake (FAKE_*).
func newSup(t *testing.T, mutate func(*Config), extraEnv ...string) (*Supervisor, *env) {
	t.Helper()
	root, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &env{root: root, dump: t.TempDir(), gradioPort: tmpPort(t), reactPort: tmpPort(t), op: &opened{}}
	cfg := Config{
		Root:          root,
		PublicPort:    e.gradioPort,
		ReactPort:     e.reactPort,
		SkipGPUCheck:  true,
		CommandPrefix: []string{fakeBin},
		Opener:        e.op.open,
		ProbeInterval: 20 * time.Millisecond,
		ReadyTimeout:  20 * time.Second,
		ExtraEnv: append([]string{
			"FAKE_DUMP_DIR=" + e.dump,
			"FAKE_GRADIO_PORT=" + strconv.Itoa(e.gradioPort),
		}, extraEnv...),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	return s, e
}

func status(s *Supervisor, name string) ProcessStatus {
	for _, st := range s.Statuses() {
		if st.Name == name {
			return st
		}
	}
	return ProcessStatus{}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; statuses: %s", what, mustJSON(s2(t)))
}

var lastSup *Supervisor

func s2(t *testing.T) any {
	if lastSup == nil {
		return nil
	}
	return lastSup.Statuses()
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func waitState(t *testing.T, s *Supervisor, name string, want State) {
	t.Helper()
	lastSup = s
	waitFor(t, name+" "+string(want), func() bool { return status(s, name).State == want })
}

type dumpT struct {
	Role string            `json:"role"`
	Args []string          `json:"args"`
	Env  map[string]string `json:"env"`
	Port int               `json:"port"`
}

func readDump(t *testing.T, dir, role string) dumpT {
	t.Helper()
	var d dumpT
	waitFor(t, role+" dump", func() bool {
		b, err := os.ReadFile(filepath.Join(dir, role+".json"))
		return err == nil && json.Unmarshal(b, &d) == nil
	})
	return d
}

func ctx() context.Context { return context.Background() }

// --- tests ---

func TestReadyAndStatusShape(t *testing.T) {
	s, e := newSup(t, nil)
	// Before Start: legacy shape, all stopped.
	for _, st := range s.Statuses() {
		if st.Running || st.State != StateStopped {
			t.Fatalf("before Start: %+v", st)
		}
	}
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{NameGradio, NameReact} {
		waitState(t, s, n, StateReady)
	}

	var raw []map[string]any
	if err := json.Unmarshal([]byte(mustJSON(s.Statuses())), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 2 {
		t.Fatalf("want 2 entries, got %v", raw)
	}
	wantNames := []string{"gradio", "react-ui"}
	for i, m := range raw {
		if m["name"] != wantNames[i] || m["running"] != true || m["state"] != "ready" {
			t.Errorf("entry %d: %v", i, m)
		}
		if _, ok := m["port"].(float64); !ok {
			t.Errorf("entry %d lacks port: %v", i, m)
		}
		if _, ok := m["exit_code"]; ok {
			t.Errorf("entry %d has exit_code while running", i)
		}
	}
	if int(raw[0]["port"].(float64)) != e.gradioPort || int(raw[1]["port"].(float64)) != e.reactPort {
		t.Errorf("ports: %v", raw)
	}
	if err := s.Start(ctx()); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Start = %v, want ErrAlreadyRunning", err)
	}
	if err := s.Start(ctx()); err == nil || err.Error() != "app already running" {
		t.Errorf("error text changed: %v", err)
	}
}

func TestChildArgsAndEnv(t *testing.T) {
	s, e := newSup(t, func(c *Config) {
		c.Share, c.Docker, c.NoTreeProxy = true, true, true
		c.GradioEntry = "server_core.py"
		c.GradioEnv = []string{"TTS_CORE_TOKEN=secret", "TTS_GRADIO_PORT=1234"}
	}, "TEST_EXTRA=yes")
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	g := readDump(t, e.dump, "gradio")

	if got, want := strings.Join(g.Args, " "), "python server_core.py --no-react --share --docker --no-tree-proxy"; got != want {
		t.Errorf("gradio args = %q, want %q", got, want)
	}
	// The React UI is served in-process by the Go front: no child runs for it.
	if _, err := os.Stat(filepath.Join(e.dump, "react.json")); err == nil {
		t.Errorf("a React child process ran")
	}
	if g.Env["REACT_UI_PORT"] != strconv.Itoa(e.reactPort) {
		t.Errorf("gradio REACT_UI_PORT = %q", g.Env["REACT_UI_PORT"])
	}
	if _, ok := g.Env["TTS_EXTENSION_MANAGER_PORT"]; ok {
		t.Errorf("gradio still gets TTS_EXTENSION_MANAGER_PORT")
	}
	if g.Env["TEST_EXTRA"] != "yes" {
		t.Errorf("gradio lost ExtraEnv")
	}
	if want := filepath.Join(e.root.InstallerFiles(), "mamba"); g.Env["MAMBA_ROOT_PREFIX"] != want {
		t.Errorf("gradio MAMBA_ROOT_PREFIX = %q, want %q", g.Env["MAMBA_ROOT_PREFIX"], want)
	}
	if g.Env["TTS_CORE_TOKEN"] != "secret" || g.Env["TTS_GRADIO_PORT"] != "1234" {
		t.Errorf("gradio lacks GradioEnv: %v", g.Env["TTS_CORE_TOKEN"])
	}
}

func TestDefaultsAndForwardedFlagsDedup(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	s, err := New(Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	c := s.Config()
	if c.PublicPort != 7770 || c.ReactPort != 3000 || c.GradioEntry != "server_core.py" ||
		strings.Join(c.GradioArgs, " ") != "--no-react" || c.ReadyTimeout != DefaultReadyTimeout {
		t.Errorf("defaults: %+v", c)
	}
	want := []string{micromambaPath(root), "-p", installEnvPath(root), "run"}
	if strings.Join(c.CommandPrefix, "|") != strings.Join(want, "|") {
		t.Errorf("prefix = %v, want %v", c.CommandPrefix, want)
	}
	c.Share = true
	c.GradioArgs = []string{"--no-react", "--share"}
	if got := strings.Join(c.gradioArgs(), " "); got != "--no-react --share" {
		t.Errorf("dedup: %q", got)
	}
	c.GradioArgs = []string{}
	c.Docker = true
	if got := strings.Join(c.gradioArgs(), " "); got != "--share --docker" {
		t.Errorf("empty args: %q", got)
	}
}

func TestConfigValidation(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	bad := []Config{
		{},
		{Root: root, PublicPort: 70000},
		{Root: root, PublicPort: 4000, ReactPort: 4000},
	}
	for i, c := range bad {
		if _, err := New(c); err == nil {
			t.Errorf("config %d accepted", i)
		}
	}
	if _, err := New(Config{Root: root, PublicPort: 4000, ReactPort: 4000, NoReact: true}); err != nil {
		t.Errorf("same port with NoReact: %v", err)
	}
}

func TestHTTPProbeWaitsForUpstream(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	s, e := newSup(t, func(c *Config) { c.NoReact = true }, "FAKE_GRADIO_READY_FILE="+ready)
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	// TCP accepts, but HTTP answers 502: the state must stay "starting".
	waitFor(t, "tcp accepting", func() bool { return tcpProbe(ctx(), e.gradioPort) })
	time.Sleep(200 * time.Millisecond)
	if st := status(s, NameGradio); st.State != StateStarting || !st.Running {
		t.Fatalf("with 502 upstream: %+v", st)
	}
	if err := os.WriteFile(ready, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateReady)
}

func TestReadyTimeoutMarksFailedKeepsProcess(t *testing.T) {
	s, _ := newSup(t, func(c *Config) { c.NoReact = true; c.ReadyTimeout = 300 * time.Millisecond },
		"FAKE_GRADIO_NOLISTEN=1")
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateFailed)
	st := status(s, NameGradio)
	if !strings.Contains(st.Error, "not ready") || !st.Running {
		t.Errorf("failed status: %+v", st)
	}
	if len(st.LogTail) == 0 || !strings.Contains(st.LogTail[0], "start") {
		t.Errorf("failed status lacks log tail: %+v", st.LogTail)
	}
	s.Stop()
	if st := status(s, NameGradio); st.Running || st.State != StateStopped {
		t.Errorf("after Stop: %+v", st)
	}
}

func TestExitReportedWithCodeAndTail(t *testing.T) {
	s, _ := newSup(t, func(c *Config) { c.NoReact = true }, "FAKE_GRADIO_EXIT=3", "FAKE_GRADIO_LINES=120")
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateExited)
	st := status(s, NameGradio)
	if st.Running || st.ExitCode == nil || *st.ExitCode != 3 {
		t.Fatalf("exit status: %+v", st)
	}
	if len(st.LogTail) != 50 || st.LogTail[49] != "line 120" || st.LogTail[0] != "line 71" {
		t.Errorf("tail: len=%d first=%q last=%q", len(st.LogTail), st.LogTail[0], st.LogTail[len(st.LogTail)-1])
	}
	// No auto-restart; the others keep running.
	time.Sleep(300 * time.Millisecond)
	if st := status(s, NameGradio); st.State != StateExited || st.Running {
		t.Errorf("restarted? %+v", st)
	}
}

func TestStartAfterAllExited(t *testing.T) {
	s, _ := newSup(t, func(c *Config) { c.NoReact = true }, "FAKE_GRADIO_EXIT=1")
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateExited)
	if err := s.Start(ctx()); err != nil {
		t.Errorf("Start after everything died: %v", err)
	}
}

func TestStartErrorMarksFailed(t *testing.T) {
	s, _ := newSup(t, func(c *Config) { c.CommandPrefix = []string{filepath.Join(t.TempDir(), "nope")} })
	// No Python is a state, not a failed Start: Gradio is reported as failed
	// with the reason, the React front is up.
	if err := s.Start(ctx()); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	st := status(s, NameGradio)
	if st.State != StateFailed || st.Running || !strings.Contains(st.Error, "no Python environment") {
		t.Errorf("status: %+v", st)
	}
	waitState(t, s, NameReact, StateReady)
}

// pidProc adds Pid() to a real proc; failRunner never starts.
type pidProc struct{ *procs.Proc }

func (pidProc) Pid() int { return 4242 }

type failRunner struct{ runner }

func (failRunner) Start(context.Context, string, []string, string, []string) error {
	return errors.New("boom")
}

// A Gradio that cannot start leaves the Go front up (it used to close it).
func TestGradioStartFailureKeepsTheReactFront(t *testing.T) {
	s, e := newSup(t, nil)
	s.newRunner = func() runner { return failRunner{} }
	if err := s.Start(ctx()); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	if g := status(s, NameGradio); g.State != StateFailed || g.Error != "boom" {
		t.Errorf("gradio: %+v", g)
	}
	waitState(t, s, NameReact, StateReady)
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", e.reactPort), time.Second)
	if err != nil {
		t.Fatalf("the front must be up: %v", err)
	}
	_ = c.Close()
	s.Stop()
	waitFor(t, "react port closed", func() bool {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", e.reactPort), 100*time.Millisecond)
		if err != nil {
			return true
		}
		_ = c.Close()
		return false
	})
}

func TestReactIsServedByTheGoFront(t *testing.T) {
	s, e := newSup(t, nil)
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameReact, StateReady)
	if st := status(s, NameReact); !st.Running || st.PID != 0 {
		t.Errorf("react status %+v: want running with no process id", st)
	}
	s.Stop()
	if st := status(s, NameReact); st.Running {
		t.Errorf("react still running after Stop: %+v", st)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", e.reactPort)
	if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		_ = c.Close()
		t.Error("the React port still accepts connections after Stop")
	}
}

func TestPidReportedWhenAvailable(t *testing.T) {
	s, _ := newSup(t, func(c *Config) { c.NoReact = true })
	s.newRunner = func() runner { return pidProc{&procs.Proc{}} }
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	if st := status(s, NameGradio); st.PID != 4242 {
		t.Errorf("pid = %d", st.PID)
	}
}

func TestBrowserOrderGradioThenReact(t *testing.T) {
	// React is ready long before Gradio; Gradio's tab still opens first.
	s, e := newSup(t, func(c *Config) { c.OpenBrowser = true }, "FAKE_GRADIO_DELAY=600ms")
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameReact, StateReady)
	if got := e.op.get(); len(got) != 0 {
		t.Fatalf("opened before gradio ready: %v", got)
	}
	waitFor(t, "both tabs", func() bool { return len(e.op.get()) == 2 })
	want := []string{fmt.Sprintf("http://127.0.0.1:%d", e.gradioPort), fmt.Sprintf("http://127.0.0.1:%d", e.reactPort)}
	if got := e.op.get(); got[0] != want[0] || got[1] != want[1] {
		t.Errorf("opened %v, want %v", got, want)
	}
}

func TestBrowserNoReactAndDisabledAndFailure(t *testing.T) {
	t.Run("NoReact", func(t *testing.T) {
		s, e := newSup(t, func(c *Config) { c.OpenBrowser, c.NoReact = true, true })
		if err := s.Start(ctx()); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "gradio tab", func() bool { return len(e.op.get()) >= 1 })
		time.Sleep(200 * time.Millisecond)
		if got := e.op.get(); len(got) != 1 || got[0] != fmt.Sprintf("http://127.0.0.1:%d", e.gradioPort) {
			t.Errorf("opened %v", got)
		}
		if st := status(s, NameReact); st.State != StateDisabled || st.Running {
			t.Errorf("react status with NoReact: %+v", st)
		}
	})
	t.Run("OpenBrowser off", func(t *testing.T) {
		s, e := newSup(t, nil)
		if err := s.Start(ctx()); err != nil {
			t.Fatal(err)
		}
		waitState(t, s, NameGradio, StateReady)
		waitState(t, s, NameReact, StateReady)
		time.Sleep(100 * time.Millisecond)
		if got := e.op.get(); len(got) != 0 {
			t.Errorf("opened %v", got)
		}
	})
	t.Run("gradio dies", func(t *testing.T) {
		s, e := newSup(t, func(c *Config) { c.OpenBrowser = true }, "FAKE_GRADIO_EXIT=1")
		if err := s.Start(ctx()); err != nil {
			t.Fatal(err)
		}
		waitState(t, s, NameGradio, StateExited)
		waitState(t, s, NameReact, StateReady)
		time.Sleep(200 * time.Millisecond)
		// Gradio's own tab is not opened, but the Go core's React tab is.
		want := fmt.Sprintf("http://127.0.0.1:%d", e.reactPort)
		if got := e.op.get(); len(got) != 1 || got[0] != want {
			t.Errorf("opened %v, want only %s", got, want)
		}
	})
}

func TestStopRestartAndOwnedContext(t *testing.T) {
	s, _ := newSup(t, nil)
	setup, cancelSetup := context.WithCancel(ctx())
	if err := s.Start(setup); err != nil {
		t.Fatal(err)
	}
	cancelSetup() // an HTTP request ending must not kill the app
	waitState(t, s, NameGradio, StateReady)
	time.Sleep(200 * time.Millisecond)
	if st := status(s, NameGradio); !st.Running {
		t.Fatalf("app died with the setup context: %+v", st)
	}
	if err := s.Start(setup); err == nil {
		t.Error("Start with a cancelled context should fail")
	}

	if err := s.Restart(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateReady)
	s.Stop()
	for _, st := range s.Statuses() {
		if st.Running || st.State != StateStopped {
			t.Errorf("after Stop: %+v", st)
		}
	}
	s.Stop() // idempotent
	if tcpProbe(ctx(), s.Config().PublicPort) {
		t.Error("gradio port still answering after Stop")
	}
}

func TestSubscribeStatus(t *testing.T) {
	s, _ := newSup(t, func(c *Config) { c.NoReact = true })
	ch, cancel := s.SubscribeStatus()
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(15 * time.Second)
	for {
		select {
		case msg := <-ch:
			var sts []ProcessStatus
			if err := json.Unmarshal([]byte(msg), &sts); err != nil || len(sts) != 2 {
				t.Fatalf("bad message %q: %v", msg, err)
			}
			if sts[0].State == StateReady && sts[1].State == StateDisabled {
				cancel()
				cancel() // second cancel is a no-op
				return
			}
		case <-deadline:
			t.Fatal("never saw gradio ready (react disabled) in the stream")
		}
	}
}

func TestSubscribePublishRace(t *testing.T) {
	s, _ := newSup(t, nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_, c := s.SubscribeStatus()
				c()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				s.publish()
			}
		}()
	}
	wg.Wait()
}

func TestAppLinkUsesConfiguredPort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	root, _ := paths.New(t.TempDir())
	s, err := New(Config{Root: root, PublicPort: port, ReactPort: tmpPort(t)})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.AppLink(), fmt.Sprintf("http://127.0.0.1:%d", port); got != want {
		t.Errorf("AppLink = %q, want %q", got, want)
	}
	srv.Close()
	if got := s.AppLink(); got != "" {
		t.Errorf("AppLink after close = %q", got)
	}
}

func TestGPUGate(t *testing.T) {
	s, e := newSup(t, func(c *Config) { c.SkipGPUCheck = false; c.NoReact = true })
	err := s.Start(ctx())
	if err == nil || !strings.Contains(err.Error(), "GPU label not set") {
		t.Fatalf("missing .gpu: %v", err)
	}
	if err := os.MkdirAll(e.root.InstallerScripts(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.root.MarkerFile(".gpu"), []byte("bogus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(ctx()); err == nil || !strings.Contains(err.Error(), "invalid GPU label") {
		t.Fatalf("bad .gpu: %v", err)
	}
	if err := os.WriteFile(e.root.MarkerFile(".gpu"), []byte("CPU\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(ctx()); err != nil {
		t.Fatalf("valid .gpu: %v", err)
	}
	waitState(t, s, NameGradio, StateReady)
}
