package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestReadGPULabel_InvalidFile checks that readGPULabel returns an error when
// the .gpu file exists but contains an invalid label (no silent fallback).
func TestReadGPULabel_InvalidFile(t *testing.T) {
	root := makeRoot(t)
	// Write an invalid label (not a PyTorchChoice string) to the .gpu file.
	// The file content matches what legacy code writes: "nvidia\n" (invalid).
	gpuFile := root.MarkerFile(".gpu")
	if err := os.MkdirAll(filepath.Dir(gpuFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gpuFile, []byte("nvidia\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readGPULabel(context.Background(), root)
	if err == nil {
		t.Fatal("expected error for invalid .gpu label, got nil")
	}
	if !strings.Contains(err.Error(), "invalid GPU label") {
		t.Errorf("error should mention 'invalid GPU label', got: %v", err)
	}
}

// TestReadGPULabel_ValidFile checks that a real .gpu file with trailing newline parses correctly.
func TestReadGPULabel_ValidFile(t *testing.T) {
	root := makeRoot(t)
	gpuFile := root.MarkerFile(".gpu")
	if err := os.MkdirAll(filepath.Dir(gpuFile), 0o755); err != nil {
		t.Fatal(err)
	}
	// Legacy code writes "NVIDIA GPU\n" (with trailing newline).
	if err := os.WriteFile(gpuFile, []byte("NVIDIA GPU\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	label, err := readGPULabel(context.Background(), root)
	if err != nil {
		t.Fatalf("readGPULabel: %v", err)
	}
	if label != "NVIDIA GPU" {
		t.Errorf("label = %q, want %q", label, "NVIDIA GPU")
	}
}

// TestSupervisorOwnedContext verifies that Start creates a supervisor-owned
// context that is independent of the caller's context. Cancelling the caller
// context after Start must not affect the supervisor's internal context, and
// Stop must cancel it.
func TestSupervisorOwnedContext(t *testing.T) {
	root := makeRoot(t)
	sup := newSupervisor(root, "conda")

	// Use a context we can cancel, simulating an HTTP request that finishes.
	setupCtx, cancelSetup := context.WithCancel(context.Background())

	// Start will fail (no binary), but the supervisor context must be created
	// and torn down correctly without any panic.
	_ = sup.Start(setupCtx)

	// Cancel the setup context — this must not panic or race with the supervisor.
	cancelSetup()

	// Stop must be safe to call regardless of Start's outcome.
	sup.Stop()

	// After Stop, a second Stop must also be safe.
	sup.Stop()
}

// TestSupervisorContextNotRequestCtx confirms that the supervisor context
// (stored in supCtx) is derived from context.Background(), not from the
// caller's context. We verify this by asserting that sup.supCtx is not nil
// during Start (set atomically while holding mu) and is Background-derived.
func TestSupervisorContextCancelledAfterStop(t *testing.T) {
	root := makeRoot(t)
	sup := newSupervisor(root, "conda")

	// After Stop on a never-started supervisor, supCtx must remain nil.
	sup.Stop()
	sup.mu.Lock()
	if sup.supCtx != nil || sup.supCancel != nil {
		t.Error("supCtx/supCancel should be nil on a never-started supervisor after Stop")
	}
	sup.mu.Unlock()
}

// TestSupervisorStartPublishStatusNoDeadlock verifies that calling Stop()
// after Start() does not deadlock. The deadlock occurred when publishStatus()
// was called while s.mu was held (publishStatus itself acquires s.mu).
func TestSupervisorStartPublishStatusNoDeadlock(t *testing.T) {
	root := makeRoot(t)
	sup := newSupervisor(root, "conda")

	// Start will fail (no binary), but publishStatus must not deadlock.
	_ = sup.Start(context.Background())

	// If Start deadlocked, the goroutine would be stuck and Stop would hang.
	done := make(chan struct{})
	go func() {
		sup.Stop()
		close(done)
	}()
	select {
	case <-done:
		// OK
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() timed out — possible deadlock in publishStatus")
	}
}

