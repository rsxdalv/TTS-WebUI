// fakemamba is a stub micromamba binary used in the M1 smoke test.
//
// Handled invocations (matching what the Supervisor sends in conda mode):
//   -p <envPath> run node <script> --silent   → write .git_version and exit
//   -p <envPath> run python <args…>           → exec python from the env
//   -p <envPath> run npm <args…>              → exit 0 (no npm in test)
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	fmt.Println("fakemamba: " + strings.Join(os.Args[1:], " "))

	// Extract -p <envPath> argument.
	var envPath string
	for i, a := range os.Args {
		if a == "-p" && i+1 < len(os.Args) {
			envPath = os.Args[i+1]
		}
	}

	cwd, _ := os.Getwd()

	for i, a := range os.Args {
		if a != "run" || i+1 >= len(os.Args) {
			continue
		}
		subcmd := os.Args[i+1]
		subcmdArgs := os.Args[i+2:]

		switch subcmd {
		case "node":
			// Install step: write .git_version.
			for _, arg := range subcmdArgs {
				if strings.HasSuffix(arg, "init_app.js") {
					marker := filepath.Join(cwd, "installer_scripts", ".git_version")
					if err := os.WriteFile(marker, []byte("smoke-test-version\n"), 0o644); err != nil {
						fmt.Fprintf(os.Stderr, "fakemamba: write .git_version: %v\n", err)
						os.Exit(1)
					}
					fmt.Println("fakemamba: init_app.js complete")
					return
				}
			}
			fmt.Println("fakemamba: node (no init_app.js)")
			return

		case "python":
			// Delegate to the env Python.
			bin := pythonBin(envPath)
			cmd := exec.Command(bin, subcmdArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				// Binary not found or OS error — exit cleanly so the supervisor
				// does not fail the whole group.
				fmt.Fprintf(os.Stderr, "fakemamba: exec python: %v\n", err)
				os.Exit(0)
			}
			return

		case "npm":
			// No npm in the smoke fixture — exit quietly.
			fmt.Println("fakemamba: npm (not available in smoke fixture)")
			return
		}
	}
	fmt.Println("fakemamba: unhandled args")
}

// pythonBin returns the Python binary path inside envPath.
func pythonBin(envPath string) string {
	if envPath == "" {
		if runtime.GOOS == "windows" {
			return "python.exe"
		}
		return "python"
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(envPath, "python.exe")
	}
	return filepath.Join(envPath, "bin", "python")
}
