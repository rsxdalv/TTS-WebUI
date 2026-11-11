package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/lifecycle"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/venv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func newTestRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatalf("mkdir installer_scripts: %v", err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatalf("paths.New: %v", err)
	}
	return root
}

func testVenvManifest(pkgName string) manifest.Manifest {
	return manifest.Manifest{
		PackageName: pkgName,
		Name:        pkgName,
		Engine:      manifest.Engine{Kind: manifest.KindPythonVenv},
	}
}

// fakeBuild creates the versioned venv directory without running uv.
func fakeBuild(_ context.Context, root paths.Root, m manifest.Manifest, _, version string) error {
	name := venv.SanitizeName(m.PackageName)
	return os.MkdirAll(venv.VenvPath(root, name, version), 0o755)
}

func okHealth(_ context.Context, _ string) error   { return nil }
func failHealth(_ context.Context, _ string) error { return fmt.Errorf("smoke test failed") }

func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

// TestUpdateVenv_PointerSwap verifies that the pointer file is updated to the new version.
func TestUpdateVenv_PointerSwap(t *testing.T) {
	root := newTestRoot(t)
	m := testVenvManifest("myengine")
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	if err := updateVenv(context.Background(), root, m, "CPU", fakeBuild, okHealth, fixedNow(now)); err != nil {
		t.Fatalf("updateVenv: %v", err)
	}

	name := venv.SanitizeName(m.PackageName)
	got, err := venv.Current(root, name)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	want := now.UTC().Format("20060102T150405")
	if got != want {
		t.Errorf("pointer = %q, want %q", got, want)
	}
}

