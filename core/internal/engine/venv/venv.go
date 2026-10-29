// Package venv manages per-engine Python virtual environments.
// It ports venv_setup_wrapper from tts_webui/utils/pip_install.py.
package venv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// runFn is an injectable subprocess runner for tests.
type runFn func(ctx context.Context, bin string, args []string, env []string) ([]byte, error)

func execRun(ctx context.Context, bin string, args []string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

// uvBinary returns the path to the uv binary inside installer_files/tools/uv/.
func uvBinary(root paths.Root) string {
	name := "uv"
	if runtime.GOOS == "windows" {
		name = "uv.exe"
	}
	return filepath.Join(root.InstallerFiles(), "tools", "uv", name)
}

// pythonBin returns the Python executable path inside a venv.
func pythonBin(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "python.exe")
	}
	return filepath.Join(venvPath, "bin", "python")
}

// VenvPath returns the path to the versioned venv directory.
// name is typically the sanitized package_name; version is the installed version
// (currently fixed at "0" until the manifest schema adds a version field).
func VenvPath(root paths.Root, name, version string) string {
	return filepath.Join(root.Venvs(), name+"@"+version)
}

// pointerPath returns the path to the .current pointer file.
func pointerPath(root paths.Root, name string) string {
	return filepath.Join(root.Venvs(), name+".current")
}

// Current reads the pointer file and returns the installed version.
// Returns ("", os.ErrNotExist) if no venv has been installed yet.
func Current(root paths.Root, name string) (string, error) {
	data, err := os.ReadFile(pointerPath(root, name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// venvVersion is the initial version tag used for all engine venvs until the
// manifest schema adds a proper version field.
const venvVersion = "0"

// Install creates or updates the Python venv for the given engine manifest.
//
// gpuLabel controls the torch wheel variant (values: "nvidia", "amd", "apple", "cpu").
// Idempotent: if the versioned directory exists and the pointer file matches, Install returns nil.
// Uses uv from root.InstallerFiles()/tools/uv/ (D3: uv cache hardlinks within volume).
func Install(ctx context.Context, root paths.Root, m manifest.Manifest, gpuLabel string) error {
	return install(ctx, root, m, gpuLabel, execRun)
}

func install(ctx context.Context, root paths.Root, m manifest.Manifest, gpuLabel string, run runFn) error {
	name := sanitizeName(m.PackageName)
	venvPath := VenvPath(root, name, venvVersion)

	// Idempotency: skip if the venv dir and pointer file already match.
	if cur, err := Current(root, name); err == nil && cur == venvVersion {
		if _, err := os.Stat(venvPath); err == nil {
			return nil
		}
	}

	uv := uvBinary(root)
	cacheDir := filepath.Join(root.InstallerFiles(), "uvcache")
	uvEnv := append(os.Environ(), "UV_CACHE_DIR="+cacheDir)
	python := pythonBin(venvPath)

	// Step 1: create the venv.
	if out, err := run(ctx, uv, []string{"venv", venvPath, "--allow-existing"}, uvEnv); err != nil {
		return fmt.Errorf("uv venv: %w\n%s", err, out)
	}

	// Step 2: install torch for the GPU label.
	if err := installTorch(ctx, run, uv, uvEnv, python, gpuLabel); err != nil {
		return err
	}

	// Step 3: torchcodec (CPU index — safe, as in venv_setup_wrapper).
	if out, err := run(ctx, uv, []string{
		"pip", "install", "--python", python,
		"torchcodec", "--index-url=https://download.pytorch.org/whl/cpu",
	}, uvEnv); err != nil {
		return fmt.Errorf("uv pip install torchcodec: %w\n%s", err, out)
	}

	// Step 4: xformers.
	if out, err := run(ctx, uv, []string{
		"pip", "install", "--python", python, "xformers==0.0.35",
	}, uvEnv); err != nil {
		return fmt.Errorf("uv pip install xformers: %w\n%s", err, out)
	}

	// Step 5: engine requirements + compatibility pins.
	reqs := compatPins()
	if m.Engine.Install != nil {
		reqs = append(reqs, m.Engine.Install.Requirements...)
	}
	if len(reqs) > 0 {
		args := append([]string{"pip", "install", "--python", python}, reqs...)
		if out, err := run(ctx, uv, args, uvEnv); err != nil {
			return fmt.Errorf("uv pip install requirements: %w\n%s", err, out)
		}
	}

	// Write pointer file after successful install.
	if err := os.MkdirAll(root.Venvs(), 0o755); err != nil {
		return fmt.Errorf("creating venvs dir: %w", err)
	}
	return os.WriteFile(pointerPath(root, name), []byte(venvVersion+"\n"), 0o644)
}

// installTorch installs torch/torchvision/torchaudio for the given GPU label.
// Versions and index URLs match get_torch_command.py.
func installTorch(ctx context.Context, run runFn, uv string, uvEnv []string, python, gpuLabel string) error {
	const torchVersion = "2.11.0"
	var pkgs []string
	var indexURL string

	switch gpuLabel {
	case "nvidia":
		pkgs = []string{
			"torch==" + torchVersion + "+cu128",
			"torchvision",
			"torchaudio==" + torchVersion + "+cu128",
		}
		indexURL = "https://download.pytorch.org/whl/cu128"
	case "amd":
		pkgs = []string{"torch==" + torchVersion, "torchvision", "torchaudio==" + torchVersion}
		indexURL = "https://download.pytorch.org/whl/rocm7.2"
	case "apple":
		pkgs = []string{"torch==" + torchVersion, "torchvision", "torchaudio==" + torchVersion}
	case "cpu":
		pkgs = []string{
			"torch==" + torchVersion + "+cpu",
			"torchvision",
			"torchaudio==" + torchVersion,
		}
		indexURL = "https://download.pytorch.org/whl/cpu"
	default:
		// intel or unknown: use default PyPI index.
		pkgs = []string{"torch==" + torchVersion, "torchvision", "torchaudio==" + torchVersion}
	}

	args := append([]string{"pip", "install", "--python", python}, pkgs...)
	if indexURL != "" {
		args = append(args, "--index-url", indexURL)
	}
	if out, err := run(ctx, uv, args, uvEnv); err != nil {
		return fmt.Errorf("uv pip install torch: %w\n%s", err, out)
	}
	return nil
}

// compatPins returns the compatibility constraints from venv_setup_wrapper in pip_install.py.
func compatPins() []string {
	return []string{
		"gradio<=5.49.1",
		"gradio-goodtabs>=0.0.5",
		"gradio-goodtab>=0.0.5",
		"gradio-iconbutton>=0.0.1",
		"ffmpeg-python==0.2.0",
		"matplotlib",
	}
}

// sanitizeName converts a package_name into a filesystem-safe venv directory component.
func sanitizeName(packageName string) string {
	return strings.NewReplacer("/", "_", ".", "_").Replace(packageName)
}
