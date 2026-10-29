package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/rsxdalv/TTS-WebUI/core/internal/env"
)

func init() {
	register(command{
		name:    "env",
		summary: "manage Python virtual environments",
		run:     runEnv,
	})
}

func runEnv(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	rootDir := fs.String("root", "", "install root (default: binary location)")
	gpu := fs.String("gpu", "", `GPU label: "NVIDIA GPU", "AMD GPU (ROCM, Linux only)", "Apple M Series Chip", "Intel GPU (XPU)", "CPU", "Custom (Manual torch install)"`)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(fs.Output(), "usage: tts-webui env <subcommand>")
		fmt.Fprintln(fs.Output(), "  create main   install Python 3.10.11 and create main venv")
		return nil
	}

	switch fs.Arg(0) {
	case "create":
		if fs.NArg() < 2 || fs.Arg(1) != "main" {
			return fmt.Errorf("usage: tts-webui env create main [--gpu <label>]")
		}
		root, err := resolveRoot(*rootDir)
		if err != nil {
			return err
		}
		return env.CreateMain(ctx, root, *gpu)
	default:
		return fmt.Errorf("unknown env subcommand %q", fs.Arg(0))
	}
}
