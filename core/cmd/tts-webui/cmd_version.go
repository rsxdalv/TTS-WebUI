package main

import (
	"context"
	"fmt"

	"github.com/rsxdalv/TTS-WebUI/core/internal/version"
)

func init() {
	register(command{
		name:    "version",
		summary: "print version information",
		run: func(ctx context.Context, args []string) error {
			fmt.Printf("tts-webui %s\n", version.String())
			return nil
		},
	})
}
