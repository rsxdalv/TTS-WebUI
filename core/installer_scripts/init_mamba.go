package installer_scripts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// InitMamba ports installer_scripts/init_mamba.bat (Windows) and
// installer_scripts/init_mamba.sh (Linux, macOS). It downloads micromamba and
// creates installer_files/env with git, python, node, conda, ninja, postgresql
// and ffmpeg.
//
// exitCode is what the script would exit with. On Windows that is the batch
// file's last ERRORLEVEL, which the port tracks the same way.
//
// Deliberate differences, both because nobody can answer:
//   - The scripts wait for a key ("pause", "read -p") before they exit after a
//     failure. The port does not wait.
//   - init_mamba.bat runs "set PYTHONHOME=" (unset); the runner can only set it
//     to the empty string, which Python treats the same.
func InitMamba(ctx context.Context, s *Step) (exitCode int, err error) {
	return initMamba(ctx, s, runtime.GOOS, runtime.GOARCH)
}

func initMamba(ctx context.Context, s *Step, goos, goarch string) (int, error) {
	if goos == "windows" {
		return initMambaWindows(ctx, s)
	}
	return initMambaUnix(ctx, s, goos, goarch)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// run runs c and returns its exit code.
func (s *Step) run(ctx context.Context, c Command) (int, error) {
	res, err := s.Run.Run(ctx, c)
	if err != nil {
		return 1, err
	}
	return res.ExitCode, nil
}

// ---- init_mamba.bat ----

func initMambaWindows(ctx context.Context, s *Step) (int, error) {
	root := s.Root
	mambaRoot := filepath.Join(root, "installer_files", "mamba")
	envDir := filepath.Join(root, "installer_files", "env")
	downloadURL := "https://github.com/easydiffusion/easydiffusion/releases/download/v1.1/micromamba.exe"
	umamba := filepath.Join(mambaRoot, "micromamba.exe")
	umambaExists := false

	appdata := filepath.Join(root, "installer_files", "appdata")
	userprofile := filepath.Join(root, "installer_files", "profile")
	env := []string{
		"MAMBA_ROOT_PREFIX=" + mambaRoot,
		"PYTHONHOME=",
		"APPDATA=" + appdata,
		"USERPROFILE=" + userprofile,
	}
	if fileExists(envDir) {
		path := strings.Join([]string{envDir, filepath.Join(envDir, "Library", "bin"), filepath.Join(envDir, "Scripts"), filepath.Join(envDir, "Library", "usr", "bin")}, ";")
		env = append(env, "PATH="+path+";"+os.Getenv("PATH"))
	}

	packages := "git python=3.10.11 conda-forge::nodejs=22.9.0 conda ninja conda-forge::postgresql=16.4 conda-forge::vswhere conda-forge::ffmpeg=4.4.2[build=lgpl*]"

	// level mirrors the batch file's ERRORLEVEL, which is the exit code of the script.
	level := 0
	runCmd := func(c Command) error {
		c.Env = env
		code, err := s.run(ctx, c)
		level = code
		return err
	}
	mkdir := func(p string) {
		if fileExists(p) {
			level = 1
			return
		}
		if err := os.MkdirAll(p, 0o777); err != nil {
			level = 1
			return
		}
		level = 0
	}

	tmp := filepath.Join(root, "temp")
	if !fileExists(tmp) {
		mkdir(tmp)
	}
	if err := runCmd(Command{Shell: fmt.Sprintf(`call "%s" --version >%s 2>%s`, umamba, filepath.Join(tmp, "tmp1"), filepath.Join(tmp, "tmp2"))}); err != nil {
		return 1, err
	}
	if level == 0 {
		umambaExists = true
	}

	if !umambaExists {
		s.Log(fmt.Sprintf(`"Downloading micromamba from %s to %s"`, downloadURL, umamba))
		mkdir(mambaRoot)
		if err := runCmd(Command{Shell: fmt.Sprintf(`call curl -Lk "%s" > "%s"`, downloadURL, umamba)}); err != nil {
			return 1, err
		}
		if level != 0 {
			s.Log(`"There was a problem downloading micromamba. Cannot continue."`)
			return pauseExit(s), nil
		}
		mkdir(appdata)
		mkdir(userprofile)
		s.Log("Micromamba version:")
		if err := runCmd(Command{Shell: fmt.Sprintf(`call "%s" --version`, umamba)}); err != nil {
			return 1, err
		}
	}

	s.Log(`"Checking if the Micromamba/Conda environment is already installed."`)
	if !fileExists(envDir) {
		s.Log(fmt.Sprintf(`"Creating the Micromamba/Conda environment in %s"`, envDir))
		if err := runCmd(Command{Name: umamba, Args: []string{"create", "-y", "--prefix", envDir}}); err != nil {
			return 1, err
		}
		s.Log(fmt.Sprintf(`"Packages to install:%s"`, packages))
		args := append([]string{"install", "-y", "--prefix", envDir, "-c", "conda-forge"}, strings.Fields(packages)...)
		if err := runCmd(Command{Name: umamba, Args: args}); err != nil {
			return 1, err
		}
	} else {
		s.Log(fmt.Sprintf(`"Micromamba/Conda environment already installed in %s, skipping installation"`, envDir))
	}

	if !fileExists(envDir) {
		s.Log(fmt.Sprintf(`"There was a problem while installing%s using micromamba. Cannot continue."`, packages))
		return pauseExit(s), nil
	}
	return level, nil
}

// ---- init_mamba.sh ----

func initMambaUnix(ctx context.Context, s *Step, goos, goarch string) (int, error) {
	fail := func(msg string) (int, error) {
		s.Log("")
		s.Log("EEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE")
		s.Log("")
		if msg != "" {
			s.Log("ERROR: " + msg)
		} else {
			s.Log("An error occurred.")
		}
		for _, l := range []string{
			"",
			"Error downloading TTS WebUI. Sorry about that, please try to:",
			" 1. Run this installer again.",
			" 2. If those steps don't help, please copy *all* the error messages in this window, and ask the community at https://discord.gg/V8BKTVRtJ9",
			" 3. If that doesn't solve the problem, please file an issue at https://github.com/rsxdalv/tts-webui/issues",
			"",
			"Thanks!",
			"",
			"",
		} {
			s.Log(l)
		}
		return 1, nil
	}

	var osName string
	switch goos {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "osx"
	default:
		s.Log("Unknown OS: " + goos + "! This script runs only on Linux or Mac")
		return 0, nil // the script's "&& exit" exits with echo's status
	}
	var osArch string
	switch goarch {
	case "amd64":
		osArch = "64"
	case "arm64":
		osArch = "arm64"
	default:
		s.Log("Unknown system architecture: " + goarch + "! This script runs only on x86_64 or arm64")
		return 0, nil
	}

	baseEnv := os.Environ()
	for _, tool := range []string{"curl", "tar", "bzip2"} {
		p, err := lookPath(tool, baseEnv)
		if err != nil {
			return fail(fmt.Sprintf("'%s' not found. Please install %s.", tool, tool))
		}
		s.Log(p)
	}

	if strings.Contains(s.Root, " ") {
		s.Log(s.Root)
		return fail("The installation directory's path contains a space character. Conda will fail to install. Please change the directory.")
	}

	if osName == "linux" && osArch == "arm64" {
		osArch = "aarch64"
	}

	mambaRoot := filepath.Join(s.Root, "installer_files", "mamba")
	envDir := filepath.Join(s.Root, "installer_files", "env")
	downloadURL := fmt.Sprintf("https://micro.mamba.pm/api/micromamba/%s-%s/latest", osName, osArch)
	umamba := filepath.Join(mambaRoot, "micromamba")
	umambaExists := false

	env := []string{"MAMBA_ROOT_PREFIX=" + mambaRoot}
	if fileExists(envDir) {
		env = append(env, "PATH="+filepath.Join(envDir, "bin")+":"+os.Getenv("PATH"))
	}

	packages := "python=3.10.11 conda-forge::nodejs=22.9.0 conda ninja conda-forge::postgresql=16.4 conda-forge::ffmpeg=4.4.2[build=lgpl*]"
	if _, err := lookPath("git", mergeEnv(baseEnv, env)); err != nil {
		packages += " git"
	}

	run := func(c Command) (int, error) {
		c.Env = env
		return s.run(ctx, c)
	}

	// "micromamba --version" fails when the binary is missing; that only means
	// "not downloaded yet", as in the original.
	if fileExists(umamba) {
		code, err := run(Command{Name: umamba, Args: []string{"--version"}, Capture: true})
		if err != nil {
			return 1, err
		}
		umambaExists = code == 0
	}

	if !umambaExists {
		s.Log(fmt.Sprintf("Downloading micromamba from %s to %s/micromamba", downloadURL, mambaRoot))
		if err := os.MkdirAll(mambaRoot, 0o777); err != nil {
			return 1, err
		}
		line := fmt.Sprintf(`set -o pipefail; curl -L "%s" | tar -xvj -O bin/micromamba > "%s"`, downloadURL, umamba)
		code, err := run(Command{Name: "bash", Args: []string{"-c", line}})
		if err != nil {
			return 1, err
		}
		if code != 0 {
			s.Log("")
			s.Log("EE micromamba download failed")
			s.Log("EE If the lines above contain 'bzip2: Cannot exec', your system doesn't have bzip2 installed")
			s.Log("EE If there are network errors, please check your internet setup")
			return fail("micromamba download failed")
		}
		if err := os.Chmod(umamba, mode(umamba)|0o100); err != nil {
			return 1, err
		}
		s.Log("Micromamba version:")
		if _, err := run(Command{Name: umamba, Args: []string{"--version"}}); err != nil {
			return 1, err
		}
	}

	s.Log("Checking if the Micromamba/Conda environment is already installed")
	if !fileExists(envDir) {
		s.Log("Creating the Micromamba/Conda environment in " + envDir)
		code, err := run(Command{Name: umamba, Args: []string{"create", "-y", "--prefix", envDir}})
		if err != nil {
			return 1, err
		}
		if code != 0 {
			return fail("unable to create the install environment")
		}
		s.Log("Packages to install:" + packages)
		args := append([]string{"install", "-y", "--prefix", envDir, "-c", "conda-forge"}, strings.Fields(packages)...)
		code, err = run(Command{Name: umamba, Args: args})
		if err != nil {
			return 1, err
		}
		if code != 0 {
			return fail(fmt.Sprintf("Installation of the packages '%s' failed.", packages))
		}
	} else {
		s.Log("Micromamba/Conda environment already installed in " + envDir + ", skipping installation")
	}

	if !fileExists(envDir) {
		return fail(fmt.Sprintf("There was a problem while installing %s using micromamba. Cannot continue.", packages))
	}
	return 0, nil
}

func mode(p string) os.FileMode {
	if fi, err := os.Stat(p); err == nil {
		return fi.Mode().Perm()
	}
	return 0
}

// pauseExit stands in for the batch file's "pause" followed by "exit /b". The
// prompt is printed but nobody is waited for. pause leaves ERRORLEVEL at 0, so
// the script exits 0 even though it failed (checked against the real script).
func pauseExit(s *Step) int {
	s.Log("Press any key to continue . . . ")
	return 0
}
