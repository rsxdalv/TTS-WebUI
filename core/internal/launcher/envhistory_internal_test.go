package launcher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// writeEnvHistory gives the install a finished env: conda-meta/history as
// micromamba leaves it after init_mamba's install (lines from a real env).
func writeEnvHistory(t *testing.T, root paths.Root) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "conda-meta-history"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root.InstallerFiles(), "env", "conda-meta")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "history"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
