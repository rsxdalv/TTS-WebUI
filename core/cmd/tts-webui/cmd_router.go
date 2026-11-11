package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"

	"github.com/rsxdalv/TTS-WebUI/core/internal/router"
)

func init() {
	register(command{
		name:    "router",
		summary: "run the reverse proxy router",
		run:     runRouter,
	})
}

func runRouter(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("router", flag.ContinueOnError)
	listen := fs.String("listen", ":7700", "listen address")
	var routes []string
	fs.Func("route", "/prefix=http://host:port (repeatable)", func(s string) error {
		routes = append(routes, s)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return err
	}

	rtr := router.New()
	for _, r := range routes {
		prefix, target, err := router.ParseRoute(r)
		if err != nil {
			return fmt.Errorf("parsing route %q: %w", r, err)
		}
		rtr.AddRoute(prefix, target)
		slog.Info("route registered", "prefix", prefix, "target", target)
	}

	if len(routes) == 0 {
		// Demo mode: proxy everything to a local Gradio instance.
		target, _ := url.Parse("http://127.0.0.1:7860")
		rtr.AddRoute("/", target)
		slog.Info("demo mode: proxying / to http://127.0.0.1:7860")
	}

	l, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", *listen, err)
	}
	defer l.Close()

	slog.Info("router listening", "addr", l.Addr())
	fmt.Println("router listening on", l.Addr())

	srv := &http.Server{Handler: rtr}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(l) }()

	select {
	case <-ctx.Done():
		srv.Shutdown(context.Background())
		return nil
	case err := <-errCh:
		return err
	}
}
