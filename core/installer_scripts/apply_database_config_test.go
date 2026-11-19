package installer_scripts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newDBStep(t *testing.T) (*Step, *[]string, string) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "installer_scripts"), 0o777); err != nil {
		t.Fatal(err)
	}
	var lines []string
	return &Step{Root: root, Log: func(l string) { lines = append(lines, l) }}, &lines, root
}

func TestApplyDatabaseConfigFresh(t *testing.T) {
	s, lines, root := newDBStep(t)
	code, err := ApplyDatabaseConfig(context.Background(), s)
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "sqlite")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "installer_scripts", ".db_version")); err == nil {
		t.Fatal("fresh install must not create .db_version")
	}
	want := "Creating SQLite database directory...|Successfully created SQLite database directory"
	if got := strings.Join(*lines, "|"); got != want {
		t.Fatalf("log = %q", got)
	}
	// Second run: directory exists.
	*lines = nil
	if _, err := ApplyDatabaseConfig(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(*lines, "|"); got != "SQLite database directory already exists, skipping..." {
		t.Fatalf("log = %q", got)
	}
}

func TestApplyDatabaseConfigUpToDate(t *testing.T) {
	s, lines, root := newDBStep(t)
	f := filepath.Join(root, "installer_scripts", ".db_version")
	os.WriteFile(f, []byte("1"), 0o666)
	code, err := ApplyDatabaseConfig(context.Background(), s)
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if got := (*lines)[len(*lines)-1]; got != "Database is already up to date with version=1, skipping..." {
		t.Fatalf("last log = %q", got)
	}
}

// A trailing newline is a different version, because the original compares raw bytes.
// The upgrade then fails as the original does (createDB is undefined) and the marker is untouched.
func TestApplyDatabaseConfigUpgradeFailsLikeOriginal(t *testing.T) {
	for _, old := range []string{"0", "1\n", ""} {
		s, lines, root := newDBStep(t)
		f := filepath.Join(root, "installer_scripts", ".db_version")
		os.WriteFile(f, []byte(old), 0o666)
		code, err := ApplyDatabaseConfig(context.Background(), s)
		if code != 1 || err == nil || err.Error() != "createDB is not defined" {
			t.Fatalf("old=%q code=%d err=%v", old, code, err)
		}
		if b, _ := os.ReadFile(f); string(b) != old {
			t.Fatalf("old=%q: marker rewritten to %q", old, b)
		}
		joined := strings.Join(*lines, "\n")
		for _, want := range []string{"upgrading...", "Using SQLite database...", filepath.Join("data", "sqlite", "webui.db")} {
			if !strings.Contains(joined, want) {
				t.Fatalf("old=%q: log lacks %q:\n%s", old, want, joined)
			}
		}
	}
}
