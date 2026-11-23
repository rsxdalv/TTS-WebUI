package installer_scripts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// rootFixture makes an install where micromamba and the env already exist, so
// InitMamba takes its skip path, and returns the step with a recording runner.
func rootFixture(t *testing.T, answer func(c Command) (int, string)) (*Step, *mambaFakeRunner, *[]string, string) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "installer_scripts"), 0o777)
	os.MkdirAll(filepath.Join(root, "installer_files", "env", "Library", "bin"), 0o777)
	var lines []string
	fr := &mambaFakeRunner{}
	s := &Step{Root: root, Run: &captureRunner{inner: fr, answer: answer}, Log: func(l string) { lines = append(lines, l) }}
	return s, fr, &lines, root
}

// captureRunner lets a test choose the exit code and stdout of each command.
type captureRunner struct {
	inner  *mambaFakeRunner
	answer func(c Command) (int, string)
}

func (r *captureRunner) Run(ctx context.Context, c Command) (Result, error) {
	r.inner.cmds = append(r.inner.cmds, c)
	code, out := 0, ""
	if r.answer != nil {
		code, out = r.answer(c)
	}
	return Result{ExitCode: code, Stdout: out}, nil
}

func isPS(c Command, needle string) bool {
	return c.Name == "powershell" && strings.Contains(strings.Join(c.Args, " "), needle)
}

func noConda(t *testing.T, extra ...string) {
	dir := t.TempDir()
	for _, n := range extra {
		os.WriteFile(filepath.Join(dir, n+".exe"), nil, 0o755)
		os.WriteFile(filepath.Join(dir, n), nil, 0o755)
	}
	t.Setenv("PATH", dir)
	t.Setenv("Path", dir)
}

func happyAnswers(c Command) (int, string) {
	switch {
	case isPS(c, "FileSystem"):
		if strings.Contains(strings.Join(c.Args, " "), "LongPathsEnabled") {
			return 0, "1\r\n"
		}
		return 0, "NTFS\r\n"
	case c.Name == "vswhere":
		return 0, `[{"installationPath":"C:\\VS"}]`
	}
	return 0, ""
}

