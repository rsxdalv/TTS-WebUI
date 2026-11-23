package installer_scripts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Root ports installer_scripts/root.ps1 (Windows) and installer_scripts/root.sh
// (Linux, macOS): the bootstrap that checks the machine, runs init_mamba, then
// init_app and start_app.
//
// exitCode is what the script would exit with.
//
// Differences from the originals:
//   - Both scripts tee their output to installer_scripts/output.log. The port
//     writes the same lines there, without the PowerShell transcript header and
//     footer.
//   - Where root.ps1 asks "Are you sure you want to proceed?" (default No) and
//     Step.Prompt is nil, the port logs why and exits 1, as the contract says.
//   - init_mamba is the Go port (InitMamba), not the init_mamba script.
//     init_app.bat and start_app.bat still run as scripts on Windows, as in the
//     original, until their own ports are switched on.
func Root(ctx context.Context, s *Step) (exitCode int, err error) {
	logFile, ferr := os.Create(filepath.Join(s.Root, "installer_scripts", "output.log"))
	orig := s.Log
	if ferr == nil {
		defer logFile.Close()
		s2 := *s
		s2.Log = func(l string) {
			orig(l)
			_, _ = logFile.WriteString(l + "\n")
		}
		s = &s2
	}
	return root(ctx, s, runtime.GOOS, runtime.GOARCH)
}

func root(ctx context.Context, s *Step, goos, goarch string) (int, error) {
	if goos == "windows" {
		return rootWindows(ctx, s)
	}
	return rootUnix(ctx, s, goos, goarch)
}

// confirm asks the question root.ps1 asks, with No as the default. It returns
// true when the script should go on.
func (s *Step) confirm(ctx context.Context) (bool, error) {
	if s.Prompt == nil {
		s.Log("Cannot ask 'Are you sure you want to proceed?': nobody can answer, so the answer is No.")
		return false, nil
	}
	i, err := s.Prompt(ctx, "Confirmation: Are you sure you want to proceed?", []string{"Yes", "No"})
	if err != nil {
		return false, err
	}
	return i == 0, nil
}

