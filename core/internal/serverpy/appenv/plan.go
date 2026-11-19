package appenv

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

const (
	treeBackendPort = 7767 // gradio's port behind the proxy tree
	dockerPublic    = 7770
	defaultTreePort = 7769 // GRADIO_TREE_PORT default in gradio_proxy_tree/main.py
	defaultReact    = 3000
)

// PortPlan is what server.py's start_gradio_server decides, computed up front.
type PortPlan struct {
	PublicPort int    // port the user connects to (proxy tree / router front door)
	GradioPort int    // port gradio itself binds
	Host       string // gradio server_name; the front door binds the same host
	ReactPort  int    // REACT_UI_PORT
	Share      bool
	InBrowser  bool // python's in_browser: open browser tabs after launch
	NoReact    bool
	// TreeProxy: a front door sits ahead of gradio (GRADIO_TREE_URL is ""),
	// i.e. no --no-tree-proxy, or --docker.
	TreeProxy bool
	// TreePort is the port python's proxy tree listens on. server.py starts it
	// even with --no-tree-proxy; then it is GRADIO_TREE_PORT or 7769.
	TreePort int
	// GradioInBrowser is the inbrowser value handed to gradio launch().
	GradioInBrowser bool
	// BrowserURLs are the pages python opens when InBrowser is set.
	BrowserURLs []string
	// ReactBackend is GRADIO_BACKEND_AUTOMATIC for react-ui. Legacy quirk:
	// server_hypervisor() runs before ports are reassigned, so this is the
	// config port (the public port, except under --docker with another port).
	ReactBackend string
	Auth         Auth

	env []string
}

// Env is the environment python expects in this mode, as NAME=value.
func (p PortPlan) Env() []string { return append([]string(nil), p.env...) }

// ReactEnv is the extra environment for the react-ui child.
func (p PortPlan) ReactEnv() []string { return []string{"GRADIO_BACKEND_AUTOMATIC=" + p.ReactBackend} }

// String is safe to log (the auth password is redacted).
func (p PortPlan) String() string {
	return fmt.Sprintf("public=%d gradio=%d host=%s react=%d(no-react=%v) share=%v tree=%v/%d %s",
		p.PublicPort, p.GradioPort, p.Host, p.ReactPort, p.NoReact, p.Share, p.TreeProxy, p.TreePort, p.Auth)
}

// Plan computes the plan from root/config.json, the merged environment
// (from LoadEnv) and the flags.
func Plan(root string, env []string, f Flags) (PortPlan, error) {
	cfg, err := LoadConfig(root)
	if err != nil {
		return PortPlan{}, err
	}
	return PlanFor(cfg, env, f)
}

// PlanFor is Plan with the config already loaded.
func PlanFor(cfg Config, env []string, f Flags) (PortPlan, error) {
	fold := runtime.GOOS == "windows"
	p := PortPlan{
		Host:            cfg.Host,
		Share:           f.Share || cfg.Share,
		InBrowser:       cfg.InBrowser,
		NoReact:         f.NoReact,
		GradioPort:      cfg.Port,
		PublicPort:      cfg.Port,
		GradioInBrowser: cfg.InBrowser,
		Auth:            cfg.Auth,
		ReactBackend:    fmt.Sprintf("http://127.0.0.1:%d/", cfg.Port),
		TreeProxy:       !f.NoTreeProxy || f.Docker,
	}

	p.ReactPort = defaultReact
	if v, ok := lookupEnv(env, "REACT_UI_PORT", fold); ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 1 || n > 65535 {
			if !f.NoReact {
				return PortPlan{}, fmt.Errorf("REACT_UI_PORT=%q is not a port", v)
			}
			n = 0
		}
		p.ReactPort = n
	}

	treeEnv := false // GRADIO_TREE_PORT/URL set by python
	treePortEnv := ""
	// Same order as start_gradio_server.
	if !f.NoTreeProxy {
		treeEnv = true
		treePortEnv = strconv.Itoa(cfg.Port)
		p.GradioPort = treeBackendPort
		p.GradioInBrowser = false
	}
	if f.Docker {
		p.Host = "0.0.0.0"
		p.GradioPort = treeBackendPort
		treeEnv = true
		treePortEnv = strconv.Itoa(dockerPublic)
	}

	switch {
	case treeEnv:
		p.TreePort, _ = strconv.Atoi(treePortEnv)
		p.PublicPort = p.TreePort
	default: // --no-tree-proxy only: gradio is the front door, the tree is a side process
		p.TreePort = defaultTreePort
		if v, ok := lookupEnv(env, "GRADIO_TREE_PORT", fold); ok {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return PortPlan{}, fmt.Errorf("GRADIO_TREE_PORT=%q is not an integer", v)
			}
			p.TreePort = n
		}
		p.PublicPort = p.GradioPort
	}

	if p.InBrowser {
		if !f.NoTreeProxy {
			p.BrowserURLs = append(p.BrowserURLs, fmt.Sprintf("http://localhost:%s", treePortEnv))
		} else {
			p.BrowserURLs = append(p.BrowserURLs, fmt.Sprintf("http://localhost:%d", p.GradioPort))
		}
		if !f.NoReact {
			p.BrowserURLs = append(p.BrowserURLs, fmt.Sprintf("http://localhost:%d", p.ReactPort))
		}
	}

	if treeEnv {
		p.env = append(p.env, "GRADIO_TREE_PORT="+treePortEnv, "GRADIO_TREE_URL=")
	}
	p.env = append(p.env,
		"TTS_GRADIO_PORT="+strconv.Itoa(p.GradioPort),
		"TTS_GRADIO_HOST="+p.Host)
	return p, nil
}
