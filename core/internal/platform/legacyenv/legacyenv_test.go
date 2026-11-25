package legacyenv

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

func newRoot(t *testing.T) paths.Root {
	t.Helper()
	r, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCommandIsMicromambaRun(t *testing.T) {
	root := newRoot(t)
	p := New(Options{Root: root, Env: []string{"A=1"}})
	c, err := p.Command(nil, []string{"python", "-m", "pip", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Program != micromambaPath(root) {
		t.Fatalf("program = %q", c.Program)
	}
	want := []string{"-p", envPath(root), "run", "python", "-m", "pip", "list"}
	if !reflect.DeepEqual(c.Args, want) {
		t.Fatalf("args = %q, want %q", c.Args, want)
	}
	if c.Dir != root.Dir() {
		t.Fatalf("dir = %q", c.Dir)
	}
	mamba := "MAMBA_ROOT_PREFIX=" + filepath.Join(root.InstallerFiles(), "mamba")
	var gotMamba, gotA bool
	for _, kv := range c.Env {
		gotMamba = gotMamba || kv == mamba
		gotA = gotA || kv == "A=1"
	}
	if !gotMamba || !gotA {
		t.Fatalf("env lacks MAMBA_ROOT_PREFIX or the option variable (mamba=%v A=%v)", gotMamba, gotA)
	}
}

func TestCommandEscapesOnlyOnWindows(t *testing.T) {
	p := New(Options{Root: newRoot(t)})
	c, _ := p.Command(nil, []string{"uv", "pip", "install", "gradio>=1", "a|b"})
	got := c.Args[len(c.Args)-2:]
	want := []string{"gradio>=1", "a|b"}
	if runtime.GOOS == "windows" {
		want = []string{"gradio^>=1", "a^|b"}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestEscapeForMambaRun(t *testing.T) {
	got := escapeForMambaRun([]string{"a>=1", "x%y", "p^q", "(r)"})
	want := []string{"a^>=1", "x%%y", "p^^q", "^(r^)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// An own-venv entry's python is run directly, never through micromamba.
func TestOwnVenvPythonIsRunDirectly(t *testing.T) {
	root := newRoot(t)
	py := platform.VenvPython(root.Dir(), "tts_webui_extension.piper_tts")
	if err := os.MkdirAll(filepath.Dir(py), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(py, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	e := platform.Entry{Package: "tts_webui_extension.piper_tts", Own: true}
	c, _ := New(Options{Root: root}).Command(&e, []string{"python", "-m", "x", "a>=1"})
	if c.Program != py || !reflect.DeepEqual(c.Args, []string{"-m", "x", "a>=1"}) {
		t.Fatalf("got %q %q", c.Program, c.Args)
	}
	// uv of an own-venv entry still comes from the main env.
	u, _ := New(Options{Root: root}).Command(&e, []string{"uv", "venv"})
	if u.Program != micromambaPath(root) {
		t.Fatalf("uv ran as %q", u.Program)
	}
}

func TestStatusSaysWhatIsMissing(t *testing.T) {
	root := newRoot(t)
	p := New(Options{Root: root})
	st := p.Status(context.Background())
	if st.Ready || !strings.Contains(st.Detail, "no Python environment: ") || !strings.Contains(st.Detail, "was not found (the install has not been done)") {
		t.Fatalf("no micromamba: %+v", st)
	}
	if err := os.MkdirAll(filepath.Dir(micromambaPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(micromambaPath(root), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	st = p.Status(context.Background())
	if st.Ready || st.Detail != "no Python environment: "+envPath(root)+" does not exist (the install has not been done)" {
		t.Fatalf("no env: %+v", st)
	}
	if err := os.MkdirAll(envPath(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if st = p.Status(context.Background()); !st.Ready {
		t.Fatalf("env present: %+v", st)
	}
}

func TestAdoptedPlatformIsNotInstallable(t *testing.T) {
	p := New(Options{Root: newRoot(t)})
	for name, err := range map[string]error{
		"install":   p.Install(context.Background(), nil),
		"repair":    p.Repair(context.Background(), nil),
		"uninstall": p.Uninstall(context.Background(), nil),
	} {
		if err != platform.ErrNotInstallable {
			t.Errorf("%s: %v", name, err)
		}
	}
}
