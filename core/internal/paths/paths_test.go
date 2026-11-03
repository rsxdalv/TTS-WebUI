package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// makeRoot creates a temporary directory that looks like a TTS WebUI install
// root (has installer_scripts/ subdirectory).
func makeRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatalf("creating installer_scripts: %v", err)
	}
	return dir
}

func TestNew_Valid(t *testing.T) {
	dir := makeRoot(t)
	root, err := paths.New(dir)
	if err != nil {
		t.Fatalf("New(%q) = %v, want nil", dir, err)
	}
	if root.Dir() != filepath.Clean(dir) {
		t.Errorf("Dir() = %q, want %q", root.Dir(), filepath.Clean(dir))
	}
}

func TestNew_Missing(t *testing.T) {
	dir := t.TempDir() // no installer_scripts
	_, err := paths.New(dir)
	if err != paths.ErrUninitializedRoot {
		t.Errorf("New(%q) error = %v, want ErrUninitializedRoot", dir, err)
	}
}

func TestNew_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	// Create a file named installer_scripts (not a dir)
	f, _ := os.Create(filepath.Join(dir, "installer_scripts"))
	f.Close()
	_, err := paths.New(dir)
	if err != paths.ErrUninitializedRoot {
		t.Errorf("New with file installer_scripts error = %v, want ErrUninitializedRoot", err)
	}
}

func TestSubPaths(t *testing.T) {
	dir := makeRoot(t)
	root, _ := paths.New(dir)

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"InstallerScripts", root.InstallerScripts(), filepath.Join(dir, "installer_scripts")},
		{"InstallerFiles", root.InstallerFiles(), filepath.Join(dir, "installer_files")},
		{"Venvs", root.Venvs(), filepath.Join(dir, ".venvs")},
		{"Outputs", root.Outputs(), filepath.Join(dir, "outputs")},
		{"Favorites", root.Favorites(), filepath.Join(dir, "favorites")},
		{"Voices", root.Voices(), filepath.Join(dir, "voices")},
		{"CoreData", root.CoreData(), filepath.Join(dir, "installer_files", "core")},
		{"MarkerFile .gpu", root.MarkerFile(".gpu"), filepath.Join(dir, "installer_scripts", ".gpu")},
		{"MarkerFile .preflight_ok", root.MarkerFile(".preflight_ok"), filepath.Join(dir, "installer_scripts", ".preflight_ok")},
	}
	for _, tc := range cases {
		if filepath.Clean(tc.got) != filepath.Clean(tc.want) {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
