package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/rsxdalv/TTS-WebUI/core/internal/app"
)

func init() {
	register(command{
		name:    "app",
		summary: "manage the app package (install, update, version)",
		run:     runApp,
	})
}

func runApp(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: tts-webui app <subcommand> [flags]")
		fmt.Fprintln(fs.Output(), "")
		fmt.Fprintln(fs.Output(), "subcommands:")
		fmt.Fprintln(fs.Output(), "  update    install or upgrade the tts-webui Python package via uv")
		fmt.Fprintln(fs.Output(), "  version   print the currently installed app version")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return nil
	}

	switch fs.Arg(0) {
	case "update":
		return runAppUpdate(ctx, fs.Args()[1:])
	case "version":
		return runAppVersion(ctx, fs.Args()[1:])
	default:
		return fmt.Errorf("unknown app subcommand %q", fs.Arg(0))
	}
}

func runAppUpdate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("app update", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root (default: binary location)")
	gpu := fs.String("gpu", "", `GPU label: "NVIDIA GPU", "AMD GPU (ROCM, Linux only)", "Apple M Series Chip", "Intel GPU (XPU)", "CPU", "Custom (Manual torch install)"`)
	checkOnly := fs.Bool("check-only", false, "only check whether an update is available; exit 1 if one is")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root, err := resolveRoot(*rootDir)
	if err != nil {
		return err
	}

	current, err := app.CurrentVersion(root)
	if err != nil {
		return fmt.Errorf("reading current version: %w", err)
	}

	latest, err := app.LatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("fetching latest version: %w", err)
	}

	if current == latest {
		fmt.Fprintf(fs.Output(), "tts-webui is up to date (%s)\n", current)
		return nil
	}

	if current == "" {
		fmt.Fprintf(fs.Output(), "tts-webui not installed; latest available: %s\n", latest)
	} else {
		fmt.Fprintf(fs.Output(), "update available: %s → %s\n", current, latest)
	}

	if app.IsMigratingFromGit(root) {
		fmt.Fprintln(fs.Output(), "note: migrating from git-based install to uv package")
	}

	if *checkOnly {
		// Return ErrUpdateAvailable so callers can detect this with errors.Is.
		return fmt.Errorf("%w (%s)", app.ErrUpdateAvailable, latest)
	}

	fmt.Fprintln(fs.Output(), "installing…")
	// Pass the already-fetched version to avoid a second PyPI round-trip.
	if err := app.UpdateTo(ctx, root, *gpu, latest); err != nil {
		return err
	}
	fmt.Fprintf(fs.Output(), "tts-webui %s installed\n", latest)
	return nil
}

func runAppVersion(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("app version", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root (default: binary location)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = ctx

	root, err := resolveRoot(*rootDir)
	if err != nil {
		return err
	}

	ver, err := app.CurrentVersion(root)
	if err != nil {
		return err
	}
	if ver == "" {
		fmt.Fprintln(fs.Output(), "(not installed)")
		return nil
	}
	fmt.Fprintln(fs.Output(), ver)
	return nil
}
