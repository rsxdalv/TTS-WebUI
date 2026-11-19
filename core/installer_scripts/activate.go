package installer_scripts

import (
	"context"
	"runtime"
)

// Activate ports installer_scripts/activate.bat and activate.sh.
//
// The originals are meant to be called or sourced by another script, so that
// the caller's shell ends up with installer_files/env active. A Go port cannot
// change another process's shell, so the other ports ask the runner for the
// same thing with Command.Activated. Activate itself runs the original
// activation line in its own shell and returns its exit code, which is what
// running the original script directly does.
func Activate(ctx context.Context, s *Step) (exitCode int, err error) {
	return activate(ctx, s, runtime.GOOS)
}

func activate(ctx context.Context, s *Step, goos string) (int, error) {
	var c Command
	if goos == "windows" {
		c = Command{Shell: `@echo off & call .\installer_files\mamba\condabin\activate.bat .\installer_files\env`}
	} else {
		c = Command{Name: "bash", Args: []string{"-c", `export MAMBA_ROOT_PREFIX="$(pwd)/installer_files/mamba"
export MICROMAMBA_EXE="$MAMBA_ROOT_PREFIX/micromamba"
export PATH="$(pwd)/installer_files/env/bin:$PATH"
eval "$($MICROMAMBA_EXE shell hook -s bash)"
$MICROMAMBA_EXE activate -p "$(pwd)/installer_files/env"`}}
	}
	res, err := s.Run.Run(ctx, c)
	if err != nil {
		return 1, err
	}
	return res.ExitCode, nil
}
