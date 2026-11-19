package installer_scripts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// mambaFakeRunner records commands and answers with exit codes chosen by match.
type mambaFakeRunner struct {
	cmds []Command
	code func(c Command) int
}

func (f *mambaFakeRunner) Run(ctx context.Context, c Command) (Result, error) {
	f.cmds = append(f.cmds, c)
	code := 0
	if f.code != nil {
		code = f.code(c)
	}
	return Result{ExitCode: code}, nil
}

func (f *mambaFakeRunner) desc() []string {
	var out []string
	for _, c := range f.cmds {
		if c.Shell != "" {
			out = append(out, "sh: "+c.Shell)
		} else {
			out = append(out, c.Name+" "+strings.Join(c.Args, " "))
		}
	}
	return out
}

func mkMambaStep(t *testing.T, fr *mambaFakeRunner) (*Step, *[]string, string) {
	root := t.TempDir()
	var lines []string
	return &Step{Root: root, Run: fr, Log: func(l string) { lines = append(lines, l) }}, &lines, root
}

func mambaEnvOf(c Command, name string) (string, bool) {
	for _, kv := range c.Env {
		if k, v, _ := strings.Cut(kv, "="); k == name {
			return v, true
		}
	}
	return "", false
}

func TestInitMambaWindowsEnvExists(t *testing.T) {
	fr := &mambaFakeRunner{}
	s, lines, root := mkMambaStep(t, fr)
	envDir := filepath.Join(root, "installer_files", "env")
	os.MkdirAll(envDir, 0o777)

	code, err := initMamba(context.Background(), s, "windows", "amd64")
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if len(fr.cmds) != 1 || !strings.Contains(fr.cmds[0].Shell, `micromamba.exe" --version >`) {
		t.Fatalf("commands: %q", fr.desc())
	}
	if _, err := os.Stat(filepath.Join(root, "temp")); err != nil {
		t.Fatal("temp dir not created")
	}
	got := strings.Join(*lines, "|")
	want := `"Checking if the Micromamba/Conda environment is already installed."|"Micromamba/Conda environment already installed in ` + envDir + `, skipping installation"`
	if got != want {
		t.Fatalf("log:\n%s\nwant:\n%s", got, want)
	}
	p, _ := mambaEnvOf(fr.cmds[0], "PATH")
	if !strings.HasPrefix(p, envDir+";"+filepath.Join(envDir, "Library", "bin")+";") {
		t.Fatalf("PATH = %q", p)
	}
	if v, ok := mambaEnvOf(fr.cmds[0], "APPDATA"); !ok || v != filepath.Join(root, "installer_files", "appdata") {
		t.Fatalf("APPDATA = %q", v)
	}
	if v, ok := mambaEnvOf(fr.cmds[0], "MAMBA_ROOT_PREFIX"); !ok || v != filepath.Join(root, "installer_files", "mamba") {
		t.Fatalf("MAMBA_ROOT_PREFIX = %q", v)
	}
}

func TestInitMambaWindowsFullInstall(t *testing.T) {
	// --version fails first (no micromamba yet), then everything succeeds.
	fr := &mambaFakeRunner{code: func(c Command) int {
		if strings.Contains(c.Shell, "--version >") {
			return 9009
		}
		return 0
	}}
	s, lines, root := mkMambaStep(t, fr)
	code, err := initMamba(context.Background(), s, "windows", "amd64")
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	mamba := filepath.Join(root, "installer_files", "mamba", "micromamba.exe")
	envDir := filepath.Join(root, "installer_files", "env")
	d := fr.desc()
	if len(d) != 5 {
		t.Fatalf("commands: %q", d)
	}
	if !strings.HasPrefix(d[1], `sh: call curl -Lk "https://github.com/easydiffusion/easydiffusion/releases/download/v1.1/micromamba.exe" > "`+mamba) {
		t.Fatalf("download: %q", d[1])
	}
	if d[2] != `sh: call "`+mamba+`" --version` {
		t.Fatalf("version: %q", d[2])
	}
	if d[3] != `sh: call "`+mamba+`" create -y --prefix "`+envDir+`"` {
		t.Fatalf("create: %q", d[3])
	}
	wantInstall := `sh: call "` + mamba + `" install -y --prefix "` + envDir + `" -c conda-forge git python=3.10.11 conda-forge::nodejs=22.9.0 conda ninja conda-forge::postgresql=16.4 conda-forge::vswhere conda-forge::ffmpeg=4.4.2[build=lgpl*]`
	if d[4] != wantInstall {
		t.Fatalf("install: %q", d[4])
	}
	for _, dir := range []string{"appdata", "profile"} {
		if _, err := os.Stat(filepath.Join(root, "installer_files", dir)); err != nil {
			t.Fatalf("%s not created", dir)
		}
	}
	if (*lines)[0] != `"Downloading micromamba from https://github.com/easydiffusion/easydiffusion/releases/download/v1.1/micromamba.exe to `+mamba+`"` {
		t.Fatalf("first log: %q", (*lines)[0])
	}
}

