// Package installer_scripts ports installer_scripts/* to Go, one file per
// original script. This file holds the step runner every port shares; the
// contract is in documentation/core-rewrite/CONTRACTS.md, section "Step runner".
package installer_scripts

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

// maxLine caps one log line. Longer output is split, never dropped or blocked on.
const maxLine = 1 << 20

// drainTimeout bounds how long a finished Name command waits for output that is
// still in its pipes.
const drainTimeout = 2 * time.Second

// beforeAssign lets tests delay attaching a started process to its tree.
var beforeAssign = func() {}

// StderrPrefix marks stderr lines in the log, matching procs.Proc.
const StderrPrefix = "[stderr] "

// LogSink receives each output line. Calls are serialised by the runner.
type LogSink func(line string)

// Command is one process to run.
//
// Set exactly one of Name or Shell:
//   - Name and Args run the program directly, like spawn in js/shell.js ($, $$).
//     The command is finished when the process exits, even if a background
//     process it left behind still holds its output open.
//   - Shell runs a command line through the platform shell, like exec in
//     js/shell.js ($sh, execCapture): cmd.exe /d /s /c on Windows, /bin/sh -c
//     elsewhere. The command is finished when its output closes.
type Command struct {
	Name  string
	Args  []string
	Shell string

	// Dir is the working directory. Empty means the install root.
	Dir string
	// Env holds KEY=VALUE pairs added on top of the base environment.
	Env []string
	// Activated runs the command inside the installer_files/env conda
	// environment, as activate.bat and activate.sh do.
	Activated bool
	// Capture returns stdout in Result.Stdout instead of logging it, like
	// execCapture. Stderr is still logged.
	Capture bool
}

// Result is how a finished command ended.
type Result struct {
	// ExitCode is the process exit code, or -1 if it was killed by a signal.
	ExitCode int
	// Stdout is set only when Command.Capture is true.
	Stdout string
}

// Check returns an error when the command exited non-zero, which is when the
// promises in js/shell.js reject.
func (r Result) Check() error {
	if r.ExitCode != 0 {
		return fmt.Errorf("command failed with exit code %d", r.ExitCode)
	}
	return nil
}

// Runner runs commands for a ported script. A non-zero exit is a Result, not an
// error. The error is for a command that could not start or was cancelled.
type Runner interface {
	Run(ctx context.Context, c Command) (Result, error)
}

// Step is what every ported script receives. Each port exports one function
//
//	func Name(ctx context.Context, s *Step) (exitCode int, err error)
//
// where exitCode is what the original script would exit with.
type Step struct {
	// Root is the absolute install root, the working directory of the originals.
	Root string
	// Run executes commands. Tests may replace it.
	Run Runner
	// Log receives progress lines, like displayMessage.
	Log LogSink
	// Prompt asks the user to pick one of choices and returns its index. It is
	// nil when nobody can answer; a port that needs an answer then logs why and
	// exits 1.
	Prompt func(ctx context.Context, label string, choices []string) (int, error)
}

// ResetActivated forgets the captured conda environment. Call it after a command
// that changes the packages installed in installer_files/env. It does nothing
// for a Runner that keeps no such environment.
func (s *Step) ResetActivated() {
	if r, ok := s.Run.(interface{ ResetActivated() }); ok {
		r.ResetActivated()
	}
}

// NewStep returns a Step that runs real processes for the install at root.
func NewStep(root string, log LogSink) *Step {
	var mu sync.Mutex
	safe := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		log(line)
	}
	return &Step{Root: root, Run: &ExecRunner{Root: root, Log: safe}, Log: safe}
}

// ExecRunner is the Runner that starts real processes.
type ExecRunner struct {
	Root string
	Log  LogSink

	mu        sync.Mutex
	activated []string
}

// ResetActivated forgets the captured conda environment. Call it after a
// command that changes the packages installed in installer_files/env.
func (r *ExecRunner) ResetActivated() {
	r.mu.Lock()
	r.activated = nil
	r.mu.Unlock()
}

