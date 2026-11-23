package extcatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
)

// DecoratorsEnv is the variable that carries the resolved decorators to
// extension processes. They have no control API (no token), but their decorator
// imports are resolved at import time like the main process's.
const DecoratorsEnv = "TTS_WEBUI_DECORATORS"

// ConfigSource gives the disabled and autostart lists of config.json. It is
// appconfig.Store, the only reader and writer of that file.
type ConfigSource interface {
	ExtensionLists() (appconfig.ExtensionLists, error)
}

// Loadable is one extension Python imports and registers, in load order.
type Loadable struct {
	PackageName  string `json:"package_name"`
	Name         string `json:"name"`
	Requirements string `json:"requirements"`
	Proxy        string `json:"proxy"` // "native" runs in its own venv; "" imports in this process
	Enabled      bool   `json:"enabled"`
	Autostart    bool   `json:"autostart"`
	// Installed is check_if_package_installed: the venv of a native extension
	// exists, a built-in one, or the main env has the distribution. When pip
	// cannot be read it is true for an extension that needs pip, because
	// Python's import of the module is the real test and shows its own error.
	Installed bool `json:"installed"`
	// Version is the distribution's version in the main env, "" for a native or
	// built-in extension or when it is not installed.
	Version string `json:"version"`
}

// Resolved is the answer of Resolve.
type Resolved struct {
	Extensions []Loadable `json:"extensions"`
	// DisabledNames (decorators only) is the whole disabled_decorators list:
	// package names, which Enabled already reflects, and decorator function
	// names, which only the importing side can match.
	DisabledNames []string `json:"disabled_names,omitempty"`
}

// Resolve is the ordered list of extensions of one kind and class that Python
// loads. It is the merged list (external > catalog > base) reduced to the
// entries of that type and class, each with the state from config.json.
// Disabled entries stay in the list: the interface shows a "[Disabled]" tab and
// the decorator loader prints that it skips them. An entry without a
// package_name cannot be imported and is left out.
func (s *Store) Resolve(kind, class string) (Resolved, error) {
	list, err := s.List(kind)
	if err != nil {
		return Resolved{}, err
	}
	if class == "" {
		return Resolved{}, invalidf("extension_class is required")
	}
	if s.opts.Config == nil {
		return Resolved{}, fmt.Errorf("extcatalog: no config source configured")
	}
	lists, err := s.opts.Config.ExtensionLists()
	if err != nil {
		return Resolved{}, err
	}
	disabled := lists.Disabled
	if kind == "decorator" {
		disabled = lists.DisabledDecorators
	}
	out := Resolved{Extensions: []Loadable{}}
	installs := s.Installs(context.Background())
	if installs.Err != nil {
		s.opts.Logf("extcatalog: cannot read the installed packages, assuming they are installed: %v", installs.Err)
	}
	for _, x := range list {
		// The "extension_type" of an entry is the kind of the list it belongs to.
		if x.Str("extension_type") != kind || x.Str("extension_class") != class {
			continue
		}
		pkg := x.Str("package_name")
		if pkg == "" {
			s.opts.Logf("extcatalog: skipping an extension without package_name in the %s list", kind)
			continue
		}
		name := x.Str("name")
		if name == "" {
			name = pkg
		}
		l := Loadable{
			PackageName:  pkg,
			Name:         name,
			Requirements: x.Str("requirements"),
			Proxy:        x.Str("proxy"),
			Enabled:      !slices.Contains(disabled, pkg),
		}
		l.Installed, l.Version = installs.Of(pkg, l.Proxy)
		if installs.Err != nil && NeedsPip(pkg, l.Proxy) {
			l.Installed = true
		}
		if kind == "interface" {
			l.Autostart = slices.Contains(lists.Autostart, pkg)
		}
		out.Extensions = append(out.Extensions, l)
	}
	if kind == "decorator" {
		out.DisabledNames = lists.DisabledDecorators
	}
	return out, nil
}

// DecoratorEnv is the environment variable that hands the resolved decorators
// (both classes) to an extension process: {"outer": Resolved, "inner": Resolved}.
func (s *Store) DecoratorEnv() ([]string, error) {
	both := map[string]Resolved{}
	for _, class := range []string{"outer", "inner"} {
		r, err := s.Resolve("decorator", class)
		if err != nil {
			return nil, err
		}
		both[class] = r
	}
	b, err := json.Marshal(both)
	if err != nil {
		return nil, err
	}
	return []string{DecoratorsEnv + "=" + string(b)}, nil
}
