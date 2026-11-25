package launcher

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/audiocpp"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/detect"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/uv"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/engines"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extprocs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extui"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/history"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/launchopts"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/logview"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/piplist"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/router"
)

// serverPyEntry is the Gradio entry script. It installs the Python config
// bridge, then runs the thin Gradio entry (tts_webui.core_server).
const serverPyEntry = "server_core.py"

// gradioHost is where Gradio itself binds: loopback, behind the router, which
// is the only thing that faces the network.
const gradioHost = "127.0.0.1"

// serverPy is the Go side of one app run: the router on the public port (the
// proxy tree's job), the extension processes, and the control API both of them
// serve to the gradio child.
type serverPy struct {
	root string // install folder, the key in runningEngines
	rt   *router.Router
	mgr  *extprocs.Manager
	ctl  *http.Server
	ui   *http.Server

	// publicAddr is where the router listens (the address users open).
	publicAddr string
	// gradioEnv goes to the gradio child only: it carries the token. It is
	// the loaded .env environment followed by the Go-core variables.
	gradioEnv          []string
	coreURL, coreToken string

	stopRouter context.CancelFunc
	routerDone chan struct{}
	closeOnce  sync.Once
}

// startServerPy reads the public host and port the way server.py does
// (config.json, .env, .env.user) and starts the Go side on them. p is the
// platform the children run on; its command environment is what the launcher
// gives every child. The environment the children get is that plus .env and
// .env.user.
func startServerPy(root paths.Root, p platform.Platform, flags appenv.Flags) (*serverPy, error) {
	baseEnv := commandEnv(p)
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
	plan, err := appenv.Plan(root.Dir(), rep.Env, flags)
	if err != nil {
		return nil, err
	}
	// rep.Env (launcher env + .env + .env.user) is what the children run with.
	return startServerPyOn(root, plan.Host, plan.PublicPort, p.WithEnv(rep.Env), flags)
}