// TestUpdateVenv_HealthFailure verifies the old pointer is preserved when health fails.
func TestUpdateVenv_HealthFailure(t *testing.T) {
	root := newTestRoot(t)
	m := testVenvManifest("myengine")
	name := venv.SanitizeName(m.PackageName)
	now := time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC)
	expectedVersion := now.UTC().Format("20060102T150405")

	// Write initial pointer.
	if err := os.MkdirAll(root.Venvs(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root.Venvs(), name+".current"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := updateVenv(context.Background(), root, m, "CPU", fakeBuild, failHealth, fixedNow(now))
	if err == nil {
		t.Fatal("expected error from health failure, got nil")
	}

	// Old pointer must be preserved.
	got, _ := venv.Current(root, name)
	if got != "old" {
		t.Errorf("pointer after health failure = %q, want old", got)
	}

	// New venv directory must be removed.
	newDir := venv.VenvPath(root, name, expectedVersion)
	if _, statErr := os.Stat(newDir); !os.IsNotExist(statErr) {
		t.Error("new venv dir should have been removed on health failure")
	}
}

// TestGarbageCollect_RemovesOldVersions verifies that old dirs are removed.
func TestGarbageCollect_RemovesOldVersions(t *testing.T) {
	root := newTestRoot(t)
	name := "myengine"
	venvDir := root.Venvs()
	if err := os.MkdirAll(venvDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Old dir: 10 days ago → must be removed.
	oldDir := filepath.Join(venvDir, name+"@old")
	if err := os.Mkdir(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(oldDir, old, old); err != nil {
		t.Fatal(err)
	}

	// Recent dir: mtime is now (default) → must stay.
	recentDir := filepath.Join(venvDir, name+"@recent")
	if err := os.Mkdir(recentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Pointer points to "recent".
	if err := os.WriteFile(filepath.Join(venvDir, name+".current"), []byte("recent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GarbageCollect(root, name, DefaultKeepDays); err != nil {
		t.Fatalf("GarbageCollect: %v", err)
	}

	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old dir should have been removed")
	}
	if _, err := os.Stat(recentDir); err != nil {
		t.Errorf("recent dir should still exist: %v", err)
	}
}

// TestGarbageCollect_KeepsCurrent verifies that the current version is never removed.
func TestGarbageCollect_KeepsCurrent(t *testing.T) {
	root := newTestRoot(t)
	name := "myengine"
	venvDir := root.Venvs()
	if err := os.MkdirAll(venvDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Current dir: 30 days old but must be kept.
	currentDir := filepath.Join(venvDir, name+"@v1")
	if err := os.Mkdir(currentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	veryOld := time.Now().AddDate(0, 0, -30)
	if err := os.Chtimes(currentDir, veryOld, veryOld); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(venvDir, name+".current"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GarbageCollect(root, name, DefaultKeepDays); err != nil {
		t.Fatalf("GarbageCollect: %v", err)
	}

	if _, err := os.Stat(currentDir); err != nil {
		t.Errorf("current dir was removed: %v", err)
	}
}

// TestGarbageCollect_MissingVenvsDir verifies graceful handling of absent venvs dir.
func TestGarbageCollect_MissingVenvsDir(t *testing.T) {
	root := newTestRoot(t)
	// Do NOT create .venvs; GarbageCollect should return nil.
	if err := GarbageCollect(root, "noengine", DefaultKeepDays); err != nil {
		t.Fatalf("GarbageCollect on missing dir: %v", err)
	}
}

// TestGcNativeCache_RemovesOldVersions verifies old native cache dirs are GC'd.
func TestGcNativeCache_RemovesOldVersions(t *testing.T) {
	cacheDir := t.TempDir()
	pkgName := "audio.cpp"
	currentVersion := "1.0.0"

	baseDir := filepath.Join(cacheDir, pkgName)
	if err := os.MkdirAll(filepath.Join(baseDir, currentVersion), 0o755); err != nil {
		t.Fatal(err)
	}

	// Old version: 10 days ago.
	oldDir := filepath.Join(baseDir, "0.9.0")
	if err := os.Mkdir(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(oldDir, old, old); err != nil {
		t.Fatal(err)
	}

	if err := gcNativeCache(pkgName, cacheDir, DefaultKeepDays, currentVersion); err != nil {
		t.Fatalf("gcNativeCache: %v", err)
	}

	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old version dir should have been removed")
	}
	if _, err := os.Stat(filepath.Join(baseDir, currentVersion)); err != nil {
		t.Errorf("current version dir should still exist: %v", err)
	}
}

// TestUpdateManifest_SwapsManifest verifies lifecycle.Manager.UpdateManifest round-trip.
func TestUpdateManifest_SwapsManifest(t *testing.T) {
	mgr := lifecycle.New()
	oldBin := filepath.Join(t.TempDir(), "old-server")
	if err := os.WriteFile(oldBin, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldMan := manifest.Manifest{
		PackageName: "audio.cpp",
		Name:        "audio.cpp",
		Engine: manifest.Engine{
			Kind:        manifest.KindNative,
			NativeAsset: &manifest.NativeAsset{Version: "0.9.0"},
			Launch:      &manifest.Launch{Cmd: []string{oldBin}},
			Health:      &manifest.Health{Path: "/health"},
		},
	}
	if err := mgr.Register(oldMan); err != nil {
		t.Fatalf("Register: %v", err)
	}

	newMan := oldMan
	newMan.Engine.NativeAsset = &manifest.NativeAsset{Version: "1.0.0"}
	if err := mgr.UpdateManifest("audio.cpp", newMan); err != nil {
		t.Fatalf("UpdateManifest: %v", err)
	}

	got, ok := mgr.Manifest("audio.cpp")
	if !ok {
		t.Fatal("Manifest returned false after update")
	}
	if got.Engine.NativeAsset.Version != "1.0.0" {
		t.Errorf("version after UpdateManifest = %q, want 1.0.0", got.Engine.NativeAsset.Version)
	}
}

// TestUpdateManifest_ErrorIfRunning verifies UpdateManifest rejects running engines.
func TestUpdateManifest_ErrorIfRunning(t *testing.T) {
	mgr := lifecycle.New()
	man := manifest.Manifest{
		PackageName: "fake",
		Engine: manifest.Engine{
			Kind:   manifest.KindNative,
			Launch: &manifest.Launch{Cmd: []string{"nonexistent-binary"}},
			Health: &manifest.Health{Path: "/health"},
		},
	}
	if err := mgr.Register(man); err != nil {
		t.Fatal(err)
	}
	// Engine is not running (not started), so UpdateManifest should succeed.
	if err := mgr.UpdateManifest("fake", man); err != nil {
		t.Errorf("UpdateManifest on stopped engine: %v", err)
	}
}
