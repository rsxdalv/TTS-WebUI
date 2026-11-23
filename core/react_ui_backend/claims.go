// Package react_ui_backend is the Go server for the React UI on port 3000. It
// serves the static export (react-ui/out, built by Vite) and answers the API
// routes claimed by the files in this package. See
// documentation/core-rewrite/CONTRACTS.md, sections 4 to 7 and 10.
package react_ui_backend

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Deps is what a claimed route receives.
type Deps struct {
	// Root is the absolute install root.
	Root string
	// GradioBackend is the Gradio address: the process environment, then the
	// react-ui env files.
	GradioBackend string
	// GradioAuth is user:password for Gradio, or empty.
	GradioAuth string
}

// Claim is an API route answered by Go.
type Claim struct {
	// Switch is the name used in TTS_GO_ROUTES.
	Switch string
	// Patterns are net/http ServeMux patterns.
	Patterns []string
	// Handler builds the route's handler.
	Handler func(Deps) http.Handler
}

var (
	claimsMu sync.Mutex
	claims   []Claim
)

// RegisterClaim adds a route. Call it from init() in the route's own file.
func RegisterClaim(c Claim) {
	if c.Switch == "" || len(c.Patterns) == 0 || c.Handler == nil {
		panic("react_ui_backend: a claim needs a switch, patterns and a handler")
	}
	claimsMu.Lock()
	defer claimsMu.Unlock()
	for _, existing := range claims {
		if existing.Switch == c.Switch {
			panic("react_ui_backend: duplicate claim " + c.Switch)
		}
	}
	claims = append(claims, c)
}

// registeredClaims returns the registered claims sorted by switch name.
func registeredClaims() []Claim {
	claimsMu.Lock()
	defer claimsMu.Unlock()
	out := slices.Clone(claims)
	slices.SortFunc(out, func(a, b Claim) int { return strings.Compare(a.Switch, b.Switch) })
	return out
}

// ClaimNames returns the switch names of all registered claims, for checking
// TTS_GO_ROUTES before anything starts.
func ClaimNames() []string { return claimNames() }

// claimNames returns the switch names of all registered claims.
func claimNames() []string {
	var names []string
	for _, c := range registeredClaims() {
		names = append(names, c.Switch)
	}
	return names
}

// newDeps resolves the settings claimed routes share.
func newDeps(root string) Deps {
	lookup := envLookup(root)
	return Deps{
		Root:          root,
		GradioBackend: gradioBackend(lookup),
		GradioAuth:    lookup("GRADIO_AUTH"),
	}
}

// gradioBackend matches react-ui/src/backend-utils/defaultBackend.tsx.
func gradioBackend(lookup func(string) string) string {
	for _, name := range []string{"GRADIO_BACKEND", "GRADIO_BACKEND_AUTOMATIC"} {
		if v := lookup(name); v != "" {
			return v
		}
	}
	return "http://127.0.0.1:7770/"
}

// envLookup reads a setting from the process environment first, then from the
// react-ui env files (an install from the Next.js days may still have them), in
// this order: .env.production.local, .env.local, .env.production, .env.
func envLookup(root string) func(string) string {
	files := []string{".env.production.local", ".env.local", ".env.production", ".env"}
	values := map[string]string{}
	for _, name := range files {
		for k, v := range readEnvFile(filepath.Join(root, "react-ui", name)) {
			if _, seen := values[k]; !seen {
				values[k] = v
			}
		}
	}
	return func(name string) string {
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return values[name]
	}
}

// readEnvFile parses KEY=VALUE lines. A missing file has no values.
func readEnvFile(path string) map[string]string {
	values := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return values
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		} else if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		values[strings.TrimSpace(key)] = value
	}
	return values
}

// claimPrefix returns the path of a {name...} pattern without its trailing
// slash, or "" for any other pattern.
func claimPrefix(pattern string) (string, error) {
	path := pattern
	if _, after, ok := strings.Cut(pattern, " "); ok {
		path = strings.TrimSpace(after)
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("claim pattern %q must start with a method or a path", pattern)
	}
	i := strings.LastIndex(path, "/{")
	if i < 0 || !strings.HasSuffix(path, "...}") {
		return "", nil
	}
	return path[:i], nil
}
