package main

import (
	"context"
	"fmt"

	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

func init() {
	register(command{
		name:    "run",
		summary: "run a command under the process supervisor",
		run:     runRun,
	})
}

func runRun(ctx context.Context, args []string) error {
	// Syntax: run [--] <command> [args...]
	sepIdx := -1
	for i, a := range args {
		if a == "--" {
			sepIdx = i
			break
		}
	}

	var cmdArgs []string
	if sepIdx >= 0 {
		cmdArgs = args[sepIdx+1:]
	} else {
		cmdArgs = args
	}
	if len(cmdArgs) == 0 {
		return fmt.Errorf("usage: run [--] <command> [args...]")
	}

	p := &procs.Proc{}
	sub, unsub := p.Subscribe()
	defer unsub()

	// Stream output to stdout.
	go func() {
		for line := range sub {
			fmt.Println(line)
		}
	}()

	if err := p.Start(ctx, cmdArgs[0], cmdArgs[1:], "", nil); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return p.Stop()
	}
}
