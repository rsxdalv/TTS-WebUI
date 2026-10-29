package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

func init() {
	register(command{
		name:    "engine",
		summary: "manage TTS engine manifests",
		run:     runEngine,
	})
}

func runEngine(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("engine", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root (default: binary location)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(fs.Output(), "usage: tts-webui engine <subcommand>")
		fmt.Fprintln(fs.Output(), "  list   print engine manifests as JSON")
		return nil
	}

	switch fs.Arg(0) {
	case "list":
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		manifests, err := manifest.Load(root.Dir())
		if err != nil {
			return fmt.Errorf("loading manifests: %w", err)
		}
		enc := json.NewEncoder(fs.Output())
		enc.SetIndent("", "  ")
		return enc.Encode(manifests)
	default:
		return fmt.Errorf("unknown engine subcommand %q", fs.Arg(0))
	}
}
