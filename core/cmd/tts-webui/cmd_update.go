package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/rsxdalv/TTS-WebUI/core/internal/selfupdate"
	"github.com/rsxdalv/TTS-WebUI/core/internal/version"
)

func init() {
	register(command{
		name:    "update",
		summary: "check for and apply a core binary update",
		run:     runUpdate,
	})
}

func runUpdate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	checkOnly := fs.Bool("check-only", false, "only check for update, do not apply")
	if err := fs.Parse(args); err != nil {
		return err
	}

	currentVersion := version.Version

	rel, hasUpdate, err := selfupdate.Check(ctx, currentVersion)
	if err != nil {
		return fmt.Errorf("checking for update: %w", err)
	}

	if !hasUpdate {
		fmt.Fprintln(fs.Output(), "already up to date")
		return nil
	}

	fmt.Fprintf(fs.Output(), "update available: %s → %s\n", currentVersion, rel.Version)

	if *checkOnly {
		return nil
	}

	fmt.Fprintln(fs.Output(), "applying update…")
	if err := selfupdate.Apply(ctx, rel, ""); err != nil {
		return fmt.Errorf("applying update: %w", err)
	}
	fmt.Fprintln(fs.Output(), "update applied; please restart tts-webui")
	return nil
}
