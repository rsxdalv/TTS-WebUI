// Package switches reads the environment variables that turn on the Go ports.
//
// The proxy and route switches are off by default (the app runs Next exactly as
// before). TTS_GO_STEPS is on by default: TTS_GO_STEPS=none runs the scripts. See
// documentation/core-rewrite/CONTRACTS.md, section "Switches".
package switches

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Names of the environment variables.
const (
	EnvProxy  = "TTS_GO_PROXY"
	EnvRoutes = "TTS_GO_ROUTES"
	EnvSteps  = "TTS_GO_STEPS"
)

// all is the list value that enables every known name.
const all = "all"

var nameRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Set is one parsed reading of the switches.
type Set struct {
	// Proxy is true when Go listens on port 3000 in front of Next.
	Proxy bool

	routes    map[string]bool
	allRoutes bool
	steps     map[string]bool
	allSteps  bool
}

// FromEnv parses the switches from the process environment.
func FromEnv() (Set, error) { return Parse(os.LookupEnv) }

// Parse parses the switches using lookup. Unknown values are errors: a mistyped
// switch must not silently run the other code path.
func Parse(lookup func(string) (string, bool)) (Set, error) {
	var s Set

	if v, ok := lookup(EnvProxy); ok {
		switch strings.TrimSpace(v) {
		case "", "0":
		case "1":
			s.Proxy = true
		default:
			return Set{}, fmt.Errorf("%s must be 0 or 1, got %q", EnvProxy, v)
		}
	}

	var err error
	if s.routes, s.allRoutes, err = parseList(lookup, EnvRoutes); err != nil {
		return Set{}, err
	}
	if s.steps, s.allSteps, err = parseSteps(lookup); err != nil {
		return Set{}, err
	}

	if !s.Proxy && (s.allRoutes || len(s.routes) > 0) {
		return Set{}, fmt.Errorf("%s needs %s=1: routes can only be claimed by the proxy", EnvRoutes, EnvProxy)
	}
	return s, nil
}

func parseList(lookup func(string) (string, bool), env string) (map[string]bool, bool, error) {
	v, ok := lookup(env)
	if !ok || strings.TrimSpace(v) == "" {
		return nil, false, nil
	}
	names := map[string]bool{}
	everything := false
	for _, raw := range strings.Split(v, ",") {
		name := strings.TrimSpace(raw)
		if name == all {
			everything = true
			continue
		}
		if !nameRE.MatchString(name) {
			return nil, false, fmt.Errorf("%s: invalid name %q", env, name)
		}
		names[name] = true
	}
	return names, everything, nil
}

// none is the TTS_GO_STEPS value that runs the original scripts for every step.
const none = "none"

// parseSteps reads TTS_GO_STEPS. Unlike the other switches the steps are ON by
// default (decision D19): unset or empty means every Go port runs, "none" runs
// the original scripts, and a list runs only the named ports.
func parseSteps(lookup func(string) (string, bool)) (map[string]bool, bool, error) {
	v, ok := lookup(EnvSteps)
	if !ok || strings.TrimSpace(v) == "" {
		return nil, true, nil
	}
	if strings.TrimSpace(v) == none {
		return map[string]bool{}, false, nil
	}
	for _, raw := range strings.Split(v, ",") {
		if strings.TrimSpace(raw) == none {
			return nil, false, fmt.Errorf("%s: %q cannot be combined with other names", EnvSteps, none)
		}
	}
	return parseList(lookup, EnvSteps)
}

// Route reports whether the route claim called name is on.
func (s Set) Route(name string) bool { return s.allRoutes || s.routes[name] }

// Step reports whether the install step called name runs the Go port.
func (s Set) Step(name string) bool { return s.allSteps || s.steps[name] }

// CheckRoutes returns an error if a named route claim is not in known.
func (s Set) CheckRoutes(known []string) error { return check(EnvRoutes, s.routes, known) }

// CheckSteps returns an error if a named step is not in known.
func (s Set) CheckSteps(known []string) error { return check(EnvSteps, s.steps, known) }

func check(env string, names map[string]bool, known []string) error {
	for name := range names {
		found := false
		for _, k := range known {
			if k == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s: unknown name %q (known: %s)", env, name, strings.Join(known, ", "))
		}
	}
	return nil
}
