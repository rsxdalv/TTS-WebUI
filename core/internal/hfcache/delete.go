package hfcache

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Selector names one revision. CommitHash is required. RepoID and RepoType
// narrow it to one repo; without them the hash must exist in exactly one.
type Selector struct {
	CommitHash string `json:"commit_hash"`
	RepoID     string `json:"repo_id,omitempty"`
	RepoType   string `json:"repo_type,omitempty"`
}

// ErrBadSelector is a request that is refused before anything is read or deleted.
var ErrBadSelector = errors.New("bad revision")

// ErrAmbiguous means a commit hash exists in more than one repo.
var ErrAmbiguous = errors.New("ambiguous revision")

// ValidHash is what a commit hash may look like in a request: one path
// element of letters, digits, dot, dash and underscore. A real hash is 40 or
// 64 hex characters; the wider rule only keeps odd snapshot names reachable.
// Nothing here is ever joined to a path (the plan is built from the scan, and
// the request only picks entries from it), so this is a second line of defence
// and an early, clear error: "..", slashes, drive letters and absolute paths
// are refused.
func ValidHash(s string) bool {
	if s == "" || len(s) > 128 || s[0] == '.' || s[0] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_'
		if !ok {
			return false
		}
	}
	return !containsDotDot(s)
}

func containsDotDot(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return true
		}
	}
	return false
}

// Plan is what a deletion will remove, built from a scan.
type Plan struct {
	CacheDir string
	// Blobs, Refs, Repos and Snapshots are removed in this order, like
	// DeleteCacheStrategy.execute.
	Blobs     []string
	Refs      []string
	Repos     []string
	Snapshots []string
	// Deleted are the selectors that matched; NotFound those that did not.
	Deleted  []Selector
	NotFound []Selector
	// ExpectedFreed is the sum of the blobs (and of whole repos) it removes.
	ExpectedFreed int64
}