func TestRootWindowsHappyPath(t *testing.T) {
	noConda(t, "vswhere") // on the base PATH, so the lookup also works where PATH uses ":"
	s, fr, lines, root := rootFixture(t, happyAnswers)
	os.WriteFile(filepath.Join(root, "installer_files", "env", "Library", "bin", "vswhere.exe"), nil, 0o755)

	code, err := root_(t, s)
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v log=%q", code, err, *lines)
	}
	d := fr.desc()
	scripts := filepath.Join(root, "installer_scripts")
	last2 := d[len(d)-2:]
	if last2[0] != `sh: "`+filepath.Join(scripts, "init_app.bat")+`"` || last2[1] != `sh: "`+filepath.Join(scripts, "start_app.bat")+`"` {
		t.Fatalf("last commands: %q", last2)
	}
	wantPath := string(os.PathListSeparator) + scripts + `\..\installer_files\env\Library\bin\`
	p, _ := mambaEnvOf(fr.cmds[len(fr.cmds)-1], "PATH")
	if !strings.HasSuffix(p, wantPath) {
		t.Fatalf("PATH = %q, want suffix %q", p, wantPath)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "Visual Studio Build Tools is installed, continuing...") {
		t.Fatalf("log: %q", *lines)
	}
}

func root_(t *testing.T, s *Step) (int, error) {
	return root(context.Background(), s, "windows", "amd64")
}

func TestRootWindowsLongPathsDisabled(t *testing.T) {
	noConda(t)
	answer := func(c Command) (int, string) {
		if isPS(c, "LongPathsEnabled") {
			return 0, "\r\n"
		}
		return happyAnswers(c)
	}
	// Nobody to ask: the answer is No, so the script stops before init_mamba.
	s, fr, lines, _ := rootFixture(t, answer)
	code, _ := root_(t, s)
	if code != 1 || len(fr.cmds) != 2 {
		t.Fatalf("code=%d commands=%q", code, fr.desc())
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "Cannot ask 'Are you sure you want to proceed?'") {
		t.Fatalf("log: %q", *lines)
	}

	// Answering No (index 1) also stops.
	s, fr, _, _ = rootFixture(t, answer)
	s.Prompt = func(ctx context.Context, label string, choices []string) (int, error) { return 1, nil }
	if code, _ := root_(t, s); code != 1 || len(fr.cmds) != 2 {
		t.Fatalf("No: code=%d commands=%q", code, fr.desc())
	}

	// Answering Yes (index 0) goes on to init_mamba and the two scripts.
	s, fr, _, _ = rootFixture(t, answer)
	s.Prompt = func(ctx context.Context, label string, choices []string) (int, error) {
		if len(choices) != 2 || choices[0] != "Yes" || choices[1] != "No" {
			t.Errorf("choices %q", choices)
		}
		return 0, nil
	}
	if code, _ := root_(t, s); code != 0 {
		t.Fatalf("Yes: code=%d", code)
	}
	if n := len(fr.cmds); n < 5 {
		t.Fatalf("Yes: expected the run to continue, commands=%q", fr.desc())
	}
}

func TestRootWindowsSystem32(t *testing.T) {
	s, fr, lines, _ := rootFixture(t, nil)
	s.Root = `C:\Windows\System32`
	code, _ := root_(t, s)
	if code != 1 || len(fr.cmds) != 0 || (*lines)[0] != "ERROR: Do not run this script as administrator!" {
		t.Fatalf("code=%d log=%q", code, *lines)
	}
}

func TestRootWindowsStepFailures(t *testing.T) {
	for _, tc := range []struct{ name, fail, msg string }{
		{"init_app", "init_app.bat", "Failed to init the app, exiting..."},
		{"start_app", "start_app.bat", "Failed to start the app, exiting..."},
	} {
		noConda(t)
		s, _, lines, _ := rootFixture(t, func(c Command) (int, string) {
			if strings.Contains(c.Shell, tc.fail) {
				return 4, ""
			}
			return happyAnswers(c)
		})
		code, _ := root_(t, s)
		if code != 1 || (*lines)[len(*lines)-1] != tc.msg {
			t.Fatalf("%s: code=%d log=%q", tc.name, code, *lines)
		}
	}
}

func TestRootWindowsCondaBaseActive(t *testing.T) {
	noConda(t, "conda")
	answer := func(c Command) (int, string) {
		if c.Name == "conda" {
			return 0, "# conda environments:\r\nbase                  *  C:\\miniconda3\r\n"
		}
		return happyAnswers(c)
	}
	s, fr, lines, _ := rootFixture(t, answer)
	code, _ := root_(t, s)
	joined := strings.Join(*lines, "\n")
	if code != 1 || !strings.Contains(joined, "Warning: Base environment is active") || !strings.Contains(joined, "Warning: Conda is already installed") {
		t.Fatalf("code=%d log=%q cmds=%q", code, *lines, fr.desc())
	}
	// Not active: only the warning, and the run goes on.
	s, _, lines, _ = rootFixture(t, func(c Command) (int, string) {
		if c.Name == "conda" {
			return 0, "base                     C:\\miniconda3\r\n"
		}
		return happyAnswers(c)
	})
	if code, _ := root_(t, s); code != 0 || strings.Contains(strings.Join(*lines, "\n"), "Base environment is active") {
		t.Fatalf("code=%d log=%q", code, *lines)
	}
}

func TestRootJSONLength(t *testing.T) {
	for in, want := range map[string]int{"": 0, "[]": 0, `[{"a":1},{"a":2}]`: 2, `{"a":1}`: 1} {
		if got := jsonLength(in); got != want {
			t.Errorf("jsonLength(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestRootUnix(t *testing.T) {
	noConda(t) // no gcc on PATH
	s, fr, lines, root := rootFixture(t, nil)
	// InitMamba needs curl, tar, bzip2 on PATH before it can skip.
	noConda(t, "curl", "tar", "bzip2")
	os.WriteFile(filepath.Join(root, "installer_files", "mamba", "micromamba"), nil, 0o755)
	os.MkdirAll(filepath.Join(root, "installer_files", "mamba"), 0o777)
	os.WriteFile(filepath.Join(root, "installer_files", "mamba", "micromamba"), nil, 0o755)

	code, err := rootUnixForTest(s)
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v log=%q", code, err, *lines)
	}
	d := fr.desc()
	mamba := filepath.Join(root, "installer_files", "mamba", "micromamba")
	n := len(d)
	if d[n-2] != mamba+" run -p ./installer_files/env node ./installer_scripts/init_app.js" || d[n-1] != filepath.Join(root, "tts-webui")+" launch --start" {
		t.Fatalf("commands: %q", d)
	}
	if v, ok := mambaEnvOf(fr.cmds[n-1], "GCC_AVAILABLE"); !ok || v != "F" {
		t.Fatalf("GCC_AVAILABLE missing, env=%q", fr.cmds[n-1].Env)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "Warning: gcc is not installed, pip install might fail.") {
		t.Fatalf("log: %q", *lines)
	}
}

func rootUnixForTest(s *Step) (int, error) {
	return root(context.Background(), s, "linux", "amd64")
}

func TestRootWritesOutputLog(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Root uses the real platform flow; the Windows flow is covered by the tests above")
	}
	noConda(t)
	s, _, _, root := rootFixture(t, happyAnswers)
	if code, err := Root(context.Background(), s); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	b, err := os.ReadFile(filepath.Join(root, "installer_scripts", "output.log"))
	if err != nil || !strings.Contains(string(b), "Checking if the Micromamba/Conda environment is already installed.") {
		t.Fatalf("output.log = %q err=%v", b, err)
	}
}
