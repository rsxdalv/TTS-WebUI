package supervise

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrepareErrorAbortsStart(t *testing.T) {
	boom := errors.New("bad switch")
	s, _ := newSup(t, func(c *Config) {
		c.Prepare = func(context.Context) (Plan, error) { return Plan{}, boom }
	})
	if err := s.Start(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Start = %v, want %v", err, boom)
	}
	if s.aliveLocked() {
		t.Fatal("children started after Prepare failed")
	}
}

func TestHooksSideAndRecord(t *testing.T) {
	var closed, started, stopped atomic.Int32
	var refs []ProcRef
	s, e := newSup(t, func(c *Config) {
		c.StartGradioSide = func(prefix, baseEnv []string) (*GradioSide, error) {
			return &GradioSide{Close: func() { closed.Add(1) }}, nil
		}
		c.OnStarted = func(r []ProcRef) { refs = r; started.Add(1) }
		c.OnStopped = func() { stopped.Add(1) }
	})
	_ = e
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "gradio ready", func() bool { return status(s, NameGradio).State == StateReady })
	if started.Load() != 1 || len(refs) != 2 {
		t.Fatalf("OnStarted calls=%d refs=%v", started.Load(), refs)
	}
	for _, r := range refs {
		if r.PID <= 0 {
			t.Fatalf("no pid for %s", r.Name)
		}
	}
	s.Stop()
	if closed.Load() != 1 || stopped.Load() != 1 {
		t.Fatalf("closed=%d stopped=%d, want 1 and 1", closed.Load(), stopped.Load())
	}
}

func TestSideStartErrorStopsStart(t *testing.T) {
	s, _ := newSup(t, func(c *Config) {
		c.StartGradioSide = func(_, _ []string) (*GradioSide, error) { return nil, errors.New("port taken") }
	})
	if err := s.Start(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
	if s.aliveLocked() {
		t.Fatal("children started")
	}
}

func TestSkipReactTabAndPlanOpenBrowser(t *testing.T) {
	on := true
	s, e := newSup(t, func(c *Config) {
		c.SkipReactTab = true
		c.Prepare = func(context.Context) (Plan, error) { return Plan{OpenBrowser: &on}, nil }
	})
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "react ready", func() bool { return status(s, NameReact).State == StateReady })
	time.Sleep(200 * time.Millisecond)
	if urls := e.op.get(); len(urls) != 1 || urls[0] != publicURL(e.gradioPort) {
		t.Fatalf("opened %v, want only the public URL", urls)
	}
}
