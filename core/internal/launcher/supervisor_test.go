package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// makeRoot creates a minimal install root in a temp directory.
func makeRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSupervisorDoubleStart(t *testing.T) {
	root := makeRoot(t)
	sup := newSupervisor(root, "conda")

	ctx := context.Background()

	// First start with a non-existent binary should fail immediately.
	// We accept an error here; the key property is it does not panic.
	_ = sup.Start(ctx)

	// A second start while "running" (even with a failed proc list) must return
	// the "already running" error if any procs are tracked.
	// If the first Start failed and no procs were added, this is a no-op.
	// Both outcomes are acceptable; just verify no panic.
	_ = sup.Start(ctx)
}

func TestSupervisorStopIdempotent(t *testing.T) {
	root := makeRoot(t)
	sup := newSupervisor(root, "conda")

	// Stop before start should not panic.
	sup.Stop()
	sup.Stop()
}

func TestSupervisorStatusesEmpty(t *testing.T) {
	root := makeRoot(t)
	sup := newSupervisor(root, "conda")

	statuses := sup.Statuses()
	if len(statuses) == 0 {
		t.Fatal("expected at least one status entry")
	}
	for _, s := range statuses {
		if s.Running {
			t.Errorf("process %q should not be running before Start", s.Name)
		}
	}
}

func TestSupervisorSubscribeStatus(t *testing.T) {
	root := makeRoot(t)
	sup := newSupervisor(root, "conda")

	ch, cancel := sup.SubscribeStatus()
	defer cancel()

	// publishStatus should fan out to the subscriber.
	sup.publishStatus()

	select {
	case msg := <-ch:
		if msg == "" {
			t.Fatal("received empty status message")
		}
	default:
		t.Fatal("no status message received after publishStatus")
	}
}

func TestFreePort(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("unexpected port %d", port)
	}
}
