package launcher

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// micromambaPath returns the platform-specific micromamba binary path.
func micromambaPath(root paths.Root) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root.InstallerFiles(), "mamba", "micromamba.exe")
	}
	return filepath.Join(root.InstallerFiles(), "mamba", "bin", "micromamba")
}

// installEnvPath returns the path to the conda env used by micromamba.
func installEnvPath(root paths.Root) string {
	return filepath.Join(root.InstallerFiles(), "env")
}

// mambaScript returns the platform-specific mamba init script path.
func mambaScript(root paths.Root) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root.InstallerScripts(), "init_mamba.bat")
	}
	return filepath.Join(root.InstallerScripts(), "init_mamba.sh")
}

// appPython returns the platform-specific Python binary inside the conda env.
func appPython(root paths.Root) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root.InstallerFiles(), "env", "python.exe")
	}
	return filepath.Join(root.InstallerFiles(), "env", "bin", "python")
}

// downloadDone returns true when installer_scripts/ directory exists.
func downloadDone(root paths.Root) bool {
	info, err := os.Stat(root.InstallerScripts())
	return err == nil && info.IsDir()
}

// preflightDone returns true when .preflight_ok marker exists.
func preflightDone(root paths.Root) bool {
	_, err := os.Stat(root.MarkerFile(".preflight_ok"))
	return err == nil
}

// mambaDone returns true when the micromamba binary is present.
// Bug fix: legacy code only checked the directory; we check the binary.
func mambaDone(root paths.Root) bool {
	_, err := os.Stat(micromambaPath(root))
	return err == nil
}

// gpuDone returns true when installer_scripts/.gpu exists.
func gpuDone(root paths.Root) bool {
	_, err := os.Stat(root.MarkerFile(".gpu"))
	return err == nil
}

// installDone returns true when installer_scripts/.git_version exists.
func installDone(root paths.Root) bool {
	_, err := os.Stat(root.MarkerFile(".git_version"))
	return err == nil
}

// uvPythonPath returns the Python binary inside the uv-managed main venv
// (root.Venvs()/main). This is the Python used when --env=uv is active.
func uvPythonPath(root paths.Root) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root.Venvs(), "main", "Scripts", "python.exe")
	}
	return filepath.Join(root.Venvs(), "main", "bin", "python")
}
