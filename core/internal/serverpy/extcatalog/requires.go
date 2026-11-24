package extcatalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/requires"
)

// entryRequires is the `requires` of an entry: its own list when it has one,
// otherwise the one derived from what it is (proxy: native runs on uv, anything
// else in the Gradio host). The explicit list is checked for syntax.
func entryRequires(e *Obj) ([]string, error) {
	v, ok := e.Get("requires")
	if !ok {
		return requires.Derive(e.Str("proxy") == "native"), nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("requires must be a list of names")
	}
	out := make([]string, 0, len(list))
	for _, x := range list {
		s, ok := x.(string)
		if !ok {
			return nil, fmt.Errorf("requires must be a list of names")
		}
		out = append(out, s)
	}
	if err := requires.Validate(out); err != nil {
		return nil, err
	}
	return out, nil
}

// RequiresGraph is the catalog's requires, with what is installed now and what
// the active platform provides.
func (s *Store) RequiresGraph(ctx context.Context) (requires.Graph, error) {
	g := requires.Graph{Entries: map[string]requires.Entry{}, Provided: map[string]string{}}
	in := s.Installs(ctx)
	for _, kind := range []string{"interface", "decorator"} {
		list, err := s.List(kind)
		if err != nil {
			return g, err
		}
		for _, e := range list {
			pkg := e.Str("package_name")
			if pkg == "" {
				continue
			}
			if requires.Reserved(pkg) {
				return g, fmt.Errorf("%s: this name is reserved for a platform", pkg)
			}
			req, err := entryRequires(e)
			if err != nil {
				return g, fmt.Errorf("%s: %v", pkg, err)
			}
			installed, _ := in.Of(pkg, e.Str("proxy"))
			// A built-in tab is part of the host, not something installed on
			// it: it must not stop the host from being removed.
			installed = installed && !platform.Builtin(pkg)
			g.Entries[pkg] = requires.Entry{Name: pkg, Requires: req, Installed: installed}
		}
	}
	s.addHost(ctx, &g)
	if s.opts.UV != nil {
		g.Installable = map[string]string{"uv": uvWould}
		if s.opts.UV.Status(ctx).Ready {
			g.Provided["uv"] = "uv"
		}
	}
	if s.opts.Platform != nil && s.opts.Platform.Status(ctx).Ready {
		p := s.opts.Platform
		g.Provided[p.Name()] = p.Name()
		for _, n := range p.Provides() {
			g.Provided[n] = p.Name()
		}
	}
	return g, nil
}

// blockedMessage says what an install cannot start with.
func blockedMessage(name string, blocked []requires.Req) string {
	var parts []string
	for _, b := range blocked {
		parts = append(parts, fmt.Sprintf("%s (%s)", b.Name, b.Would))
	}
	return fmt.Sprintf("%s requires %s", name, strings.Join(parts, ", "))
}

// plan is what an install takes: platforms first, then entries in order, the
// requested one last. Total counts all of them.
type plan struct {
	Platforms []string
	Steps     []string
	Total     int
	GPU       gpu.Label // the choice for the uv platform, when it is in Platforms
}

// planInstall resolves what installing pkg takes. A requirement that is not
// there and cannot be installed refuses the install, saying what it needs.
func (s *Store) planInstall(ctx context.Context, g requires.Graph, pkg string) (plan, error) {
	p, err := g.Resolve(pkg)
	if err != nil {
		return plan{}, invalidf("%v", err)
	}
	if len(p.Blocked) > 0 {
		return plan{}, invalidf("%s", blockedMessage(pkg, p.Blocked))
	}
	return plan{Platforms: p.Platforms, Steps: p.Steps, Total: len(p.Platforms) + len(p.Steps)}, nil
}

// refuseIfRequired refuses the removal of an entry that installed entries need.
func refuseIfRequired(g requires.Graph, pkg string) error {
	if deps := g.Dependants(pkg); len(deps) > 0 {
		return invalidf("cannot remove %s: required by %s", pkg, strings.Join(deps, ", "))
	}
	return nil
}

// planPasted plans the install of an entry the caller handed over: its
// requirements are looked up in the catalog, and the pasted entry stands in for
// a catalog entry of the same name.
func (s *Store) planPasted(ctx context.Context, entry *Obj) (plan, error) {
	g, err := s.RequiresGraph(ctx)
	if err != nil {
		return plan{}, invalidf("%v", err)
	}
	req, err := entryRequires(entry)
	if err != nil {
		return plan{}, invalidf("entry: %v", err)
	}
	name := entry.Str("package_name")
	g.Entries[name] = requires.Entry{Name: name, Requires: req}
	return s.planInstall(ctx, g, name)
}