// startServerPyOn starts the router on host:publicPort, routing / to Gradio on
// a free loopback port, and the control API on another loopback port.
func startServerPyOn(root paths.Root, host string, publicPort int, p platform.Platform, flags appenv.Flags) (*serverPy, error) {
	gradioPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("allocating the Gradio port: %w", err)
	}
	token, err := newCoreToken()
	if err != nil {
		return nil, err
	}

	// The public port's "/" is Gradio. Without the Gradio host there is nothing
	// to send it to: it answers a page that says so (not a 502) and points at the
	// Vite UI. The port stays open: engine pages and the extension pages use it.
	st := p.Status(context.Background())
	noHost := detect.Host(p) != nil && !st.Ready
	var home http.Handler
	if noHost {
		home = noHostPage(st.Detail)
	}
	rt := router.New(router.Options{Token: token, Home: home})
	if !noHost {
		if err := rt.AddRoute("/", "http://"+net.JoinHostPort(gradioHost, strconv.Itoa(gradioPort))); err != nil {
			return nil, err
		}
	}
	// Bind before any child starts, so a running app on the same port is an
	// error here, not a crashed child later.
	pub, err := router.Listen(host, publicPort)
	if err != nil {
		return nil, fmt.Errorf("port %d is in use (is another TTS WebUI running?): %w", publicPort, err)
	}
	// ssl_certfile and ssl_keyfile of config.json: the router serves HTTPS.
	if cfg, err := appenv.LoadConfig(root.Dir()); err == nil {
		tlsLn, err := tlsListener(pub, cfg.SSL)
		if err != nil {
			_ = pub.Close()
			return nil, err
		}
		pub = tlsLn
	}
	ctlLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = pub.Close()
		return nil, fmt.Errorf("control API: %w", err)
	}

	// Go is the only reader and writer of config.json (see appconfig).
	// One Store serves the document and the single-setting operations, so every
	// change to the file goes through the same mutex.
	cfg := appconfig.New(appconfig.Options{
		Root: root.Dir(), Token: token,
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	})
	// Extension lists, which extensions load, pip/uv installs (see extcatalog).
	pip := piplist.New(piplist.Options{
		Root: root.Dir(), Token: token,
		Platform: p,
		Logf:     func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	})
	var mgr *extprocs.Manager
	ext := extcatalog.New(extcatalog.Options{
		Root: root.Dir(), Token: token, Config: cfg, Pip: pip,
		Platform: p, UV: uv.New(uv.Options{Root: root}), AudioCpp: audiocpp.New(audiocpp.Options{Root: root}),
		Host: detect.Host(p), AppRunning: func() bool { return true },
		StopEngine: func(pkg string) bool { return mgr.StopIfRunning(pkg) },
		Logf:       func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	})

	mgr = extprocs.New(extprocs.Options{
		Root:     root,
		Platform: p,
		// Extension processes get no control token, so they receive the
		// resolved decorators instead of asking for them.
		ExtraEnv: ext.DecoratorEnv,
		Routes:   rt,
		Token:    token,
		Logf:     func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	})
	// The launcher's Extensions page removes engines too, with its own store.
	runningEngines.Store(root.Dir(), mgr)

	mux := http.NewServeMux()
	mux.Handle("/api/routes", rt.ControlHandler())
	mux.Handle(appconfig.Path, cfg.Handler())
	mux.Handle(appconfig.OpsPrefix, cfg.OpsHandler())
	// Gallery History tab: Go does the filesystem work (see history).
	mux.Handle(history.Prefix, history.New(history.Options{
		Root: root.Dir(), Token: token,
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	}).Handler())
	mux.Handle(extcatalog.Prefix, ext.Handler())
	// The keyword arguments of Gradio's launch(): Python only passes them on.
	mux.Handle(launchopts.Path, launchopts.New(launchopts.Options{
		Config: cfg, Token: token, GradioHost: gradioHost, GradioPort: gradioPort, Flags: flags,
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	}).Handler())
	// .env is rewritten only here (the Model Location tab; see appenv).
	mux.Handle(appenv.APIPrefix, appenv.NewAPI(appenv.APIOptions{
		Root: root.Dir(), Token: token,
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	}).Handler())
	// Settings > Installed Packages: pip list in the main env (see piplist).
	mux.Handle(piplist.Path, pip.Handler())
	// Log Viewer: list, read, search and delete installer_scripts/logs (see logview).
	mux.Handle(logview.Prefix, logview.New(logview.Options{
		Root: root.Dir(), Token: token,
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	}).Handler())
	// The Vite engine page starts an installed own-venv engine on open (D31).
	mux.Handle(hostStatusPath, hostStatusHandler(p, token, root.Dir()))
	mux.Handle(engines.Prefix, engines.New(engines.Options{
		Cat: ext, Procs: mgr, Token: token, PublicPort: tcpPort(pub.Addr()),
	}))
	// The openai platform: named remote endpoints, keys kept by Go (see platform/openai).
	mux.Handle(openai.Prefix, openai.NewHandler(openai.NewClient(openai.Options{
		Store: openai.NewStore(root.Dir()), Root: root.Dir(),
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	}), token))
	// The audiocpp platform: the audio.cpp server and its models (see platform/audiocpp).
	mux.Handle(audiocpp.Prefix, audiocpp.NewHandler(root, token, func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) }))
	mux.Handle("/api/extensions", mgr.Handler())
	mux.Handle("/api/extensions/", mgr.Handler())
	ctl := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	// The Extensions List pages Gradio shows in an iframe come from the same
	// origin, through the router (see extui). They have their own listener and
	// rules: the control API above never faces the browser.
	uiLn, ui, err := startExtUI(rt, ext, cfg, token, pub.Addr(), host)
	if err != nil {
		_ = pub.Close()
		_ = ctlLn.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	sp := &serverPy{
		root:       root.Dir(),
		rt:         rt,
		mgr:        mgr,
		ctl:        ctl,
		ui:         ui,
		publicAddr: pub.Addr().String(),
		gradioEnv: append(commandEnv(p),
			"TTS_CORE_URL=http://"+ctlLn.Addr().String(),
			"TTS_CORE_TOKEN="+token,
		),
		coreURL:    "http://" + ctlLn.Addr().String(),
		coreToken:  token,
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
	go func() { _ = ui.Serve(uiLn) }()
	slog.Info(fmt.Sprintf("serverpy: router on %s -> gradio %s", sp.publicAddr, net.JoinHostPort(gradioHost, strconv.Itoa(gradioPort))))
	return sp, nil
}

// Close stops the extensions Go started, then the router and the control API.
// It is safe to call more than once and from several goroutines.
func (sp *serverPy) Close() {
	sp.closeOnce.Do(func() {
		sp.mgr.StopAll()
		if r, err := paths.New(sp.root); err == nil {
			audiocpp.ServerFor(r).Stop()
		}
		runningEngines.CompareAndDelete(sp.root, sp.mgr)
		sp.stopRouter()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sp.ctl.Shutdown(ctx)
		_ = sp.ui.Shutdown(ctx)
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

// freePort asks the OS for a free TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	// Brief pause so the OS can reclaim the ephemeral port before the child claims it.
	time.Sleep(10 * time.Millisecond)
	return port, nil
}

// startExtUI builds the extension page handler, listens for it on a loopback
// port and routes /_core there. The pages are read-only when the router is
// reachable from other devices (host is not a loopback address).
func startExtUI(rt *router.Router, ext *extcatalog.Store, cfg *appconfig.Store, token string, pub net.Addr, host string) (net.Listener, *http.Server, error) {
	port := 0
	if a, ok := pub.(*net.TCPAddr); ok {
		port = a.Port
	}
	h, err := extui.New(extui.Options{
		Cat: ext, CatToken: token, Cfg: cfg, Port: port, Exposed: !isLoopbackHost(host),
		Logf: func(f string, a ...any) { slog.Info(fmt.Sprintf(f, a...)) },
	})
	if err != nil {
		return nil, nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("extension pages: %w", err)
	}
	if err := rt.AddRoute(extui.Prefix, "http://"+ln.Addr().String()); err != nil {
		_ = ln.Close()
		return nil, nil, err
	}
	return ln, &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}, nil
}

// isLoopbackHost is whether a listen host only accepts connections from this machine.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// commandEnv is the environment the platform gives a command.
func commandEnv(p platform.Platform) []string {
	c, err := p.Command(nil, []string{"python"})
	if err != nil {
		return os.Environ()
	}
	return c.Env
}

// tcpPort is the port of a TCP address, 0 for any other kind.
func tcpPort(a net.Addr) int {
	if t, ok := a.(*net.TCPAddr); ok {
		return t.Port
	}
	return 0
}

// runningEngines maps an install folder to the extension process manager of the
// app running on it. The launcher's Extensions page has its own catalog store,
// which uses this to stop an engine before removing its venv.
var runningEngines sync.Map // string -> *extprocs.Manager

// stopRunningEngine stops pkg in the app running on root, if there is one.
func stopRunningEngine(root, pkg string) bool {
	if m, ok := runningEngines.Load(root); ok {
		return m.(*extprocs.Manager).StopIfRunning(pkg)
	}
	return false
}
