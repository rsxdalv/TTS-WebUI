package installer_scripts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// flowRunner fakes the commands of init_app.js. Commands exit 0 unless a
// handler says otherwise.
type flowRunner struct {
	root     string
	mu       sync.Mutex
	cmds     []Command
	handlers map[string]func(ctx context.Context, c Command) (Result, error)
	resets   int
}

func cmdKey(c Command) string {
	if c.Shell != "" {
		return c.Shell
	}
	return strings.TrimSpace(c.Name + " " + strings.Join(c.Args, " "))
}

func (r *flowRunner) Run(ctx context.Context, c Command) (Result, error) {
	r.mu.Lock()
	r.cmds = append(r.cmds, c)
	h := r.handlers[cmdKey(c)]
	r.mu.Unlock()
	if h != nil {
		return h(ctx, c)
	}
	switch cmdKey(c) {
	case "python --version > installer_scripts/.python_version":
		err := os.WriteFile(filepath.Join(r.root, "installer_scripts", ".python_version"), []byte("Python 3.10.11\r\n"), 0o666)
		return Result{}, err
	case "git rev-parse HEAD":
		return Result{Stdout: "abc123\n"}, nil
	}
	return Result{}, nil
}

func (r *flowRunner) ResetActivated() {
	r.mu.Lock()
	r.resets++
	r.mu.Unlock()
}

// background lists the commands init_app.js starts without awaiting them.
var backgroundCmds = map[string]bool{
	"conda info --envs": true, "node --version": true, "python --version": true, "pip --version": true,
	"pip show torch torchvision torchaudio":                               true,
	"python -c import torch; exit(0 if torch.cuda.is_available() else 1)": true,
}

// foreground returns the awaited commands in order.
func (r *flowRunner) foreground() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, c := range r.cmds {
		if !backgroundCmds[cmdKey(c)] {
			out = append(out, cmdKey(c))
		}
	}
	return out
}

func (r *flowRunner) ran(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.cmds {
		if cmdKey(c) == key {
			return true
		}
	}
	return false
}

type flowLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *flowLog) add(s string) {
	l.mu.Lock()
	l.lines = append(l.lines, s)
	l.mu.Unlock()
}

func (l *flowLog) has(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.lines {
		if line == s {
			return true
		}
	}
	return false
}

// newFlow builds an install root with what init_app.js expects before it runs:
// .git, versions.json, react-ui/src/extensions and the .gpu the launcher writes.
func newFlow(t *testing.T) (*Step, *flowRunner, *flowLog) {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{".git", "installer_scripts", filepath.Join("react-ui", "src", "extensions")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, root, "installer_scripts/versions.json", `{"version":"1.0.0","pip_packages":"1.2.0","npm_packages":4,"react_ui":4}`)
	writeFile(t, root, "installer_scripts/.gpu", "NVIDIA GPU")
	writeFile(t, root, "react-ui/package.json", `{"scripts":{"build":"next build","build:static":"node build-static.js","start":"next start"}}`)
	r := &flowRunner{root: root, handlers: map[string]func(context.Context, Command) (Result, error){}}
	log := &flowLog{}
	return &Step{Root: root, Run: r, Log: log.add}, r, log
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
}

func readMarker(t *testing.T, root, name string) (string, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "installer_scripts", name))
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), true
}

func exitWith(code int) func(context.Context, Command) (Result, error) {
	return func(context.Context, Command) (Result, error) { return Result{ExitCode: code}, nil }
}

func run(t *testing.T, s *Step) int {
	t.Helper()
	code, err := InitAppJS(context.Background(), s)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	return code
}

const (
	nvidiaInstall = "pip install -U torch==2.11.0+cu128 torchvision torchaudio==2.11.0 xformers==0.0.35 --index-url https://download.pytorch.org/whl/cu128"
	singleInstall = `pip install -r requirements.txt "tts-webui-extension.rvc>=0.0.5" "tts-webui-extension.styletts2>=0.1.1" --extra-index-url https://tts-webui.github.io/extensions-index/ torch==2.11.0`
	torchFound    = `python -c import importlib.util; import sys; package_name = "torch"; spec = importlib.util.find_spec(package_name); sys.exit(0) if spec else sys.exit(1)`
)

func equalLists(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands:\n got  %q\n want %q", got, want)
	}
}

