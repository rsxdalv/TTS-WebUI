package gradiohost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/uv"
)

// TestMain makes the test binary a fake uv when GH_FAKE_UV is set: it stops at
// the boundary, creating what a real uv would and logging its arguments.
func TestMain(m *testing.M) {
	if os.Getenv("GH_FAKE_UV") != "" {
		os.Exit(fakeUV(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func fakeUV(args []string) int {
	if log := os.Getenv("GH_FAKE_LOG"); log != "" {
		f, _ := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		f.WriteString(strings.Join(args, " ") + "\n")
		f.Close()
	}
	if len(args) > 0 && args[0] == "venv" {
		bin, exe := "bin", "python"
		if runtime.GOOS == "windows" {
			bin, exe = "Scripts", "python.exe"
		}
		_ = os.MkdirAll(filepath.Join(args[1], bin), 0o755)
		_ = os.WriteFile(filepath.Join(args[1], bin, exe), []byte("fake"), 0o755)
	}
	if len(args) > 0 && args[0] == "pip" && strings.Contains(os.Getenv("GH_FAKE_FAIL"), "pip") {
		os.Stderr.WriteString("error: no matching distribution\n")
		return 1
	}
	return 0
}

type fixture struct {
	root paths.Root
	uv   *uv.Platform
	p    *Platform
	log  string
	out  []string
}

func (f *fixture) emit(s string) { f.out = append(f.out, s) }

// newFixture is an install folder with a (fake) installed uv platform on the
// CPU row and a requirements.txt.
func newFixture(t *testing.T, uvInstalled bool) *fixture {
	t.Helper()
	root, _ := paths.New(t.TempDir())
	exe, _ := os.Executable()
	f := &fixture{root: root, log: filepath.Join(t.TempDir(), "uv.log")}
	env := []string{"GH_FAKE_UV=1", "GH_FAKE_LOG=" + f.log}
	f.uv = uv.New(uv.Options{Root: root, GPU: gpu.CPU, Prefix: []string{exe}, Env: env, SkipPathCheck: true})
	if uvInstalled {
		if err := os.MkdirAll(f.uv.PythonDir(), 0o755); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(uv.State{Name: uv.Name, UVVersion: uv.Version, Python: uv.PythonVersion, GPU: "CPU"})
		if err := os.WriteFile(filepath.Join(f.uv.Dir(), "platform.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root.Dir(), "requirements.txt"), []byte("gradio==5.49.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.p = New(Options{Root: root, UV: f.uv})
	return f
}

func (f *fixture) calls(t *testing.T) []string {
	b, err := os.ReadFile(f.log)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestNotInstalledSaysHowToInstallIt(t *testing.T) {
	f := newFixture(t, true)
	st := f.p.Status(context.Background())
	if st.Ready || !strings.Contains(st.Detail, "Gradio host is not installed") {
		t.Fatalf("status = %+v", st)
	}
	if _, err := f.p.Command(nil, []string{"python", "x.py"}); err == nil {
		t.Fatal("a python command resolved with no host")
	}
	if got := f.p.EntryStatus(context.Background(), platform.Entry{Package: "some_tab"}); got.Installed {
		t.Fatalf("a tab is installed with no host: %+v", got)
	}
}

func TestInstallNeedsTheUVPlatform(t *testing.T) {
	f := newFixture(t, false)
	err := f.p.Install(context.Background(), f.emit)
	if err == nil || !strings.Contains(err.Error(), "needs the uv platform") {
		t.Fatalf("err = %v", err)
	}
	if _, serr := os.Stat(platform.VenvDir(f.root.Dir(), Name)); serr == nil {
		t.Fatal("a venv was made without uv")
	}
}

func TestInstallMakesTheVenvAndHoldsTorch(t *testing.T) {
	f := newFixture(t, true)
	if err := f.p.Install(context.Background(), f.emit); err != nil {
		t.Fatalf("install: %v (%v)", err, f.out)
	}
	if st := f.p.Status(context.Background()); !st.Ready {
		t.Fatalf("not ready after install: %+v", st)
	}
	calls := strings.Join(f.calls(t), "\n")
	for _, want := range []string{"venv", "-r requirements.txt", "pip ", "torch==2.11.0+cpu"} {
		if !strings.Contains(calls, want) {
			t.Errorf("uv was not run with %q:\n%s", want, calls)
		}
	}
	b, err := os.ReadFile(f.p.ConstraintsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "torch==2.11.0+cpu") || !strings.Contains(string(b), "torchaudio==2.11.0") {
		t.Fatalf("constraints = %q", b)
	}
	c, err := f.p.Command(nil, []string{"python", "server_core.py", "--no-react"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Program != platform.VenvPython(f.root.Dir(), Name) || c.Dir != f.root.Dir() || strings.Join(c.Args, " ") != "server_core.py --no-react" {
		t.Fatalf("command = %+v", c)
	}
}

func TestATabInstallsWithPipAndTheTorchPin(t *testing.T) {
	f := newFixture(t, true)
	if err := f.p.Install(context.Background(), f.emit); err != nil {
		t.Fatal(err)
	}
	var got platform.Command
	x := f.p.Entries
	x.Cmd = func(e *platform.Entry, argv []string) (platform.Command, error) {
		c, err := f.p.Command(e, argv)
		got = c
		got.Args = argv
		return platform.Command{Program: os.Args[0], Env: append(c.Env, "GH_FAKE_UV=1"), Dir: c.Dir}, err
	}
	x.PipInstall(context.Background(), platform.Entry{Package: "tab", Requirements: []string{"six"}}, false, f.emit)
	line := strings.Join(got.Args, " ")
	if !strings.Contains(line, "pip install") || !strings.Contains(line, "-c "+f.p.ConstraintsPath()) ||
		!strings.Contains(line, "six") || !strings.Contains(line, platform.GradioPin) {
		t.Fatalf("pip line = %q", line)
	}
}

func TestRemoveDeletesTheVenv(t *testing.T) {
	f := newFixture(t, true)
	if err := f.p.Install(context.Background(), f.emit); err != nil {
		t.Fatal(err)
	}
	if err := f.p.Uninstall(context.Background(), f.emit); err != nil {
		t.Fatal(err)
	}
	if st := f.p.Status(context.Background()); st.Ready {
		t.Fatal("still ready after removal")
	}
	if _, err := os.Stat(platform.VenvDir(f.root.Dir(), Name)); err == nil {
		t.Fatal("the venv is still there")
	}
}

func TestAFailedInstallIsPartialAndRunsAgain(t *testing.T) {
	f := newFixture(t, true)
	f.uv = uv.New(uv.Options{Root: f.root, GPU: gpu.CPU, Prefix: []string{os.Args[0]}, SkipPathCheck: true,
		Env: []string{"GH_FAKE_UV=1", "GH_FAKE_FAIL=pip", "GH_FAKE_LOG=" + f.log}})
	f.p = New(Options{Root: f.root, UV: f.uv})
	if err := f.p.Install(context.Background(), f.emit); err == nil {
		t.Fatal("a failing uv installed the host")
	}
	st := f.p.Status(context.Background())
	if st.Ready || !strings.Contains(st.Detail, "did not finish") {
		t.Fatalf("status = %+v", st)
	}
	f.uv = uv.New(uv.Options{Root: f.root, GPU: gpu.CPU, Prefix: []string{os.Args[0]}, SkipPathCheck: true,
		Env: []string{"GH_FAKE_UV=1", "GH_FAKE_LOG=" + f.log}})
	f.p = New(Options{Root: f.root, UV: f.uv})
	if err := f.p.Install(context.Background(), f.emit); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !f.p.Status(context.Background()).Ready {
		t.Fatal("not ready after the second install")
	}
}
