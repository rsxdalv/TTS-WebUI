package supervise

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func portOpen(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// The React front can be launched alone, and Gradio added later without
// touching the front.
func TestStartPartsFrontThenGradio(t *testing.T) {
	s, e := newSup(t, nil)
	if err := s.StartParts(ctx(), NameReact); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameReact, StateReady)
	if st := status(s, NameGradio); st.Running || st.State != StateStopped {
		t.Fatalf("gradio started with the front alone: %+v", st)
	}
	if portOpen(e.gradioPort) {
		t.Fatal("the Gradio port answers although only the front was started")
	}

	if err := s.StartParts(ctx(), NameReact); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("starting a running part again: %v", err)
	}
	if err := s.StartParts(ctx(), NameGradio); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateReady)
	if st := status(s, NameReact); !st.Running || st.State != StateReady {
		t.Fatalf("the front did not stay up: %+v", st)
	}
}

// Gradio alone, then the front: the same run.
func TestStartPartsGradioThenFront(t *testing.T) {
	s, e := newSup(t, nil)
	if err := s.StartParts(ctx(), NameGradio); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateReady)
	if portOpen(e.reactPort) {
		t.Fatal("the front answers although only Gradio was started")
	}
	if err := s.StartParts(ctx(), NameReact); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameReact, StateReady)
	if st := status(s, NameGradio); !st.Running {
		t.Fatalf("gradio did not stay up: %+v", st)
	}
}

// Start still starts both, and also fills in the missing part of a run.
func TestStartFillsInTheMissingPart(t *testing.T) {
	s, _ := newSup(t, nil)
	if err := s.StartParts(ctx(), NameReact); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameReact, StateReady)
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateReady)
	if err := s.Start(ctx()); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("Start with both up: %v", err)
	}
}

// StopPart stops one part; the last one takes the whole run down, so a new
// Start works and the ports are free.
func TestStopPart(t *testing.T) {
	s, e := newSup(t, nil)
	if err := s.Start(ctx()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateReady)
	waitState(t, s, NameReact, StateReady)

	s.StopPart(NameReact)
	if st := status(s, NameReact); st.Running || st.State != StateStopped {
		t.Fatalf("front after StopPart: %+v", st)
	}
	if portOpen(e.reactPort) {
		t.Error("the front port still answers after StopPart")
	}
	if st := status(s, NameGradio); !st.Running {
		t.Fatalf("gradio stopped with the front: %+v", st)
	}

	// The front can come back.
	if err := s.StartParts(ctx(), NameReact); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameReact, StateReady)

	s.StopPart(NameGradio)
	s.StopPart(NameReact)
	for _, st := range s.Statuses() {
		if st.Running || st.State != StateStopped {
			t.Errorf("after the last StopPart: %+v", st)
		}
	}
	if err := s.Start(ctx()); err != nil {
		t.Fatalf("Start after the run was stopped part by part: %v", err)
	}
	waitState(t, s, NameGradio, StateReady)
}

func TestStartPartsRefusesUnknownAndDisabled(t *testing.T) {
	s, _ := newSup(t, nil)
	if err := s.StartParts(ctx(), "nope"); err == nil {
		t.Error("an unknown part was accepted")
	}
	s2, _ := newSup(t, func(c *Config) { c.NoReact = true })
	if err := s2.StartParts(ctx(), NameReact); err == nil {
		t.Error("the React part started although it is turned off")
	}
}

// Requests that arrive together (a double click, two pages) must not both
// start the same part: one starts it, the others find it running.
func TestConcurrentStartsDoNotCollide(t *testing.T) {
	s, _ := newSup(t, nil)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = s.StartParts(ctx(), NameReact)
		}(i)
	}
	wg.Wait()
	started := 0
	for _, err := range errs {
		switch {
		case err == nil:
			started++
		case errors.Is(err, ErrAlreadyRunning):
		default:
			t.Errorf("a concurrent start failed: %v", err)
		}
	}
	if started != 1 {
		t.Errorf("%d requests started the part, want 1", started)
	}
	waitState(t, s, NameReact, StateReady)
}
