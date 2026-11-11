package legacy

import (
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"os"
	"path/filepath"
)

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

func TestLegacyGradioManifest(t *testing.T) {
	root := makeRoot(t)
	m := LegacyGradioManifest(root)

	if m.PackageName != "legacy-gradio" {
		t.Errorf("unexpected PackageName %q", m.PackageName)
	}
	if m.Engine.Kind != manifest.KindPythonVenv {
		t.Errorf("unexpected Kind %q", m.Engine.Kind)
	}
	if m.Engine.Health == nil {
		t.Fatal("Health should not be nil")
	}
	if m.Engine.Health.TimeoutS != 60 {
		t.Errorf("unexpected health timeout %d", m.Engine.Health.TimeoutS)
	}
	if len(m.Engine.Launch.Cmd) < 2 {
		t.Errorf("Launch.Cmd too short: %v", m.Engine.Launch.Cmd)
	}
}

func TestLoadNativeExtensionsEmpty(t *testing.T) {
	dir := t.TempDir()
	// Write an extensions.json with no proxy:native entries.
	if err := os.WriteFile(filepath.Join(dir, "extensions.json"), []byte(`{"tabsInGroups":{"extensions":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	exts, err := LoadNativeExtensions(dir)
	if err != nil {
		t.Fatalf("LoadNativeExtensions: %v", err)
	}
	if len(exts) != 0 {
		t.Errorf("expected 0 native extensions, got %d", len(exts))
	}
}
