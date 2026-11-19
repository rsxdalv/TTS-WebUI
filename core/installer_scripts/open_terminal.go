package installer_scripts

import (
	"context"
	"path/filepath"
	"runtime"
)

// OpenTerminal ports installer_scripts/tools/open_terminal.bat and
// open_terminal.sh: open a shell inside the activated install environment.
//
// Windows prints the tool versions first and then runs "cmd /k"; Unix runs
// "micromamba run -p installer_files/env bash". The shell gets whatever stdin
// the runner gives it, so with no console it exits at once.
//
// exitCode is the exit code of that shell.
func OpenTerminal(ctx context.Context, s *Step) (exitCode int, err error) {
	return openTerminal(ctx, s, runtime.GOOS)
}

func openTerminal(ctx context.Context, s *Step, goos string) (int, error) {
	run := func(c Command) (int, error) {
		res, err := s.Run.Run(ctx, c)
		if err != nil {
			return 1, err
		}
		return res.ExitCode, nil
	}
	if goos != "windows" {
		mamba := filepath.Join(s.Root, "installer_files", "mamba", "micromamba")
		return run(Command{Name: mamba, Args: []string{"run", "-p", filepath.Join(s.Root, "installer_files", "env"), "bash"}})
	}
	s.Log("Starting command prompt for user to run commands in")
	if _, err := run(Command{Name: "conda", Args: []string{"--version"}, Activated: true}); err != nil {
		return 1, err
	}
	s.Log("Node")
	if _, err := run(Command{Name: "node", Args: []string{"--version"}, Activated: true}); err != nil {
		return 1, err
	}
	if _, err := run(Command{Name: "python", Args: []string{"--version"}, Activated: true}); err != nil {
		return 1, err
	}
	// cmd /k "%*": no arguments, so the command is empty.
	return run(Command{Name: "cmd", Args: []string{"/k", ""}, Activated: true})
}
