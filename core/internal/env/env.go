// Package env manages Python virtual environments for TTS WebUI using uv.
package env

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// pythonVersion is the pinned Python version for the main venv (D-open-6).
const pythonVersion = "3.10.11"

// runFn is the signature for executing a subprocess.
// Returns combined stdout+stderr output and an error.
type runFn func(ctx context.Context, bin string, args []string, env []string) ([]byte, error)

// execRun is the production runFn using os/exec.
func execRun(ctx context.Context, bin string, args []string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

// TorchIndexURL returns the PyTorch wheel index URL for a GPU label.
// Returns ("", true) for labels that use the default PyPI index.
// Returns ("", false) for unrecognised labels.
func TorchIndexURL(gpuLabel string) (url string, known bool) {
	switch gpuLabel {
	case "nvidia":
		return "https://download.pytorch.org/whl/cu126", true
	case "amd":
		return "https://download.pytorch.org/whl/rocm6.2", true
	case "apple":
		return "", true // MPS support in base torch, default PyPI
	case "intel_extension_for_pytorch":
		return "", true // default PyPI + intel-extension-for-pytorch package
	case "cpu":
		return "https://download.pytorch.org/whl/cpu", true
	default:
		return "", false
	}
}

// uvBinary returns the path to the uv executable in the tools directory.
func uvBinary(root paths.Root) string {
	name := "uv"
	if runtime.GOOS == "windows" {
		name = "uv.exe"
	}
	return filepath.Join(root.InstallerFiles(), "tools", "uv", name)
}

// CreateMain creates the main Python venv at root.Venvs()/main.
//
// It installs Python pythonVersion via uv python install, creates the venv,
// then installs torch (+ torchvision + torchaudio) for the given GPU label.
// UV_CACHE_DIR is set to root.InstallerFiles()/uvcache so uv can use
// hardlinks within the same volume (D3).
func CreateMain(ctx context.Context, root paths.Root, gpuLabel string) error {
	return createMain(ctx, root, gpuLabel, execRun)
}

// createMain is the testable implementation with an injectable runner.
func createMain(ctx context.Context, root paths.Root, gpuLabel string, run runFn) error {
	uv := uvBinary(root)
	cacheDir := filepath.Join(root.InstallerFiles(), "uvcache")
	venvPath := filepath.Join(root.Venvs(), "main")

	uvEnv := append(os.Environ(), "UV_CACHE_DIR="+cacheDir)

	if err := runUV(ctx, run, uv, uvEnv, "python", "install", pythonVersion); err != nil {
		return fmt.Errorf("uv python install: %w", err)
	}

	if err := runUV(ctx, run, uv, uvEnv, "venv", "--python", pythonVersion, venvPath); err != nil {
		return fmt.Errorf("uv venv: %w", err)
	}

	if err := installTorch(ctx, run, uv, uvEnv, venvPath, gpuLabel); err != nil {
		return err
	}

	return nil
}

// installTorch installs torch, torchvision and torchaudio into the venv.
func installTorch(ctx context.Context, run runFn, uv string, env []string, venvPath, gpuLabel string) error {
	indexURL, known := TorchIndexURL(gpuLabel)
	if !known {
		slog.Warn("unknown GPU label, using default PyPI for torch", "gpu", gpuLabel)
	}

	pkgs := []string{"torch", "torchvision", "torchaudio"}
	if gpuLabel == "intel_extension_for_pytorch" {
		pkgs = append(pkgs, "intel-extension-for-pytorch")
	}

	args := []string{"pip", "install", "--python", venvPath}
	args = append(args, pkgs...)
	if indexURL != "" {
		args = append(args, "--index-url", indexURL)
	}

	return runUV(ctx, run, uv, env, args...)
}

// runUV invokes the uv binary, logs a warning if it falls back to copying instead of linking.
func runUV(ctx context.Context, run runFn, uv string, env []string, args ...string) error {
	out, err := run(ctx, uv, args, env)
	if err != nil {
		return fmt.Errorf("uv %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	if strings.Contains(string(out), "Copying instead of linking") {
		slog.Warn("uv: copying instead of linking (hardlinks unavailable; disk usage may be higher)")
	}
	return nil
}