func rootWindows(ctx context.Context, s *Step) (int, error) {
	scripts := filepath.Join(s.Root, "installer_scripts")

	if s.Root == `C:\Windows\System32` {
		s.Log("ERROR: Do not run this script as administrator!")
		s.Log("The script is running from C:\\Windows\\System32 which will cause the installation to fail.")
		return 1, nil
	}

	ps := func(expr string) (string, int, error) {
		res, err := s.Run.Run(ctx, Command{Name: "powershell", Args: []string{"-NoProfile", "-Command", expr}, Capture: true})
		return strings.TrimSpace(res.Stdout), res.ExitCode, err
	}
	stop := func(ok bool, err error) (bool, int, error) {
		if err != nil {
			return true, 1, err
		}
		if !ok {
			return true, 1, nil
		}
		return false, 0, nil
	}

	// exFAT
	if len(scripts) > 0 {
		fs, code, err := ps(fmt.Sprintf(`(Get-PSDrive -Name '%s').FileSystem`, scripts[:1]))
		switch {
		case err != nil:
			return 1, err
		case code != 0:
			s.Log("An error occurred while checking the filesystem: Get-PSDrive failed")
		case strings.EqualFold(fs, "exFAT"):
			s.Log("Warning: The installation directory is on an exFAT filesystem. Conda will fail to install. Please change the directory to NTFS.")
			s.Log(`For example, C:\AI\tts-webui\`)
			ok, cerr := s.confirm(ctx)
			if done, c, e := stop(ok, cerr); done {
				return c, e
			}
		}
	}

	// spaces
	if strings.Contains(scripts, " ") {
		s.Log("Warning: The installation directory's path contains a space character. Conda will fail to install. Please change the directory.")
		s.Log(`For example, C:\AI\tts-webui\`)
		ok, cerr := s.confirm(ctx)
		if done, c, e := stop(ok, cerr); done {
			return c, e
		}
	}

	// long paths
	lp, _, err := ps(`(Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem' -Name 'LongPathsEnabled' -ErrorAction SilentlyContinue).LongPathsEnabled`)
	if err != nil {
		return 1, err
	}
	if lp != "1" {
		for _, l := range []string{
			"Warning: Long paths are not enabled, please enable them and restart the installer.",
			"The installer is likely to fail without long paths enabled, that is why this is required.",
			"You can enable long paths by running the enable_long_paths.reg file in the installer_scripts folder.",
			"For more information, please visit:",
			"https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation?tabs=registry#enable-long-paths-in-windows-10-version-1607-and-later",
		} {
			s.Log(l)
		}
		ok, cerr := s.confirm(ctx)
		if done, c, e := stop(ok, cerr); done {
			return c, e
		}
	}

	// conda
	if _, lerr := lookPath("conda", os.Environ()); lerr == nil {
		s.Log("Warning: Conda is already installed, this might interfere with the installation.")
		res, err := s.Run.Run(ctx, Command{Name: "conda", Args: []string{"info", "--envs"}, Capture: true})
		if err != nil {
			return 1, err
		}
		active := false
		for _, line := range strings.Split(res.Stdout, "\n") {
			// Select-String "base" is case-insensitive; -match "\*" tests the matching lines.
			if strings.Contains(strings.ToLower(line), "base") && strings.Contains(line, "*") {
				active = true
			}
		}
		if active {
			s.Log("Warning: Base environment is active, please deactivate it before running the installer.")
			s.Log("You can deactivate the base environment by running 'conda deactivate' in a command prompt.")
			ok, cerr := s.confirm(ctx)
			if done, c, e := stop(ok, cerr); done {
				return c, e
			}
		}
	}

	code, err := initMamba(ctx, s, "windows", "amd64")
	if err != nil {
		return 1, err
	}
	if code != 0 {
		s.Log("Failed to init mamba, exiting...")
		return 1, nil
	}

	// The list separator is ";" on Windows, where this runs; it follows the platform so the flow can be tested elsewhere.
	// $env:Path += ";$PSScriptRoot\..\installer_files\env\Library\bin\" stays in effect for the rest.
	pathEnv := "PATH=" + os.Getenv("PATH") + string(os.PathListSeparator) + scripts + `\..\installer_files\env\Library\bin\`
	env := []string{pathEnv}
	lookEnv := mergeEnv(os.Environ(), env)

	if _, lerr := lookPath("vswhere", lookEnv); lerr != nil {
		s.Log("Critical Warning: vswhere is not installed, automatic validation of Visual Studio Build Tools installation will not work.")
		s.Log("For more information, please visit:")
		s.Log("https://github.com/microsoft/vswhere")
		s.Log("The app will try to launch but might fail.")
	} else {
		s.Log("vswhere is installed, checking for Visual Studio Build Tools installation...")
		res, err := s.Run.Run(ctx, Command{Name: "vswhere", Args: []string{"-products", "*", "-format", "json"}, Capture: true, Env: env})
		if err != nil {
			return 1, err
		}
		if jsonLength(res.Stdout) == 0 {
			s.Log("Warning: Visual Studio compiler is not installed.")
			if _, werr := lookPath("winget", lookEnv); werr != nil {
				for _, l := range []string{
					"Warning: winget is not installed, automatic installation of Visual Studio Build Tools will not work.",
					"Please install Visual Studio Build Tools manually and restart the installer.",
					"(Note: The full Visual Studio is NOT required, only the Build Tools)",
					"For more information, please visit:",
					"https://learn.microsoft.com/en-us/cpp/build/vscpp-step-0-installation?view=msvc-170",
				} {
					s.Log(l)
				}
				return 1, nil
			}
			s.Log("Attempting to install Visual Studio Build Tools using winget...")
			s.Log("This will open a new window, please follow the instructions.")
			// The exit code of winget is not checked by the original.
			if _, err := s.Run.Run(ctx, Command{Name: "winget", Args: []string{"install", "Microsoft.VisualStudio.2022.BuildTools", "--silent", "--override", "--wait --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"}, Env: env}); err != nil {
				return 1, err
			}
		} else {
			s.Log("Visual Studio Build Tools is installed, continuing...")
		}
	}

	res, err := s.Run.Run(ctx, Command{Shell: `"` + filepath.Join(scripts, "init_app.bat") + `"`, Env: env})
	if err != nil {
		return 1, err
	}
	if res.ExitCode != 0 {
		s.Log("Failed to init the app, exiting...")
		return 1, nil
	}

	res, err = s.Run.Run(ctx, Command{Shell: `"` + filepath.Join(scripts, "start_app.bat") + `"`, Env: env})
	if err != nil {
		return 1, err
	}
	if res.ExitCode != 0 {
		s.Log("Failed to start the app, exiting...")
		return 1, nil
	}
	return 0, nil
}

// jsonLength is the .length PowerShell reports for ConvertFrom-Json output:
// the element count of an array, 1 for one object, 0 for nothing.
func jsonLength(out string) int {
	out = strings.TrimSpace(out)
	if out == "" {
		return 0
	}
	var arr []json.RawMessage
	if json.Unmarshal([]byte(out), &arr) == nil {
		return len(arr)
	}
	return 1
}

func rootUnix(ctx context.Context, s *Step, goos, goarch string) (int, error) {
	code, err := initMamba(ctx, s, goos, goarch)
	if err != nil {
		return 1, err
	}
	if code != 0 {
		s.Log("Failed to init mamba, exiting...")
		return 1, nil
	}

	var env []string
	if p, gerr := lookPath("gcc", os.Environ()); gerr != nil {
		for _, l := range []string{
			"",
			"############################################################",
			"Warning: gcc is not installed, pip install might fail.",
			"Example installation for APT:",
			"sudo apt update",
			"sudo apt install build-essential",
			"Continuing...",
			"############################################################",
			"",
		} {
			s.Log(l)
		}
		env = append(env, "GCC_AVAILABLE=F")
	} else {
		s.Log(p)
	}
	env = append(env, "MICROMAMBA_ROOT_PREFIX=./installer_files/mamba", "MICROMAMBA_EXE=./installer_files/mamba/micromamba", "SCRIPTS_DIR_MAMBA=./installer_scripts")

	mamba := filepath.Join(s.Root, "installer_files", "mamba", "micromamba")
	res, err := s.Run.Run(ctx, Command{Name: mamba, Args: []string{"run", "-p", "./installer_files/env", "node", "./installer_scripts/init_app.js"}, Env: env})
	if err != nil {
		return 1, err
	}
	if res.ExitCode != 0 {
		s.Log("Failed to init the app, exiting...")
		return 1, nil
	}
	res, err = s.Run.Run(ctx, Command{Name: launcherCommand(s.Root, "linux").Name, Args: []string{"launch", "--start"}, Env: env})
	if err != nil {
		return 1, err
	}
	if res.ExitCode != 0 {
		s.Log("Failed to start the app, exiting...")
		return 1, nil
	}
	return 0, nil
}
