// Package detect picks the platform an install folder runs on.
package detect

import (
	"context"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/legacyenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/system"
)

// Options say how to build the platform.
type Options struct {
	Root paths.Root
	// SystemPython (--system-python) selects the system platform.
	SystemPython bool
	// Env is added to every command's environment (see legacyenv.Options.Env).
	Env []string
	// Packages returns the main env's distributions, for EntryStatus.
	Packages func(ctx context.Context) (map[string]string, error)
}

// Detect returns the platform of the folder: system when asked for, otherwise
// legacy-env. A folder with no env yet is still legacy-env, with a Status that
// says the install has not been done: that is the answer the core gave before
// platforms existed, and P4 is what makes a default install not need it.
func Detect(o Options) platform.Platform {
	if o.SystemPython {
		return system.New(system.Options{Root: o.Root, Env: o.Env, Packages: o.Packages})
	}
	return legacyenv.New(legacyenv.Options{Root: o.Root, Env: o.Env, Packages: o.Packages})
}
