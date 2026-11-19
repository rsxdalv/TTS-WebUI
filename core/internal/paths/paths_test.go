package paths_test

import (
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func TestNewAllowsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	root, err := paths.New(dir)
	if err != nil {
		t.Fatalf("New on empty dir: %v", err)
	}
	if root.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", root.Dir(), dir)
	}
}

func TestSubPaths(t *testing.T) {
	dir := t.TempDir()
	root, _ := paths.New(dir)
	if got, want := root.InstallerScripts(), filepath.Join(dir, "installer_scripts"); got != want {
		t.Errorf("InstallerScripts() = %q, want %q", got, want)
	}
	if got, want := root.InstallerFiles(), filepath.Join(dir, "installer_files"); got != want {
		t.Errorf("InstallerFiles() = %q, want %q", got, want)
	}
	if got, want := root.MarkerFile(".gpu"), filepath.Join(dir, "installer_scripts", ".gpu"); got != want {
		t.Errorf("MarkerFile() = %q, want %q", got, want)
	}
}
