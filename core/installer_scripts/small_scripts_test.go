package installer_scripts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// smallRunner records commands and answers with a fixed exit code per Name.
type smallRunner struct {
	cmds  []Command
	codes map[string]int
}

func (r *smallRunner) Run(ctx context.Context, c Command) (Result, error) {
	r.cmds = append(r.cmds, c)
	return Result{ExitCode: r.codes[c.Name]}, nil
}

func (r *smallRunner) names() string {
	var out []string
	for _, c := range r.cmds {
		out = append(out, c.Name+" "+strings.Join(c.Args, " "))
	}
	return strings.Join(out, " | ")
}

func smallStep(codes map[string]int) (*Step, *smallRunner, *[]string) {
	r := &smallRunner{codes: codes}
	var lines []string
	return &Step{Root: filepath.Join("x", "root"), Run: r, Log: func(l string) { lines = append(lines, l) }}, r, &lines
}

func TestInitAppRunsNodeActivated(t *testing.T) {
	s, r, _ := smallStep(map[string]int{"node": 4})
	code, err := InitApp(context.Background(), s)
	if code != 4 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	c := r.cmds[0]
	if c.Name != "node" || !c.Activated || c.Args[0] != filepath.Join("x", "root", "installer_scripts", "init_app.js") {
		t.Fatalf("%+v", c)
	}
}

func TestStartAppWindowsExitsZeroAfterPause(t *testing.T) {
	launcher := filepath.Join("x", "root", "tts-webui.exe")
	s, r, lines := smallStep(map[string]int{launcher: 9})
	code, err := startApp(context.Background(), s, "windows")
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v (pause resets ERRORLEVEL, so python's 9 is not returned)", code, err)
	}
	if r.names() != launcher+" launch --start | cmd /k " || r.cmds[0].Activated || !r.cmds[1].Activated {
		t.Fatalf("commands: %q", r.names())
	}
	want := `""|""|App exitted or crashed.|Starting command prompt for user to run commands in case of failure...|""|Press any key to continue . . . `
	if got := strings.Join(*lines, "|"); got != want {
		t.Fatalf("log:\n%s\nwant:\n%s", got, want)
	}
}

func TestStartAppUnix(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	launcher := filepath.Join("x", "root", "tts-webui")
	s, r, lines := smallStep(map[string]int{"/bin/zsh": 5})
	code, _ := startApp(context.Background(), s, "linux")
	if code != 5 || r.names() != launcher+" launch --start | /bin/zsh " {
		t.Fatalf("code=%d commands=%q", code, r.names())
	}
	if got := strings.Join(*lines, "|"); got != "||App exited or crashed.|Starting shell for user to run commands in case of failure...|" {
		t.Fatalf("log: %q", got)
	}
	// exec $SHELL with SHELL empty runs nothing and exits 0.
	t.Setenv("SHELL", "")
	s, r, _ = smallStep(nil)
	if code, _ := startApp(context.Background(), s, "linux"); code != 0 || len(r.cmds) != 1 {
		t.Fatalf("code=%d commands=%q", code, r.names())
	}
}

func TestOpenTerminal(t *testing.T) {
	s, r, lines := smallStep(map[string]int{"cmd": 3})
	code, _ := openTerminal(context.Background(), s, "windows")
	if code != 3 || r.names() != "conda --version | node --version | python --version | cmd /k " {
		t.Fatalf("code=%d commands=%q", code, r.names())
	}
	if got := strings.Join(*lines, "|"); got != "Starting command prompt for user to run commands in|Node" {
		t.Fatalf("log: %q", got)
	}
	for _, c := range r.cmds {
		if !c.Activated {
			t.Fatalf("%s not activated", c.Name)
		}
	}
	s, r, _ = smallStep(nil)
	if code, _ := openTerminal(context.Background(), s, "darwin"); code != 0 || len(r.cmds) != 1 || r.cmds[0].Args[len(r.cmds[0].Args)-1] != "bash" || r.cmds[0].Args[0] != "run" {
		t.Fatalf("unix: %q", r.names())
	}
}

func TestActivate(t *testing.T) {
	s, r, _ := smallStep(nil)
	if code, _ := activate(context.Background(), s, "windows"); code != 0 || !strings.Contains(r.cmds[0].Shell, `activate.bat .\installer_files\env`) {
		t.Fatalf("windows: %+v", r.cmds)
	}
	s, r, _ = smallStep(map[string]int{"bash": 2})
	if code, _ := activate(context.Background(), s, "linux"); code != 2 || !strings.Contains(r.cmds[0].Args[1], `$MICROMAMBA_EXE activate -p "$(pwd)/installer_files/env"`) {
		t.Fatalf("unix: %+v", r.cmds)
	}
}

func TestLauncherNamePicksTheBinaryForThisPlatform(t *testing.T) {
	root := t.TempDir()
	if got := launcherName(root, "windows", "amd64"); got != "tts-webui.exe" {
		t.Errorf("windows: %q", got)
	}
	// Nothing there: the plain name, so the runner's error says what is missing.
	if got := launcherName(root, "linux", "amd64"); got != "tts-webui" {
		t.Errorf("empty root: %q", got)
	}
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	touch("tts-webui-linux-arm64")
	touch("tts-webui-macos-arm64")
	if got := launcherName(root, "linux", "arm64"); got != "tts-webui-linux-arm64" {
		t.Errorf("linux/arm64: %q", got)
	}
	if got := launcherName(root, "darwin", "arm64"); got != "tts-webui-macos-arm64" {
		t.Errorf("darwin/arm64: %q", got)
	}
	if got := launcherName(root, "linux", "amd64"); got != "tts-webui" {
		t.Errorf("linux/amd64 has no shipped binary here: %q", got)
	}
	touch("tts-webui") // one the user built wins
	if got := launcherName(root, "linux", "arm64"); got != "tts-webui" {
		t.Errorf("a built tts-webui should win: %q", got)
	}
}
