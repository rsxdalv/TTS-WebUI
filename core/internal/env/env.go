// Package env manages Python virtual environments for TTS WebUI using uv.
package env

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
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
// Deprecated: use gpu.TorchFor; retained for external callers only.
func TorchIndexURL(gpuLabel string) (url string, known bool) {
	spec, err := gpu.TorchFor(gpu.Label(gpuLabel))
	if err != nil {
		return "", false
	}
	return spec.IndexURL, true
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

// installTorch installs torch for the given GPU label using the shared torch
// table from gpu.TorchFor (versions and index URLs match initializeApp.js).
func installTorch(ctx context.Context, run runFn, uv string, env []string, venvPath, gpuLabel string) error {
	spec, err := gpu.TorchFor(gpu.Label(gpuLabel))
	if err != nil {
		if errors.Is(err, gpu.ErrCustomManaged) {
			return nil // user manages torch manually
		}
		return fmt.Errorf("unknown GPU label %q: cannot install torch", gpuLabel)
	}

	args := []string{"pip", "install", "--python", venvPath}
	args = append(args, spec.Pkgs...)
	if spec.XFormers != "" {
		args = append(args, spec.XFormers)
	}
	if spec.IndexURL != "" {
		args = append(args, "--index-url", spec.IndexURL)
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
