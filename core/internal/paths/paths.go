// Package paths resolves the TTS WebUI install root and well-known sub-paths.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrUninitializedRoot is returned when a directory does not contain
// the installer_scripts/ subdirectory that marks a TTS WebUI install.
var ErrUninitializedRoot = errors.New("not an initialized tts-webui root (installer_scripts/ missing)")

// Root is a validated TTS WebUI install root directory.
type Root struct {
	dir string
}

// New validates dir as a TTS WebUI install root and returns a Root.
// Returns ErrUninitializedRoot if installer_scripts/ is not present.
func New(dir string) (Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Root{}, fmt.Errorf("resolving root %q: %w", dir, err)
	}
	info, err := os.Stat(filepath.Join(abs, "installer_scripts"))
	if err != nil || !info.IsDir() {
		return Root{}, ErrUninitializedRoot
	}
	return Root{dir: abs}, nil
}

// FromBinary returns a Root derived from the parent directory of the running
// binary. In production the binary lives directly in the install root.
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

// Venvs returns the path to .venvs/ (per-engine Python virtual environments).
func (r Root) Venvs() string { return filepath.Join(r.dir, ".venvs") }

// Outputs returns the path to outputs/.
func (r Root) Outputs() string { return filepath.Join(r.dir, "outputs") }

// Favorites returns the path to favorites/.
func (r Root) Favorites() string { return filepath.Join(r.dir, "favorites") }

// Voices returns the path to voices/.
func (r Root) Voices() string { return filepath.Join(r.dir, "voices") }

// CoreData returns the path to installer_files/core/ where the Go core stores
// its own state files.
func (r Root) CoreData() string { return filepath.Join(r.dir, "installer_files", "core") }

// MarkerFile returns the path to a named marker file under installer_scripts/.
// Marker files include ".gpu", ".preflight_ok", ".git_version".
func (r Root) MarkerFile(name string) string {
	return filepath.Join(r.dir, "installer_scripts", name)
}
