package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFixture(t *testing.T) {
	dir := "testdata"
	manifests, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Fixture has 3 entries: bark (proxy:native), xtts (no engine), custom_engine (engine block).
	// xtts has no engine or proxy:native, so it should be excluded.
	if len(manifests) != 2 {
		t.Fatalf("got %d manifests, want 2", len(manifests))
	}

	// bark → derived python-venv
	bark := findByPackage(manifests, "tts_webui_extension.bark")
	if bark == nil {
		t.Fatal("bark manifest missing")
	}
	if bark.Engine.Kind != KindPythonVenv {
		t.Errorf("bark kind = %q, want python-venv", bark.Engine.Kind)
	}
	if bark.Engine.Health == nil || bark.Engine.Health.Path != "/health" {
		t.Errorf("bark health not derived correctly")
	}

	// custom_engine → explicit engine block
	custom := findByPackage(manifests, "tts_webui_extension.custom_engine")
	if custom == nil {
		t.Fatal("custom_engine manifest missing")
	}
	if custom.Engine.Kind != KindPythonVenv {
		t.Errorf("custom kind = %q, want python-venv", custom.Engine.Kind)
	}
	if custom.Engine.Cancel == nil || custom.Engine.Cancel.Mode != CancelModeEndpoint {
		t.Errorf("custom cancel not parsed correctly: %+v", custom.Engine.Cancel)
	}
	if custom.Engine.Launch == nil || custom.Engine.Launch.PortArg != "--port" {
		t.Errorf("custom launch port_arg not parsed: %+v", custom.Engine.Launch)
	}
}

func TestLoadExternalFile(t *testing.T) {
	dir := t.TempDir()
	// Write a main extensions.json with one native-proxy entry.
	mainJSON := `{"tabsInGroups":{"extensions":[{"package_name":"pkg.one","name":"One","proxy":"native"}]}}`
	os.WriteFile(filepath.Join(dir, "extensions.json"), []byte(mainJSON), 0o644)

	// Write an external file with a tab.
	extJSON := `{"tabs":[{"package_name":"pkg.two","name":"Two","proxy":"native"}]}`
	os.WriteFile(filepath.Join(dir, "extensions.external.json"), []byte(extJSON), 0o644)

	manifests, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 2 {
		t.Fatalf("got %d manifests, want 2", len(manifests))
	}
}

func TestLoadMissingExternalFile(t *testing.T) {
	dir := t.TempDir()
	mainJSON := `{"tabsInGroups":{"extensions":[{"package_name":"pkg.one","name":"One","proxy":"native"}]}}`
	os.WriteFile(filepath.Join(dir, "extensions.json"), []byte(mainJSON), 0o644)
	// No extensions.external.json — should not error.
	manifests, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 1 {
		t.Fatalf("got %d manifests, want 1", len(manifests))
	}
}

func TestNoEngineEntryExcluded(t *testing.T) {
	dir := t.TempDir()
	mainJSON := `{"tabsInGroups":{"extensions":[
		{"package_name":"plain","name":"Plain","requirements":"some-pkg"}
	]}}`
	os.WriteFile(filepath.Join(dir, "extensions.json"), []byte(mainJSON), 0o644)
	manifests, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests, got %d", len(manifests))
	}
}

func findByPackage(ms []Manifest, pkg string) *Manifest {
	for i := range ms {
		if ms[i].PackageName == pkg {
			return &ms[i]
		}
	}
	return nil
}
