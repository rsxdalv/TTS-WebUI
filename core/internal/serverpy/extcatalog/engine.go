package extcatalog

import (
	"errors"
	"fmt"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// ErrNotEngine means the package is not an own-venv engine of the catalog.
var ErrNotEngine = errors.New("not an own-venv engine")

// ErrNotInstalled means the engine is in the catalog but not installed.
var ErrNotInstalled = errors.New("engine is not installed")

// Engine says whether pkg is an installed own-venv engine ("proxy": "native"
// with a finished venv) and returns its display name. It is the only check
// between a browser request and starting a process (D31), so it accepts a name
// from the catalog and nothing else: an unknown name, an in-process tab and an
// engine that is half installed are all refused.
func (s *Store) Engine(pkg string) (string, error) {
	e, err := s.find(pkg)
	if err != nil || e.Str("proxy") != "native" {
		return "", fmt.Errorf("%w: %q", ErrNotEngine, pkg)
	}
	if !platform.OwnVenvStatus(s.opts.Root, pkg).Installed {
		return "", fmt.Errorf("%w: %q", ErrNotInstalled, pkg)
	}
	if n := e.Str("name"); n != "" {
		return n, nil
	}
	return pkg, nil
}

// InstalledEngine is one row of the engine list.
type InstalledEngine struct {
	Package     string `json:"package"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Engines lists the installed own-venv engines, sorted by name.
func (s *Store) Engines() ([]InstalledEngine, error) {
	list, err := s.installable("interface")
	if err != nil {
		return nil, err
	}
	out := []InstalledEngine{}
	for _, x := range list {
		pkg := x.Str("package_name")
		if x.Str("proxy") == "native" && platform.OwnVenvStatus(s.opts.Root, pkg).Installed {
			out = append(out, InstalledEngine{Package: pkg, Name: x.Str("name"), Description: x.Str("description")})
		}
	}
	return out, nil
}
