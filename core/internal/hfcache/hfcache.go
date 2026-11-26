// Package hfcache scans and prunes a Hugging Face hub cache directory without
// Python. It is the Go side of the Huggingface Cache Manager page: Scan answers
// what huggingface_hub.scan_cache_dir (and the rows of the extension's
// scan_cache.py) answer, and PlanDelete / Plan.Execute do what
// HFCacheInfo.delete_revisions(...).execute() does.
//
// Layout it reads (one folder per repo in the cache directory):
//
//	models--org--name/            (also datasets-- and spaces--)
//	    blobs/<sha>               the files, once
//	    refs/<name>               a file holding a commit hash (refs/pr/1 is nested)
//	    snapshots/<commit>/...    symlinks (or copies) of the blobs, per revision
//
// Safety. A caller never gives a path: it names revisions (commit hash, repo
// id), and every path that is deleted comes from a listing of the cache
// directory. On top of that nothing is followed out of the cache:
//
//   - the cache directory itself is the one place that may be a symlink (it is
//     the user's setting); it is resolved once and everything is checked
//     against the resolved path;
//   - a repo folder, a snapshot folder, a refs folder or a ref file that is a
//     symlink is not followed (reported as a warning, never deleted through);
//   - a snapshot symlink whose target is not inside the same repo's blobs/
//     folder, or that resolves outside the cache, is skipped with a warning, so
//     it is never counted and never deleted;
//   - before anything is removed the path is checked to be lexically inside the
//     cache with no symlink in its parent chain.
//
// Differences from huggingface_hub (1.33), on purpose: the entries above are
// skipped with a warning where Python follows them; a missing cache directory
// is an empty answer (Python raises CacheNotFound); a revision hash that exists
// in two repos is refused unless the request names the repo (Python deletes it
// from whichever repo it meets first).
package hfcache

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrNotADirectory is returned when the cache directory is a file.
var ErrNotADirectory = errors.New("the Hugging Face cache path is a file, not a directory")

// File is one file of a revision.
type File struct {
	Name         string
	Path         string // the entry in snapshots/<commit>/
	BlobPath     string // what it points to (itself when it is a copy)
	Size         int64
	LastAccessed float64
	LastModified float64
}

// Revision is one folder of snapshots/.
type Revision struct {
	CommitHash   string
	SnapshotPath string
	Refs         []string // sorted
	Files        []File
	SizeOnDisk   int64
	LastModified float64
}

// Repo is one cached repository.
type Repo struct {
	ID           string
	Type         string // model, dataset or space
	Path         string
	NbFiles      int // distinct blobs
	SizeOnDisk   int64
	LastAccessed float64
	LastModified float64
	Revisions    []Revision // sorted by commit hash
}

// Info is the result of Scan.
type Info struct {
	CacheDir   string // resolved; the folder that was scanned
	Exists     bool   // false: nothing was scanned
	Repos      []Repo // sorted by folder name
	SizeOnDisk int64
	Warnings   []string
}

var repoTypes = map[string]string{"models": "model", "datasets": "dataset", "spaces": "space"}

var ignoredNames = map[string]bool{".DS_Store": true, "Thumbs.db": true, "desktop.ini": true}

const sharedBlobsMarker = ".huggingface-shared-blobs"

// maxRefBytes bounds a ref file: a commit hash is 40 or 64 characters.
const maxRefBytes = 4096

func seconds(t time.Time) float64 { return float64(t.UnixNano()) / 1e9 }

// within reports whether p is root or below it, lexically.
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// Resolve makes the cache directory absolute and resolves symlinks in it.
// ok is false when it does not exist.
func Resolve(dir string) (resolved string, ok bool, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false, err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return abs, false, nil
		}
		return abs, false, err
	}
	st, err := os.Stat(real)
	if err != nil {
		return real, false, err
	}
	if !st.IsDir() {
		return real, false, ErrNotADirectory
	}
	return real, true, nil
}

