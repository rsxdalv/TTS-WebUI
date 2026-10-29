package launcher

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// newTestRoot creates a minimal install root in a temp directory.
// It creates installer_scripts/ so paths.New succeeds.
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

func TestDownloadDone(t *testing.T) {
	root := newTestRoot(t)
	if !downloadDone(root) {
		t.Fatal("expected downloadDone=true after installer_scripts/ created")
	}
}

func TestDownloadDone_Missing(t *testing.T) {
	dir := t.TempDir()
	// Deliberately create a root without installer_scripts/ by bypassing paths.New.
	root := paths.Root{}
	_ = root // paths.Root is unexported-field, but we can test via a dir that later loses it.

	// Use a fresh dir with installer_scripts/ but then remove it.
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, _ := paths.New(dir)
	if err := os.RemoveAll(filepath.Join(dir, "installer_scripts")); err != nil {
		t.Fatal(err)
	}
	if downloadDone(r) {
		t.Fatal("expected downloadDone=false when installer_scripts/ removed")
	}
}

func TestPreflightDone(t *testing.T) {
	root := newTestRoot(t)
	if preflightDone(root) {
		t.Fatal("expected preflightDone=false initially")
	}
	if err := os.WriteFile(root.MarkerFile(".preflight_ok"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !preflightDone(root) {
		t.Fatal("expected preflightDone=true after writing marker")
	}
}

func TestMambaDone_BinaryCheck(t *testing.T) {
	root := newTestRoot(t)
	if mambaDone(root) {
		t.Fatal("expected mambaDone=false initially")
	}

	// Create only the directory — should still be false (legacy bug fix).
	mambaDir := filepath.Join(root.InstallerFiles(), "mamba")
	if err := os.MkdirAll(mambaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if mambaDone(root) {
		t.Fatal("expected mambaDone=false when only directory exists (bug fix)")
	}

	// Create the binary — now done.
	binaryPath := micromambaPath(root)
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !mambaDone(root) {
		t.Fatal("expected mambaDone=true after creating binary")
	}
}

func TestGPUDone(t *testing.T) {
	root := newTestRoot(t)
	if gpuDone(root) {
		t.Fatal("expected gpuDone=false initially")
	}
	if err := os.WriteFile(root.MarkerFile(".gpu"), []byte("nvidia\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !gpuDone(root) {
		t.Fatal("expected gpuDone=true after writing .gpu")
	}
}

func TestInstallDone(t *testing.T) {
	root := newTestRoot(t)
	if installDone(root) {
		t.Fatal("expected installDone=false initially")
	}
	if err := os.WriteFile(root.MarkerFile(".git_version"), []byte("abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !installDone(root) {
		t.Fatal("expected installDone=true after writing .git_version")
	}
}

func TestMicromambaPath(t *testing.T) {
	root := newTestRoot(t)
	p := micromambaPath(root)
	if runtime.GOOS == "windows" {
		if filepath.Ext(p) != ".exe" {
			t.Errorf("expected .exe on Windows, got %q", p)
		}
	} else {
		if filepath.Base(p) != "micromamba" {
			t.Errorf("expected 'micromamba' binary on Unix, got %q", p)
		}
		if filepath.Base(filepath.Dir(p)) != "bin" {
			t.Errorf("expected .../bin/micromamba on Unix, got %q", p)
		}
	}
}

func TestUVPythonPath(t *testing.T) {
	root := newTestRoot(t)
	p := uvPythonPath(root)
	// Must be under root.Venvs()/main.
	mainVenv := filepath.Join(root.Venvs(), "main")
	if !strings.HasPrefix(p, mainVenv) {
		t.Errorf("uvPythonPath = %q, want prefix %q", p, mainVenv)
	}
	if runtime.GOOS == "windows" {
		if filepath.Base(p) != "python.exe" {
			t.Errorf("expected python.exe on Windows, got %q", p)
		}
		if !strings.Contains(p, "Scripts") {
			t.Errorf("expected Scripts dir on Windows, got %q", p)
		}
	} else {
		if filepath.Base(p) != "python" {
			t.Errorf("expected python on Unix, got %q", p)
		}
		if !strings.Contains(p, "bin") {
			t.Errorf("expected bin dir on Unix, got %q", p)
		}
	}
}
