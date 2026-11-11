package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
)

func init() {
	register(command{
		name:    "gpu",
		summary: "GPU detection sub-commands (detect)",
		run:     runGPU,
	})
}

func runGPU(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gpu <subcommand>\nSubcommands: detect")
	}
	switch args[0] {
	case "detect":
		return runGPUDetect(ctx, args[1:])
	default:
		return fmt.Errorf("unknown gpu subcommand %q", args[0])
	}
}

func runGPUDetect(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("gpu detect", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print result as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := gpu.Detect(ctx)
	if err != nil {
		return fmt.Errorf("detecting GPU: %w", err)
	}

	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(struct {
			Label string `json:"label"`
			Note  string `json:"note,omitempty"`
		}{Label: string(res.Label), Note: res.Note})
	}
	fmt.Println(res.Label)
	if res.Note != "" {
		fmt.Println("note:", res.Note)
	}
	return nil
}