func TestInitMambaWindowsDownloadFails(t *testing.T) {
	fr := &mambaFakeRunner{code: func(c Command) int {
		if strings.Contains(c.Shell, "--version >") {
			return 9009
		}
		if strings.Contains(c.Shell, "curl") {
			return 22
		}
		return 0
	}}
	s, lines, _ := mkMambaStep(t, fr)
	code, err := initMamba(context.Background(), s, "windows", "amd64")
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v, want 0 (pause resets ERRORLEVEL)", code, err)
	}
	if len(fr.cmds) != 2 {
		t.Fatalf("must stop after the failed download: %q", fr.desc())
	}
	if last := (*lines)[len(*lines)-2]; last != `"There was a problem downloading micromamba. Cannot continue."` {
		t.Fatalf("last log %q", last)
	}
}

// The original ignores the exit codes of create and install, and ends with the
// last ERRORLEVEL when the env folder is missing afterwards.
func TestInitMambaWindowsInstallFailsEnvMissing(t *testing.T) {
	fr := &mambaFakeRunner{code: func(c Command) int {
		if strings.Contains(c.Shell, " install -y ") {
			return 3
		}
		if strings.Contains(c.Shell, "--version >") {
			return 9009
		}
		return 0
	}}
	s, lines, _ := mkMambaStep(t, fr)
	code, err := initMamba(context.Background(), s, "windows", "amd64")
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if last := (*lines)[len(*lines)-2]; !strings.HasPrefix(last, `"There was a problem while installinggit python=3.10.11`) {
		t.Fatalf("last log %q", last)
	}
}

func withMambaTools(t *testing.T, names ...string) {
	dir := t.TempDir()
	for _, n := range names {
		for _, f := range []string{n, n + ".exe"} {
			if err := os.WriteFile(filepath.Join(dir, f), nil, 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("Path", dir)
}

func TestInitMambaUnixUnknownPlatform(t *testing.T) {
	s, lines, _ := mkMambaStep(t, &mambaFakeRunner{})
	if code, err := initMamba(context.Background(), s, "freebsd", "amd64"); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if (*lines)[0] != "Unknown OS: freebsd! This script runs only on Linux or Mac" {
		t.Fatal((*lines)[0])
	}
	if code, _ := initMamba(context.Background(), s, "linux", "386"); code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestInitMambaUnixMissingToolAndSpace(t *testing.T) {
	withMambaTools(t, "curl", "tar") // no bzip2
	s, lines, _ := mkMambaStep(t, &mambaFakeRunner{})
	code, _ := initMamba(context.Background(), s, "linux", "amd64")
	if code != 1 || !strings.Contains(strings.Join(*lines, "\n"), "ERROR: 'bzip2' not found. Please install bzip2.") {
		t.Fatalf("code=%d log=%q", code, *lines)
	}

	withMambaTools(t, "curl", "tar", "bzip2")
	fr := &mambaFakeRunner{}
	s, lines, root := mkMambaStep(t, fr)
	s.Root = filepath.Join(root, "has space")
	code, _ = initMamba(context.Background(), s, "linux", "amd64")
	if code != 1 || !strings.Contains(strings.Join(*lines, "\n"), "ERROR: The installation directory's path contains a space character.") || len(fr.cmds) != 0 {
		t.Fatalf("code=%d log=%q cmds=%q", code, *lines, fr.desc())
	}
}

func TestInitMambaUnixFullInstall(t *testing.T) {
	withMambaTools(t, "curl", "tar", "bzip2") // no git on PATH, so git is added
	fr := &mambaFakeRunner{}
	s, _, root := mkMambaStep(t, fr)
	mamba := filepath.Join(root, "installer_files", "mamba", "micromamba")
	// The fake download "creates" the binary so chmod works.
	fr.code = func(c Command) int {
		if c.Name == "bash" {
			os.WriteFile(mamba, []byte("x"), 0o644)
		}
		if len(c.Args) > 0 && c.Args[0] == "create" {
			os.MkdirAll(filepath.Join(root, "installer_files", "env"), 0o777)
		}
		return 0
	}
	code, err := initMamba(context.Background(), s, "linux", "arm64")
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	d := fr.desc()
	if len(d) != 4 {
		t.Fatalf("commands: %q", d)
	}
	wantDL := `bash -c set -o pipefail; curl -L "https://micro.mamba.pm/api/micromamba/linux-aarch64/latest" | tar -xvj -O bin/micromamba > "` + mamba + `"`
	if d[0] != wantDL {
		t.Fatalf("download: %q", d[0])
	}
	if !strings.HasSuffix(d[3], "-c conda-forge python=3.10.11 conda-forge::nodejs=22.9.0 conda ninja conda-forge::postgresql=16.4 conda-forge::ffmpeg=4.4.2[build=lgpl*] git") {
		t.Fatalf("install: %q", d[3])
	}
}

func TestInitMambaUnixCreateFails(t *testing.T) {
	withMambaTools(t, "curl", "tar", "bzip2", "git")
	fr := &mambaFakeRunner{}
	s, lines, root := mkMambaStep(t, fr)
	mamba := filepath.Join(root, "installer_files", "mamba", "micromamba")
	fr.code = func(c Command) int {
		if c.Name == "bash" {
			os.WriteFile(mamba, []byte("x"), 0o644)
		}
		if len(c.Args) > 0 && c.Args[0] == "create" {
			return 1
		}
		return 0
	}
	code, _ := initMamba(context.Background(), s, "darwin", "amd64")
	if code != 1 || !strings.Contains(strings.Join(*lines, "\n"), "ERROR: unable to create the install environment") {
		t.Fatalf("code=%d log=%q", code, *lines)
	}
}

// runsRealFor sends commands for which match is true to a real ExecRunner and
// answers every other command with exit code 0 (so the version check "works").
type runsRealFor struct {
	real  *ExecRunner
	match func(Command) bool
	seen  []Command
}

func (r *runsRealFor) Run(ctx context.Context, c Command) (Result, error) {
	r.seen = append(r.seen, c)
	if r.match(c) {
		return r.real.Run(ctx, c)
	}
	return Result{}, nil
}

// Review F1/F2: when the downloaded file is not a program, the original's "call"
// only sets ERRORLEVEL, the script goes on to "install", prints the failure, and
// exits 0.
func TestInitMambaWindowsInvalidMicromambaFileLikeOriginal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("runs cmd.exe")
	}
	root := t.TempDir()
	mambaDir := filepath.Join(root, "installer_files", "mamba")
	os.MkdirAll(mambaDir, 0o777)
	os.WriteFile(filepath.Join(mambaDir, "micromamba.exe"), []byte("<html>captive portal</html>"), 0o644)
	var lines []string
	log := func(l string) { lines = append(lines, l) }
	r := &runsRealFor{real: &ExecRunner{Root: root, Log: log}, match: func(c Command) bool {
		return strings.Contains(c.Shell, " create -y ") || strings.Contains(c.Shell, " install -y ")
	}}
	s := &Step{Root: root, Run: r, Log: log}
	code, err := initMambaWindows(context.Background(), s)
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v log=%q", code, err, lines)
	}
	var shells []string
	for _, c := range r.seen {
		shells = append(shells, c.Shell)
	}
	joined := strings.Join(shells, "\n")
	if !strings.Contains(joined, " create -y ") || !strings.Contains(joined, " install -y ") {
		t.Fatalf("must try create and install: %q", shells)
	}
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "There was a problem while installinggit") || !strings.Contains(all, "Press any key to continue") {
		t.Fatalf("log: %q", lines)
	}
}