// Scan reads the cache directory. Problems with one repo are warnings in the
// result, and that repo is left out, like huggingface_hub does.
func Scan(cacheDir string) (Info, error) {
	real, ok, err := Resolve(cacheDir)
	info := Info{CacheDir: real, Exists: ok, Repos: []Repo{}, Warnings: []string{}}
	if err != nil {
		return info, err
	}
	if !ok {
		info.Warnings = append(info.Warnings, "Cache directory not found: "+real)
		return info, nil
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return info, err
	}
	for _, e := range entries {
		name := e.Name()
		if ignoredNames[name] || name == ".locks" || name == "CACHEDIR.TAG" {
			continue
		}
		path := filepath.Join(real, name)
		if name == "blobs" {
			if _, err := os.Lstat(filepath.Join(path, sharedBlobsMarker)); err == nil {
				continue // the shared blob store of huggingface_hub >= 1.x
			}
		}
		repo, warnings, err := scanRepo(real, path)
		if err != nil {
			info.Warnings = append(info.Warnings, err.Error())
			continue
		}
		info.Warnings = append(info.Warnings, warnings...)
		info.Repos = append(info.Repos, repo)
	}
	blobs := map[string]int64{}
	for _, r := range info.Repos {
		for _, rev := range r.Revisions {
			for _, f := range rev.Files {
				blobs[f.BlobPath] = f.Size
			}
		}
	}
	for _, s := range blobs {
		info.SizeOnDisk += s
	}
	return info, nil
}

type blobStat struct {
	size         int64
	atime, mtime float64
}

