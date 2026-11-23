package supervise

import (
	"context"
	"testing"
)

// The gradio child runs inside a closingRunner when the Go side is plugged in;
// its output must still be streamable (the launcher's --start copies it).
func TestSubscribeLogsSeesThroughTheClosingRunner(t *testing.T) {
	s, _ := newSup(t, func(c *Config) {
		c.NoReact = true
		c.StartGradioSide = func(prefix, baseEnv []string) (*GradioSide, error) {
			return &GradioSide{Close: func() {}}, nil
		}
	})
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, NameGradio, StateReady)
	lines, cancel, ok := s.SubscribeLogs(NameGradio)
	if !ok {
		t.Fatal("SubscribeLogs: not ok for a wrapped gradio child")
	}
	cancel()
	if _, _, ok := s.SubscribeLogs("nope"); ok {
		t.Error("SubscribeLogs: ok for an unknown child")
	}
	_ = lines
}