// PlanDelete turns selectors into a plan, as HFCacheInfo.delete_revisions does:
// a repo whose every revision is selected is removed whole; otherwise the
// selected snapshots and their refs go, and a blob goes only when no remaining
// revision of the repo still lists it. Nothing is changed.
func PlanDelete(info Info, sel []Selector) (Plan, error) {
	plan := Plan{CacheDir: info.CacheDir}
	type pick struct {
		repo int
		rev  int
	}
	selectedBy := map[int]map[int]bool{} // repo index -> revision indexes
	seen := map[Selector]bool{}
	for _, s := range sel {
		if !ValidHash(s.CommitHash) {
			return plan, fmt.Errorf("%w: %q is not a commit hash", ErrBadSelector, s.CommitHash)
		}
		if s.RepoType != "" && !repoTypesValid[s.RepoType] {
			return plan, fmt.Errorf("%w: unknown repo type %q", ErrBadSelector, s.RepoType)
		}
		if len(s.RepoID) > 256 {
			return plan, fmt.Errorf("%w: repo id is too long", ErrBadSelector)
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		var matches []pick
		for ri, r := range info.Repos {
			if (s.RepoID != "" && r.ID != s.RepoID) || (s.RepoType != "" && r.Type != s.RepoType) {
				continue
			}
			for vi, rev := range r.Revisions {
				if rev.CommitHash == s.CommitHash {
					matches = append(matches, pick{ri, vi})
				}
			}
		}
		switch {
		case len(matches) == 0:
			plan.NotFound = append(plan.NotFound, s)
		case len(matches) > 1:
			return plan, fmt.Errorf("%w: %s exists in %d repos; name the repo (repo_id)", ErrAmbiguous, s.CommitHash, len(matches))
		default:
			m := matches[0]
			if selectedBy[m.repo] == nil {
				selectedBy[m.repo] = map[int]bool{}
			}
			selectedBy[m.repo][m.rev] = true
			plan.Deleted = append(plan.Deleted, s)
		}
	}

	repoIdx := make([]int, 0, len(selectedBy))
	for ri := range selectedBy {
		repoIdx = append(repoIdx, ri)
	}
	sort.Ints(repoIdx)
	blobs := map[string]int64{} // to unlink -> scanned size
	blobSet := map[string]bool{}
	refSet := map[string]bool{}
	for _, ri := range repoIdx {
		repo := info.Repos[ri]
		chosen := selectedBy[ri]
		if len(chosen) == len(repo.Revisions) {
			plan.Repos = append(plan.Repos, repo.Path)
			for _, rev := range repo.Revisions {
				for _, f := range rev.Files {
					blobs[f.BlobPath] = f.Size
				}
			}
			continue
		}
		kept := map[string]bool{}
		for vi, rev := range repo.Revisions {
			if !chosen[vi] {
				for _, f := range rev.Files {
					kept[f.BlobPath] = true
				}
			}
		}
		for vi, rev := range repo.Revisions {
			if !chosen[vi] {
				continue
			}
			plan.Snapshots = append(plan.Snapshots, rev.SnapshotPath)
			for _, ref := range rev.Refs {
				p := filepath.Join(repo.Path, "refs", filepath.FromSlash(ref))
				if !refSet[p] {
					refSet[p] = true
					plan.Refs = append(plan.Refs, p)
				}
			}
			for _, f := range rev.Files {
				if within(rev.SnapshotPath, f.BlobPath) {
					// A copy that lives in the snapshot goes with the snapshot.
					blobs[f.BlobPath] = f.Size
					continue
				}
				if !kept[f.BlobPath] && !blobSet[f.BlobPath] {
					blobSet[f.BlobPath] = true
					plan.Blobs = append(plan.Blobs, f.BlobPath)
					blobs[f.BlobPath] = f.Size
				}
			}
		}
	}
	for _, s := range blobs {
		plan.ExpectedFreed += s
	}
	return plan, nil
}

var repoTypesValid = map[string]bool{"model": true, "dataset": true, "space": true}

// Result is what Execute did.
type Result struct {
	Removed []string // paths, in the order they went
	Errors  []string // paths that could not be removed, with the reason
}

// Execute removes what the plan lists. Each path is checked right before it is
// removed (see safeToRemove); one that fails the check or cannot be removed is
// an entry in Errors and the rest continues, as Python logs a warning and goes
// on.
func (p Plan) Execute() Result {
	var res Result
	do := func(kind, path string, dir bool) {
		if err := safeToRemove(p.CacheDir, path, dir); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s %s: %v", kind, path, err))
			return
		}
		var err error
		if dir {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			res.Errors = append(res.Errors, fmt.Sprintf("%s %s: %v", kind, path, err))
			return
		}
		res.Removed = append(res.Removed, path)
	}
	for _, b := range p.Blobs {
		do("blob", b, false)
	}
	for _, r := range p.Refs {
		do("ref", r, false)
	}
	for _, r := range p.Repos {
		do("repo", r, true)
	}
	for _, s := range p.Snapshots {
		do("snapshot", s, true)
	}
	return res
}

var errUnsafe = errors.New("refused: not a plain entry inside the cache")

// safeToRemove is the last check before a path is removed. The path must be
// strictly below the (resolved) cache directory, no folder above it may be a
// symlink (EvalSymlinks of the parent must be the parent), and the entry
// itself must be a folder (when a folder is meant, and not a symlink) or a
// file or symlink (when a file is meant: removing a symlink removes the link,
// never its target).
func safeToRemove(cacheDir, path string, dir bool) error {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) || path == cacheDir || !within(cacheDir, path) {
		return errUnsafe
	}
	parent := filepath.Dir(path)
	real, err := filepath.EvalSymlinks(parent)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // already gone
		}
		return err
	}
	if real != parent {
		return errUnsafe
	}
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if dir {
		if !fi.IsDir() {
			return errUnsafe // a symlink or a file where a folder was scanned
		}
		return nil
	}
	if fi.IsDir() {
		return errUnsafe
	}
	return nil
}
