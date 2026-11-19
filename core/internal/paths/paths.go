// Package paths resolves the TTS WebUI install root and well-known sub-paths.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Root is a TTS WebUI install root directory. It may be empty: the launcher
// downloads installer_scripts/ into it.
type Root struct {
	dir string
}

// New returns a Root for dir (made absolute).
func New(dir string) (Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Root{}, fmt.Errorf("resolving root %q: %w", dir, err)
	}
	return Root{dir: abs}, nil
}

// FromBinary returns a Root for the folder holding the running binary.
func FromBinary() (Root, error) {
	exe, err := os.Executable()
	if err != nil {
		return Root{}, fmt.Errorf("finding binary path: %w", err)
	}
	return New(filepath.Dir(exe))
}

// Dir returns the absolute root directory path.
func (r Root) Dir() string { return r.dir }

// InstallerScripts returns the path to installer_scripts/.
func (r Root) InstallerScripts() string { return filepath.Join(r.dir, "installer_scripts") }

// InstallerFiles returns the path to installer_files/.
func (r Root) InstallerFiles() string { return filepath.Join(r.dir, "installer_files") }

// MarkerFile returns the path to a named marker file under installer_scripts/
// (".gpu", ".preflight_ok", ".git_version").
func (r Root) MarkerFile(name string) string {
	return filepath.Join(r.dir, "installer_scripts", name)
}
