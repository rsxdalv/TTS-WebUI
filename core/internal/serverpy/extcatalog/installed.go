package extcatalog

import (
	"context"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/piplist"
)

// PackageSource is what Resolve asks for the distributions of the main env. It
// is piplist.Store; Invalidate is called after an install or an uninstall.
type PackageSource interface {
	Cached(ctx context.Context) ([]piplist.Package, error)
	Invalidate()
}

func normalizeDist(s string) string { return platform.NormalizeDist(s) }

// Installs says which extensions are installed. It is what
// check_if_package_installed and _get_version answered in Python.
type Installs struct {
	root  string
	dists map[string]string // normalised distribution name -> version
	// Err is why pip could not be read. Of then says not installed for an
	// extension that needs pip, and the caller decides what that means.
	Err error
}

// Installs reads the main env once (cached) and returns the lookup. When pip
// cannot be read it returns a lookup with Err set, never nil.
func (s *Store) Installs(ctx context.Context) Installs {
	in := Installs{root: s.opts.Root, dists: map[string]string{}}
	if s.opts.Pip == nil {
		return in
	}
	if s.hostActive() && !s.opts.Platform.Status(ctx).Ready {
		// No Gradio host: no in-process tab is installed, which is an answer,
		// not an error.
		return in
	}
	pkgs, err := s.opts.Pip.Cached(ctx)
	if err != nil {
		in.Err = err
		return in
	}
	for _, p := range pkgs {
		in.dists[normalizeDist(p.Name)] = p.Version
	}
	return in
}

// NeedsPip is whether Of depends on the pip list for this extension.
func NeedsPip(pkg, proxy string) bool {
	return proxy != "native" && !isBuiltin(pkg)
}

func isBuiltin(pkg string) bool { return platform.Builtin(pkg) }

// Of answers for one extension: a native one is installed when its venv
// exists (it has no version here), a built-in one always is, any other when
// the main env has the distribution (names compared after normalisation, not
// by find_spec) and then it has that distribution's version.
func (in Installs) Of(pkg, proxy string) (installed bool, version string) {
	st := platform.InstalledIn(in.root, platform.Entry{Package: pkg, Own: proxy == "native"}, in.dists)
	return st.Installed, st.Version
}