func TestInitAppJSFreshInstall(t *testing.T) {
	s, r, log := newFlow(t)
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d, log %q", code, log.lines)
	}
	equalLists(t, r.foreground(), []string{
		"conda --version",
		"git pull",
		"git rev-parse HEAD",
		"python --version > installer_scripts/.python_version",
		nvidiaInstall,
		singleInstall,
		"cd react-ui && npm install",
		reactUIBuild,
		"git rev-parse HEAD",
	})
	for key := range backgroundCmds {
		if !r.ran(key) {
			t.Errorf("background command %q did not run", key)
		}
	}
	for _, c := range r.cmds {
		if !c.Activated {
			t.Errorf("%q is not run inside installer_files/env", cmdKey(c))
		}
	}
	for name, want := range map[string]string{
		".major_version": "1.0.0", ".pip_packages": "1.2.0", ".git_version": "abc123", ".gpu": "NVIDIA GPU",
		".python_version": "Python 3.10.11\r\n",
	} {
		if got, _ := readMarker(t, s.Root, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if _, ok := readMarker(t, s.Root, ".db_version"); ok {
		t.Error(".db_version written on a fresh install")
	}
	if raw, _ := os.ReadFile(filepath.Join(s.Root, "react-ui", "src", "extensions", "package.json")); string(raw) != "{}" {
		t.Errorf("extensions package.json = %q", raw)
	}
	for _, d := range []string{"outputs", "favorites", filepath.Join("data", "sqlite")} {
		if info, err := os.Stat(filepath.Join(s.Root, d)); err != nil || !info.IsDir() {
			t.Errorf("%s not created", d)
		}
	}
	for _, want := range []string{"Starting init app (version: 0.2.0)...", ">[async] conda --version", ">" + nvidiaInstall,
		"  Major version update detected. Upgrading base environment", "  Using saved GPU choice: NVIDIA GPU",
		"  Successfully installed torch==2.11.0 with CUDA 12.8 support", "Checking if torch has CUDA...", "Finished init app."} {
		if !log.has(want) {
			t.Errorf("log lacks %q", want)
		}
	}
}

func TestInitAppJSUpToDateSkipsInstall(t *testing.T) {
	s, r, log := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.git_version", "abc123")
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	equalLists(t, r.foreground(), []string{"conda --version", "git pull", "git rev-parse HEAD"})
	if !log.has("No updates found, skipping...") || !log.has("Current git version: abc123") {
		t.Errorf("log %q", log.lines)
	}
	if _, ok := readMarker(t, s.Root, ".major_version"); ok {
		t.Error("installed although up to date")
	}
}

func TestInitAppJSGitVersionComparesRawBytes(t *testing.T) {
	s, r, _ := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.git_version", "abc123\n")
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !r.ran(nvidiaInstall) {
		t.Fatal("a trailing newline must count as a different version, as in the original")
	}
}

func TestInitAppJSGitPullFailureSkipsInstallAndExitsZero(t *testing.T) {
	s, r, log := newFlow(t)
	r.handlers["git pull"] = exitWith(1)
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	equalLists(t, r.foreground(), []string{"conda --version", "git pull"})
	if !log.has("[stderr] Error: Command failed with exit code 1") || !log.has("Error:") {
		t.Errorf("log %q", log.lines)
	}
	if _, ok := readMarker(t, s.Root, ".git_version"); ok {
		t.Error(".git_version written")
	}
}

func TestInitAppJSNoGitRepository(t *testing.T) {
	s, r, log := newFlow(t)
	if err := os.Remove(filepath.Join(s.Root, ".git")); err != nil {
		t.Fatal(err)
	}
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	equalLists(t, r.foreground(), []string{"conda --version"})
	for _, want := range []string{"CRITICAL: No git repository found, skipping update...", "[stderr] No git repository found", "Finished init app."} {
		if !log.has(want) {
			t.Errorf("log lacks %q: %q", want, log.lines)
		}
	}
}

func TestInitAppJSCondaMissingExitsAtOnce(t *testing.T) {
	s, r, log := newFlow(t)
	r.handlers["conda --version"] = exitWith(1)
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if log.has("Finished init app.") || r.ran("git pull") {
		t.Errorf("process.exit must end the flow at once: %q", log.lines)
	}
}

func TestInitAppJSNoGPUAndNobodyToAsk(t *testing.T) {
	s, r, log := newFlow(t)
	if err := os.Remove(filepath.Join(s.Root, "installer_scripts", ".gpu")); err != nil {
		t.Fatal(err)
	}
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if log.has("Finished init app.") || r.ran(nvidiaInstall) {
		t.Errorf("log %q", log.lines)
	}
	for _, m := range []string{".gpu", ".major_version", ".git_version"} {
		if _, ok := readMarker(t, s.Root, m); ok {
			t.Errorf("%s written", m)
		}
	}
}

func TestInitAppJSPromptsForGPU(t *testing.T) {
	s, r, log := newFlow(t)
	if err := os.Remove(filepath.Join(s.Root, "installer_scripts", ".gpu")); err != nil {
		t.Fatal(err)
	}
	var asked []string
	s.Prompt = func(_ context.Context, label string, choices []string) (int, error) {
		asked = choices
		return 2, nil
	}
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(asked) != 8 || asked[2] != "CPU" || asked[5] != "Cancel" {
		t.Fatalf("choices %q", asked)
	}
	if got, _ := readMarker(t, s.Root, ".gpu"); got != "CPU" {
		t.Errorf(".gpu = %q", got)
	}
	if !r.ran("pip install torch==2.11.0+cpu torchvision torchaudio==2.11.0 --index-url https://download.pytorch.org/whl/cpu") || !log.has("  You selected: CPU") {
		t.Errorf("log %q", log.lines)
	}
}

func TestInitAppJSInstallCommandPerChoice(t *testing.T) {
	for label, want := range map[string]string{
		"Apple M Series Chip":        "pip install torch==2.11.0 torchvision torchaudio==2.11.0",
		"AMD GPU (ROCM, Linux only)": "pip install torch==2.11.0 torchvision torchaudio==2.11.0 xformers --index-url https://download.pytorch.org/whl/rocm7.2",
		"Intel GPU (XPU)":            "pip install torch==2.11.0 torchvision torchaudio==2.11.0 --index-url https://download.pytorch.org/whl/test/xpu",
	} {
		s, r, _ := newFlow(t)
		writeFile(t, s.Root, "installer_scripts/.gpu", label)
		if code := run(t, s); code != 0 || !r.ran(want) {
			t.Errorf("%s: exit %d, commands %q", label, code, r.foreground())
		}
	}
}

func TestInitAppJSCustomTorchOnlyPrintsInstructions(t *testing.T) {
	s, r, log := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.gpu", "Custom (Manual torch install)")
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, c := range r.foreground() {
		if strings.Contains(c, "torchvision") {
			t.Errorf("custom choice installed torch: %q", c)
		}
	}
	if got, _ := readMarker(t, s.Root, ".major_version"); got != "1.0.0" || !log.has("Please install torch manually") {
		t.Errorf(".major_version %q, log %q", got, log.lines)
	}
}

func TestInitAppJSCancelledChoiceRemovesGPU(t *testing.T) {
	s, _, log := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.gpu", "Cancel")
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if _, ok := readMarker(t, s.Root, ".gpu"); ok {
		t.Error(".gpu not removed")
	}
	if !log.has("Unsupported or cancelled. Exiting...") || log.has("Finished init app.") || log.has("[stderr] Error during installation: process.exit(1)") {
		t.Errorf("log %q", log.lines)
	}
}

func TestInitAppJSTorchInstallFailure(t *testing.T) {
	s, r, log := newFlow(t)
	r.handlers[nvidiaInstall] = exitWith(1)
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !log.has("[stderr] Error during installation: Command failed with exit code 1") || !log.has("Finished init app.") {
		t.Errorf("log %q", log.lines)
	}
	for _, m := range []string{".major_version", ".git_version"} {
		if _, ok := readMarker(t, s.Root, m); ok {
			t.Errorf("%s written", m)
		}
	}
}

func TestInitAppJSFallsBackToSeparateInstalls(t *testing.T) {
	s, r, log := newFlow(t)
	r.handlers[singleInstall] = exitWith(1)
	rvc := `pip install "tts-webui-extension.rvc>=0.0.5" --extra-index-url https://tts-webui.github.io/extensions-index/ torch==2.11.0`
	r.handlers[rvc] = exitWith(1)
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	fg := r.foreground()
	equalLists(t, fg[5:9], []string{
		singleInstall,
		"pip install -r requirements.txt torch==2.11.0",
		rvc,
		`pip install "tts-webui-extension.styletts2>=0.1.1" --extra-index-url https://tts-webui.github.io/extensions-index/ torch==2.11.0`,
	})
	if got, _ := readMarker(t, s.Root, ".pip_packages"); got != "1.2.0" || !log.has("Failed to install RVC dependencies") {
		t.Errorf(".pip_packages %q, log %q", got, log.lines)
	}
}

func TestInitAppJSCorePackagesFailure(t *testing.T) {
	s, r, log := newFlow(t)
	r.handlers[singleInstall] = exitWith(1)
	r.handlers["pip install -r requirements.txt torch==2.11.0"] = exitWith(1)
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !log.has("Failed to install core packages") || r.ran("cd react-ui && npm install") {
		t.Errorf("log %q", log.lines)
	}
	for _, m := range []string{".pip_packages", ".git_version"} {
		if _, ok := readMarker(t, s.Root, m); ok {
			t.Errorf("%s written", m)
		}
	}
	if got, _ := readMarker(t, s.Root, ".major_version"); got != "1.0.0" {
		t.Errorf(".major_version = %q", got)
	}
}

func TestInitAppJSTorchAlreadyInstalled(t *testing.T) {
	s, r, log := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.major_version", "1.0.0")
	writeFile(t, s.Root, "installer_scripts/.pip_packages", "1.2.0")
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	equalLists(t, r.foreground()[3:], []string{
		"python --version > installer_scripts/.python_version",
		torchFound,
		"cd react-ui && npm install",
		reactUIBuild,
		"git rev-parse HEAD",
	})
	if !log.has("Dependencies are already up to date, skipping pip installs...") {
		t.Errorf("log %q", log.lines)
	}
}

func TestInitAppJSTorchMissingReinstalls(t *testing.T) {
	s, r, log := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.major_version", "1.0.0")
	r.handlers[torchFound] = exitWith(1)
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !r.ran(nvidiaInstall) || !r.ran(singleInstall) || !log.has("  Torch is not installed. Starting installation...") {
		t.Errorf("commands %q", r.foreground())
	}
}

func TestInitAppJSWrongPythonIsReinstalled(t *testing.T) {
	s, r, log := newFlow(t)
	r.handlers["python --version > installer_scripts/.python_version"] = func(context.Context, Command) (Result, error) {
		writeFile(t, s.Root, "installer_scripts/.python_version", "Python 3.11.2\r\n")
		return Result{}, nil
	}
	r.handlers["micromamba install -y -k -c conda-forge python=3.10.11"] = func(context.Context, Command) (Result, error) {
		return Result{ExitCode: -1}, errors.New("micromamba: executable not found on PATH")
	}
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !log.has(`Current python version is """Python 3.11.2"""`) || !log.has("[stderr] Failed to check/install python version") || r.resets != 1 {
		t.Errorf("resets %d, log %q", r.resets, log.lines)
	}
}

func TestInitAppJSReactBuildFailure(t *testing.T) {
	s, r, log := newFlow(t)
	r.handlers[reactUIBuild] = exitWith(2)
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !log.has("Failed to install node_modules or build react-ui") || !log.has("[stderr] Command failed with exit code 2") {
		t.Errorf("log %q", log.lines)
	}
	if _, ok := readMarker(t, s.Root, ".git_version"); ok {
		t.Error(".git_version written")
	}
}

func TestInitAppJSDatabaseUpgradeFailureIsLogged(t *testing.T) {
	s, _, log := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.db_version", "0")
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got, _ := readMarker(t, s.Root, ".db_version"); got != "0" || !log.has("[stderr] Failed to apply database config") {
		t.Errorf(".db_version %q, log %q", got, log.lines)
	}
}

func TestInitAppJSVersionsJSONNumber(t *testing.T) {
	s, _, _ := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/versions.json", `{"pip_packages":4}`)
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got, _ := readMarker(t, s.Root, ".pip_packages"); got != "4" {
		t.Errorf(".pip_packages = %q", got)
	}
}

func TestInitAppJSBrokenVersionsJSON(t *testing.T) {
	s, r, _ := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/versions.json", `{`)
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if r.ran("python --version > installer_scripts/.python_version") {
		t.Error("flow ran past a broken versions.json")
	}
}

// An un-awaited $sh that fails while the flow runs is an unhandled rejection:
// the process ends with exit 1 and the install stops.
func TestInitAppJSUnhandledRejectionEndsTheFlow(t *testing.T) {
	s, r, log := newFlow(t)
	r.handlers["conda info --envs"] = exitWith(1)
	r.handlers["git pull"] = func(ctx context.Context, _ Command) (Result, error) {
		select {
		case <-ctx.Done():
			return Result{ExitCode: -1}, ctx.Err()
		case <-time.After(5 * time.Second):
			return Result{}, nil
		}
	}
	if code := run(t, s); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !log.has("[stderr] Error: Command failed with exit code 1") || log.has("Finished init app.") || r.ran(nvidiaInstall) {
		t.Errorf("log %q", log.lines)
	}
}

// A background command still running 100 ms after the flow is stopped, as
// Node's exit ends it, and its later failure changes nothing.
func TestInitAppJSSlowBackgroundIsStopped(t *testing.T) {
	s, r, _ := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.git_version", "abc123")
	stopped := make(chan struct{})
	r.handlers["pip show torch torchvision torchaudio"] = func(ctx context.Context, _ Command) (Result, error) {
		select {
		case <-ctx.Done():
			close(stopped)
			return Result{ExitCode: -1}, ctx.Err()
		case <-time.After(5 * time.Second):
			return Result{ExitCode: 1}, nil
		}
	}
	start := time.Now()
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("background command still running after InitAppJS returned")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %v", d)
	}
}

