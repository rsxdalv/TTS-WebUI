package router

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

// DefaultHost is where the router binds when no host is given.
const DefaultHost = "127.0.0.1"

// BindHost returns the interface the public router must bind for a given
// Gradio server_name. The router is the real front door once it is enabled, so
// it must never be more exposed than the Gradio server the user configured
// (see gradio_proxy_tree/main.py): an empty server_name falls back to loopback,
// never to all interfaces.
func BindHost(serverName string) string {
	if serverName == "" {
		return DefaultHost
	}
	return serverName
}

// Listen opens a TCP listener for the router. host "" means [DefaultHost];
// port 0 picks a free port (read it back with ln.Addr()).
func Listen(host string, port int) (net.Listener, error) {
	return net.Listen("tcp", net.JoinHostPort(BindHost(host), strconv.Itoa(port)))
}

// Serve listens on host:port and runs the proxy until ctx is cancelled. It
// returns nil on a clean shutdown. See [Router.ServeListener].
func (rt *Router) Serve(ctx context.Context, host string, port int) error {
	ln, err := Listen(host, port)
	if err != nil {
		return err
	}
	return rt.ServeListener(ctx, ln)
}

// ServeListener runs the proxy on ln until ctx is cancelled, then shuts down:
// new connections stop, in-flight requests get a short grace period, and
// whatever is still open (SSE streams, WebSockets) is cut. It owns ln.
func (rt *Router) ServeListener(ctx context.Context, ln net.Listener) error {
	// Every request context derives from connCtx so that cancelling it also
	// ends upgraded (WebSocket) tunnels, which http.Server.Shutdown ignores.
	connCtx, cancelConns := context.WithCancel(context.Background())
	defer cancelConns()

	srv := &http.Server{
		Handler: rt,
		// Header timeout only: bodies and responses (SSE) can run for hours.
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		BaseContext:       func(net.Listener) context.Context { return connCtx },
		ErrorLog:          slog.NewLogLogger(rt.log.Handler(), slog.LevelWarn),
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		// Serve failed on its own (listener broke).
		rt.closeIdle()
		return err
	case <-ctx.Done():
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := srv.Shutdown(shutCtx)
	cancelConns()
	_ = srv.Close()
	rt.closeIdle()
	if serveErr := <-errCh; serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}
