//go:build integration

package env

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// TestIntegration_UVPythonInstall runs uv python install for real.
// Requires uv to be present in PATH or in the tools directory.
// Tag: integration (needs human verification / real download)
func TestIntegration_UVPythonInstall(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := CreateMain(context.Background(), root, "cpu"); err != nil {
		t.Fatalf("CreateMain: %v", err)
	}

	venvPath := filepath.Join(root.Venvs(), "main")
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		t.Errorf("venv not created at %s", venvPath)
	}
}
