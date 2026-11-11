package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/state"
)

// makeRoot creates a minimal TTS WebUI install root in a temp dir.
func makeRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatalf("paths.New: %v", err)
	}
	return root
}

func TestWriteThenRead(t *testing.T) {
	root := makeRoot(t)
	s := &state.State{}
	if err := state.Write(root, s); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := state.Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Schema == "" {
		t.Error("Read returned empty Schema")
	}
}

func TestRead_NotExist_ReturnsDefault(t *testing.T) {
	root := makeRoot(t)
	s, err := state.Read(root)
	if err != nil {
		t.Fatalf("Read on missing file: %v", err)
	}
	if s == nil {
		t.Fatal("Read returned nil State")
	}
}

func TestRead_SchemaMismatch(t *testing.T) {
	root := makeRoot(t)

	// Write a state file with an unknown schema
	dir := root.CoreData()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	bad := []byte(`{"schema":"future/v99"}`)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), bad, 0o644); err != nil {
		t.Fatalf("writing bad state: %v", err)
	}

	_, err := state.Read(root)
	if err == nil {
		t.Fatal("Read with bad schema: expected error, got nil")
	}
	var mismatch *state.ErrSchemaMismatch
	if !errors.As(err, &mismatch) {
		t.Errorf("expected ErrSchemaMismatch, got %T: %v", err, err)
	}
}

func TestReadGPUMarker(t *testing.T) {
	root := makeRoot(t)

	// Write a marker
	content := "NVIDIA GPU\n"
	if err := os.WriteFile(root.MarkerFile(".gpu"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing .gpu: %v", err)
	}

	got, err := state.ReadGPUMarker(root)
	if err != nil {
		t.Fatalf("ReadGPUMarker: %v", err)
	}
	if got != "NVIDIA GPU" {
		t.Errorf("ReadGPUMarker = %q, want %q", got, "NVIDIA GPU")
	}
}

func TestReadGPUMarker_Missing(t *testing.T) {
	root := makeRoot(t)
	_, err := state.ReadGPUMarker(root)
	if err == nil {
		t.Fatal("ReadGPUMarker on missing file: expected error")
	}
}

func TestReadPreflight(t *testing.T) {
	root := makeRoot(t)

	// Missing → false
	ok, err := state.ReadPreflight(root)
	if err != nil {
		t.Fatalf("ReadPreflight missing: %v", err)
	}
	if ok {
		t.Error("ReadPreflight on missing file: want false")
	}

	// Create the marker → true
	if err := os.WriteFile(root.MarkerFile(".preflight_ok"), nil, 0o644); err != nil {
		t.Fatalf("writing .preflight_ok: %v", err)
	}
	ok, err = state.ReadPreflight(root)
	if err != nil {
		t.Fatalf("ReadPreflight present: %v", err)
	}
	if !ok {
		t.Error("ReadPreflight on existing file: want true")
	}
}

func TestWriteAtomic(t *testing.T) {
	// Write should not leave a .tmp file behind
	root := makeRoot(t)
	if err := state.Write(root, &state.State{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	entries, _ := os.ReadDir(root.CoreData())
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
