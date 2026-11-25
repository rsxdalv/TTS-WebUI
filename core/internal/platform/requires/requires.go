// Package requires is the `requires` of a catalog entry: the platforms and
// entries that must be present before it installs (PLATFORM-DESIGN.md,
// section 2 and 3). It knows names, not catalogs: the caller hands it the
// entries and what the active platform provides.
package requires

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// reserved are the names no catalog entry may take: platforms, and the Gradio
// host, which the caller adds to the graph as an entry of its own when it can be
// installed (Graph.Entries); otherwise the text here is why it cannot be.
var reserved = map[string]string{
	"openai":     "the openai platform is not available in this version",
	"uv":         "the uv platform is not installed",
	"audiocpp":   "the audiocpp platform is not available in this version",
	"onnx":       "the onnx platform is not available in this version",
	"legacy-env": "legacy-env is the existing install's own Python env: it is adopted, never installed",
	"system":     "system is the Python on PATH (--system-python): it is not installed by this app",
	"gradio-host": "the Gradio host is not available as an installable entry in this version" +
		" (it is the existing Python env)",
}

// Reserved reports whether name is a platform name or the Gradio host.
func Reserved(name string) bool { _, ok := reserved[name]; return ok }

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Validate checks the syntax of a requires list: plain names, no repeats.
func Validate(names []string) error {
	seen := map[string]bool{}
	for _, n := range names {
		if !nameRe.MatchString(n) {
			return fmt.Errorf("requires: %q is not a platform or entry name", n)
		}
		if seen[n] {
			return fmt.Errorf("requires: %q is listed twice", n)
		}
		seen[n] = true
	}
	return nil
}

// Derive is the requires of an entry that has none of its own: an own-venv
// engine runs on uv; everything else (an in-process tab, a built-in tab, a
// decorator) runs inside the Gradio host.
func Derive(ownVenv bool) []string {
	if ownVenv {
		return []string{"uv"}
	}
	return []string{"gradio-host"}
}

// Entry is one catalog entry, as far as requires is concerned.
type Entry struct {
	Name      string   // package_name
	Requires  []string // explicit or derived
	Installed bool
}

// Graph is the catalog plus what the active platform provides.
type Graph struct {
	Entries map[string]Entry
	// Provided maps a name the platform satisfies to the platform's name. Empty
	// when the platform is not ready (nothing is provided).
	Provided map[string]string
	// Installable maps a platform that is not there but can be installed to what
	// installing it does ("install the uv platform ..."). A requirement named
	// here is satisfied by installing it first, not refused.
	Installable map[string]string
}

// Req is the state of one requirement of an entry.
type Req struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`            // "platform" or "entry"
	State string `json:"state"`           // "ready" or "missing"
	Via   string `json:"via,omitempty"`   // what satisfies it
	Would string `json:"would,omitempty"` // what would be installed, or why it cannot be
	// Installable is true for a missing platform that an install can set up.
	Installable bool `json:"installable,omitempty"`
}

// Check says whether every name an entry requires exists: a platform name, the
// Gradio host, or an entry of the catalog. It names the entry and the name.
func (g Graph) Check(entry string) error {
	e, ok := g.Entries[entry]
	if !ok {
		return fmt.Errorf("%s: not in the catalog", entry)
	}
	for _, n := range e.Requires {
		if _, ok := g.Entries[n]; !ok && !Reserved(n) {
			return fmt.Errorf("%s: requires %q, which is neither a platform nor an entry of the catalog", entry, n)
		}
	}
	return nil
}

// State is the state of each requirement of entry.
func (g Graph) State(entry string) []Req {
	e := g.Entries[entry]
	out := make([]Req, 0, len(e.Requires))
	for _, n := range e.Requires {
		out = append(out, g.one(n))
	}
	return out
}

func (g Graph) one(n string) Req {
	if via, ok := g.Provided[n]; ok {
		kind := "platform"
		if n == "gradio-host" {
			kind = "entry"
		}
		return Req{Name: n, Kind: kind, State: "ready", Via: via}
	}
	if what, ok := g.Installable[n]; ok {
		return Req{Name: n, Kind: "platform", State: "missing", Would: what, Installable: true}
	}
	d, isEntry := g.Entries[n]
	if !isEntry {
		if why, ok := reserved[n]; ok {
			kind := "platform"
			if n == "gradio-host" {
				kind = "entry"
			}
			return Req{Name: n, Kind: kind, State: "missing", Would: why}
		}
		return Req{Name: n, Kind: "entry", State: "missing", Would: "not in the catalog"}
	}
	if d.Installed {
		return Req{Name: n, Kind: "entry", State: "ready", Via: n}
	}
	return Req{Name: n, Kind: "entry", State: "missing", Would: "would install " + n}
}

// Plan is what installing an entry takes.
type Plan struct {
	// Steps are the entries to install, dependencies first, the entry last. An
	// entry that is already installed is not in it.
	Steps []string
	// Platforms are platforms to install before the first step, in order.
	Platforms []string
	// Blocked are requirements that cannot be satisfied by installing an entry:
	// a platform or the Gradio host that is not there.
	Blocked []Req
}

// Resolve plans the install of entry: depth first, no repeats, a cycle is an
// error.
func (g Graph) Resolve(entry string) (Plan, error) {
	if err := g.Check(entry); err != nil {
		return Plan{}, err
	}
	var p Plan
	done := map[string]bool{}
	onPath := map[string]bool{}
	blocked := map[string]bool{}
	var visit func(n string, path []string) error
	visit = func(n string, path []string) error {
		if onPath[n] {
			return fmt.Errorf("requires cycle: %s", strings.Join(append(path, n), " -> "))
		}
		if done[n] {
			return nil
		}
		onPath[n] = true
		e := g.Entries[n]
		for _, r := range e.Requires {
			dep, isEntry := g.Entries[r]
			if !isEntry {
				if q := g.one(r); q.State != "ready" && !blocked[r] {
					blocked[r] = true
					if q.Installable {
						p.Platforms = append(p.Platforms, r)
					} else {
						p.Blocked = append(p.Blocked, q)
					}
				}
				continue
			}
			if dep.Installed {
				continue
			}
			if err := g.Check(r); err != nil {
				return err
			}
			if err := visit(r, append(path, n)); err != nil {
				return err
			}
		}
		onPath[n] = false
		done[n] = true
		p.Steps = append(p.Steps, n)
		return nil
	}
	if err := visit(entry, nil); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// Dependants lists the installed entries that require name, sorted.
func (g Graph) Dependants(name string) []string {
	var out []string
	for _, e := range g.Entries {
		if !e.Installed || e.Name == name {
			continue
		}
		for _, r := range e.Requires {
			if r == name {
				out = append(out, e.Name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
