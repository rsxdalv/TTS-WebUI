package uv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// TestRealInstall installs the pinned uv and Python from the internet into the
// folder named by UV_REAL_ROOT (a scratch folder, never an install) and makes a
// venv with the flags Create uses. It is skipped unless the variable is set.
func TestRealInstall(t *testing.T) {
	dir := os.Getenv("UV_REAL_ROOT")
	if dir == "" {
		t.Skip("UV_REAL_ROOT is not set")
	}
	root, _ := paths.New(dir)
	p := New(Options{Root: root, GPU: gpu.CPU})
	var lines []string
	emit := func(l string) { t.Log(l); lines = append(lines, l) }
	if err := p.Install(context.Background(), emit); err != nil {
		t.Fatal(err)
	}
	if !p.Status(context.Background()).Ready {
		t.Fatal("not ready")
	}
	venv := filepath.Join(dir, ".venvs", "probe")
	cache := []string{"--cache-dir", p.CacheDir(), "--no-config"}
	if err := p.run(context.Background(), emit, append([]string{"venv", venv, "--python", PythonVersion, "--managed-python", "--no-python-downloads", "--allow-existing"}, cache...)...); err != nil {
		t.Fatal(err)
	}
	py := platform.VenvPython(dir, "probe")
	if err := p.run(context.Background(), emit, append([]string{"pip", "install", "--python", py}, append(cache, "six")...)...); err != nil {
		t.Fatal(err)
	}
	c, _ := p.Command(&platform.Entry{Package: "probe", Own: true}, []string{"python", "-c", "import six,sys;print(sys.version)"})
	var out []string
	if err := platform.Run(context.Background(), c, func(l string) { out = append(out, l) }); err != nil || !strings.Contains(strings.Join(out, ""), "3.10.11") {
		t.Fatalf("python: %v %v", out, err)
	}
}
