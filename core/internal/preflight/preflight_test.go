package preflight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// fakeRoot creates a temp directory that looks like a TTS WebUI install root.
func fakeRoot(t *testing.T, dir string) paths.Root {
	t.Helper()
	base := dir
	if base == "" {
		base = t.TempDir()
	}
	if err := os.MkdirAll(filepath.Join(base, "installer_scripts"), 0o755); err != nil {
		t.Fatalf("creating installer_scripts: %v", err)
	}
	root, err := paths.New(base)
	if err != nil {
		t.Fatalf("paths.New: %v", err)
	}
	return root
}

// fixedRunner returns a Runner that always returns the given outputs.
func fixedRunner(stdout, stderr string, exitCode int) Runner {
	return func(_ context.Context, _ string, _ ...string) (string, string, int, error) {
		return stdout, stderr, exitCode, nil
	}
}

// dispatchRunner returns a Runner that picks from a map[command]→fixed outputs.
func dispatchRunner(table map[string]struct {
	stdout, stderr string
	exit           int
}) Runner {
	return func(_ context.Context, name string, args ...string) (string, string, int, error) {
		key := name
		if v, ok := table[key]; ok {
			return v.stdout, v.stderr, v.exit, nil
		}
		return "", "", 1, nil
	}
}

func TestSpacesInPath(t *testing.T) {
	tests := []struct {
		name   string
		dir    string
		status CheckStatus
	}{
		{"no spaces", t.TempDir(), StatusPass},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := fakeRoot(t, tc.dir)
			c := checkSpacesInPath(root)
			if c.Status != tc.status {
				t.Errorf("status = %q, want %q (msg: %s)", c.Status, tc.status, c.Message)
			}
		})
	}

	t.Run("with space", func(t *testing.T) {
		// Build a path that contains a space without actually creating it.
		c := checkSpacesInPath(mustFakeRootWithDir(t, "path with space"))
		if c.Status != StatusFail {
			t.Errorf("expected fail, got %q", c.Status)
		}
	})
}

// mustFakeRootWithDir creates a Root whose Dir() contains the given suffix.
// Used only to test the space-in-path check without creating real dirs.
func mustFakeRootWithDir(t *testing.T, suffix string) paths.Root {
	t.Helper()
	base := filepath.Join(t.TempDir(), suffix)
	if err := os.MkdirAll(filepath.Join(base, "installer_scripts"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	root, err := paths.New(base)
	if err != nil {
		t.Fatalf("paths.New: %v", err)
	}
	return root
}

func TestSystem32InPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only check")
	}
	tests := []struct {
		name   string
		suffix string
		status CheckStatus
	}{
		{"normal", "normal", StatusPass},
		{"system32 lower", `windows\system32\app`, StatusFail},
		{"system32 mixed", "Windows/System32/app", StatusFail},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := mustFakeRootWithDir(t, tc.suffix)
			c := checkSystem32InPath(root)
			if c.Status != tc.status {
				t.Errorf("status = %q, want %q", c.Status, tc.status)
			}
		})
	}
}

func TestExFAT(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only check")
	}
	ctx := context.Background()
	root := fakeRoot(t, "")

	t.Run("ntfs pass", func(t *testing.T) {
		c, err := checkExFAT(ctx, root, fixedRunner("NTFS\n", "", 0))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusPass {
			t.Errorf("got %q", c.Status)
		}
	})
	t.Run("exfat fail", func(t *testing.T) {
		c, err := checkExFAT(ctx, root, fixedRunner("exFAT\n", "", 0))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusFail {
			t.Errorf("got %q", c.Status)
		}
	})
}

func TestLongPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only check")
	}
	ctx := context.Background()
	root := fakeRoot(t, "")

	t.Run("enabled", func(t *testing.T) {
		c, err := checkLongPaths(ctx, root, fixedRunner("1\n", "", 0))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusPass {
			t.Errorf("got %q", c.Status)
		}
	})
	t.Run("disabled", func(t *testing.T) {
		c, err := checkLongPaths(ctx, root, fixedRunner("0\n", "", 0))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusWarn {
			t.Errorf("got %q, want warn", c.Status)
		}
		if c.FixAction != "FixLongPaths" {
			t.Errorf("FixAction = %q, want FixLongPaths", c.FixAction)
		}
	})
}