func TestInitAppJSCancelled(t *testing.T) {
	s, r, _ := newFlow(t)
	ctx, cancel := context.WithCancel(context.Background())
	r.handlers["git pull"] = func(context.Context, Command) (Result, error) {
		cancel()
		return Result{ExitCode: -1}, context.Canceled
	}
	code, err := InitAppJS(ctx, s)
	if code != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("code %d err %v", code, err)
	}
	if r.ran("git rev-parse HEAD") {
		t.Error("flow continued after cancel")
	}
}

func TestJSString(t *testing.T) {
	for _, c := range []struct {
		v       any
		present bool
		want    string
	}{
		{"1.2.0", true, "1.2.0"}, {float64(4), true, "4"}, {1.5, true, "1.5"}, {nil, true, "null"},
		{nil, false, "undefined"}, {true, true, "true"}, {[]any{"a", float64(1)}, true, "a,1"},
		{map[string]any{}, true, "[object Object]"},
	} {
		if got := jsString(c.v, c.present); got != c.want {
			t.Errorf("jsString(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

// The Go steps build the static export, never the Next server build, and only
// when the original would build: on install and on an update with a new commit,
// not when .git_version already matches.
func TestInitAppJSBuildsTheStaticExportOnlyWhenTheOriginalBuilds(t *testing.T) {
	s, r, _ := newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.git_version", "older")
	writeFile(t, s.Root, "installer_scripts/.major_version", "1.0.0")
	writeFile(t, s.Root, "installer_scripts/.pip_packages", "1.2.0")
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !r.ran("cd react-ui && npm run build:static") || r.ran("cd react-ui && npm run build") {
		t.Fatalf("update with a new commit: commands %q", r.foreground())
	}

	s, r, _ = newFlow(t)
	writeFile(t, s.Root, "installer_scripts/.git_version", "abc123")
	if code := run(t, s); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, c := range r.foreground() {
		if strings.Contains(c, "npm") {
			t.Fatalf("no update, but %q ran", c)
		}
	}
}

// A tree from before S1 (main's react-ui/package.json has no build:static, and
// the launcher's Download installs main) gets the server build, with one line
// saying why. Without this a fresh Go install failed at the react-ui build.
func TestInitAppJSTreeWithoutStaticExportBuildsTheServerBuild(t *testing.T) {
	for name, pkg := range map[string]string{
		"main package.json": `{"scripts":{"build":"next build","start":"next start"}}`,
		"no package.json":   "",
	} {
		s, r, log := newFlow(t)
		path := filepath.Join(s.Root, "react-ui", "package.json")
		if pkg == "" {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFile(t, s.Root, "react-ui/package.json", pkg)
		}
		if code := run(t, s); code != 0 {
			t.Fatalf("%s: exit %d", name, code)
		}
		if !r.ran("cd react-ui && npm run build") || r.ran("cd react-ui && npm run build:static") {
			t.Errorf("%s: commands %q", name, r.foreground())
		}
		if !log.has("react-ui has no build:static script (this tree predates the static export), so building the server build") {
			t.Errorf("%s: log %q", name, log.lines)
		}
	}
}
