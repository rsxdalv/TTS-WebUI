package platform

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// GradioPin is installed with every in-process extension so pip cannot move
// Gradio. The pin moves to the gradio-host entry in P3.
const GradioPin = "gradio==5.49.1"

const compatibility = `gradio<=5.49.1 gradio-goodtabs>=0.0.5 gradio-goodtab>=0.0.5 gradio-iconbutton>=0.0.1 ffmpeg-python==0.2.0 matplotlib`

var pkgDir = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var nonAlnum = regexp.MustCompile(`[-_.]+`)

// NormalizeDist is pip's name normalisation (PEP 503, with "_" as the
// separator), enough to compare a distribution with a package name.
func NormalizeDist(s string) string {
	return nonAlnum.ReplaceAllString(strings.ToLower(s), "_")
}

// InstalledIn answers for one entry: an own-venv entry is installed when its
// venv directory exists (it has no version here), a built-in one always is,
// any other when the main env has the distribution. dists maps normalised
// distribution names to versions.
func InstalledIn(root string, e Entry, dists map[string]string) EntryStatus {
	switch {
	case e.Own:
		return EntryStatus{Installed: dirExists(VenvDir(root, e.Package))}
	case Builtin(e.Package):
		return EntryStatus{Installed: true}
	}
	v, ok := dists[NormalizeDist(e.Package)]
	return EntryStatus{Installed: ok, Version: v}
}

// Entries implements the entry level of Platform on top of a Command
// function, so every platform that runs the main env's python and uv the
// same way shares one copy of the install logic.
type Entries struct {
	Root string
	// Cmd resolves a command (Platform.Command).
	Cmd func(e *Entry, argv []string) (Command, error)
	// Packages returns the main env's distributions (normalised name ->
	// version). nil means none are known.
	Packages func(ctx context.Context) (map[string]string, error)
}

// run runs argv through the platform in the install folder.
func (x Entries) run(ctx context.Context, argv []string, emit Emit) error {
	c, err := x.Cmd(nil, argv)
	if err != nil {
		return err
	}
	c.Dir = x.Root
	return Run(ctx, c, emit)
}

// EntryStatus is InstalledIn with the main env's packages.
func (x Entries) EntryStatus(ctx context.Context, e Entry) EntryStatus {
	var dists map[string]string
	if !e.Own && !Builtin(e.Package) && x.Packages != nil {
		dists, _ = x.Packages(ctx)
	}
	return InstalledIn(x.Root, e, dists)
}

// Create installs an entry: pip into the main env, or an own venv.
func (x Entries) Create(ctx context.Context, e Entry, emit Emit) bool {
	if e.Own {
		return x.venvSetup(ctx, e, emit)
	}
	return x.PipInstall(ctx, e, false, emit)
}

// Remove uninstalls the entry's package from the main env. An own-venv
// entry's directory is left (P2 removes it).
func (x Entries) Remove(ctx context.Context, e Entry, emit Emit) bool {
	ok := x.run(ctx, []string{"python", "-m", "pip", "uninstall", "-y", e.Package}, emit) == nil
	if ok {
		emit(fmt.Sprintf("Successfully uninstalled %s (%s)", e.Package, e.Package))
	} else {
		emit(fmt.Sprintf("Failed to uninstall %s (%s)", e.Package, e.Package))
	}
	return ok
}

// PipInstall is pip_install_wrapper: pip install <requirements> gradio==5.49.1
// in the main env. With dryRun it is the dependency check (--dry-run).
func (x Entries) PipInstall(ctx context.Context, e Entry, dryRun bool, emit Emit) bool {
	args := []string{"python", "-m", "pip", "install"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	args = append(append(args, e.Requirements...), GradioPin)
	ok := x.run(ctx, args, emit) == nil
	if ok {
		emit(fmt.Sprintf("Successfully installed %s dependencies", e.Label()))
		emit("Please restart the webui to see the changes")
	} else {
		emit(fmt.Sprintf("Failed to install %s dependencies", e.Label()))
	}
	return ok
}

// venvSetup is venv_setup_wrapper: .venvs/<package> with uv, torch for the
// chosen GPU, torchcodec, xformers, then the requirements.
func (x Entries) venvSetup(ctx context.Context, e Entry, emit Emit) bool {
	emit(fmt.Sprintf("Setting up virtual environment and installing dependencies for %s...", e.Label()))
	fail := func(format string, a ...any) bool {
		emit(fmt.Sprintf(format, a...))
		return false
	}
	if !pkgDir.MatchString(e.Package) {
		return fail("Failed: %q is not a valid package name for a virtual environment", e.Package)
	}
	if len(e.Requirements) == 0 {
		return fail("Failed: bad requirements for %s", e.Label())
	}
	venv := VenvDir(x.Root, e.Package)
	uvPip := []string{"uv", "pip", "install", "--python", VenvPython(x.Root, e.Package)}
	steps := [][]string{
		{"uv", "venv", venv, "--allow-existing"},
		append(append([]string{}, uvPip...), e.Torch...),
		append(append([]string{}, uvPip...), "torchcodec", "--index-url=https://download.pytorch.org/whl/cpu"),
		append(append([]string{}, uvPip...), "xformers==0.0.35"),
		append(append(append([]string{}, uvPip...), e.Requirements...), strings.Fields(compatibility)...),
	}
	for _, args := range steps {
		if err := x.run(ctx, args, emit); err != nil {
			return fail("Failed to run %s: %v", strings.Join(args[:min(3, len(args))], " "), err)
		}
	}
	emit(fmt.Sprintf("Successfully set up virtual environment for %s with dependencies", e.Label()))
	return true
}
