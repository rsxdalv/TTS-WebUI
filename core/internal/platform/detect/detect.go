// Package detect picks the platform an install folder runs on.
package detect

import (
	"context"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/gradiohost"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/legacyenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/system"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/uv"
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

// Detect returns the platform of the folder, chosen on every call it answers:
//
//   - system when asked for;
//   - legacy-env when the folder has the existing main env (it is adopted and
//     keeps running as it did before platforms existed);
//   - otherwise the Gradio host, which the user installs from the Extensions page
//     (P3). Until then its Status says so and the core runs without Gradio.
//
// The choice is made at each call, not once, so a folder whose env the legacy
// installer step creates while the launcher is open is legacy-env from then on.
func Detect(o Options) platform.Platform {
	a := &auto{
		system: system.New(system.Options{Root: o.Root, Env: o.Env, Packages: o.Packages}),
		legacy: legacyenv.New(legacyenv.Options{Root: o.Root, Env: o.Env, Packages: o.Packages}),
		host: gradiohost.New(gradiohost.Options{Root: o.Root, UV: uv.New(uv.Options{Root: o.Root}),
			Env: o.Env, Packages: o.Packages}),
		root: o.Root, systemPython: o.SystemPython,
	}
	return a
}

// Host returns the Gradio host entry's platform when p is a Detect result that
// is not on system or legacy-env, which are adopted; nil otherwise. The Gradio
// host can be installed and removed only there.
func Host(p platform.Platform) *gradiohost.Platform {
	if a, ok := p.(*auto); ok && a.cur() == platform.Platform(a.host) {
		return a.host
	}
	return nil
}

// auto is the platform Detect returns. Every method asks the platform that is
// current now.
type auto struct {
	system       *system.Platform
	legacy       *legacyenv.Platform
	host         *gradiohost.Platform
	root         paths.Root
	systemPython bool
}

var _ platform.Platform = (*auto)(nil)

func (a *auto) cur() platform.Platform {
	switch {
	case a.systemPython:
		return a.system
	}
	if legacyenv.Present(a.root) {
		return a.legacy
	}
	return a.host
}

func (a *auto) Name() string                               { return a.cur().Name() }
func (a *auto) Provides() []string                         { return a.cur().Provides() }
func (a *auto) Status(ctx context.Context) platform.Status { return a.cur().Status(ctx) }
func (a *auto) Install(ctx context.Context, emit platform.Emit) error {
	return a.cur().Install(ctx, emit)
}
func (a *auto) Repair(ctx context.Context, emit platform.Emit) error {
	return a.cur().Repair(ctx, emit)
}
func (a *auto) Uninstall(ctx context.Context, emit platform.Emit) error {
	return a.cur().Uninstall(ctx, emit)
}
func (a *auto) Create(ctx context.Context, e platform.Entry, emit platform.Emit) bool {
	return a.cur().Create(ctx, e, emit)
}
func (a *auto) Check(ctx context.Context, e platform.Entry, emit platform.Emit) bool {
	return a.cur().Check(ctx, e, emit)
}
func (a *auto) EntryStatus(ctx context.Context, e platform.Entry) platform.EntryStatus {
	return a.cur().EntryStatus(ctx, e)
}
func (a *auto) Remove(ctx context.Context, e platform.Entry, emit platform.Emit) bool {
	return a.cur().Remove(ctx, e, emit)
}
func (a *auto) Command(e *platform.Entry, argv []string) (platform.Command, error) {
	return a.cur().Command(e, argv)
}

// WithEnv returns a copy whose commands also carry env.
func (a *auto) WithEnv(env []string) platform.Platform {
	b := *a
	b.system = a.system.WithEnv(env).(*system.Platform)
	b.legacy = a.legacy.WithEnv(env).(*legacyenv.Platform)
	b.host = a.host.WithEnv(env).(*gradiohost.Platform)
	return &b
}