// Review F1: ERRORLEVEL is non-zero at the end of the script, but the script exits 0.
func TestInitMambaWindowsExitsZeroEvenWhenInstallFailed(t *testing.T) {
	fr := &mambaFakeRunner{code: func(c Command) int {
		if strings.Contains(c.Shell, " install -y ") {
			return 5
		}
		return 0
	}}
	s, _, root := mkMambaStep(t, fr)
	os.MkdirAll(filepath.Join(root, "installer_files", "mamba"), 0o777)
	// Env folder appears after "create", so the final check passes.
	fr.code = func(c Command) int {
		if strings.Contains(c.Shell, " create -y ") {
			os.MkdirAll(filepath.Join(root, "installer_files", "env"), 0o777)
		}
		if strings.Contains(c.Shell, " install -y ") {
			return 5
		}
		if strings.Contains(c.Shell, "--version >") {
			return 9009
		}
		return 0
	}
	if code, err := initMamba(context.Background(), s, "windows", "amd64"); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v, want 0", code, err)
	}
}

// Review F3: a micromamba file that cannot start (failed earlier download, no
// execute bit) means "download again", not a failure of the port.
func TestInitMambaUnixUnusableMicromambaDownloadsAgain(t *testing.T) {
	withMambaTools(t, "curl", "tar", "bzip2")
	root := t.TempDir()
	mambaDir := filepath.Join(root, "installer_files", "mamba")
	os.MkdirAll(mambaDir, 0o777)
	mamba := filepath.Join(mambaDir, "micromamba")
	os.WriteFile(mamba, []byte("partial"), 0o644) // no execute bit
	var lines []string
	log := func(l string) { lines = append(lines, l) }
	r := &runsRealFor{real: &ExecRunner{Root: root, Log: log}, match: func(c Command) bool {
		return c.Name == mamba && len(c.Args) == 1 && c.Args[0] == "--version"
	}}
	// After the fake download, "create" must not run for real.
	s := &Step{Root: root, Run: r, Log: log}
	code, err := initMambaUnix(context.Background(), s, "linux", "amd64")
	_ = code
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	var sawDownload bool
	for _, c := range r.seen {
		if c.Name == "bash" && strings.Contains(strings.Join(c.Args, " "), "curl -L") {
			sawDownload = true
		}
	}
	if !sawDownload {
		t.Fatalf("expected a new download, got %d commands, log=%q", len(r.seen), lines)
	}
}