// scanRepo reads one repo folder. real is the resolved cache directory.
func scanRepo(real, repoPath string) (Repo, []string, error) {
	name := filepath.Base(repoPath)
	fi, err := os.Lstat(repoPath)
	if err != nil {
		return Repo{}, nil, fmt.Errorf("Repo path cannot be read: %s", repoPath)
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return Repo{}, nil, fmt.Errorf("Repo path is a symlink, not followed: %s", repoPath)
	}
	if !fi.IsDir() {
		return Repo{}, nil, fmt.Errorf("Repo path is not a directory: %s", repoPath)
	}
	prefix, id, found := strings.Cut(name, "--")
	if !found {
		return Repo{}, nil, fmt.Errorf("Repo path is not a valid HuggingFace cache directory: %s", repoPath)
	}
	repoType, okType := repoTypes[prefix]
	if !okType {
		return Repo{}, nil, fmt.Errorf("Repo type must be one of [datasets, models, spaces], found `%s` (%s).", prefix, repoPath)
	}
	id = strings.ReplaceAll(id, "--", "/")

	snapshots := filepath.Join(repoPath, "snapshots")
	if fi, err := os.Lstat(snapshots); err != nil || !fi.IsDir() {
		return Repo{}, nil, fmt.Errorf("Snapshots dir doesn't exist in cached repo: %s", snapshots)
	}

	refsByHash, err := scanRefs(repoPath)
	if err != nil {
		return Repo{}, nil, err
	}

	blobs := map[string]blobStat{}
	stat := func(p string) (blobStat, error) {
		if b, ok := blobs[p]; ok {
			return b, nil
		}
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			return blobStat{}, err
		}
		if !within(real, resolved) {
			return blobStat{}, errOutside
		}
		st, err := os.Stat(resolved)
		if err != nil {
			return blobStat{}, err
		}
		if st.IsDir() {
			return blobStat{}, errIsDir // a symlink to a folder is not a file (Python skips it too)
		}
		b := blobStat{size: st.Size(), atime: accessTime(st), mtime: seconds(st.ModTime())}
		blobs[p] = b
		return b, nil
	}

	var warnings []string
	revDirs, err := os.ReadDir(snapshots)
	if err != nil {
		return Repo{}, nil, fmt.Errorf("Snapshots dir cannot be read: %s", snapshots)
	}
	var revisions []Revision
	for _, d := range revDirs {
		if ignoredNames[d.Name()] {
			continue
		}
		revPath := filepath.Join(snapshots, d.Name())
		rfi, err := os.Lstat(revPath)
		if err != nil {
			return Repo{}, nil, fmt.Errorf("Snapshot cannot be read: %s", revPath)
		}
		if rfi.Mode()&fs.ModeSymlink != 0 {
			return Repo{}, nil, fmt.Errorf("Snapshot is a symlink, not followed: %s", revPath)
		}
		if !rfi.IsDir() {
			return Repo{}, nil, fmt.Errorf("Snapshots folder corrupted. Found a file: %s", revPath)
		}
		var files []File
		walkErr := filepath.WalkDir(revPath, func(p string, de fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() {
				return nil
			}
			blobPath := p
			if de.Type()&fs.ModeSymlink != 0 {
				target, err := os.Readlink(p)
				if err != nil {
					return err
				}
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(p), target)
				}
				blobPath = filepath.Clean(target)
				if !within(filepath.Join(repoPath, "blobs"), blobPath) || blobPath == filepath.Join(repoPath, "blobs") {
					warnings = append(warnings, fmt.Sprintf("Skipped %s: its target is not inside the repo's blobs folder", p))
					return nil
				}
			}
			b, err := stat(blobPath)
			switch {
			case errors.Is(err, errOutside):
				warnings = append(warnings, fmt.Sprintf("Skipped %s: it resolves outside the cache", p))
				return nil
			case errors.Is(err, fs.ErrNotExist):
				return fmt.Errorf("Blob missing (broken symlink): %s", blobPath)
			case errors.Is(err, errIsDir):
				return nil
			case err != nil:
				return err
			}
			files = append(files, File{Name: de.Name(), Path: p, BlobPath: blobPath, Size: b.size, LastAccessed: b.atime, LastModified: b.mtime})
			return nil
		})
		if walkErr != nil {
			return Repo{}, nil, walkErr
		}
		rev := Revision{CommitHash: d.Name(), SnapshotPath: revPath, Files: files}
		seen := map[string]bool{}
		for _, f := range files {
			if !seen[f.BlobPath] {
				seen[f.BlobPath] = true
				rev.SizeOnDisk += f.Size
			}
			if f.LastModified > rev.LastModified {
				rev.LastModified = f.LastModified
			}
		}
		if len(files) == 0 {
			rev.LastModified = seconds(rfi.ModTime())
		}
		if refs, ok := refsByHash[d.Name()]; ok {
			rev.Refs = refs
			delete(refsByHash, d.Name())
		}
		sort.Strings(rev.Refs)
		revisions = append(revisions, rev)
	}
	if len(refsByHash) > 0 {
		var hashes []string
		for h := range refsByHash {
			hashes = append(hashes, fmt.Sprintf("%q", h))
		}
		sort.Strings(hashes)
		return Repo{}, nil, fmt.Errorf("Reference(s) refer to missing commit hashes: %s (%s).", strings.Join(hashes, ", "), repoPath)
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i].CommitHash < revisions[j].CommitHash })

	repo := Repo{ID: id, Type: repoType, Path: repoPath, NbFiles: len(blobs), Revisions: revisions}
	if len(blobs) > 0 {
		for _, b := range blobs {
			repo.SizeOnDisk += b.size
			repo.LastAccessed = math.Max(repo.LastAccessed, b.atime)
			repo.LastModified = math.Max(repo.LastModified, b.mtime)
		}
	} else {
		repo.LastAccessed, repo.LastModified = accessTime(fi), seconds(fi.ModTime())
	}
	return repo, warnings, nil
}

var (
	errOutside = errors.New("outside the cache")
	errIsDir   = errors.New("is a folder")
)

// scanRefs maps commit hash to the ref names that hold it.
func scanRefs(repoPath string) (map[string][]string, error) {
	refs := map[string][]string{}
	refsPath := filepath.Join(repoPath, "refs")
	fi, err := os.Lstat(refsPath)
	if errors.Is(err, fs.ErrNotExist) {
		return refs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Refs directory cannot be read: %s", refsPath)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("Refs directory cannot be a file or a symlink: %s", refsPath)
	}
	err = filepath.WalkDir(refsPath, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() || ignoredNames[de.Name()] || de.Type()&fs.ModeSymlink != 0 {
			return nil // a symlink is not read: a ref is a plain file
		}
		rel, err := filepath.Rel(refsPath, p)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		buf := make([]byte, maxRefBytes+1)
		n, _ := f.Read(buf)
		if n > maxRefBytes {
			return fmt.Errorf("Ref file is too large: %s", p)
		}
		hash := string(buf[:n])
		refs[hash] = append(refs[hash], filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Refs of %s cannot be read: %v", repoPath, err)
	}
	return refs, nil
}
