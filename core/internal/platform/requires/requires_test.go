package requires

import (
	"reflect"
	"strings"
	"testing"
)

func graph(provided map[string]string, es ...Entry) Graph {
	g := Graph{Entries: map[string]Entry{}, Provided: provided}
	for _, e := range es {
		g.Entries[e.Name] = e
	}
	return g
}

func TestDerive(t *testing.T) {
	if got := Derive(true); !reflect.DeepEqual(got, []string{"uv"}) {
		t.Errorf("own venv: %v", got)
	}
	if got := Derive(false); !reflect.DeepEqual(got, []string{"gradio-host"}) {
		t.Errorf("in process: %v", got)
	}
}

func TestValidate(t *testing.T) {
	if err := Validate([]string{"uv", "tts_webui_extension.bark", "a-b"}); err != nil {
		t.Errorf("valid: %v", err)
	}
	for _, bad := range [][]string{{""}, {"-x"}, {"a b"}, {"a/b"}, {"a;b"}, {"uv", "uv"}, {"git+https://x"}} {
		if err := Validate(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestStateReadyAndMissing(t *testing.T) {
	prov := map[string]string{"uv": "legacy-env", "gradio-host": "legacy-env"}
	g := graph(prov, Entry{Name: "eng", Requires: []string{"uv"}}, Entry{Name: "tab", Requires: []string{"gradio-host"}})
	if r := g.State("eng")[0]; r.State != "ready" || r.Via != "legacy-env" || r.Kind != "platform" {
		t.Errorf("uv on legacy-env: %+v", r)
	}
	if r := g.State("tab")[0]; r.State != "ready" || r.Kind != "entry" {
		t.Errorf("gradio-host: %+v", r)
	}
	// A new folder: nothing is provided, and the answer says what would be installed.
	g.Provided = nil
	r := g.State("eng")[0]
	if r.State != "missing" || !strings.Contains(r.Would, "uv platform") {
		t.Errorf("new folder: %+v", r)
	}
}

func TestCheckNamesEntryAndName(t *testing.T) {
	g := graph(nil, Entry{Name: "e", Requires: []string{"nope"}})
	err := g.Check("e")
	if err == nil || !strings.Contains(err.Error(), `e: requires "nope"`) {
		t.Fatalf("got %v", err)
	}
	if err := graph(nil, Entry{Name: "e", Requires: []string{"uv", "gradio-host"}}).Check("e"); err != nil {
		t.Fatal(err)
	}
}

func TestResolveInstallsDependenciesFirst(t *testing.T) {
	prov := map[string]string{"uv": "legacy-env"}
	g := graph(prov,
		Entry{Name: "top", Requires: []string{"mid", "uv"}},
		Entry{Name: "mid", Requires: []string{"base"}},
		Entry{Name: "base", Requires: []string{"uv"}},
		Entry{Name: "done", Requires: []string{"uv"}, Installed: true},
	)
	p, err := g.Resolve("top")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Steps, []string{"base", "mid", "top"}) || len(p.Blocked) != 0 {
		t.Fatalf("plan = %+v", p)
	}
	g.Entries["base"] = Entry{Name: "base", Requires: []string{"uv"}, Installed: true}
	if p, _ = g.Resolve("top"); !reflect.DeepEqual(p.Steps, []string{"mid", "top"}) {
		t.Fatalf("with base installed: %+v", p)
	}
}

func TestResolveBlockedByMissingPlatform(t *testing.T) {
	g := graph(nil, Entry{Name: "e", Requires: []string{"uv"}})
	p, err := g.Resolve("e")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Blocked) != 1 || p.Blocked[0].Name != "uv" || !strings.Contains(p.Blocked[0].Would, "uv platform") {
		t.Fatalf("blocked = %+v", p.Blocked)
	}
}

func TestResolveCycleAndUnknown(t *testing.T) {
	g := graph(nil, Entry{Name: "a", Requires: []string{"b"}}, Entry{Name: "b", Requires: []string{"a"}})
	if _, err := g.Resolve("a"); err == nil || !strings.Contains(err.Error(), "cycle: a -> b -> a") {
		t.Fatalf("cycle: %v", err)
	}
	g = graph(nil, Entry{Name: "a", Requires: []string{"zz"}})
	if _, err := g.Resolve("a"); err == nil || !strings.Contains(err.Error(), `"zz"`) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestDependantsOnlyInstalled(t *testing.T) {
	g := graph(nil,
		Entry{Name: "base", Installed: true},
		Entry{Name: "y", Requires: []string{"base"}, Installed: true},
		Entry{Name: "x", Requires: []string{"base"}, Installed: true},
		Entry{Name: "n", Requires: []string{"base"}},
	)
	if got := g.Dependants("base"); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Fatalf("got %v", got)
	}
}
