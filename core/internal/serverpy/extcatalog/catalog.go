// Package extcatalog is the Go side of the extension list tabs and the Custom
// Extensions Installer: it reads and merges the extension files, validates and
// records new entries, and runs the pip / uv installs, streaming their output.
// Python reaches it through tts_webui/core_bridge (bridge.extcatalog).
//
// It also decides which extensions Python loads, in which order and in which
// state (resolve.go). The merge is the one extensions_data_loader.py did (the file is
// gone; the launcher's Extensions page uses this package with the app stopped).
package extcatalog

import (
	"errors"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/system"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	baseFile     = "extensions.json"
	externalFile = "extensions.external.json"
)

var catalogFile = filepath.Join("data", "extensions-catalog", "lib", "extensions.json")

// Options configures a Store.
type Options struct {
	Root  string // project root: holds extensions.json, .venvs/, installer_scripts/
	Token string // required; requests without it get 403
	// Platform is the runtime installs run on (the main env's python and uv).
	Platform platform.Platform
	// Config supplies the disabled and autostart lists for Resolve (appconfig.Store).
	Config ConfigSource
	// Pip lists the main env's distributions for Resolve's installed and
	// version (piplist.Store). Nil means no distribution is known.
	Pip  PackageSource
	Logf func(format string, args ...any)
}

// Store serves the extcatalog API for one project root.
type Store struct {
	opts    Options
	mu      sync.Mutex // serialises writes to extensions.external.json
	install sync.Mutex // one install at a time
}

func New(o Options) *Store {
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	if o.Platform == nil {
		// Installs run the python and uv on PATH, like a store with no prefix did.
		if root, err := paths.New(o.Root); err == nil {
			o.Platform = system.New(system.Options{Root: root})
		}
	}
	return &Store{opts: o}
}

// load reads one extension file. A missing or unreadable file is an empty
// document, like load_json_file (which prints and returns {}).
func (s *Store) load(rel string) *Obj {
	b, err := os.ReadFile(filepath.Join(s.opts.Root, rel))
	if err != nil {
		if !os.IsNotExist(err) {
			s.opts.Logf("extcatalog: cannot read %s: %v", rel, err)
		}
		return newObj()
	}
	v, err := Parse(b)
	o, ok := v.(*Obj)
	if err != nil || !ok {
		s.opts.Logf("extcatalog: %s is not a JSON object", rel)
		return newObj()
	}
	return o
}

func entries(v any) []*Obj {
	arr, _ := v.([]any)
	var out []*Obj
	for _, e := range arr {
		if o, ok := e.(*Obj); ok {
			out = append(out, o)
		}
	}
	return out
}

// dedupe keeps the first entry per package_name; entries without one are kept.
func dedupe(in []*Obj) []*Obj {
	seen := map[string]bool{}
	var out []*Obj
	for _, e := range in {
		if pkg := e.Str("package_name"); pkg != "" {
			if seen[pkg] {
				continue
			}
			seen[pkg] = true
		}
		out = append(out, e)
	}
	return out
}

// flattenTabs is _flatten_interface_tabs: "tabs" then every list in "tabsInGroups".
func flattenTabs(doc *Obj) []*Obj {
	v, _ := doc.Get("tabs")
	out := entries(v)
	if g, ok := doc.Get("tabsInGroups"); ok {
		if groups, ok := g.(*Obj); ok {
			for _, k := range groups.Keys {
				out = append(out, entries(groups.Vals[k])...)
			}
		}
	}
	return dedupe(out)
}

// Interface is get_interface_extensions: external > catalog > base.
func (s *Store) Interface() []*Obj {
	base, cat, ext := s.load(baseFile), s.load(catalogFile), s.load(externalFile)
	return dedupe(append(append(flattenTabs(ext), flattenTabs(cat)...), flattenTabs(base)...))
}

// Decorators is get_decorator_extensions: external > catalog > base.
func (s *Store) Decorators() []*Obj {
	var all []*Obj
	for _, f := range []string{externalFile, catalogFile, baseFile} {
		v, _ := s.load(f).Get("decorators")
		all = append(all, entries(v)...)
	}
	return dedupe(all)
}

// List returns the merged list for kind "interface" or "decorator".
func (s *Store) List(kind string) ([]*Obj, error) {
	switch kind {
	case "interface":
		return s.Interface(), nil
	case "decorator":
		return s.Decorators(), nil
	}
	return nil, invalidf("kind must be interface or decorator, got %q", kind)
}

// Table is the Markdown table of the list tabs.
func (s *Store) Table(kind string) (string, error) {
	list, err := s.List(kind)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("| Title | Description |\n| --- | --- |\n")
	for _, x := range list {
		fmt.Fprintf(&b, "| %s | %s (website: %s) (extension_website: %s) |\n",
			x.Str("name"), x.Str("description"), x.Str("website"), x.Str("extension_website"))
	}
	return b.String(), nil
}

// installable lists the extensions the install and uninstall dropdowns offer:
// everything that is not built in; the interface list is sorted by name.
func (s *Store) installable(kind string) ([]*Obj, error) {
	list, err := s.List(kind)
	if err != nil {
		return nil, err
	}
	var out []*Obj
	for _, x := range list {
		if !strings.Contains(x.Str("package_name"), "builtin") {
			out = append(out, x)
		}
	}
	if kind == "interface" {
		sort.SliceStable(out, func(i, j int) bool {
			return strings.ToLower(out[i].Str("name")) < strings.ToLower(out[j].Str("name"))
		})
	}
	return out, nil
}

// Choices are the package names of the install and uninstall dropdowns.
func (s *Store) Choices(kind string) ([]string, error) {
	list, err := s.installable(kind)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, x := range list {
		out = append(out, x.Str("package_name"))
	}
	return out, nil
}

// find returns the installable (non-builtin) extension with this package name,
// looking in the interface list first, then the decorators.
func (s *Store) find(pkg string) (*Obj, error) {
	for _, kind := range []string{"interface", "decorator"} {
		list, _ := s.installable(kind)
		for _, x := range list {
			if x.Str("package_name") == pkg {
				return x, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: no installable extension %q", errNotFound, pkg)
}

var (
	errInvalid  = errors.New("invalid")
	errNotFound = errors.New("not found")
	errBusy     = errors.New("busy")
)

func invalidf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errInvalid, fmt.Sprintf(format, a...))
}
