package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// isGitUpdated returns true if the local .git_version matches @{upstream},
// or HEAD when no upstream tracking branch is configured.
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
	raw, err := os.ReadFile(root.MarkerFile(".git_version"))
	if err != nil {
		return true, nil
	}
	local := strings.TrimSpace(string(raw))

	// Use the current branch's remote tracking ref; fall back to HEAD.
	remote, err := run(ctx, root.Dir(), "rev-parse", "@{upstream}")
	if err != nil {
		remote, err = run(ctx, root.Dir(), "rev-parse", "HEAD")
		if err != nil {
			return true, nil
		}
	}

	return local == remote, nil
}

// restoreExecBits gives back the exec bit to files that git tracks as 100755
// but whose working copy lacks it, as `chmod +x` would. An install extracted by
// a Go Download before R2-2 was fixed has such files, and with core.filemode
// they count as local changes that make `git pull` abort once a commit touches
// them. Nothing else in the tree is changed. It returns the paths it changed.
func restoreExecBits(ctx context.Context, dir string, run gitRunner) ([]string, error) {
	out, err := run(ctx, dir, "ls-files", "-s", "-z")
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, entry := range strings.Split(out, "\x00") {
		meta, path, ok := strings.Cut(entry, "\t")
		if !ok || !strings.HasPrefix(meta, "100755 ") {
			continue
		}
		full := filepath.Join(dir, filepath.FromSlash(path))
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 != 0 {
			continue
		}
		if err := os.Chmod(full, info.Mode().Perm()|0o111); err != nil {
			return changed, err
		}
		changed = append(changed, path)
	}
	return changed, nil
}
