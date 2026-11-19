// Command serverpy-demo runs the four serverpy stages together without the
// launcher, so they can be tried end to end:
//
//	appenv    folders, .env, dotenv load, port plan     (stage 2)
//	router    Go reverse proxy on the public port       (stage 3)
//	extprocs  Go-run proxy extensions + control API     (stage 4)
//	supervise Gradio (server_core.py), React, ext. mgr  (stage 1)
//
// It is a test harness, not the product: the launcher wiring is described in
// each package's INTEGRATION.md. See README.md next to this file.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extprocs"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/router"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/supervise"
)

func main() {
	var (
		rootFlag    = flag.String("root", "..", "repo root holding server_core.py (cwd of the Python children)")
		envRootFlag = flag.String("env-root", "", "install holding installer_files/ (micromamba + env); default = -root")
		publicPort  = flag.Int("public-port", 17770, "port the Go router serves (the address you open)")
		withReact   = flag.Bool("react", false, "also start the React UI (needs react-ui/node_modules and a build in -root)")
		reactPort   = flag.Int("react-port", 13000, "React UI port when -react is set")
		openBrowser = flag.Bool("browser", true, "open the browser when Gradio is ready")
		share       = flag.Bool("share", false, "forward --share to Gradio")
		showLogs    = flag.Bool("logs", true, "print the children's output")
		exitAfter   = flag.Duration("exit-after", 0, "stop everything after this long (0 = run until Ctrl+C)")
	)
	flag.Parse()
	log.SetFlags(log.Ltime)

	if err := run(*rootFlag, *envRootFlag, *publicPort, *withReact, *reactPort, *openBrowser, *share, *showLogs, *exitAfter); err != nil {
		log.Fatalf("serverpy-demo: %v", err)
	}
}

