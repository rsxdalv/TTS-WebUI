package launcher

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extprocs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/history"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/router"
)

// EnvServerPy turns on the Go side of the server.py port
// (core/internal/serverpy): with "1" the app runs server_core.py behind the Go
// router instead of server.py with its Python proxy tree, and Go runs the proxy
// extensions. Unset, "" or "0" keeps server.py as before.
const EnvServerPy = "TTS_GO_SERVERPY"

// serverPyEnabled reads EnvServerPy. Any other value is an error, so a typo
// never silently runs the other code path.
func serverPyEnabled(lookup func(string) (string, bool)) (bool, error) {
	v, _ := lookup(EnvServerPy)
	switch strings.TrimSpace(v) {
	case "", "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be 0 or 1, got %q", EnvServerPy, v)
	}
}

// serverPyEntry is the Gradio entry script when the port is on. It installs
// the Python bridges (routes and extension processes go to Go), then runs
// server.py unchanged.
const serverPyEntry = "server_core.py"

// serverPy is the Go side of one app run: the router on the public port (the
// proxy tree's job), the extension manager, and the control API both of them
// serve to the gradio child.
type serverPy struct {
	rt  *router.Router
	mgr *extprocs.Manager
	ctl *http.Server

	// publicAddr is where the router listens (the address users open).
	publicAddr string
	// gradioEnv goes to the gradio child only: it carries the token. It is
	// the loaded .env environment followed by the Go-core variables.
	gradioEnv []string

	stopRouter context.CancelFunc
	routerDone chan struct{}
	closeOnce  sync.Once
}

// startServerPy reads the public host and port the way server.py does
// (config.json, .env, .env.user) and starts the Go side on them. prefix is the
// `micromamba -p <env> run` command that runs the main env's python; baseEnv is
// the environment the launcher gives every child. The environment the children
// get is baseEnv plus .env and .env.user.
func startServerPy(root paths.Root, prefix, baseEnv []string) (*serverPy, error) {
	// Startup prep that server.py does in-process: output folders, a default
	// .env when missing, then .env and .env.user on top of the launcher env.
	if err := appenv.EnsureFolders(root.Dir()); err != nil {
		return nil, fmt.Errorf("creating outputs/favorites: %w", err)
	}
	if created, err := appenv.EnsureEnv(root.Dir(), baseEnv); err != nil {
		return nil, fmt.Errorf("writing .env: %w", err)
	} else if created {
		slog.Info("serverpy: .env not found, created the default")
	}
	rep, err := appenv.LoadEnvReport(root.Dir(), baseEnv)
	if err != nil {
		return nil, fmt.Errorf("reading .env: %w", err)
	}
	for _, w := range rep.Warnings {
		slog.Warn("serverpy: " + w)
	}
	// The launcher runs React itself, so server.py never sees it.
	plan, err := appenv.Plan(root.Dir(), rep.Env, appenv.Flags{NoReact: true})
	if err != nil {
		return nil, err
	}
	// rep.Env (launcher env + .env + .env.user) is what the children run with.
	return startServerPyOn(root, plan.Host, plan.PublicPort, prefix, rep.Env)
}

// startServerPyOn starts the router on host:publicPort, routing / to Gradio on
// a free loopback port, and the control API on another loopback port.
func startServerPyOn(root paths.Root, host string, publicPort int, prefix, baseEnv []string) (*serverPy, error) {
	gradioPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("allocating the Gradio port: %w", err)
	}
	token, err := newCoreToken()
	if err != nil {
		return nil, err
	}

	rt := router.New(router.Options{Token: token})
	if err := rt.AddRoute("/", "http://127.0.0.1:"+strconv.Itoa(gradioPort)); err != nil {
		return nil, err
	}
	// Bind before any child starts, so a running app on the same port is an
	// error here, not a crashed child later.
	pub, err := router.Listen(host, publicPort)
	if err != nil {
		return nil, fmt.Errorf("port %d is in use (is another TTS WebUI running?): %w", publicPort, err)
	}
	ctlLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = pub.Close()
		return nil, fmt.Errorf("control API: %w", err)
	}

	mgr := extprocs.New(extprocs.Options{
		Root:       root,
		MainPython: append(append([]string(nil), prefix...), "python"),
		BaseEnv:    append([]string(nil), baseEnv...),
		Routes:     rt,
		Token:      token,
		Logf:       func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	})

	mux := http.NewServeMux()
	mux.Handle("/api/routes", rt.ControlHandler())
	// Go is the only reader and writer of config.json (see appconfig).
	mux.Handle(appconfig.Path, appconfig.New(appconfig.Options{
		Root: root.Dir(), Token: token,
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	}).Handler())
	// Gallery History tab: Go does the filesystem work (see history).
	mux.Handle(history.Prefix, history.New(history.Options{
		Root: root.Dir(), Token: token,
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	}).Handler())
	mux.Handle("/api/extensions", mgr.Handler())
	mux.Handle("/api/extensions/", mgr.Handler())
	ctl := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	sp := &serverPy{
		rt:         rt,
		mgr:        mgr,
		ctl:        ctl,
		publicAddr: pub.Addr().String(),
		gradioEnv: append(append([]string(nil), baseEnv...),
			"TTS_CORE_URL=http://"+ctlLn.Addr().String(),
			"TTS_CORE_TOKEN="+token,
			"TTS_GRADIO_PORT="+strconv.Itoa(gradioPort),
			// Gradio stays on loopback; the router is the front door.
			"TTS_GRADIO_HOST=127.0.0.1",
		),
		stopRouter: cancel,
		routerDone: make(chan struct{}),
	}
	go func() {
		defer close(sp.routerDone)
		if err := rt.ServeListener(ctx, pub); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("serverpy router: " + err.Error())
		}
	}()
	go func() { _ = ctl.Serve(ctlLn) }()
	slog.Info(fmt.Sprintf("serverpy: router on %s -> gradio 127.0.0.1:%d", sp.publicAddr, gradioPort))
	return sp, nil
}

// Close stops the extensions Go started, then the router and the control API.
// It is safe to call more than once and from several goroutines.
func (sp *serverPy) Close() {
	sp.closeOnce.Do(func() {
		sp.mgr.StopAll()
		sp.stopRouter()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sp.ctl.Shutdown(ctx)
		<-sp.routerDone
	})
}

// newCoreToken is the shared secret between Go and the gradio child.
func newCoreToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating the core token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
