package launcher

import (
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

func TestFreePort(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("unexpected port %d", port)
	}
}
