package installer_scripts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMain lets the test binary act as the child process and as a stub
// micromamba, so the tests run real processes on every platform.
func TestMain(m *testing.M) {
	switch os.Getenv("RUNNER_TEST_CHILD") {
	case "":
		os.Exit(m.Run())
	case "print":
		fmt.Println("to stdout")
		fmt.Fprintln(os.Stderr, "to stderr")
		fmt.Println("VALUE=" + os.Getenv("RUNNER_TEST_VALUE"))
		os.Exit(7)
	case "long":
		// One line far larger than a pipe buffer and bufio.Scanner's limit.
		fmt.Println(strings.Repeat("x", 3*maxLine))
		fmt.Print("10%\r50%\r100%\r\ndone\n")
		os.Exit(0)
	case "cr":
		fmt.Print("50%\r")
		time.Sleep(3 * time.Second)
		fmt.Print("done\n")
		os.Exit(0)
	case "spawn":
		// Leaves a background process holding stdout and stderr, then exits.
		exe, err := os.Executable()
		if err != nil {
			os.Exit(98)
		}
		child := exec.Command(exe)
		child.Env = append(os.Environ(), "RUNNER_TEST_CHILD=hold")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(98)
		}
		fmt.Println("parent done")
		os.Exit(0)
	case "hold":
		ms, _ := strconv.Atoi(os.Getenv("RUNNER_TEST_HOLD_MS"))
		time.Sleep(time.Duration(ms) * time.Millisecond)
		os.Exit(0)
	case "sleep":
		fmt.Println("started")
		time.Sleep(time.Minute)
		os.Exit(0)
	case "mamba":
		// Stub for: micromamba -p <env> run python -c <code>
		fmt.Println("a micromamba notice")
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
			"PATH":              os.Getenv("PATH"),
			"SystemRoot":        os.Getenv("SystemRoot"),
			"CONDA_PREFIX":      "from-activation",
			"RUNNER_TEST_CHILD": "print",
			"RUNNER_TEST_ARGS":  strings.Join(os.Args[1:4], " "),
		})
		os.Exit(0)
	}
	os.Exit(99)
}

type collector struct {
	mu    sync.Mutex
	lines []string
}

func (c *collector) add(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, line)
}

func (c *collector) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lines...)
}

