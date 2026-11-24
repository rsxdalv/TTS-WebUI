package extcatalog

import (
	"context"
	"net/http"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/gradiohost"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/requires"
)

// hostActive is whether the main env of this install is the Gradio host: the
// only runtime this app installs and removes. On legacy-env and system the host
// is the existing Python env.
func (s *Store) hostActive() bool {
	return s.opts.Host != nil && s.opts.Platform != nil && s.opts.Platform.Name() == gradiohost.Name
}

// addHost puts the Gradio host in the graph as an entry of its own. It requires
// uv; it is installed when the main env is ready. On an install that runs on an
// adopted env the host is a satisfied requirement, never a step.
func (s *Store) addHost(ctx context.Context, g *requires.Graph) {
	if s.opts.Platform == nil {
		return
	}
	ready := s.opts.Platform.Status(ctx).Ready
	switch {
	case s.hostActive():
		g.Entries[gradiohost.Name] = requires.Entry{Name: gradiohost.Name, Requires: []string{"uv"}, Installed: ready}
	case ready:
		g.Entries[gradiohost.Name] = requires.Entry{Name: gradiohost.Name, Installed: true}
	}
}

// installHost is the Gradio host step of a plan.
func (s *Store) installHost(ctx context.Context, emit func(string)) bool {
	if !s.hostActive() {
		emit("Failed: the " + gradiohost.Title + " cannot be installed here")
		return false
	}
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	err := s.opts.Host.Install(ctx, rec)
	if err != nil {
		if ctx.Err() != nil {
			rec("Cancelled: the " + gradiohost.Title + " was not finished (install it again to continue; downloads are kept)")
		} else {
			rec("Failed to install the " + gradiohost.Title + ": " + err.Error())
		}
	}
	s.writeLog("gradio-host-install", gradiohost.Name, log)
	return err == nil
}

// uninstallHost removes the host's venv.
func (s *Store) uninstallHost(ctx context.Context, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	err := s.opts.Host.Uninstall(ctx, rec)
	if err != nil {
		rec("Failed to remove the " + gradiohost.Title + ": " + err.Error())
	} else {
		rec("Successfully uninstalled " + gradiohost.Title)
	}
	s.writeLog("gradio-host-uninstall", gradiohost.Name, log)
	return err == nil
}

// serveHost is serveInstall for the Gradio host: it is not in the catalog, so it
// has its own checks, then the same plan, lock and stream as any entry.
func (s *Store) serveHost(w http.ResponseWriter, r *http.Request, op string, o *Obj) {
	if !s.hostActive() {
		s.fail(w, invalidf("the %s is this install's own Python env (%s): it is managed outside this app",
			gradiohost.Title, s.opts.Platform.Name()))
		return
	}
	if op != "install" && op != "uninstall" {
		s.fail(w, invalidf("%s has no %s", gradiohost.Title, op))
		return
	}
	g, err := s.RequiresGraph(r.Context())
	var pl plan
	if err == nil && op == "install" {
		if g.Entries[gradiohost.Name].Installed {
			err = invalidf("the %s is already installed", gradiohost.Title)
		} else if pl, err = s.planInstall(r.Context(), g, gradiohost.Name); err == nil {
			err = s.chooseGPU(&pl, o.Str("gpu"))
		}
	} else if err == nil {
		if !g.Entries[gradiohost.Name].Installed && !s.opts.Host.Installed().Partial {
			err = invalidf("the %s is not installed", gradiohost.Title)
		} else if err = refuseIfRequired(g, gradiohost.Name); err == nil && s.opts.AppRunning != nil && s.opts.AppRunning() {
			err = invalidf("stop the app first: the %s cannot be removed while it runs", gradiohost.Title)
		}
	}
	if err != nil {
		s.failInstall(w, err)
		return
	}
	if !s.install.TryLock() {
		s.fail(w, errBusy)
		return
	}
	defer s.install.Unlock()
	if s.opts.Pip != nil {
		defer s.opts.Pip.Invalidate()
	}
	s.stream(w, r, func(ctx context.Context, emit func(string)) bool {
		if op == "uninstall" {
			return s.uninstallHost(ctx, emit)
		}
		return s.runSteps(ctx, pl, nil, emit)
	})
}

// HostActive is whether the Gradio host is an entry of this install that can be
// installed and removed (not the existing Python env).
func (s *Store) HostActive() bool { return s.hostActive() }
