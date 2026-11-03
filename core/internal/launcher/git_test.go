package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// fakeGit returns a gitRunner that behaves as configured.
type fakeGit struct {
	fetchErr   bool
	remoteHash string
	runErr     bool
}

func (f *fakeGit) run(ctx context.Context, dir string, args ...string) (string, error) {
	if f.runErr {
		return "", context.DeadlineExceeded
	}
	// fetch — always succeeds unless fetchErr
	if len(args) > 0 && args[0] == "fetch" {
		if f.fetchErr {
			return "", context.DeadlineExceeded
		}
		return "", nil
	}
	// rev-parse
	return f.remoteHash, nil
}

func setupGitRoot(t *testing.T, localHash string) paths.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if localHash != "" {
		if err := os.WriteFile(root.MarkerFile(".git_version"), []byte(localHash+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestIsGitUpdated_UpToDate(t *testing.T) {
	root := setupGitRoot(t, "abc123")
	fg := &fakeGit{remoteHash: "abc123"}
	got, err := isGitUpdatedWith(context.Background(), root, fg.run)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected up-to-date when hashes match")
	}
}

func TestIsGitUpdated_UpdateAvailable(t *testing.T) {
	root := setupGitRoot(t, "abc123")
	fg := &fakeGit{remoteHash: "def456"}
	got, err := isGitUpdatedWith(context.Background(), root, fg.run)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("expected not up-to-date when hashes differ")
	}
}

func TestIsGitUpdated_FetchError_SafeDefault(t *testing.T) {
	root := setupGitRoot(t, "abc123")
	fg := &fakeGit{fetchErr: true}
	got, err := isGitUpdatedWith(context.Background(), root, fg.run)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected safe default (true) when fetch fails")
	}
}

func TestIsGitUpdated_NoMarker_SafeDefault(t *testing.T) {
	root := setupGitRoot(t, "") // no .git_version file
	fg := &fakeGit{remoteHash: "abc123"}
	got, err := isGitUpdatedWith(context.Background(), root, fg.run)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected safe default (true) when marker file missing")
	}
}

func TestIsGitUpdated_RunError_SafeDefault(t *testing.T) {
	root := setupGitRoot(t, "abc123")
	fg := &fakeGit{runErr: true}
	got, err := isGitUpdatedWith(context.Background(), root, fg.run)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected safe default (true) when runner errors")
	}
}
