package installer_scripts

import (
	"context"
	"os"
	"runtime"
)

// batchPausePrompt is what "pause" prints. The port prints it and does not wait.
const batchPausePrompt = "Press any key to continue . . . "

// StartApp ports installer_scripts/start_app.bat and start_app.sh: activate the
// environment, run "python server.py", and when it exits (or crashes) say so and
// open a shell for the user.
//
// The shell is started as the original starts it ("cmd /k" on Windows, "exec
// $SHELL" elsewhere), with whatever stdin the runner gives it. With no
// console the shell reads end of input and exits at once, like the original does
// when it runs without a console.
//
// exitCode: start_app.bat ends with "pause", which leaves ERRORLEVEL at 0, so it
// exits 0 whatever python returned (checked against the real script). The Unix
// script ends with "exec $SHELL" and exits with the shell's code, or 0 when
// $SHELL is empty.
func StartApp(ctx context.Context, s *Step) (exitCode int, err error) {
	return startApp(ctx, s, runtime.GOOS)
}

func startApp(ctx context.Context, s *Step, goos string) (int, error) {
	if _, err := s.Run.Run(ctx, Command{Name: "python", Args: []string{"server.py"}, Activated: true}); err != nil {
		return 1, err
	}
	if goos == "windows" {
		s.Log(`""`)
		s.Log(`""`)
		s.Log("App exitted or crashed.")
		s.Log("Starting command prompt for user to run commands in case of failure...")
		s.Log(`""`)
		// cmd /k "%*": the script has no arguments, so the command is empty.
		if _, err := s.Run.Run(ctx, Command{Name: "cmd", Args: []string{"/k", ""}, Activated: true}); err != nil {
			return 1, err
		}
		s.Log(batchPausePrompt)
		return 0, nil
	}
	return startAppUnixTail(ctx, s)
}

func startAppUnixTail(ctx context.Context, s *Step) (int, error) {
	// start_app.sh echoes empty strings, not quotes.
	s.Log("")
	s.Log("")
	s.Log("App exited or crashed.")
	s.Log("Starting shell for user to run commands in case of failure...")
	s.Log("")
	shell := os.Getenv("SHELL")
	if shell == "" {
		return 0, nil
	}
	res, err := s.Run.Run(ctx, Command{Name: shell})
	if err != nil {
		return 1, err
	}
	return res.ExitCode, nil
}
