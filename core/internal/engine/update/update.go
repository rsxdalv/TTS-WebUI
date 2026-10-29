// Package update provides blue/green updates for venv and native engine kinds (C7.2).
package update

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/lifecycle"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/native"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/venv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// DefaultKeepDays is the retention period for old versioned venv/binary directories.
const DefaultKeepDays = 7

// UpdateVenv performs a blue/green update for a python-venv engine:
//  1. Installs the engine into a new timestamped venv directory.
//  2. Runs a Python smoke-test in the new venv (verifies the binary is executable).
//  3. Atomically writes the pointer file to the new version on success.
//  4. Removes old venvs older than DefaultKeepDays days.
//
// On health check failure the new venv directory is removed and the existing
// pointer file is left untouched.
func UpdateVenv(ctx context.Context, root paths.Root, m manifest.Manifest, gpuLabel string) error {
	return updateVenv(ctx, root, m, gpuLabel, venv.BuildVenv, venvSmokeTest, time.Now)
}

type (
	buildVenvFn func(ctx context.Context, root paths.Root, m manifest.Manifest, gpuLabel, version string) error
	healthFn    func(ctx context.Context, venvPath string) error
)

func updateVenv(
	ctx context.Context,
	root paths.Root,
	m manifest.Manifest,
	gpuLabel string,
	buildFn buildVenvFn,
	hFn healthFn,
	nowFn func() time.Time,
) error {
	name := venv.SanitizeName(m.PackageName)
	newVersion := nowFn().UTC().Format("20060102T150405")
	newVenvPath := venv.VenvPath(root, name, newVersion)

	slog.Info("update venv: building", "engine", m.PackageName, "version", newVersion)

	// Build phase: install into the new versioned path.
	if err := buildFn(ctx, root, m, gpuLabel, newVersion); err != nil {
		return fmt.Errorf("installing new venv: %w", err)
	}

	// Health check: remove new venv and leave old pointer intact on failure.
	if err := hFn(ctx, newVenvPath); err != nil {
		_ = os.RemoveAll(newVenvPath)
		return fmt.Errorf("health check failed for %s@%s: %w", name, newVersion, err)
	}

	// Atomic pointer swap.
	pp := pointerPath(root, name)
	if err := atomicWrite(pp, newVersion+"\n"); err != nil {
		_ = os.RemoveAll(newVenvPath)
		return fmt.Errorf("swapping pointer file: %w", err)
	}

	slog.Info("update venv: done", "engine", m.PackageName, "version", newVersion)

	return GarbageCollect(root, name, DefaultKeepDays)
}

// UpdateNative performs a blue/green update for a native engine:
//  1. Resolves the new binary via native.Resolve (download + checksum verify).
//  2. Stops the engine if it is running.
//  3. Updates the lifecycle manager's registered manifest with the new binary path.
//  4. Restarts the engine; on restart failure, restores the old manifest and restart old.
//  5. Garbage-collects old cached versions beyond DefaultKeepDays days.
//
// mgr may be nil when invoked from a one-shot CLI call; in that case the engine
// is not stopped/started and only the binary download and GC are performed.
func UpdateNative(ctx context.Context, m manifest.Manifest, gpuLabel, cacheDir string, mgr *lifecycle.Manager) error {
	a := m.Engine.NativeAsset
	if a == nil {
		return fmt.Errorf("engine %q: no native_asset in manifest", m.PackageName)
	}

	slog.Info("update native: resolving", "engine", m.PackageName, "version", a.Version)

	// Resolve new binary (downloads and verifies checksum via native.Resolve).
	newBinPath, err := native.Resolve(ctx, m, gpuLabel, cacheDir)
	if err != nil {
		return fmt.Errorf("resolving new binary: %w", err)
	}

	if mgr == nil {
		// CLI mode: no running manager; binary is now cached, just GC.
		return gcNativeCache(m.PackageName, cacheDir, DefaultKeepDays, a.Version)
	}

	oldMan, registered := mgr.Manifest(m.PackageName)
	wasRunning := false
	if registered {
		st := mgr.Status(m.PackageName)
		wasRunning = st.Running
		if wasRunning {
			slog.Info("update native: stopping engine", "engine", m.PackageName)
			if err := mgr.Stop(m.PackageName); err != nil {
				return fmt.Errorf("stopping engine for update: %w", err)
			}
		}
		newMan := patchedManifest(m, newBinPath)
		if err := mgr.UpdateManifest(m.PackageName, newMan); err != nil {
			return fmt.Errorf("updating manifest: %w", err)
		}
	} else {
		newMan := patchedManifest(m, newBinPath)
		if err := mgr.Register(newMan); err != nil {
			return fmt.Errorf("registering new manifest: %w", err)
		}
	}

	if wasRunning {
		slog.Info("update native: restarting engine", "engine", m.PackageName)
		if err := mgr.Start(ctx, m.PackageName); err != nil {
			// Restart failed: restore old manifest and restart old binary.
			if registered {
				_ = mgr.UpdateManifest(m.PackageName, oldMan)
				_ = mgr.Start(ctx, m.PackageName)
			}
			return fmt.Errorf("restarting engine with new binary: %w", err)
		}
	}

	slog.Info("update native: done", "engine", m.PackageName, "version", a.Version)

	return gcNativeCache(m.PackageName, cacheDir, DefaultKeepDays, a.Version)
}

