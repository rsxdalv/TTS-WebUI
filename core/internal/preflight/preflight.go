// Package preflight runs system pre-flight checks before installation.
package preflight

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// CheckStatus is the outcome of a single preflight check.
type CheckStatus string

const (
	StatusPass CheckStatus = "pass"
	StatusWarn CheckStatus = "warn"
	StatusFail CheckStatus = "fail"
)

// Check is the result of one preflight check.
type Check struct {
	ID        string      `json:"id"`
	Status    CheckStatus `json:"status"`
	Message   string      `json:"message,omitempty"`
	FixAction string      `json:"fix_action,omitempty"`
}

// Runner executes a named command and returns stdout, stderr, and exit code.
// A non-zero exit code is not an error; infrastructure failures (context
// cancellation, etc.) are returned in err.
type Runner func(ctx context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error)

// DefaultRunner executes real system commands.
func DefaultRunner(ctx context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if runErr != nil {
		if exit, ok := runErr.(*exec.ExitError); ok {
			exitCode = exit.ExitCode()
		} else {
			err = runErr
		}
	}
	return
}

// Run executes all platform-appropriate preflight checks.
// Pass a nil runner to use DefaultRunner.
func Run(ctx context.Context, root paths.Root, runner Runner) ([]Check, error) {
	if runner == nil {
		runner = DefaultRunner
	}
	var checks []Check

	checks = append(checks, checkSpacesInPath(root))

	if runtime.GOOS == "windows" {
		checks = append(checks, checkSystem32InPath(root))

		c, err := checkExFAT(ctx, root, runner)
		if err != nil {
			return nil, fmt.Errorf("exfat check: %w", err)
		}
		checks = append(checks, c)

		c, err = checkLongPaths(ctx, root, runner)
		if err != nil {
			return nil, fmt.Errorf("long-paths check: %w", err)
		}
		checks = append(checks, c)

		c, err = checkConflictingConda(ctx, root, runner)
		if err != nil {
			return nil, fmt.Errorf("conda check: %w", err)
		}
		checks = append(checks, c)

		c, err = checkMSVC(ctx, runner)
		if err != nil {
			return nil, fmt.Errorf("msvc check: %w", err)
		}
		checks = append(checks, c)
	} else {
		c, err := checkGCC(ctx, runner)
		if err != nil {
			return nil, fmt.Errorf("gcc check: %w", err)
		}
		checks = append(checks, c)
	}

	return checks, nil
}

// AllPassed returns true when no check carries StatusFail.
func AllPassed(checks []Check) bool {
	for _, c := range checks {
		if c.Status == StatusFail {
			return false
		}
	}
	return true
}

// WriteMarker writes the .preflight_ok marker file when all checks pass or warn.
func WriteMarker(root paths.Root) error {
	marker := root.MarkerFile(".preflight_ok")
	if err := os.WriteFile(marker, []byte("ok\n"), 0o644); err != nil {
		return fmt.Errorf("writing preflight marker: %w", err)
	}
	return nil
}

// --- individual check implementations ---

func checkSpacesInPath(root paths.Root) Check {
	if strings.Contains(root.Dir(), " ") {
		return Check{
			ID:      "SpacesInPath",
			Status:  StatusFail,
			Message: fmt.Sprintf("installation path contains a space: %q — conda will fail; move to a path without spaces", root.Dir()),
		}
	}
	return Check{ID: "SpacesInPath", Status: StatusPass}
}

func checkSystem32InPath(root paths.Root) Check {
	lower := strings.ToLower(root.Dir())
	if strings.Contains(lower, `windows\system32`) || strings.Contains(lower, "windows/system32") {
		return Check{
			ID:      "System32InPath",
			Status:  StatusFail,
			Message: `installation path is inside C:\Windows\System32; move the folder`,
		}
	}
	return Check{ID: "System32InPath", Status: StatusPass}
}

