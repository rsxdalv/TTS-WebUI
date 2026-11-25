// Package platform is the seam between the core and the runtime an entry runs
// on (PLATFORM-DESIGN.md). A platform answers four questions about the
// runtime (status, install, repair, uninstall) and four about one entry on it
// (create, status, remove, command). Every other package asks a Platform for a
// command instead of building the main env's command line itself.
//
// P1 has two implementations, both adopted rather than installed: legacy-env
// (the existing main env) and system (the Python on PATH).
package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Emit receives one output line of an install, a removal or a check.
type Emit func(line string)

// ErrNotInstallable is what Install, Repair and Uninstall answer for a platform
// that is adopted: legacy-env and system exist outside this app.
var ErrNotInstallable = errors.New("platform is managed outside this app")

// Status is the state of a platform's runtime. Ready means a Command can run;
// Detail is what the user is told when it is not.
type Status struct {
	Ready  bool
	Detail string
}

// Entry is what a platform needs to know about one catalog entry.
type Entry struct {
	Package      string   // package_name
	Name         string   // display name, for messages ("" = Package)
	Own          bool     // runs in its own venv (proxy: native); else in the main env
	Requirements []string // already validated requirement tokens
	Python       string   // Python version of an own-venv entry ("" = the platform's pin)
	TorchRow     string   // torch table row of an own-venv entry ("" = the default row of the GPU choice)
}

// Label is the name shown in messages.
func (e Entry) Label() string {
	if e.Name != "" {
		return e.Name
	}
	return e.Package
}

// EntryStatus is whether an entry is installed on a platform.
type EntryStatus struct {
	Installed bool
	Partial   bool   // a directory is there but the install did not finish (own-venv)
	Version   string // the pip distribution's version; "" for an own-venv or built-in entry
}

// Command is a resolved process: the program, its arguments, the complete
// environment and the directory. No shell, no prefix.
type Command struct {
	Program string
	Args    []string
	Env     []string
	Dir     string
}

// Platform is a shared runtime.
type Platform interface {
	Name() string
	// Provides lists the names an entry may require that this platform
	// satisfies when its Status is Ready.
	Provides() []string

	Status(ctx context.Context) Status
	Install(ctx context.Context, emit Emit) error
	Repair(ctx context.Context, emit Emit) error
	Uninstall(ctx context.Context, emit Emit) error

	// Create installs an entry; it reports success by the returned bool and
	// says what happened through emit.
	Create(ctx context.Context, e Entry, emit Emit) bool
	// Check says what Create would do for an in-process entry, without doing it
	// (pip --dry-run), through emit.
	Check(ctx context.Context, e Entry, emit Emit) bool
	EntryStatus(ctx context.Context, e Entry) EntryStatus
	Remove(ctx context.Context, e Entry, emit Emit) bool
	// WithEnv returns a copy whose commands also carry env (KEY=VALUE, after
	// everything else, so it wins).
	WithEnv(env []string) Platform
	// Command resolves argv, whose first element is a logical program
	// ("python", "uv", "npm"), for entry e. e may be nil for the main env.
	Command(e *Entry, argv []string) (Command, error)
}

// VenvDir is the directory of an own-venv entry.
func VenvDir(root, pkg string) string { return filepath.Join(root, ".venvs", pkg) }

// VenvPython is the python of an own-venv entry.
func VenvPython(root, pkg string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(VenvDir(root, pkg), "Scripts", "python.exe")
	}
	return filepath.Join(VenvDir(root, pkg), "bin", "python")
}

// OwnVenvExists reports whether the entry's python is there (the check
// extprocs makes before it starts an engine).
func OwnVenvExists(root, pkg string) bool {
	st, err := os.Stat(VenvPython(root, pkg))
	return err == nil && !st.IsDir()
}

// Builtin reports whether a package name is a built-in extension.
func Builtin(pkg string) bool { return strings.Contains(pkg, "builtin") }

// OwnVenvCommand resolves "python" of an own-venv entry to the venv's python,
// run directly (no shell, no prefix). ok is false for anything else, which is
// then the platform's own business.
func OwnVenvCommand(root string, e *Entry, argv []string, env []string) (c Command, ok bool) {
	if e == nil || !e.Own || len(argv) == 0 || argv[0] != "python" || !OwnVenvExists(root, e.Package) {
		return Command{}, false
	}
	return Command{Program: VenvPython(root, e.Package), Args: append([]string(nil), argv[1:]...), Env: env, Dir: root}, true
}

// Files an own-venv install leaves in .venvs/<package>/ (the uv platform writes
// them): EntryFile when it is done, InstallingFile while it is not.
const (
	EntryFile      = "entry.json"
	InstallingFile = "installing.json"
)

// OwnVenvStatus is the state of an own-venv entry on disk: installed when
// entry.json is there. A venv an older install made has neither file and still
// has its python: it is installed too (an existing install is adopted, not
// migrated). A directory with the install marker, or with no python, is
// partial; no directory is absent.
func OwnVenvStatus(root, pkg string) EntryStatus {
	dir := VenvDir(root, pkg)
	if !dirExists(dir) {
		return EntryStatus{}
	}
	if _, err := os.Stat(filepath.Join(dir, EntryFile)); err == nil {
		return EntryStatus{Installed: true}
	}
	if _, err := os.Stat(filepath.Join(dir, InstallingFile)); err == nil || !OwnVenvExists(root, pkg) {
		return EntryStatus{Partial: true}
	}
	return EntryStatus{Installed: true}
}
