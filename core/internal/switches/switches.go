// Package switches reads the environment variables that turn on the Go ports.
//
// After the cutover the proxy and every route claim (D18) and the Go installer
// steps (D19) are on by default. TTS_GO_PROXY=0, TTS_GO_ROUTES=none and
// TTS_GO_STEPS=none turn them off. See
// documentation/core-rewrite/CONTRACTS.md, section "Switches".
package switches

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
)

// Names of the environment variables.
const (
	EnvProxy  = "TTS_GO_PROXY"
	EnvRoutes = "TTS_GO_ROUTES"
	EnvSteps  = "TTS_GO_STEPS"

	// EnvProxyListen is "loopback" (the default) or "lan".
	EnvProxyListen = "TTS_GO_PROXY_LISTEN"
	// EnvProxyAllowedHosts lists extra host names the proxy accepts in LAN mode.
	EnvProxyAllowedHosts = "TTS_GO_PROXY_ALLOWED_HOSTS"
)

// all is the list value that enables every known name.
const all = "all"

var nameRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Set is one parsed reading of the switches.
type Set struct {
	// Proxy is true when Go listens on port 3000 in front of Next.
	Proxy bool
	// ProxyLAN is true when the proxy listens on every interface, not only
	// loopback. Next and Gradio stay on loopback either way.
	ProxyLAN bool
	// ProxyAllowedHosts are extra host names (lower case, no port) the proxy
	// accepts in LAN mode, such as a machine name or a DNS name.
	ProxyAllowedHosts []string

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
	s := Set{Proxy: true} // D18: the proxy is on unless TTS_GO_PROXY=0

	if v, ok := lookup(EnvProxy); ok {
		switch strings.TrimSpace(v) {
		case "", "1":
		case "0":
			s.Proxy = false
		default:
			return Set{}, fmt.Errorf("%s must be 0 or 1, got %q", EnvProxy, v)
		}
	}

	var err error
	routesGiven := false
	if v, ok := lookup(EnvRoutes); ok && strings.TrimSpace(v) != "" {
		routesGiven = true
	}
	if v, _ := lookup(EnvRoutes); strings.TrimSpace(v) == none {
		// An explicit "none": no claim is on. Allowed with the proxy off.
	} else if s.routes, s.allRoutes, err = parseList(lookup, EnvRoutes); err != nil {
		return Set{}, err
	} else if s.routes[none] {
		return Set{}, fmt.Errorf("%s: invalid name %q: none cannot be combined with other names", EnvRoutes, none)
	} else if routesGiven && !s.Proxy {
		return Set{}, fmt.Errorf("%s needs the proxy: unset %s or set it to 1", EnvRoutes, EnvProxy)
	} else if !routesGiven && s.Proxy {
		s.allRoutes = true // D18: every claim is on unless TTS_GO_ROUTES says otherwise
	}
	if s.steps, s.allSteps, err = parseSteps(lookup); err != nil {
		return Set{}, err
	}

	if v, ok := lookup(EnvProxyListen); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "", "loopback":
		case "lan":
			s.ProxyLAN = true
		default:
			return Set{}, fmt.Errorf("%s must be loopback or lan, got %q", EnvProxyListen, v)
		}
	}
	if v, ok := lookup(EnvProxyAllowedHosts); ok && strings.TrimSpace(v) != "" {
		for _, raw := range strings.Split(v, ",") {
			host := strings.ToLower(strings.TrimSpace(raw))
			if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && !strings.Contains(host, "%") {
				host = ip.String() // [fe80::1] and fe80:0::1 both become fe80::1
			}
			if !validHostName(host) {
				return Set{}, fmt.Errorf("%s: invalid host %q (a host name or IP address, without a port)", EnvProxyAllowedHosts, raw)
			}
			s.ProxyAllowedHosts = append(s.ProxyAllowedHosts, host)
		}
		if !s.ProxyLAN {
			return Set{}, fmt.Errorf("%s needs %s=lan", EnvProxyAllowedHosts, EnvProxyListen)
		}
	}
	if s.ProxyLAN && !s.Proxy {
		return Set{}, fmt.Errorf("%s=lan needs the proxy (unset %s or set it to 1): only the proxy can listen on the network", EnvProxyListen, EnvProxy)
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

// none turns a list switch off: TTS_GO_ROUTES=none turns every claim off, and
// TTS_GO_STEPS=none runs the original scripts for every step.
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

var hostNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// validHostName accepts a DNS name or an IP address literal, with no port.
func validHostName(h string) bool {
	if h == "" || len(h) > 253 {
		return false
	}
	if ip := net.ParseIP(strings.Trim(h, "[]")); ip != nil {
		return !strings.Contains(h, "%")
	}
	return hostNameRE.MatchString(h)
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
