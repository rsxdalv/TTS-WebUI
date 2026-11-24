package platform

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// GradioPin is installed with every in-process extension so pip cannot move
// Gradio. The pin moves to the gradio-host entry in P3.
const GradioPin = "gradio==5.49.1"

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
		return OwnVenvStatus(root, e.Package)
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
	// Constraints is a pip constraints file passed to every install when it
	// exists (the Gradio host holds torch with it).
	Constraints string
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

// Create installs an in-process entry: pip into the main env. An own-venv
// entry belongs to the uv platform.
func (x Entries) Create(ctx context.Context, e Entry, emit Emit) bool {
	if e.Own {
		emit(fmt.Sprintf("Failed: %s runs in its own venv, which the uv platform makes", e.Label()))
		return false
	}
	return x.PipInstall(ctx, e, false, emit)
}

// Check is the dependency check of an in-process entry.
func (x Entries) Check(ctx context.Context, e Entry, emit Emit) bool {
	return x.PipInstall(ctx, e, true, emit)
}

// Remove uninstalls the entry's package from the main env.
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
	if x.Constraints != "" {
		if _, err := os.Stat(x.Constraints); err == nil {
			args = append(args, "-c", x.Constraints)
		}
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
