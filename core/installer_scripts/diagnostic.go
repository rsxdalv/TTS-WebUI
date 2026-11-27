package installer_scripts

import (
	"context"
	"runtime"
)

// Diagnostic ports installer_scripts/tools/diagnostic.bat (Windows) and
// installer_scripts/tools/diagnostic.sh with diagnostic_inner.sh (Linux,
// macOS). It writes installer_scripts/diagnostic.txt.
//
// Every line of the original runs as its own shell line with the same text and
// redirection, so the file gets the same bytes the script would write (the
// shell evaluates %date%, $(date), where, which and so on). The only part that
// cannot run as written is "call activate.bat" / "source activate.sh": the
// original runs it in the same shell session, so later lines see the activated
// environment. The port runs the later lines with Command.Activated instead,
// which captures the environment of "micromamba -p installer_files/env run".
// On Unix the whole inner script already runs under that command, so every
// line is activated, as in the original.
//
// exitCode is 0 unless a command could not be started, as in the originals,
// whose last line is an echo.
func Diagnostic(ctx context.Context, s *Step) (exitCode int, err error) {
	return diagnostic(ctx, s, runtime.GOOS)
}

func diagnostic(ctx context.Context, s *Step, goos string) (int, error) {
	var lines []diagLine
	if goos == "windows" {
		lines = diagnosticWindows()
	} else {
		s.Log("Generating diagnostic...")
		lines = diagnosticUnix()
	}
	for _, l := range lines {
		if _, err := s.Run.Run(ctx, Command{Shell: l.line, Activated: l.activated}); err != nil {
			return 1, err
		}
	}
	if goos != "windows" {
		s.Log("Diagnostic generated.")
	}
	return 0, nil
}

type diagLine struct {
	line      string
	activated bool
}

func diagnosticWindows() []diagLine {
	const out = `.\installer_scripts\diagnostic.txt`
	var l []diagLine
	activated := false
	add := func(s string) { l = append(l, diagLine{s, activated}) }
	echo := func(text string) { add("echo " + text + " >> " + out + " 2>&1") }
	call := func(cmd string) { add("call " + cmd + " >> " + out + " 2>&1") }
	blank := func() { add("echo: >> " + out) }
	section := func(title, cmd string) { echo(title); call(cmd); blank() }

	add("echo Starting diagnostic... > " + out + " 2>&1")
	blank()
	echo("%date% %time%")
	blank()
	echo("CD: %cd%")
	blank()
	tools := []string{"git", "python", "conda", "node"}
	for _, t := range tools {
		section("where "+t+" (before calling activate.bat):", "where "+t)
	}

	// call .\installer_scripts\activate.bat
	activated = true

	blank()
	for _, t := range tools {
		section("where "+t+" (after calling activate.bat):", "where "+t)
	}
	section("python --version:", "python --version")
	section("node --version:", "node --version")
	section("conda --version:", "conda --version")
	section("git status:", "git status")
	section("git log HEAD~1..HEAD:", "git log HEAD~1..HEAD")
	section("pip list:", "pip list")
	section("conda list:", "conda list")
	section("npm list -g:", "npm list -g")
	section("pip show python-dotenv gradio torch torchaudio torchvision:", "pip show python-dotenv gradio torch torchaudio torchvision")
	for _, f := range []string{".gpu", ".major_version", ".python_version"} {
		echo(f + ":")
		add(`type .\installer_scripts\` + f + " >> " + out + " 2>&1")
		blank()
	}
	// The original's last section has no colon after the title.
	echo("npm list --prefix ./frontend/")
	call("npm list --prefix ./frontend/")
	blank()
	return l
}

func diagnosticUnix() []diagLine {
	const out = "./installer_scripts/diagnostic.txt"
	var l []diagLine
	add := func(s string) { l = append(l, diagLine{s, true}) }
	echo := func(text string) { add(`echo "` + text + `" >> ` + out) }
	run := func(cmd string) { add(cmd + " >> " + out + " 2>&1") }
	blank := func() { echo("") }
	section := func(title, cmd string) { echo(title); run(cmd); blank() }

	add(`echo "Starting diagnostic..." > ` + out)
	blank()
	add(`echo "$(date)" >> ` + out)
	blank()
	add(`echo "CD: $(pwd)" >> ` + out)
	blank()
	tools := []string{"git", "python", "conda", "node"}
	for _, t := range tools {
		section("which "+t+" (before calling activate.sh):", "which "+t)
	}
	// source activate.sh runs here in the original; the lines are already activated.
	blank()
	for _, t := range tools {
		section("which "+t+" (after calling activate.sh):", "which "+t)
	}
	section("python --version:", "python --version")
	section("node --version:", "node --version")
	section("conda --version:", "conda --version")
	section("git status:", "git status")
	section("git log HEAD~1..HEAD:", "git log HEAD~1..HEAD")
	section("pip list:", "pip list")
	section("conda list:", "conda list")
	section("npm list -g:", "npm list -g")
	section("pip show python-dotenv gradio torch torchaudio torchvision:", "pip show python-dotenv gradio torch torchaudio torchvision")
	for _, f := range []string{".gpu", ".major_version", ".python_version"} {
		echo(f + ":")
		run("cat ./installer_scripts/" + f)
		blank()
	}
	echo("npm list --prefix ./frontend/")
	run("npm list --prefix ./frontend/")
	blank()
	return l
}
