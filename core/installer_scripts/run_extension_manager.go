package installer_scripts

import (
	"context"
	"runtime"
)

// RunExtensionManager ports installer_scripts/tools/run_extension_manager.bat,
// run_extension_manager.sh and run_extension_manager.command (the last two are
// identical): activate the environment and run extension_manager.py.
//
// exitCode: the Windows script ends with "pause", which leaves ERRORLEVEL at 0
// (checked against the real script), so it exits 0. The Unix scripts exit with
// the code of python.
func RunExtensionManager(ctx context.Context, s *Step) (exitCode int, err error) {
	return runExtensionManager(ctx, s, runtime.GOOS)
}

func runExtensionManager(ctx context.Context, s *Step, goos string) (int, error) {
	run := func(c Command) (int, error) {
		c.Activated = true
		res, err := s.Run.Run(ctx, c)
		if err != nil {
			return 1, err
		}
		return res.ExitCode, nil
	}
	if goos != "windows" {
		s.Log("Starting Extension Management UI...")
		s.Log("Activating conda environment...")
		s.Log("Running Extension server...")
		return run(Command{Name: "python", Args: []string{"./installer_scripts/tools/extension_manager.py"}})
	}
	if _, err := run(Command{Name: "conda", Args: []string{"--version"}}); err != nil {
		return 1, err
	}
	s.Log("Node")
	if _, err := run(Command{Name: "node", Args: []string{"--version"}}); err != nil {
		return 1, err
	}
	if _, err := run(Command{Name: "python", Args: []string{"--version"}}); err != nil {
		return 1, err
	}
	s.Log("Starting Extension Management UI...")
	if _, err := run(Command{Name: "python", Args: []string{`.\installer_scripts\tools\extension_manager.py`}}); err != nil {
		return 1, err
	}
	s.Log(batchPausePrompt)
	return 0, nil
}