func self(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

func contains(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

func TestRunStreamsAndReportsExit(t *testing.T) {
	var log collector
	r := &ExecRunner{Root: t.TempDir(), Log: log.add}
	res, err := r.Run(context.Background(), Command{
		Name: self(t),
		Env:  []string{"RUNNER_TEST_CHILD=print", "RUNNER_TEST_VALUE=abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 7 {
		t.Fatalf("exit = %d, want 7", res.ExitCode)
	}
	if res.Check() == nil {
		t.Fatal("Check accepted a non-zero exit")
	}
	lines := log.all()
	for _, want := range []string{"to stdout", StderrPrefix + "to stderr", "VALUE=abc"} {
		if !contains(lines, want) {
			t.Fatalf("missing %q in %q", want, lines)
		}
	}
}

func TestCaptureReturnsStdout(t *testing.T) {
	var log collector
	r := &ExecRunner{Root: t.TempDir(), Log: log.add}
	res, err := r.Run(context.Background(), Command{
		Name:    self(t),
		Env:     []string{"RUNNER_TEST_CHILD=print"},
		Capture: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Stdout, "to stdout") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
	if contains(log.all(), "to stdout") {
		t.Fatal("captured stdout was also logged")
	}
}

func TestLongLinesAndProgressDoNotBlock(t *testing.T) {
	var log collector
	r := &ExecRunner{Root: t.TempDir(), Log: log.add}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := r.Run(ctx, Command{Name: self(t), Env: []string{"RUNNER_TEST_CHILD=long"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d", res.ExitCode)
	}
	lines := log.all()
	for _, want := range []string{"10%", "50%", "100%", "done"} {
		if !contains(lines, want) {
			t.Fatalf("missing %q", want)
		}
	}
	total := 0
	for _, l := range lines {
		if l == "" {
			t.Fatal("a line end produced an extra empty line")
		}
		if strings.HasPrefix(l, "x") {
			if len(l) > maxLine {
				t.Fatalf("line of %d bytes exceeds the cap", len(l))
			}
			total += len(l)
		}
	}
	if total != 3*maxLine {
		t.Fatalf("got %d bytes of the long line, want %d", total, 3*maxLine)
	}
}

func TestLoneCarriageReturnFlushesAtOnce(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]time.Time{}
	r := &ExecRunner{Root: t.TempDir(), Log: func(line string) {
		mu.Lock()
		defer mu.Unlock()
		seen[line] = time.Now()
	}}
	if _, err := r.Run(context.Background(), Command{Name: self(t), Env: []string{"RUNNER_TEST_CHILD=cr"}}); err != nil {
		t.Fatal(err)
	}
	progress, ok1 := seen["50%"]
	done, ok2 := seen["done"]
	if !ok1 || !ok2 {
		t.Fatalf("missing lines: %v", seen)
	}
	if gap := done.Sub(progress); gap < 2*time.Second {
		t.Fatalf("50%% arrived only %v before done; it was held back", gap)
	}
}

func TestNameFinishesWhenTheProcessExits(t *testing.T) {
	var log collector
	r := &ExecRunner{Root: t.TempDir(), Log: log.add}
	begin := time.Now()
	res, err := r.Run(context.Background(), Command{
		Name: self(t),
		Env:  []string{"RUNNER_TEST_CHILD=spawn", "RUNNER_TEST_HOLD_MS=12000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(begin); elapsed > 8*time.Second {
		t.Fatalf("Run took %v: it waited for the background process", elapsed)
	}
	if res.ExitCode != 0 || !contains(log.all(), "parent done") {
		t.Fatalf("exit %d, log %q", res.ExitCode, log.all())
	}
}

func TestShellWaitsForOutputToClose(t *testing.T) {
	var log collector
	r := &ExecRunner{Root: t.TempDir(), Log: log.add}
	begin := time.Now()
	res, err := r.Run(context.Background(), Command{
		Shell: `"` + self(t) + `"`,
		Env:   []string{"RUNNER_TEST_CHILD=spawn", "RUNNER_TEST_HOLD_MS=3000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(begin); elapsed < 2500*time.Millisecond {
		t.Fatalf("Run took %v: it did not wait for the output to close", elapsed)
	}
	if res.ExitCode != 0 || !contains(log.all(), "parent done") {
		t.Fatalf("exit %d, log %q", res.ExitCode, log.all())
	}
}

func TestStepResetActivatedAcceptsAnyRunner(t *testing.T) {
	(&Step{Run: fakeRunner{}}).ResetActivated()

	r := &ExecRunner{activated: []string{"A=1"}}
	(&Step{Run: r}).ResetActivated()
	if r.activated != nil {
		t.Fatal("the environment was kept")
	}
}

type fakeRunner struct{}

func (fakeRunner) Run(context.Context, Command) (Result, error) { return Result{}, nil }

func TestShellKeepsInnerQuotes(t *testing.T) {
	var log collector
	dir := t.TempDir()
	r := &ExecRunner{Root: dir, Log: log.add}
	// The quoted value holds > which the shell would otherwise treat as a redirect.
	res, err := r.Run(context.Background(), Command{Shell: `echo "pkg>=1.0"`})
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Check(); err != nil {
		t.Fatal(err)
	}
	want := "pkg>=1.0"
	if runtime.GOOS == "windows" {
		want = `"pkg>=1.0"` // cmd.exe's echo prints the quotes
	}
	if !contains(log.all(), want) {
		t.Fatalf("output %q does not contain %q", log.all(), want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the shell created a redirect file: %v", entries)
	}
}

func TestShellReportsExit(t *testing.T) {
	r := &ExecRunner{Root: t.TempDir(), Log: func(string) {}}
	res, err := r.Run(context.Background(), Command{Shell: "exit 3"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("exit = %d, want 3", res.ExitCode)
	}
}

func TestCancelKillsTheProcess(t *testing.T) {
	started := make(chan struct{})
	var once sync.Once
	r := &ExecRunner{Root: t.TempDir(), Log: func(string) { once.Do(func() { close(started) }) }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-started
		cancel()
	}()
	begin := time.Now()
	_, err := r.Run(ctx, Command{Name: self(t), Env: []string{"RUNNER_TEST_CHILD=sleep"}})
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if time.Since(begin) > 30*time.Second {
		t.Fatal("cancel did not stop the process")
	}
}

func TestMissingProgramIsAnError(t *testing.T) {
	r := &ExecRunner{Root: t.TempDir(), Log: func(string) {}}
	if _, err := r.Run(context.Background(), Command{Name: "no-such-program-here"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestNameAndShellAreExclusive(t *testing.T) {
	r := &ExecRunner{Root: t.TempDir(), Log: func(string) {}}
	if _, err := r.Run(context.Background(), Command{}); err == nil {
		t.Fatal("empty command accepted")
	}
	if _, err := r.Run(context.Background(), Command{Name: "a", Shell: "b"}); err == nil {
		t.Fatal("command with both Name and Shell accepted")
	}
}

func TestActivatedUsesTheCondaEnvironment(t *testing.T) {
	root := t.TempDir()
	mambaDir := filepath.Join(root, "installer_files", "mamba")
	if err := os.MkdirAll(mambaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "micromamba"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, err := os.ReadFile(self(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mambaDir, name), data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RUNNER_TEST_CHILD", "mamba")

	var log collector
	r := &ExecRunner{Root: root, Log: log.add}
	res, err := r.Run(context.Background(), Command{
		Name:      self(t),
		Activated: true,
		Env:       []string{"RUNNER_TEST_VALUE=x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The stub's environment sets RUNNER_TEST_CHILD=print, so exit 7 proves the
	// child ran with the activated environment rather than the launcher's.
	if res.ExitCode != 7 {
		t.Fatalf("exit = %d, want 7; log %q", res.ExitCode, log.all())
	}
	if r.activated == nil || envValue(r.activated, "CONDA_PREFIX") != "from-activation" {
		t.Fatalf("activated env not captured: %q", r.activated)
	}
	wantArgs := "-p " + filepath.Join(root, "installer_files", "env") + " run"
	if got := envValue(r.activated, "RUNNER_TEST_ARGS"); got != wantArgs {
		t.Fatalf("micromamba args = %q, want %q", got, wantArgs)
	}

	r.ResetActivated()
	if r.activated != nil {
		t.Fatal("ResetActivated kept the environment")
	}
}

func TestMergeEnvReplaces(t *testing.T) {
	got := mergeEnv([]string{"A=1", "B=2"}, []string{"B=3", "C=4"})
	want := "A=1 B=3 C=4"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
