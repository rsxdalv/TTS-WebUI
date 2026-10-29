package venv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// newTestRoot creates a minimal install root in a temp directory.
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

// fakeRun records invocations and returns success.
type fakeRun struct {
	calls [][]string // each call's args
}

func (f *fakeRun) run(ctx context.Context, bin string, args []string, env []string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{bin}, args...))
	return nil, nil
}

func testManifest(pkgName string) manifest.Manifest {
	return manifest.Manifest{
		PackageName: pkgName,
		Name:        pkgName,
		Engine: manifest.Engine{
			Kind: manifest.KindPythonVenv,
			Install: &manifest.Install{
				Requirements: []string{"some-package>=1.0"},
			},
		},
	}
}

func TestInstall_WritesPointerFile(t *testing.T) {
	root := newTestRoot(t)
	fake := &fakeRun{}
	m := testManifest("tts_webui_extension.kokoro")

	if err := install(context.Background(), root, m, "cpu", fake.run); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Pointer file must exist and contain the version.
	name := sanitizeName(m.PackageName)
	got, err := Current(root, name)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got != venvVersion {
		t.Errorf("pointer file version = %q, want %q", got, venvVersion)
	}
}

func TestInstall_Idempotent(t *testing.T) {
	root := newTestRoot(t)
	fake := &fakeRun{}
	m := testManifest("tts_webui_extension.mms")
	name := sanitizeName(m.PackageName)
	venvPath := VenvPath(root, name, venvVersion)

	// First install.
	if err := install(context.Background(), root, m, "cpu", fake.run); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// Create the venv dir to satisfy the idempotency check.
	if err := os.MkdirAll(venvPath, 0o755); err != nil {
		t.Fatalf("mkdir venv: %v", err)
	}
	firstCount := len(fake.calls)

	// Second install must skip (no new uv calls).
	if err := install(context.Background(), root, m, "cpu", fake.run); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if len(fake.calls) != firstCount {
		t.Errorf("second install made %d extra uv calls, want 0", len(fake.calls)-firstCount)
	}
}

func TestInstall_PointerUpdatedAfterVenvCreation(t *testing.T) {
	root := newTestRoot(t)
	fake := &fakeRun{}
	m := testManifest("tts_webui_extension.f5_tts")
	name := sanitizeName(m.PackageName)

	if err := install(context.Background(), root, m, "nvidia", fake.run); err != nil {
		t.Fatalf("install: %v", err)
	}

	cur, _ := Current(root, name)
	if cur != venvVersion {
		t.Errorf("pointer = %q, want %q", cur, venvVersion)
	}
}

func TestVenvPath(t *testing.T) {
	root := newTestRoot(t)
	got := VenvPath(root, "myengine", "1.2.3")
	if !strings.HasSuffix(got, "myengine@1.2.3") {
		t.Errorf("VenvPath = %q, want suffix myengine@1.2.3", got)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"tts_webui_extension.kokoro", "tts_webui_extension_kokoro"},
		{"extensions.builtin.foo", "extensions_builtin_foo"},
		{"simple", "simple"},
	}
	for _, c := range cases {
		if got := sanitizeName(c.in); got != c.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUVPythonPath(t *testing.T) {
	root := newTestRoot(t)
	name := sanitizeName("tts_webui_extension.kokoro")
	vp := VenvPath(root, name, venvVersion)
	p := pythonBin(vp)
	if !strings.HasPrefix(p, vp) {
		t.Errorf("pythonBin(%q) = %q, not under venv path", vp, p)
	}
}