// Run implements Runner.
func (r *ExecRunner) Run(ctx context.Context, c Command) (Result, error) {
	if (c.Name == "") == (c.Shell == "") {
		return Result{ExitCode: -1}, errors.New("command needs exactly one of Name or Shell")
	}

	env := os.Environ()
	if c.Activated {
		var err error
		if env, err = r.activatedEnv(ctx); err != nil {
			return Result{ExitCode: -1}, err
		}
	}
	env = mergeEnv(env, c.Env)

	cmd, err := buildCmd(c, env)
	if err != nil {
		return Result{ExitCode: -1}, err
	}
	cmd.Dir = c.Dir
	if cmd.Dir == "" {
		cmd.Dir = r.Root
	}
	cmd.Env = env

	tree, err := procs.NewTree(cmd)
	if err != nil {
		return Result{ExitCode: -1}, fmt.Errorf("platform setup: %w", err)
	}
	if c.Shell != "" {
		setShellLine(cmd, c.Shell)
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return Result{ExitCode: -1}, err
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		return Result{ExitCode: -1}, err
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW
	err = cmd.Start()
	_ = stdoutW.Close()
	_ = stderrW.Close()
	if err != nil {
		_ = stdoutR.Close()
		_ = stderrR.Close()
		return Result{ExitCode: -1}, fmt.Errorf("starting %s: %w", cmd.Path, err)
	}

	out := &output{log: r.Log}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer stdoutR.Close()
		if c.Capture {
			_, _ = io.Copy(out, stdoutR)
			return
		}
		readLines(stdoutR, "", out.line)
	}()
	go func() {
		defer wg.Done()
		defer stderrR.Close()
		readLines(stderrR, StderrPrefix, out.line)
	}()
	readersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(readersDone)
	}()

	beforeAssign()
	// A short command can finish before it is attached. That is a normal finish.
	if err := tree.Assign(cmd); err != nil && !errors.Is(err, procs.ErrProcessExited) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		out.finish()
		return Result{ExitCode: -1}, fmt.Errorf("assigning process tree: %w", err)
	}

	finished := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			_ = tree.Kill()
		case <-finished:
		}
	}()

	var waitErr error
	if c.Shell != "" {
		// Like exec's close event: wait for the output to end.
		select {
		case <-readersDone:
		case <-ctx.Done():
			drain(readersDone)
		}
		waitErr = cmd.Wait()
	} else {
		// Like spawn's exit event: the process ending is what counts.
		waitErr = cmd.Wait()
		drain(readersDone)
	}
	close(finished)
	<-watcherDone
	stdout := out.finish()
	tree.Release()

	res := Result{ExitCode: cmd.ProcessState.ExitCode(), Stdout: stdout}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return res, waitErr
	}
	return res, nil
}

// drain waits a bounded time for the output readers to finish.
func drain(readersDone <-chan struct{}) {
	select {
	case <-readersDone:
	case <-time.After(drainTimeout):
	}
}

// output receives a command's output until the command counts as finished. A
// background process may keep writing after that. Its output is still read, so
// it never blocks on a full pipe, but it is dropped.
type output struct {
	mu       sync.Mutex
	closed   bool
	log      LogSink
	captured bytes.Buffer
}

func (o *output) line(s string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		o.log(s)
	}
}

// Write collects captured stdout.
func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		o.captured.Write(p)
	}
	return len(p), nil
}

// finish stops collecting and returns the captured stdout.
func (o *output) finish() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	return o.captured.String()
}

// buildCmd resolves the program against env's PATH, not the launcher's.
func buildCmd(c Command, env []string) (*exec.Cmd, error) {
	if c.Shell != "" {
		return shellCmd(c.Shell, env), nil
	}
	path, err := lookPath(c.Name, env)
	if err != nil {
		return nil, err
	}
	return exec.Command(path, c.Args...), nil
}

