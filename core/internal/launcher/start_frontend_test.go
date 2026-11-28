package launcher

import (
	"context"
	"errors"
	"testing"
)

// recordSup is an app supervisor that only records StartParts.
type recordSup struct {
	appSupervisor
	parts []string
	err   error
}

func (r *recordSup) StartParts(_ context.Context, parts ...string) error {
	r.parts = append(r.parts, parts...)
	return r.err
}

func TestStartFrontendStartsOnlyTheReactPart(t *testing.T) {
	rs := &recordSup{}
	l := &Launcher{sup: rs}
	if err := l.StartFrontend(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(rs.parts) != 1 || rs.parts[0] != "react-ui" {
		t.Fatalf("started %v, want only react-ui", rs.parts)
	}
}

func TestStartFrontendIgnoresARunningApp(t *testing.T) {
	l := &Launcher{sup: &recordSup{err: errors.New("app already running")}}
	if err := l.StartFrontend(context.Background()); err != nil {
		t.Fatalf("a running app is not an error, got %v", err)
	}
	l = &Launcher{sup: &recordSup{err: errors.New("boom")}}
	if err := l.StartFrontend(context.Background()); err == nil {
		t.Fatal("a real failure was swallowed")
	}
}