func run(rootDir, envRootDir string, publicPort int, withReact bool, reactPort int, openBrowser, share, showLogs bool, exitAfter time.Duration) error {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(rootAbs, "server_core.py")); err != nil {
		return fmt.Errorf("-root %s has no server_core.py (run from core/ of the serverpy/integration worktree, or pass -root)", rootAbs)
	}
	if envRootDir == "" {
		envRootDir = rootAbs
	}
	envRoot, err := paths.New(envRootDir)
	if err != nil {
		return err
	}
	root, err := paths.New(rootAbs)
	if err != nil {
		return err
	}
	mamba := filepath.Join(envRoot.InstallerFiles(), "mamba", "micromamba")
	if runtime.GOOS == "windows" {
		mamba += ".exe"
	}
	if _, err := os.Stat(mamba); err != nil {
		return fmt.Errorf("no micromamba at %s (pass -env-root pointing at an installed TTS-WebUI)", mamba)
	}
	prefix := []string{mamba, "-p", filepath.Join(envRoot.InstallerFiles(), "env"), "run"}
	mambaRoot := "MAMBA_ROOT_PREFIX=" + filepath.Join(envRoot.InstallerFiles(), "mamba")

	// Stage 2: the prep server.py used to do, and the port plan.
	base := os.Environ()
	if err := appenv.EnsureFolders(rootAbs); err != nil {
		return fmt.Errorf("folders: %w", err)
	}
	if created, err := appenv.EnsureEnv(rootAbs, base); err != nil {
		return fmt.Errorf(".env: %w", err)
	} else if created {
		log.Printf("appenv: wrote default .env in %s", rootAbs)
	}
	rep, err := appenv.LoadEnvReport(rootAbs, base)
	if err != nil {
		return fmt.Errorf("dotenv: %w", err)
	}
	for _, w := range rep.Warnings {
		log.Printf("appenv: %s", w)
	}
	plan, err := appenv.Plan(rootAbs, rep.Env, appenv.Flags{Share: share, NoReact: !withReact})
	if err != nil {
		return fmt.Errorf("port plan: %w", err)
	}
	log.Printf("appenv: legacy plan would be %s", plan)

	// The demo keeps clear of a running app: its own public port, a private
	// Gradio port, and Gradio bound to loopback behind the router.
	gradioPort, err := freePort()
	if err != nil {
		return err
	}
	token, err := randomToken()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stage 3: the router replaces the Python proxy tree.
	// The supervisor's readiness probe goes through the router, so every probe
	// before Gradio is up logs "upstream unavailable"; show each message once
	// per 15 s.
	rt := router.New(router.Options{Token: token, Logger: slog.New(&dedupHandler{
		Handler: slog.NewTextHandler(os.Stderr, nil),
		every:   15 * time.Second,
		last:    map[string]time.Time{},
	})})
	if err := rt.AddRoute("/", fmt.Sprintf("http://127.0.0.1:%d", gradioPort)); err != nil {
		return err
	}
	pub, err := net.Listen("tcp", net.JoinHostPort(router.BindHost(plan.Host), strconv.Itoa(publicPort)))
	if err != nil {
		return fmt.Errorf("public port %d: %w", publicPort, err)
	}
	go func() {
		if err := rt.ServeListener(ctx, pub); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("router: %v", err)
		}
	}()

	// Stage 4: Go runs the proxy extensions.
	mgr := extprocs.New(extprocs.Options{
		Root:       root,
		MainPython: append(append([]string(nil), prefix...), "python"),
		BaseEnv:    append(append([]string(nil), rep.Env...), mambaRoot),
		Routes:     rt,
		Token:      token,
		Logf:       func(f string, a ...any) { log.Printf("extprocs: "+f, a...) },
	})

	// Control API on its own loopback listener (the launcher's mux in the product).
	ctl, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	coreURL := "http://" + ctl.Addr().String()

	// Stage 1: the supervisor runs server_core.py instead of server.py.
	sup, err := supervise.New(supervise.Config{
		Root:        root,
		PublicPort:  publicPort,
		ReactPort:   reactPort,
		NoReact:     !withReact,
		GradioEntry: "server_core.py",
		Share:       share,
		ExtraEnv:    append(append([]string(nil), rep.Env...), mambaRoot),
		GradioEnv: []string{
			"TTS_CORE_URL=" + coreURL,
			"TTS_CORE_TOKEN=" + token,
			"TTS_GRADIO_PORT=" + strconv.Itoa(gradioPort),
			"TTS_GRADIO_HOST=127.0.0.1",
		},
		OpenBrowser:   openBrowser,
		SkipGPUCheck:  true, // a fresh worktree has no installer_scripts/.gpu marker
		CommandPrefix: prefix,
	})
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/api/routes", rt.ControlHandler())
	mux.Handle("/api/extensions", mgr.Handler())
	mux.Handle("/api/extensions/", mgr.Handler())
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"public_url":  fmt.Sprintf("http://127.0.0.1:%d/", publicPort),
			"gradio_port": gradioPort,
			"processes":   sup.Statuses(),
			"extensions":  mgr.List(),
			"routes":      rt.Routes(),
		})
	})
	ctlSrv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = ctlSrv.Serve(ctl) }()

	log.Printf("router:  http://127.0.0.1:%d/  ->  gradio 127.0.0.1:%d", publicPort, gradioPort)
	log.Printf("control: %s  (GET %s/status for processes, extensions, routes)", coreURL, coreURL)
	log.Printf("starting %s with %s", filepath.Join(rootAbs, "server_core.py"), strings.Join(prefix, " "))

	if err := sup.Start(ctx); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	var wg sync.WaitGroup
	if showLogs {
		for _, name := range []string{supervise.NameGradio, supervise.NameReact, supervise.NameExtMgr} {
			ch, stop, ok := sup.SubscribeLogs(name)
			if !ok {
				continue
			}
			defer stop()
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				for line := range ch {
					fmt.Printf("[%s] %s\n", name, line)
				}
			}(name)
		}
	}
	statusCh, stopStatus := sup.SubscribeStatus()
	defer stopStatus()
	go printStateChanges(statusCh)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	var timeout <-chan time.Time
	if exitAfter > 0 {
		timeout = time.After(exitAfter)
	}
	select {
	case <-sig:
		log.Printf("interrupt: stopping")
	case <-timeout:
		log.Printf("-exit-after %s reached: stopping", exitAfter)
	}

	mgr.StopAll()
	sup.Stop()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	_ = ctlSrv.Shutdown(shutCtx)
	cancel()
	log.Printf("stopped")
	return nil
}

// printStateChanges logs a line each time a child's state changes.
func printStateChanges(ch <-chan string) {
	last := map[string]string{}
	for raw := range ch {
		var sts []supervise.ProcessStatus
		if json.Unmarshal([]byte(raw), &sts) != nil {
			continue
		}
		for _, st := range sts {
			s := string(st.State)
			if st.ExitCode != nil {
				s += fmt.Sprintf(" (exit %d)", *st.ExitCode)
			}
			if st.Error != "" {
				s += " (" + st.Error + ")"
			}
			if last[st.Name] != s {
				last[st.Name] = s
				log.Printf("state: %-17s %s", st.Name, s)
			}
		}
	}
}

// dedupHandler drops a log record whose message was already logged within
// the last `every`.
type dedupHandler struct {
	slog.Handler
	every time.Duration
	mu    sync.Mutex
	last  map[string]time.Time
}

func (h *dedupHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	t, seen := h.last[r.Message]
	if seen && r.Time.Sub(t) < h.every {
		h.mu.Unlock()
		return nil
	}
	h.last[r.Message] = r.Time
	h.mu.Unlock()
	return h.Handler.Handle(ctx, r)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