// GarbageCollect removes versioned venv directories under root.Venvs()/<name>@* whose
// mtime is older than keepDays days, except the version pointed to by the pointer file.
func GarbageCollect(root paths.Root, name string, keepDays int) error {
	current, _ := venv.Current(root, name)
	venvDir := root.Venvs()
	entries, err := os.ReadDir(venvDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading venvs dir: %w", err)
	}
	prefix := name + "@"
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		version := strings.TrimPrefix(e.Name(), prefix)
		if version == current {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			dir := filepath.Join(venvDir, e.Name())
			slog.Info("gc: removing old venv", "dir", dir)
			_ = os.RemoveAll(dir)
		}
	}
	return nil
}

// gcNativeCache removes versioned subdirectories in cacheDir/<pkgName>/ older than
// keepDays days, except the directory matching currentVersion.
func gcNativeCache(pkgName, cacheDir string, keepDays int, currentVersion string) error {
	baseDir := filepath.Join(cacheDir, pkgName)
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading native cache dir: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == currentVersion {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			dir := filepath.Join(baseDir, e.Name())
			slog.Info("gc: removing old native cache", "dir", dir)
			_ = os.RemoveAll(dir)
		}
	}
	return nil
}

// venvSmokeTest checks that the Python binary in the new venv is executable.
func venvSmokeTest(ctx context.Context, venvPath string) error {
	python := filepath.Join(venvPath, pythonRelPath())
	if _, err := os.Stat(python); err != nil {
		return fmt.Errorf("python binary missing in new venv: %w", err)
	}
	sCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(sCtx, python, "-c", "import sys; sys.exit(0)")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("python smoke test: %w\n%s", err, out)
	}
	return nil
}

func pythonRelPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join("Scripts", "python.exe")
	}
	return filepath.Join("bin", "python")
}

// pointerPath returns the .current pointer file path for the given sanitized engine name.
func pointerPath(root paths.Root, name string) string {
	return filepath.Join(root.Venvs(), name+".current")
}

// atomicWrite writes content to path via a temporary file + rename.
func atomicWrite(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// patchedManifest returns a copy of m with Launch.Cmd[0] replaced by binPath.
func patchedManifest(m manifest.Manifest, binPath string) manifest.Manifest {
	patched := m
	launch := manifest.Launch{}
	if m.Engine.Launch != nil {
		launch = *m.Engine.Launch
	}
	cmd := make([]string, len(launch.Cmd))
	copy(cmd, launch.Cmd)
	if len(cmd) == 0 {
		cmd = []string{binPath}
	} else {
		cmd[0] = binPath
	}
	launch.Cmd = cmd
	patched.Engine.Launch = &launch
	return patched
}