// activatedEnv captures the environment of an activated installer_files/env
// once, by asking Python inside `micromamba run` for it. This includes what the
// environment's activate.d scripts set.
func (r *ExecRunner) activatedEnv(ctx context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activated != nil {
		return r.activated, nil
	}

	mambaRoot := filepath.Join(r.Root, "installer_files", "mamba")
	mamba := filepath.Join(mambaRoot, "micromamba")
	if runtime.GOOS == "windows" {
		mamba += ".exe"
	}
	envDir := filepath.Join(r.Root, "installer_files", "env")
	cmd := exec.CommandContext(ctx, mamba, "-p", envDir, "run", "python", "-c",
		"import json,os;print(json.dumps(dict(os.environ)))")
	cmd.Dir = r.Root
	cmd.Env = mergeEnv(os.Environ(), []string{"MAMBA_ROOT_PREFIX=" + mambaRoot})
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("activating %s: %w", envDir, err)
	}

	// micromamba may print notices first; the JSON object is the last line.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var vars map[string]string
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &vars); err != nil {
		return nil, fmt.Errorf("reading activated environment: %w", err)
	}
	env := make([]string, 0, len(vars))
	for k, v := range vars {
		env = append(env, k+"="+v)
	}
	r.activated = env
	return env, nil
}

// mergeEnv returns base with extra applied. Names compare case-insensitively on
// Windows, and a later value replaces an earlier one.
func mergeEnv(base, extra []string) []string {
	key := func(kv string) string {
		k, _, _ := strings.Cut(kv, "=")
		if runtime.GOOS == "windows" {
			return strings.ToUpper(k)
		}
		return k
	}
	replaced := map[string]bool{}
	for _, kv := range extra {
		replaced[key(kv)] = true
	}
	out := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		if !replaced[key(kv)] {
			out = append(out, kv)
		}
	}
	return append(out, extra...)
}

// envValue returns the value of name in env.
func envValue(env []string, name string) string {
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && (k == name || (runtime.GOOS == "windows" && strings.EqualFold(k, name))) {
			return v
		}
	}
	return ""
}

// lookPath finds name on the PATH in env.
func lookPath(name string, env []string) (string, error) {
	exts := []string{""}
	if runtime.GOOS == "windows" {
		pathExt := envValue(env, "PATHEXT")
		if pathExt == "" {
			pathExt = ".COM;.EXE;.BAT;.CMD"
		}
		if filepath.Ext(name) == "" {
			exts = strings.Split(pathExt, ";")
		}
	}
	if strings.ContainsAny(name, `/\`) {
		for _, ext := range exts {
			if isExecutable(name + ext) {
				return name + ext, nil
			}
		}
		return "", fmt.Errorf("%s: executable not found", name)
	}
	for _, dir := range filepath.SplitList(envValue(env, "PATH")) {
		if dir == "" {
			continue
		}
		for _, ext := range exts {
			candidate := filepath.Join(dir, name+ext)
			if isExecutable(candidate) {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("%s: executable not found on PATH", name)
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode()&0o111 != 0
}

// readLines sends each line of r to log. Carriage returns end a line too, so
// progress bars from pip and curl appear as they are drawn. Input is always
// drained, so a child can never block on a full pipe.
func readLines(r io.Reader, prefix string, log LogSink) {
	br := bufio.NewReaderSize(r, 64*1024)
	var line []byte
	afterCR := false
	atCap := false
	flush := func() {
		log(prefix + string(line))
		line = line[:0]
	}
	for {
		b, err := br.ReadByte()
		if err != nil {
			if len(line) > 0 {
				flush()
			}
			return
		}
		// The \r of a \r\n already ended this line.
		if afterCR && b == '\n' {
			afterCR = false
			continue
		}
		afterCR = false
		switch b {
		case '\n', '\r':
			// A line split at the cap has already been delivered in full.
			if !atCap {
				// Flush now: a progress bar may print nothing more for a long time.
				flush()
			}
			atCap = false
			afterCR = b == '\r'
		default:
			atCap = false
			line = append(line, b)
			if len(line) >= maxLine {
				flush()
				atCap = true
			}
		}
	}
}
