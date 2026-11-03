package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"text/tabwriter"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/preflight"
)

func init() {
	register(command{
		name:    "preflight",
		summary: "run system pre-flight checks",
		run:     runPreflight,
	})
}

func runPreflight(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit results as JSON")
	rootDir := fs.String("root", "", "install root (default: binary location)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root, err := resolveRoot(*rootDir)
	if err != nil {
		return err
	}

	checks, err := preflight.Run(ctx, root, nil)
	if err != nil {
		return fmt.Errorf("running preflight: %w", err)
	}

	if *asJSON {
		enc := json.NewEncoder(fs.Output())
		enc.SetIndent("", "  ")
		return enc.Encode(checks)
	}

	w := tabwriter.NewWriter(fs.Output(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tMESSAGE")
	for _, c := range checks {
		fix := ""
		if c.FixAction != "" {
			fix = " [fix: " + c.FixAction + "]"
		}
		fmt.Fprintf(w, "%s\t%s\t%s%s\n", c.ID, c.Status, c.Message, fix)
	}
	w.Flush()

	if preflight.AllPassed(checks) {
		if err := preflight.WriteMarker(root); err != nil {
			slog.Warn("could not write preflight marker", "err", err)
		}
	}

	// Exit 1 if any check failed.
	for _, c := range checks {
		if c.Status == preflight.StatusFail {
			return fmt.Errorf("preflight failed")
		}
	}
	return nil
}

// resolveRoot returns a Root from --root or the binary location.
func resolveRoot(rootDir string) (paths.Root, error) {
	if rootDir != "" {
		return paths.New(rootDir)
	}
	return paths.FromBinary()
}