func TestConflictingConda(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only check")
	}
	ctx := context.Background()
	root := fakeRoot(t, "")

	t.Run("no conda", func(t *testing.T) {
		c, err := checkConflictingConda(ctx, root, fixedRunner("", "", 1))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusPass {
			t.Errorf("got %q", c.Status)
		}
	})
	t.Run("ours only", func(t *testing.T) {
		// Path under installer_files/ — no conflict.
		condaPath := filepath.Join(root.InstallerFiles(), "mamba", "condabin", "conda") + "\n"
		c, err := checkConflictingConda(ctx, root, fixedRunner(condaPath, "", 0))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusPass {
			t.Errorf("got %q (msg: %s)", c.Status, c.Message)
		}
	})
	t.Run("system conda", func(t *testing.T) {
		c, err := checkConflictingConda(ctx, root, fixedRunner(`C:\Users\user\miniconda3\condabin\conda`+"\n", "", 0))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusWarn {
			t.Errorf("got %q, want warn", c.Status)
		}
	})
}

func TestMSVC(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only check")
	}
	ctx := context.Background()

	t.Run("vswhere not found", func(t *testing.T) {
		c, err := checkMSVC(ctx, fixedRunner("", "", 1))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusWarn {
			t.Errorf("got %q", c.Status)
		}
	})
	t.Run("vswhere found but no VS", func(t *testing.T) {
		runner := dispatchRunner(map[string]struct {
			stdout, stderr string
			exit           int
		}{
			"where":   {stdout: `C:\path\vswhere.exe` + "\n", exit: 0},
			"vswhere": {stdout: "[]", exit: 0},
		})
		c, err := checkMSVC(ctx, runner)
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusWarn {
			t.Errorf("got %q", c.Status)
		}
		if c.FixAction != "FixMSVC" {
			t.Errorf("FixAction = %q", c.FixAction)
		}
	})
	t.Run("VS installed", func(t *testing.T) {
		vsJSON := `[{"displayName":"Visual Studio Build Tools 2022"}]`
		runner := dispatchRunner(map[string]struct {
			stdout, stderr string
			exit           int
		}{
			"where": {stdout: `C:\path\vswhere.exe` + "\n", exit: 0},
			// The full path returned by `where vswhere` is used as the key.
			`C:\path\vswhere.exe`: {stdout: vsJSON, exit: 0},
		})
		c, err := checkMSVC(ctx, runner)
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusPass {
			t.Errorf("got %q", c.Status)
		}
	})
}

func TestGCC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux/macOS-only check")
	}
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		c, err := checkGCC(ctx, fixedRunner("gcc (Ubuntu 11.4.0) 11.4.0\n", "", 0))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusPass {
			t.Errorf("got %q", c.Status)
		}
	})
	t.Run("not found", func(t *testing.T) {
		c, err := checkGCC(ctx, fixedRunner("", "", 1))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != StatusWarn {
			t.Errorf("got %q", c.Status)
		}
	})
}

func TestAllPassed(t *testing.T) {
	pass := []Check{{ID: "a", Status: StatusPass}, {ID: "b", Status: StatusWarn}}
	if !AllPassed(pass) {
		t.Error("expected true")
	}
	fail := []Check{{ID: "a", Status: StatusPass}, {ID: "b", Status: StatusFail}}
	if AllPassed(fail) {
		t.Error("expected false")
	}
}

func TestWriteMarker(t *testing.T) {
	root := fakeRoot(t, "")
	if err := WriteMarker(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(root.MarkerFile(".preflight_ok"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "ok" {
		t.Errorf("marker content = %q", data)
	}
}

func TestRunReturnsPlatformChecks(t *testing.T) {
	root := fakeRoot(t, "")
	ctx := context.Background()
	noop := fixedRunner("", "", 0)

	checks, err := Run(ctx, root, noop)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) == 0 {
		t.Error("expected at least one check")
	}
	// SpacesInPath is always present.
	found := false
	for _, c := range checks {
		if c.ID == "SpacesInPath" {
			found = true
		}
		if c.Status == "" {
			t.Errorf("check %q has empty status", c.ID)
		}
	}
	if !found {
		t.Error("SpacesInPath check missing")
	}
	_ = fmt.Sprintf("%v", checks) // compile smoke
}
