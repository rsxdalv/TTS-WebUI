package launcher

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// micromambaPath returns the platform-specific micromamba binary path.
func micromambaPath(root paths.Root) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root.InstallerFiles(), "mamba", "micromamba.exe")
	}
	// init_mamba.sh writes installer_files/mamba/micromamba (no bin/ subdirectory).
	return filepath.Join(root.InstallerFiles(), "mamba", "micromamba")
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

// mambaEnv returns os.Environ() with MAMBA_ROOT_PREFIX pinned to the install's
// mamba directory, preventing micromamba from falling back to ~/.micromamba.
func mambaEnv(root paths.Root) []string {
	return append(os.Environ(),
		"MAMBA_ROOT_PREFIX="+filepath.Join(root.InstallerFiles(), "mamba"))
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

// mambaDone returns true when the install environment is complete: the
// micromamba binary exists and installer_files/env/conda-meta/history records
// python and nodejs as installed. micromamba writes those "+" lines only when
// the install transaction has finished, minutes after the binary appears. The
// binary alone made the step look done while env creation still ran, so Install
// could start and fail (R2-3).
func mambaDone(root paths.Root) bool {
	if _, err := os.Stat(micromambaPath(root)); err != nil {
		return false
	}
	return envComplete(root)
}

// envComplete reports whether the env history records python and nodejs as
// installed, which init_mamba's single install transaction does at its end.
func envComplete(root paths.Root) bool {
	raw, err := os.ReadFile(filepath.Join(root.InstallerFiles(), "env", "conda-meta", "history"))
	if err != nil {
		return false
	}
	var python, node bool
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "+") {
			continue
		}
		python = python || strings.Contains(line, "::python-3")
		node = node || strings.Contains(line, "::nodejs-")
	}
	return python && node
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

// legacy reports whether this launcher's folder is a micromamba install.
func (l *Launcher) legacy() bool { return legacyInstall(l.root, l.systemPython) }

// refuseNotLegacy answers 400 and returns true for a step that only a micromamba
// install has, so a new folder cannot create installer_files by a stray request.
func (l *Launcher) refuseNotLegacy(w http.ResponseWriter, step string) bool {
	if l.legacy() {
		return false
	}
	http.Error(w, step+" is a step of a micromamba install; a new install has none (install uv, the Gradio host and engines from the Extensions page)", http.StatusBadRequest)
	return true
}

// legacyMarkers are the files only the micromamba install writes.
var legacyMarkers = []string{".gpu", ".git_version", ".python_version", ".pip_packages"}

// legacyInstall reports whether the folder holds a micromamba install, or the
// start of one: the installer_files directory the mamba step creates, or a marker
// the install step writes. Such a folder keeps the mamba, gpu and install steps
// and their markers (P4). A new folder from the archive has none of these and
// shows download, pre-flight and update only: the engines and the Gradio host are
// installed from the Extensions page. --system-python never has them.
func legacyInstall(root paths.Root, systemPython bool) bool {
	if systemPython {
		return false
	}
	if st, err := os.Stat(root.InstallerFiles()); err == nil && st.IsDir() {
		return true
	}
	for _, m := range legacyMarkers {
		if _, err := os.Stat(root.MarkerFile(m)); err == nil {
			return true
		}
	}
	return false
}
