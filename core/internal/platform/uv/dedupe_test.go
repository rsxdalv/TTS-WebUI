package uv

import (
	"os"
	"testing"
)

func TestDedupeEnvKeepsTheToolsPathOnWindows(t *testing.T) {
	env := []string{"Path=C:/Windows", "FOO=1", "PATH=C:/tools;C:/Windows", "foo=2"}
	got := dedupeEnv("windows", env)
	if len(got) != 2 || got[0] != "PATH=C:/tools;C:/Windows" || got[1] != "foo=2" {
		t.Fatalf("got %v", got)
	}
	// Elsewhere names are case sensitive.
	if got := dedupeEnv("linux", []string{"Path=a", "PATH=b"}); len(got) != 2 {
		t.Fatalf("linux: %v", got)
	}
}

func TestWithToolsFirstLeavesOnePath(t *testing.T) {
	sep := string(os.PathListSeparator)
	env := []string{"Path=a" + sep + "b", "FOO=1"}
	got := withToolsFirst("windows", env, []string{"tools1", "tools2"})
	want := "PATH=tools1" + sep + "tools2" + sep + "a" + sep + "b"
	if len(got) != 2 || got[0] != "FOO=1" || got[1] != want {
		t.Fatalf("got %v, want PATH %q", got, want)
	}
	if got := withToolsFirst("windows", env, nil); len(got) != 2 || got[0] != "Path=a"+sep+"b" {
		t.Fatalf("no tools must leave the environment alone: %v", got)
	}
}
