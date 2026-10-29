package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// gitRunner executes a git subcommand in dir and returns trimmed stdout.
// Infrastructure errors (not-found, timeout) are returned in err.
// A non-zero git exit code is treated as an error.
type gitRunner func(ctx context.Context, dir string, args ...string) (string, error)

// defaultGitRunner is the production git runner.
func defaultGitRunner(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// isGitUpdated returns true if the local .git_version matches origin/main.
// On any error it returns (true, nil) — safe default: don't force-update.
func isGitUpdated(ctx context.Context, root paths.Root) (bool, error) {
	return isGitUpdatedWith(ctx, root, defaultGitRunner)
}

// isGitUpdatedWith is the testable variant with an injected git runner.
func isGitUpdatedWith(ctx context.Context, root paths.Root, run gitRunner) (bool, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := run(fetchCtx, root.Dir(), "fetch", "origin"); err != nil {
		return true, nil // safe: treat fetch failure as "up to date"
	}

	// Read local hash from marker file.
	// Bug fix: use filepath.Join via root.MarkerFile(), not "\\" path concatenation.
	raw, err := os.ReadFile(root.MarkerFile(".git_version"))
	if err != nil {
		return true, nil
	}
	local := strings.TrimSpace(string(raw))

	remote, err := run(ctx, root.Dir(), "rev-parse", "origin/main")
	if err != nil {
		return true, nil
	}

	return local == remote, nil
}
