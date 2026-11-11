package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/rsxdalv/TTS-WebUI/core/internal/tools"
)

func init() {
	register(command{
		name:    "tools",
		summary: "manage pinned tool binaries (uv, ffmpeg, mingit)",
		run:     runTools,
	})
}

func runTools(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tools", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root (default: binary location)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(fs.Output(), "usage: tts-webui tools <subcommand>")
		fmt.Fprintln(fs.Output(), "  ensure   download and verify all pinned tools")
		return nil
	}

	switch fs.Arg(0) {
	case "ensure":
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		fmt.Fprintln(fs.Output(), "ensuring pinned tools...")
		if err := tools.EnsureAll(ctx, root); err != nil {
			return fmt.Errorf("tools ensure: %w", err)
		}
		fmt.Fprintln(fs.Output(), "done")
	default:
		return fmt.Errorf("unknown tools subcommand %q", fs.Arg(0))
	}
	return nil
}
