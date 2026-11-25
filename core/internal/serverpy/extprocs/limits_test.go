package extprocs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func startReady(t *testing.T, m *Manager, fr *fakeRoutes, pkg string) {
	t.Helper()
	if _, err := m.Start(context.Background(), pkg, StartRequest{}); err != nil {
		t.Fatalf("start %s: %v", pkg, err)
	}
	eventually(t, pkg+" ready", func() bool { _, ok := fr.get("/" + pkg); return ok })
}

func TestCapStopsLeastRecentlyUsedIdleEngine(t *testing.T) {
	m, fr, _ := newMgr(t, func(o *Options) { o.MaxRunning = 2 })
	defer m.StopAll()
	startReady(t, m, fr, "pkg_a")
	startReady(t, m, fr, "pkg_b")
	m.Activity("/pkg_a")() // a was used after b: b is the one that goes
	startReady(t, m, fr, "pkg_c")
	if _, ok := fr.get("/pkg_b"); ok {
		t.Fatal("pkg_b is still routed after the cap was reached")
	}
	if _, ok := fr.get("/pkg_a"); !ok {
		t.Fatal("pkg_a, the more recently used engine, was stopped")
	}
}

func TestCapNeverStopsBusyEngine(t *testing.T) {
	m, fr, _ := newMgr(t, func(o *Options) { o.MaxRunning = 1 })
	defer m.StopAll()
	startReady(t, m, fr, "pkg_a")
	done := m.Activity("/pkg_a")
	_, err := m.Start(context.Background(), "pkg_b", StartRequest{})
	if !errors.Is(err, ErrCapReached) {
		t.Fatalf("start with the only engine busy: %v, want ErrCapReached", err)
	}
	if _, ok := fr.get("/pkg_a"); !ok {
		t.Fatal("the busy engine was stopped")
	}
	done()
	startReady(t, m, fr, "pkg_b")
}

func TestNoCapByDefault(t *testing.T) {
	m, fr, _ := newMgr(t, nil)
	defer m.StopAll()
	for _, p := range []string{"pkg_a", "pkg_b", "pkg_c"} {
		startReady(t, m, fr, p)
	}
}

func TestIdleStopIsOffByDefaultAndSkipsBusyEngines(t *testing.T) {
	m, fr, _ := newMgr(t, nil)
	defer m.StopAll()
	startReady(t, m, fr, "pkg_a")
	if got := m.reapIdle(time.Now().Add(100 * time.Hour)); len(got) != 0 {
		t.Fatalf("idle stop is off but stopped %v", got)
	}

	m2, fr2, _ := newMgr(t, func(o *Options) { o.IdleTimeout = time.Minute })
	defer m2.StopAll()
	startReady(t, m2, fr2, "pkg_a")
	startReady(t, m2, fr2, "pkg_b")
	done := m2.Activity("/pkg_a") // a is generating
	later := time.Now().Add(time.Hour)
	got := m2.reapIdle(later)
	if len(got) != 1 || got[0] != "pkg_b" {
		t.Fatalf("reaped %v, want only pkg_b (pkg_a is busy)", got)
	}
	if _, ok := fr2.get("/pkg_a"); !ok {
		t.Fatal("the busy engine lost its route")
	}
	done()
	if got := m2.reapIdle(time.Now()); len(got) != 0 {
		t.Fatalf("an engine used just now was reaped: %v", got)
	}
}
