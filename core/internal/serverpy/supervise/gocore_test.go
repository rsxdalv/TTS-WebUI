package supervise

// The Go core runs without Gradio: its side (router, control API, services)
// is started first, has its own lifetime, and is closed only by Stop (or by the
// next Start). A Gradio that cannot start, has no Python, or dies is a state.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// sideProbe is a fake Go side: a loopback server standing in for the control API.
type sideProbe struct {
	created, closed atomic.Int32
	mu              sync.Mutex
	urls            []string
}

func (p *sideProbe) start(_, _ []string) (*GradioSide, error) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	p.created.Add(1)
	p.mu.Lock()
	p.urls = append(p.urls, srv.URL)
	p.mu.Unlock()
	return &GradioSide{
		CoreURL: srv.URL, CoreToken: "t",
		Close: func() { p.closed.Add(1); srv.Close() },
	}, nil
}

func (p *sideProbe) last() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.urls[len(p.urls)-1]
}

func (p *sideProbe) answers() bool {
	resp, err := http.Get(p.last())
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == 200
}

func TestGoSideOutlivesGradio(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Supervisor)
		env    []string
		state  State
	}{
		"gradio cannot start": {mutate: func(s *Supervisor) { s.newRunner = func() runner { return failRunner{} } }, state: StateFailed},
		"gradio dies later":   {env: []string{"FAKE_GRADIO_EXIT=1"}, state: StateExited},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := &sideProbe{}
			s, _ := newSup(t, func(cfg *Config) { cfg.StartGradioSide = p.start }, c.env...)
			if c.mutate != nil {
				c.mutate(s)
			}
			if err := s.Start(ctx()); err != nil {
				t.Fatalf("Start: %v", err)
			}
			waitState(t, s, NameGradio, c.state)
			waitState(t, s, NameReact, StateReady)
			if !p.answers() || p.closed.Load() != 0 {
				t.Fatalf("the Go side must stay up: answers=%v closed=%d", p.answers(), p.closed.Load())
			}
			s.Stop()
			if p.closed.Load() != 1 {
				t.Fatalf("Stop must close the Go side once, closed=%d", p.closed.Load())
			}
		})
	}
}

// No Python environment at all (the default micromamba prefix does not exist)
// and no GPU label: the core starts, Gradio is reported with the reason.
func TestCoreStartsWithoutPythonAndWithoutGPULabel(t *testing.T) {
	p := &sideProbe{}
	s, _ := newSup(t, func(c *Config) {
		c.StartGradioSide = p.start
		c.CommandPrefix = nil // the default micromamba prefix; nothing is installed in the temp root
		c.SkipGPUCheck = false
	})
	if err := s.Start(ctx()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	g := status(s, NameGradio)
	if g.State != StateFailed || g.Running || !strings.Contains(g.Error, "no Python environment") {
		t.Errorf("gradio: %+v", g)
	}
	waitState(t, s, NameReact, StateReady)
	if !p.answers() {
		t.Error("the Go side is not up")
	}
	if !s.Statuses()[1].Running {
		t.Error("the React front must be reported as running")
	}
	s.Stop()
}

// With Python present but no GPU label the install is half done: Start still
// stops (TestGPUGate); the gate is only dropped when Gradio cannot start at all.

// Gradio died and only the Go side is left (no React): Start again releases the
// old Go side, so the new one can bind the same ports.
func TestStartAgainReleasesTheOldGoSide(t *testing.T) {
	p := &sideProbe{}
	s, _ := newSup(t, func(c *Config) { c.NoReact = true; c.StartGradioSide = p.start }, "FAKE_GRADIO_EXIT=1")
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateExited)
	if err := s.Start(ctx()); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if p.created.Load() != 2 || p.closed.Load() != 1 {
		t.Fatalf("created=%d closed=%d, want 2 and 1", p.created.Load(), p.closed.Load())
	}
	s.Stop()
	if p.closed.Load() != 2 {
		t.Fatalf("closed=%d after Stop, want 2", p.closed.Load())
	}
}
