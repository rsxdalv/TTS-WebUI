package extprocs

import (
	"context"
	"errors"
	"testing"
)

func TestExtraEnvWinsOverTheRequest(t *testing.T) {
	m, _, _ := newMgr(t, func(o *Options) {
		o.ExtraEnv = func() ([]string, error) { return []string{"DELTA_MARK=from-go"}, nil }
	})
	if _, err := m.Start(context.Background(), "extra", StartRequest{
		Port: freeP(t),
		Env:  map[string]string{"DELTA_MARK": "from-python"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := firstReport(t, m, "extra").Env["DELTA_MARK"]; got != "from-go" {
		t.Fatalf("DELTA_MARK = %q", got)
	}
}

func TestExtraEnvErrorStopsTheStart(t *testing.T) {
	m, _, _ := newMgr(t, func(o *Options) {
		o.ExtraEnv = func() ([]string, error) { return nil, errors.New("config unreadable") }
	})
	if _, err := m.Start(context.Background(), "extra", StartRequest{Port: freeP(t)}); err == nil {
		t.Fatal("started with an environment Go could not compute")
	}
	if len(m.List()) != 0 {
		t.Fatalf("a failed start left an entry: %+v", m.List())
	}
}