func checkExFAT(ctx context.Context, root paths.Root, runner Runner) (Check, error) {
	drive := driveLetter(root.Dir())
	if drive == "" {
		return Check{ID: "ExFAT", Status: StatusPass}, nil
	}
	stdout, _, _, err := runner(ctx, "powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`(Get-Volume -DriveLetter '%s' -ErrorAction SilentlyContinue).FileSystem`, drive))
	if err != nil {
		return Check{}, err
	}
	fs := strings.TrimSpace(strings.ToLower(stdout))
	if fs == "exfat" {
		return Check{
			ID:      "ExFAT",
			Status:  StatusFail,
			Message: fmt.Sprintf("drive %s: uses exFAT — conda cannot install on exFAT; use an NTFS drive", drive),
		}, nil
	}
	return Check{ID: "ExFAT", Status: StatusPass}, nil
}

func checkLongPaths(ctx context.Context, root paths.Root, runner Runner) (Check, error) {
	stdout, _, _, err := runner(ctx, "powershell", "-NoProfile", "-Command",
		`(Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem' -Name 'LongPathsEnabled' -ErrorAction SilentlyContinue).LongPathsEnabled`)
	if err != nil {
		return Check{}, err
	}
	if strings.TrimSpace(stdout) != "1" {
		regPath := filepath.Join(root.InstallerScripts(), "enable_long_paths.reg")
		return Check{
			ID:        "LongPaths",
			Status:    StatusWarn,
			Message:   fmt.Sprintf("Windows long paths not enabled; to fix, run as admin: reg import %q then restart", regPath),
			FixAction: "FixLongPaths",
		}, nil
	}
	return Check{ID: "LongPaths", Status: StatusPass}, nil
}

func checkConflictingConda(ctx context.Context, root paths.Root, runner Runner) (Check, error) {
	stdout, _, exitCode, err := runner(ctx, "where", "conda")
	if err != nil {
		return Check{}, err
	}
	if exitCode != 0 || strings.TrimSpace(stdout) == "" {
		return Check{ID: "ConflictingConda", Status: StatusPass, Message: "no system conda found"}, nil
	}
	installerPrefix := strings.ToLower(root.InstallerFiles())
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(strings.ToLower(raw))
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, installerPrefix) {
			return Check{
				ID:      "ConflictingConda",
				Status:  StatusWarn,
				Message: fmt.Sprintf("system conda found at %q (outside installer_files/) — may interfere; run 'conda deactivate' before installing", strings.TrimSpace(stdout)),
			}, nil
		}
	}
	return Check{ID: "ConflictingConda", Status: StatusPass}, nil
}

func checkMSVC(ctx context.Context, runner Runner) (Check, error) {
	const defaultVswhere = `C:\Program Files (x86)\Microsoft Visual Studio\Installer\vswhere.exe`
	// Try PATH first (allows injection in tests), then the well-known default path.
	vswhere := ""
	stdout, _, exitCode, err := runner(ctx, "where", "vswhere")
	if err != nil {
		return Check{}, err
	}
	if exitCode == 0 && strings.TrimSpace(stdout) != "" {
		first, _, _ := strings.Cut(strings.TrimSpace(stdout), "\n")
		vswhere = strings.TrimSpace(first)
	}
	if vswhere == "" {
		// Try the well-known installer path via runner (keeps the function fully testable).
		stdout2, _, exitCode2, err2 := runner(ctx, "where", "/F", defaultVswhere)
		if err2 == nil && exitCode2 == 0 && strings.TrimSpace(stdout2) != "" {
			vswhere = defaultVswhere
		} else if _, statErr := os.Stat(defaultVswhere); statErr == nil {
			// Physical file exists on this machine; note: tests should mock 'where vswhere'.
			vswhere = defaultVswhere
		}
	}
	if vswhere == "" {
		return Check{
			ID:      "MSVC",
			Status:  StatusWarn,
			Message: "vswhere not found; cannot verify Visual Studio Build Tools — some Python packages requiring C++ compilation may fail",
		}, nil
	}
	stdout, _, _, err = runner(ctx, vswhere, "-products", "*", "-format", "json")
	if err != nil {
		return Check{}, err
	}
	out := strings.TrimSpace(stdout)
	if out == "" || out == "[]" {
		// Correct winget form for winget >= 1.5: install then configure from VS installer.
		return Check{
			ID:        "MSVC",
			Status:    StatusWarn,
			Message:   "Visual Studio Build Tools (MSVC) not installed — some Python packages requiring C++ compilation may fail",
			FixAction: "FixMSVC",
		}, nil
	}
	return Check{ID: "MSVC", Status: StatusPass, Message: "Visual Studio Build Tools found"}, nil
}

func checkGCC(ctx context.Context, runner Runner) (Check, error) {
	stdout, _, exitCode, err := runner(ctx, "gcc", "--version")
	if err != nil {
		return Check{}, err
	}
	if exitCode != 0 || strings.TrimSpace(stdout) == "" {
		hint := "sudo apt update && sudo apt install build-essential"
		if runtime.GOOS == "darwin" {
			hint = "xcode-select --install"
		}
		return Check{
			ID:      "GCC",
			Status:  StatusWarn,
			Message: fmt.Sprintf("gcc not found — pip may fail for packages requiring C++ compilation; fix: %s", hint),
		}, nil
	}
	firstLine, _, _ := strings.Cut(strings.TrimSpace(stdout), "\n")
	return Check{ID: "GCC", Status: StatusPass, Message: firstLine}, nil
}

// driveLetter extracts the drive letter from a Windows path (e.g. "C:\..." → "C").
func driveLetter(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return string(p[0])
	}
	return ""
}
