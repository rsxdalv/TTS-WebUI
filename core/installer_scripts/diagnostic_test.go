package installer_scripts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// originalLines returns the command lines of an original script: blank lines,
// comments and the lines named in skip are dropped.
func originalLines(t *testing.T, rel string, skip func(string) bool) []string {
	b, err := os.ReadFile(filepath.Join("..", "..", "installer_scripts", "tools", rel))
	if err != nil {
		t.Skipf("original script not available: %v", err)
	}
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "@REM") || strings.HasPrefix(l, "#") || skip(l) {
			continue
		}
		out = append(out, l)
	}
	return out
}

func TestDiagnosticWindowsMatchesOriginalLines(t *testing.T) {
	want := originalLines(t, "diagnostic.bat", func(l string) bool {
		return l == "@echo off" || strings.HasPrefix(l, "cd %~dp0") || strings.HasPrefix(l, "call .\\installer_scripts\\activate.bat")
	})
	var got []string
	for _, l := range diagnosticWindows() {
		got = append(got, l.line)
	}
	compareLines(t, got, want)
}

func TestDiagnosticUnixMatchesOriginalLines(t *testing.T) {
	want := originalLines(t, "diagnostic_inner.sh", func(l string) bool {
		return strings.HasPrefix(l, "ROOT_DIR=") || strings.HasPrefix(l, "cd \"$ROOT_DIR\"") || strings.HasPrefix(l, "source ")
	})
	var got []string
	for _, l := range diagnosticUnix() {
		got = append(got, l.line)
	}
	compareLines(t, got, want)
}

func compareLines(t *testing.T, got, want []string) {
	if len(got) != len(want) {
		t.Errorf("%d lines, original has %d", len(got), len(want))
	}
	for i := 0; i < len(got) && i < len(want); i++ {
		if got[i] != want[i] {
			t.Fatalf("line %d:\n got  %q\n want %q", i, got[i], want[i])
		}
	}
}

func TestDiagnosticActivationBoundary(t *testing.T) {
	// Windows: lines before activate.bat run in the plain environment, the rest activated.
	var firstActivated, plain int
	for i, l := range diagnosticWindows() {
		if l.activated {
			if firstActivated == 0 {
				firstActivated = i
			}
		} else {
			if firstActivated != 0 {
				t.Fatalf("line %d is not activated after the boundary", i)
			}
			plain++
		}
	}
	if plain != 18 || firstActivated != 18 {
		t.Fatalf("plain=%d firstActivated=%d", plain, firstActivated)
	}
	// Unix: the whole inner script runs under micromamba run.
	for i, l := range diagnosticUnix() {
		if !l.activated {
			t.Fatalf("unix line %d is not activated", i)
		}
	}
}

func TestDiagnosticRunsEveryLineAndLogsOnUnix(t *testing.T) {
	fr := &diagFakeRunner{}
	var lines []string
	s := &Step{Root: t.TempDir(), Run: fr, Log: func(l string) { lines = append(lines, l) }}
	if code, err := diagnostic(context.Background(), s, "windows"); code != 0 || err != nil || len(fr.cmds) != len(diagnosticWindows()) {
		t.Fatalf("windows: code=%d err=%v cmds=%d", code, err, len(fr.cmds))
	}
	if len(lines) != 0 {
		t.Fatalf("windows logs nothing itself: %q", lines)
	}
	fr.cmds = nil
	if code, err := diagnostic(context.Background(), s, "linux"); code != 0 || err != nil || len(fr.cmds) != len(diagnosticUnix()) {
		t.Fatalf("linux: code=%d err=%v cmds=%d", code, err, len(fr.cmds))
	}
	if strings.Join(lines, "|") != "Generating diagnostic...|Diagnostic generated." {
		t.Fatalf("log: %q", lines)
	}
}

type diagFakeRunner struct{ cmds []Command }

func (f *diagFakeRunner) Run(ctx context.Context, c Command) (Result, error) {
	f.cmds = append(f.cmds, c)
	return Result{}, nil
}
