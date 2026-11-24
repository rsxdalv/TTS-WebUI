package detect

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func TestNewFolderIsOnTheGradioHost(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	p := Detect(Options{Root: root})
	if p.Name() != "gradio-host" || Host(p) == nil {
		t.Fatalf("platform = %s, host = %v", p.Name(), Host(p))
	}
	st := p.Status(context.Background())
	if st.Ready || !strings.Contains(st.Detail, "Gradio host is not installed") {
		t.Fatalf("status = %+v", st)
	}
}

func TestAFolderWithTheLegacyEnvStaysOnLegacyEnvEvenWhenItAppearsLater(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	p := Detect(Options{Root: root})
	if p.Name() != "gradio-host" {
		t.Fatalf("before: %s", p.Name())
	}
	if err := os.MkdirAll(filepath.Join(root.InstallerFiles(), "env"), 0o755); err != nil {
		t.Fatal(err)
	}
	if p.Name() != "legacy-env" || Host(p) != nil {
		t.Fatalf("after the installer step: %s (host %v)", p.Name(), Host(p))
	}
	if q := p.WithEnv([]string{"A=1"}); q.Name() != "legacy-env" {
		t.Fatalf("WithEnv: %s", q.Name())
	}
}

func TestSystemPythonWinsOverEverything(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	if err := os.MkdirAll(filepath.Join(root.InstallerFiles(), "env"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := Detect(Options{Root: root, SystemPython: true})
	if p.Name() != "system" || Host(p) != nil {
		t.Fatalf("platform = %s", p.Name())
	}
}
