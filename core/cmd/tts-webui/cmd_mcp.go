package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rsxdalv/TTS-WebUI/core/internal/mcp"
)

// runMCP starts the MCP server: stdio by default, Streamable HTTP with --http.
// Stdout carries the protocol in stdio mode, so every message goes to stderr.
func runMCP(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tts-webui mcp", flag.ContinueOnError)
	backend := fs.String("backend", os.Getenv("TTS_MCP_BACKEND"), "OpenAI-compatible API base URL (default "+mcp.DefaultBackend+")")
	httpAddr := fs.String("http", "", "serve Streamable HTTP on this loopback address instead of stdio, e.g. 127.0.0.1:7779")
	tokenEnv := fs.String("token-env", "TTS_MCP_TOKEN", "name of the environment variable holding the HTTP bearer token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	b, err := mcp.NewBackend(*backend)
	if err != nil {
		return err
	}
	srv := mcp.New(b)

	if *httpAddr == "" {
		err := srv.Run(ctx, &sdk.StdioTransport{})
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	token := os.Getenv(*tokenEnv)
	if err := mcp.CheckListen(*httpAddr, token); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		return err
	}
	hs := &http.Server{Handler: mcp.Handler(srv, token), ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintf(os.Stderr, "tts-webui mcp: listening on http://%s (bearer token required)\n", ln.Addr())
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(shutdown)
	}()
	if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
