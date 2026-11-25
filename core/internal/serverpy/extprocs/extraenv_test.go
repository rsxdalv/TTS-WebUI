package extprocs

import (
	"context"
	"errors"
	"testing"
)

func TestExtraEnvWinsOverTheBase(t *testing.T) {
	m, _, _ := newMgr(t, func(o *Options) {
		t.Setenv("DELTA_MARK", "from-base")
		o.ExtraEnv = func() ([]string, error) { return []string{"DELTA_MARK=from-go"}, nil }
	})
	if _, err := m.Start(context.Background(), "extra", StartRequest{Port: freeP(t)}); err != nil {
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

func TestPackageEnvWinsOverExtraAndBase(t *testing.T) {
	m, _, _ := newMgr(t, func(o *Options) {
		t.Setenv("DELTA_MARK", "from-base")
		o.ExtraEnv = func() ([]string, error) { return []string{"DELTA_MARK=from-extra"}, nil }
		o.PackageEnv = func(pkg string) ([]string, error) {
			if pkg != "extra" {
				t.Errorf("asked for %q", pkg)
			}
			return []string{"DELTA_MARK=from-entry"}, nil
		}
	})
	if _, err := m.Start(context.Background(), "extra", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	if got := firstReport(t, m, "extra").Env["DELTA_MARK"]; got != "from-entry" {
		t.Fatalf("DELTA_MARK = %q", got)
	}
}

func TestPackageEnvErrorStopsTheStart(t *testing.T) {
	m, _, _ := newMgr(t, func(o *Options) {
		o.PackageEnv = func(string) ([]string, error) { return nil, errors.New("env not allowed") }
	})
	if _, err := m.Start(context.Background(), "extra", StartRequest{Port: freeP(t)}); err == nil {
		t.Fatal("started with a bad entry env")
	}
}
