package extcatalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/gradiohost"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/piplist"
)

// hostStore is a store on the Gradio host platform (uv stands in for the real
// one; there is no Python to run, so only what Go decides is tested here).
func hostStore(t *testing.T) (*Store, string, *fakePip, *bool) {
	t.Helper()
	s, root, _ := installStore(t)
	pr, _ := paths.New(root)
	uvp := uvStandIn(t, root)
	host := gradiohost.New(gradiohost.Options{Root: pr, UV: uvp})
	pip := &fakePip{}
	running := false
	s.opts.Platform, s.opts.Host, s.opts.UV, s.opts.Pip = host, host, uvp, pip
	s.opts.AppRunning = func() bool { return running }
	writeFile(t, root, "requirements.txt", "gradio==5.49.1\n")
	writeFile(t, root, "extensions.json", `{"tabs":[`+
		entry("ext.tab", "A Tab", `"requirements":"six"`)+`,`+
		entry("ext.native", "Native Ext", `"proxy":"native"`)+`,`+
		entry("extensions.builtin.x", "Builtin")+`]}`)
	return s, root, pip, &running
}

func TestAnInProcessTabRequiresTheHostAndTheHostRequiresUV(t *testing.T) {
	s, _, _, _ := hostStore(t)
	g, err := s.RequiresGraph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	h := g.Entries["gradio-host"]
	if h.Installed || len(h.Requires) != 1 || h.Requires[0] != "uv" {
		t.Fatalf("host entry = %+v", h)
	}
	p, err := g.Resolve("ext.tab")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(p.Steps, ",") != "gradio-host,ext.tab" || strings.Join(p.Platforms, ",") != "uv" || len(p.Blocked) != 0 {
		t.Fatalf("plan = %+v", p)
	}
	st := g.State("ext.tab")
	if len(st) != 1 || st[0].State != "missing" || !strings.Contains(st[0].Would, "gradio-host") {
		t.Fatalf("state = %+v", st)
	}
}

func TestInstallingTheHostInstallsUVFirstAndThenTabsAreReady(t *testing.T) {
	s, root, _, _ := hostStore(t)
	// The fake uv has to leave a python where uv would.
	_, got, _ := stream(t, s, `install {"package_name":"gradio-host","gpu":"Custom (Manual torch install)"}`)
	joined := strings.Join(got.lines, "\n")
	if !got.ok {
		t.Fatalf("install: %s", joined)
	}
	for _, want := range []string{"Installing the uv platform (step 1 of 2)", "Installing the Gradio host (step 2 of 2)", "Successfully set up the Gradio host"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	for _, f := range []string{".venvs/gradio-host/entry.json", ".venvs/gradio-host/constraints.txt", "installer_scripts/logs/gradio-host-install-gradio-host.log"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	g, _ := s.RequiresGraph(context.Background())
	if st := g.State("ext.tab"); st[0].State != "ready" || st[0].Via != "gradio-host" {
		t.Fatalf("tab state = %+v", st)
	}
	// Installing it again is refused, not repeated.
	if code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"gradio-host"}`, "tok"); code != 400 || !strings.Contains(body, "already installed") {
		t.Fatalf("second install: %d %s", code, body)
	}
}

func TestTheHostIsRefusedWhileATabNeedsItAndWhileTheAppRuns(t *testing.T) {
	s, root, pip, running := hostStore(t)
	if code, body := call(t, s.Handler(), "POST", Prefix+"uninstall", `{"package_name":"gradio-host"}`, "tok"); code != 400 || !strings.Contains(body, "not installed") {
		t.Fatalf("remove with no host: %d %s", code, body)
	}
	stream(t, s, `install {"package_name":"gradio-host","gpu":"Custom (Manual torch install)"}`)
	pip.pkgs = []piplist.Package{{Name: "ext.tab", Version: "1"}, {Name: "extensions.builtin.x", Version: "1"}}
	code, body := call(t, s.Handler(), "POST", Prefix+"uninstall", `{"package_name":"gradio-host"}`, "tok")
	if code != 400 || !strings.Contains(body, "cannot remove gradio-host: required by ext.tab") || strings.Contains(body, "builtin") {
		t.Fatalf("with a tab: %d %s", code, body)
	}
	pip.pkgs = nil
	*running = true
	if code, body = call(t, s.Handler(), "POST", Prefix+"uninstall", `{"package_name":"gradio-host"}`, "tok"); code != 400 || !strings.Contains(body, "stop the app first") {
		t.Fatalf("while running: %d %s", code, body)
	}
	*running = false
	_, got, _ := stream(t, s, `uninstall {"package_name":"gradio-host"}`)
	if !got.ok {
		t.Fatalf("remove: %v", got.lines)
	}
	if _, err := os.Stat(filepath.Join(root, ".venvs", "gradio-host")); err == nil {
		t.Fatal("the venv is still there")
	}
	if st := s.opts.Platform.Status(context.Background()); st.Ready {
		t.Fatal("the platform is still ready")
	}
}

func TestWithoutTheHostNoTabIsInstalledAndThereIsNoWarning(t *testing.T) {
	s, _, pip, _ := hostStore(t)
	pip.pkgs = []piplist.Package{{Name: "ext.tab", Version: "1"}} // would be true if pip could be asked
	in := s.Installs(context.Background())
	if in.Err != nil {
		t.Fatalf("err = %v", in.Err)
	}
	if ok, _ := in.Of("ext.tab", ""); ok {
		t.Fatal("a tab is installed with no host")
	}
}

func TestTheHostIsManagedOutsideThisAppOnAnAdoptedEnv(t *testing.T) {
	s, _, _ := installStore(t) // legacy-env platform, no Host
	if code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"gradio-host"}`, "tok"); code != 400 || !strings.Contains(body, "managed outside this app") {
		t.Fatalf("install: %d %s", code, body)
	}
	if code, body := call(t, s.Handler(), "POST", Prefix+"uninstall", `{"package_name":"gradio-host"}`, "tok"); code != 400 || !strings.Contains(body, "managed outside this app") {
		t.Fatalf("uninstall: %d %s", code, body)
	}
	var _ platform.Platform = s.opts.Platform
}
