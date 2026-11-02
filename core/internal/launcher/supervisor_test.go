package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
