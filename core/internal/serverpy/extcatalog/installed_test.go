package extcatalog

import (
	"context"
	"errors"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/piplist"
)

type fakePip struct {
	pkgs        []piplist.Package
	err         error
	calls       int
	invalidated int
}

func (f *fakePip) Cached(context.Context) ([]piplist.Package, error) {
	f.calls++
	return f.pkgs, f.err
}
func (f *fakePip) Invalidate() { f.invalidated++ }

func TestInstallsOf(t *testing.T) {
	s, root := newStore(t)
	s.opts.Pip = &fakePip{pkgs: []piplist.Package{
		{Name: "tts-webui-extension.mms", Version: "1.2.3"},
		{Name: "Some_Other.Pkg", Version: "4"},
	}}
	finishedVenv(t, root, "native.ext")
	in := s.Installs(context.Background())
	for _, c := range []struct {
		pkg, proxy string
		installed  bool
		version    string
	}{
		// pip's normalisation: case and runs of - _ . are one separator
		{"tts_webui_extension.mms", "", true, "1.2.3"},
		{"some-other-pkg", "", true, "4"},
		{"not.there", "", false, ""},
		// a built-in one is always installed and has no pip version
		{"extensions.builtin.extension_x", "", true, ""},
		// a native one is its venv
		{"native.ext", "native", true, ""},
		{"native.missing", "native", false, ""},
		// a native one is not looked up in pip even if the name matches
		{"tts_webui_extension.mms", "native", false, ""},
	} {
		got, v := in.Of(c.pkg, c.proxy)
		if got != c.installed || v != c.version {
			t.Errorf("Of(%q, %q) = %v, %q; want %v, %q", c.pkg, c.proxy, got, v, c.installed, c.version)
		}
	}
}

func TestResolveSaysInstalledAndVersion(t *testing.T) {
	s := resolveStore(t, fakeConfig{})
	fp := &fakePip{pkgs: []piplist.Package{{Name: "base.tts", Version: "0.9"}}}
	s.opts.Pip = fp
	finishedVenv(t, s.opts.Root, "base.tool")
	tts, err := s.Resolve("interface", "text-to-speech")
	if err != nil {
		t.Fatal(err)
	}
	var base, off, ext *Loadable
	for i := range tts.Extensions {
		switch tts.Extensions[i].PackageName {
		case "base.tts":
			base = &tts.Extensions[i]
		case "base.off":
			off = &tts.Extensions[i]
		case "ext.tts":
			ext = &tts.Extensions[i]
		}
	}
	if base == nil || !base.Installed || base.Version != "0.9" {
		t.Errorf("base.tts: %+v", base)
	}
	if off == nil || off.Installed || off.Version != "" {
		t.Errorf("base.off is not in pip: %+v", off)
	}
	if ext == nil || ext.Installed {
		t.Errorf("ext.tts: %+v", ext)
	}
	tools, err := s.Resolve("interface", "tools")
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Extensions) != 1 || !tools.Extensions[0].Installed || tools.Extensions[0].Version != "" {
		t.Errorf("a native extension with a venv: %+v", tools.Extensions)
	}
	if fp.calls != 2 {
		t.Errorf("one pip read per Resolve, got %d for two", fp.calls)
	}
}

func TestResolveAssumesInstalledWhenPipCannotBeRead(t *testing.T) {
	s := resolveStore(t, fakeConfig{lists: appconfig.ExtensionLists{}})
	s.opts.Pip = &fakePip{err: errors.New("pip exploded")}
	got, err := s.Resolve("interface", "text-to-speech")
	if err != nil {
		t.Fatalf("a pip failure must not fail the list: %v", err)
	}
	for _, l := range got.Extensions {
		if !l.Installed || l.Version != "" {
			t.Errorf("%s: %+v (Python's import is the real test, so it is tried)", l.PackageName, l)
		}
	}
	// a native extension without its venv is still not installed
	tools, err := s.Resolve("interface", "tools")
	if err != nil {
		t.Fatal(err)
	}
	if tools.Extensions[0].Installed {
		t.Errorf("native without a venv: %+v", tools.Extensions[0])
	}
}

func TestInstallOperationsInvalidateTheCache(t *testing.T) {
	s, root, _ := installStore(t)
	fp := &fakePip{}
	s.opts.Pip = fp
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("ext.pip", "Pip Ext")+`]}`)
	for _, op := range []string{"install", "uninstall"} {
		before := fp.invalidated
		if _, got, _ := stream(t, s, op+` {"package_name":"ext.pip"}`); !got.ok {
			t.Fatalf("%s: %+v", op, got)
		}
		if fp.invalidated != before+1 {
			t.Errorf("%s did not invalidate the pip cache", op)
		}
	}
	before := fp.invalidated
	if _, got, _ := stream(t, s, `check-dependencies {"package_name":"ext.pip"}`); !got.ok {
		t.Fatalf("check: %+v", got)
	}
	if fp.invalidated != before {
		t.Error("a dry run changes nothing and must keep the cache")
	}
}
