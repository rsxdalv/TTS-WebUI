package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"

	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

func init() {
	register(command{
		name:    "serve",
		summary: "start the local HTTP server",
		run:     runServe,
	})
}

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 7700, "port to listen on (default: D-open-1)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	srv, err := server.New(*port)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}

	l, err := srv.Listen()
	if err != nil {
		return fmt.Errorf("listening on %s: %w", srv.Addr(), err)
	}
	defer l.Close()

	slog.Info("server started", "url", "http://"+srv.Addr())
	fmt.Println("http://" + srv.Addr())

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(l) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}
