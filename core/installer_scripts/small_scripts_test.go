package installer_scripts

import (
	"context"
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
	s, r, lines := smallStep(map[string]int{"python": 9})
	code, err := startApp(context.Background(), s, "windows")
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v (pause resets ERRORLEVEL, so python's 9 is not returned)", code, err)
	}
	if r.names() != "python server.py | cmd /k " || !r.cmds[0].Activated || !r.cmds[1].Activated {
		t.Fatalf("commands: %q", r.names())
	}
	want := `""|""|App exitted or crashed.|Starting command prompt for user to run commands in case of failure...|""|Press any key to continue . . . `
	if got := strings.Join(*lines, "|"); got != want {
		t.Fatalf("log:\n%s\nwant:\n%s", got, want)
	}
}

func TestStartAppUnix(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	s, r, lines := smallStep(map[string]int{"/bin/zsh": 5})
	code, _ := startApp(context.Background(), s, "linux")
	if code != 5 || r.names() != "python server.py | /bin/zsh " {
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

func TestRunExtensionManager(t *testing.T) {
	s, r, lines := smallStep(map[string]int{"python": 6})
	code, _ := runExtensionManager(context.Background(), s, "windows")
	want := `conda --version | node --version | python --version | python .\installer_scripts\tools\extension_manager.py`
	if code != 0 || r.names() != want {
		t.Fatalf("code=%d commands=%q", code, r.names())
	}
	if got := strings.Join(*lines, "|"); got != "Node|Starting Extension Management UI...|Press any key to continue . . . " {
		t.Fatalf("log: %q", got)
	}
	s, r, lines = smallStep(map[string]int{"python": 6})
	code, _ = runExtensionManager(context.Background(), s, "linux")
	if code != 6 || r.names() != "python ./installer_scripts/tools/extension_manager.py" {
		t.Fatalf("unix code=%d commands=%q", code, r.names())
	}
	if got := strings.Join(*lines, "|"); got != "Starting Extension Management UI...|Activating conda environment...|Running Extension server..." {
		t.Fatalf("unix log: %q", got)
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
