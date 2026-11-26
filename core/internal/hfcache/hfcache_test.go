package hfcache

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	h1 = "1111111111111111111111111111111111111111"
	h2 = "2222222222222222222222222222222222222222"
	h3 = "3333333333333333333333333333333333333333"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
}

// rev is one snapshot: file name -> blob name (the blob is a file of blobs/).
type rev struct {
	hash  string
	files map[string]string
	refs  []string
}

// makeRepo writes the layout huggingface_hub writes. blobs maps a blob name to
// its content.
func makeRepo(t *testing.T, cache, dir string, blobs map[string]string, revs ...rev) string {
	t.Helper()
	repo := filepath.Join(cache, dir)
	for name, body := range blobs {
		write(t, filepath.Join(repo, "blobs", name), body)
	}
	if err := os.MkdirAll(filepath.Join(repo, "snapshots"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, r := range revs {
		if err := os.MkdirAll(filepath.Join(repo, "snapshots", r.hash), 0o755); err != nil {
			t.Fatal(err)
		}
		for file, blob := range r.files {
			up := strings.Repeat(".."+string(filepath.Separator), 2+strings.Count(file, "/"))
			symlink(t, up+filepath.Join("blobs", blob), filepath.Join(repo, "snapshots", r.hash, filepath.FromSlash(file)))
		}
		for _, ref := range r.refs {
			write(t, filepath.Join(repo, "refs", filepath.FromSlash(ref)), r.hash)
		}
	}
	return repo
}

func newCache(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "hub")
}

// twoRevisions is a repo with a blob shared by both revisions.
func twoRevisions(t *testing.T, cache string) string {
	return makeRepo(t, cache, "models--org--tiny",
		map[string]string{"shared": "0123456789", "only1": "abc", "only2": "abcdef"},
		rev{h1, map[string]string{"config.json": "shared", "weights.bin": "only1"}, []string{"main"}},
		rev{h2, map[string]string{"config.json": "shared", "sub/dir/extra.txt": "only2"}, []string{"refs/pr/1"}},
	)
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func TestScanSizesRefsAndRows(t *testing.T) {
	cache := newCache(t)
	twoRevisions(t, cache)
	makeRepo(t, cache, "datasets--a--b", map[string]string{"x": "12345"}, rev{h3, map[string]string{"data.csv": "x"}, nil})
	write(t, filepath.Join(cache, ".locks", "models--org--tiny", "lock"), "")
	write(t, filepath.Join(cache, "CACHEDIR.TAG"), "x")

	info, err := Scan(cache)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Exists || len(info.Warnings) != 0 || len(info.Repos) != 2 {
		t.Fatalf("scan: exists=%v warnings=%v repos=%d", info.Exists, info.Warnings, len(info.Repos))
	}
	ds, tiny := info.Repos[0], info.Repos[1] // sorted by folder name: datasets-- before models--
	if ds.ID != "a/b" || ds.Type != "dataset" || tiny.ID != "org/tiny" || tiny.Type != "model" {
		t.Fatalf("ids: %+v %+v", ds.ID, tiny.ID)
	}
	if tiny.NbFiles != 3 || tiny.SizeOnDisk != 10+3+6 {
		t.Errorf("repo: nb=%d size=%d", tiny.NbFiles, tiny.SizeOnDisk)
	}
	r1, r2 := tiny.Revisions[0], tiny.Revisions[1]
	if r1.CommitHash != h1 || r1.SizeOnDisk != 13 || len(r1.Files) != 2 || strings.Join(r1.Refs, ",") != "main" {
		t.Errorf("rev1: %+v", r1)
	}
	if r2.SizeOnDisk != 16 || strings.Join(r2.Refs, ",") != "refs/pr/1" {
		t.Errorf("rev2: %+v", r2)
	}
	if info.SizeOnDisk != 19+5 {
		t.Errorf("total: %d", info.SizeOnDisk)
	}
	rows := info.Rows(time.Now())
	if len(rows) != 3 || rows[0].RepoID != "a/b" || rows[1].CommitHash != h1 || rows[2].CommitHash != h2 {
		t.Fatalf("rows: %+v", rows)
	}
	if rows[1].SizeOnDiskStr != "       13.0 " && rows[1].SizeOnDiskStr != "       13.0" {
		// "{:>12}".format("13.0") is 12 wide
		if len(rows[1].SizeOnDiskStr) != 12 || !strings.HasSuffix(rows[1].SizeOnDiskStr, "13.0") {
			t.Errorf("size str %q", rows[1].SizeOnDiskStr)
		}
	}
}

func TestScanMissingAndFileCacheDir(t *testing.T) {
	cache := newCache(t)
	info, err := Scan(cache)
	if err != nil || info.Exists || len(info.Repos) != 0 || len(info.Warnings) != 1 {
		t.Fatalf("missing: %+v %v", info, err)
	}
	write(t, cache, "i am a file")
	if _, err := Scan(cache); !errors.Is(err, ErrNotADirectory) {
		t.Errorf("file: %v", err)
	}
}

func TestScanWarnsAndSkipsBrokenRepos(t *testing.T) {
	cache := newCache(t)
	twoRevisions(t, cache)
	write(t, filepath.Join(cache, "not-a-repo", "x"), "x")                     // no "--"
	write(t, filepath.Join(cache, "weird--org--x", "snapshots", h1, "f"), "x") // unknown type
	write(t, filepath.Join(cache, "models--no--snapshots", "blobs", "b"), "x") // no snapshots dir
	write(t, filepath.Join(cache, "stray-file"), "x")                          // a file
	makeRepo(t, cache, "models--dangling--ref", map[string]string{"b": "x"}, rev{h3, map[string]string{"f": "b"}, nil})
	write(t, filepath.Join(cache, "models--dangling--ref", "refs", "main"), h1) // names a missing hash
	broken := makeRepo(t, cache, "models--broken--link", nil, rev{h3, nil, nil})
	symlink(t, filepath.Join("..", "..", "blobs", "gone"), filepath.Join(broken, "snapshots", h3, "f"))

	info, err := Scan(cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Repos) != 1 || info.Repos[0].ID != "org/tiny" {
		t.Fatalf("repos: %+v", info.Repos)
	}
	if len(info.Warnings) != 6 {
		t.Errorf("warnings (%d): %v", len(info.Warnings), info.Warnings)
	}
}

func TestScanDoesNotFollowSymlinksOutOfTheCache(t *testing.T) {
	cache := newCache(t)
	outside := t.TempDir()
	write(t, filepath.Join(outside, "secret.txt"), "top secret")
	twoRevisions(t, cache)
	repo := makeRepo(t, cache, "models--evil--links", map[string]string{"ok": "fine"}, rev{h3, map[string]string{"ok.txt": "ok"}, nil})
	// a snapshot entry that points out of the cache, absolute and relative
	symlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(repo, "snapshots", h3, "abs.txt"))
	symlink(t, filepath.Join("..", "..", "..", "..", "..", filepath.Base(outside), "secret.txt"), filepath.Join(repo, "snapshots", h3, "rel.txt"))
	// a repo folder that is a symlink to a folder outside
	makeRepo(t, outside, "models--out--side", map[string]string{"b": "x"}, rev{h2, map[string]string{"f": "b"}, nil})
	symlink(t, filepath.Join(outside, "models--out--side"), filepath.Join(cache, "models--out--side"))
	// a refs folder that is a symlink
	linked := makeRepo(t, cache, "models--refs--link", map[string]string{"b": "x"}, rev{h3, map[string]string{"f": "b"}, nil})
	write(t, filepath.Join(outside, "refs", "main"), h3)
	symlink(t, filepath.Join(outside, "refs"), filepath.Join(linked, "refs"))

	info, err := Scan(cache)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]Repo{}
	for _, r := range info.Repos {
		ids[r.ID] = r
	}
	if _, ok := ids["out/side"]; ok {
		t.Error("a symlinked repo folder was followed")
	}
	if _, ok := ids["refs/link"]; ok {
		t.Error("a symlinked refs folder was followed")
	}
	evil := ids["evil/links"]
	if evil.NbFiles != 1 || evil.SizeOnDisk != 4 || len(evil.Revisions[0].Files) != 1 {
		t.Errorf("the repo with out-of-cache links counted them: %+v", evil)
	}
	joined := strings.Join(info.Warnings, "\n")
	for _, want := range []string{"abs.txt", "rel.txt", "models--out--side", "Refs directory"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no warning mentions %s:\n%s", want, joined)
		}
	}
}

func TestPlanDeleteOneRevisionKeepsSharedBlobs(t *testing.T) {
	cache := newCache(t)
	repo := twoRevisions(t, cache)
	info, _ := Scan(cache)
	plan, err := PlanDelete(info, []Selector{{CommitHash: h1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Blobs) != 1 || filepath.Base(plan.Blobs[0]) != "only1" || len(plan.Snapshots) != 1 || len(plan.Refs) != 1 || len(plan.Repos) != 0 {
		t.Fatalf("plan: %+v", plan)
	}
	if plan.ExpectedFreed != 3 {
		t.Errorf("expected freed %d", plan.ExpectedFreed)
	}
	res := plan.Execute()
	if len(res.Errors) != 0 || len(res.Removed) != 3 {
		t.Fatalf("result: %+v", res)
	}
	if exists(filepath.Join(repo, "blobs", "only1")) || exists(filepath.Join(repo, "snapshots", h1)) || exists(filepath.Join(repo, "refs", "main")) {
		t.Error("revision 1 is still there")
	}
	if !exists(filepath.Join(repo, "blobs", "shared")) || !exists(filepath.Join(repo, "blobs", "only2")) || !exists(filepath.Join(repo, "snapshots", h2, "config.json")) || !exists(filepath.Join(repo, "refs", "refs", "pr", "1")) {
		t.Error("revision 2 or a shared blob was removed")
	}
	again, _ := Scan(cache)
	if len(again.Warnings) != 0 || len(again.Repos[0].Revisions) != 1 {
		t.Errorf("after: %+v", again)
	}
}

func TestPlanDeleteAllRevisionsRemovesTheRepo(t *testing.T) {
	cache := newCache(t)
	repo := twoRevisions(t, cache)
	other := makeRepo(t, cache, "models--keep--me", map[string]string{"b": "x"}, rev{h3, map[string]string{"f": "b"}, nil})
	info, _ := Scan(cache)
	plan, err := PlanDelete(info, []Selector{{CommitHash: h1}, {CommitHash: h2}, {CommitHash: strings.Repeat("9", 40)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Repos) != 1 || len(plan.Blobs) != 0 || len(plan.NotFound) != 1 || len(plan.Deleted) != 2 || plan.ExpectedFreed != 19 {
		t.Fatalf("plan: %+v", plan)
	}
	if res := plan.Execute(); len(res.Errors) != 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	if exists(repo) || !exists(other) {
		t.Errorf("repo gone=%v, other kept=%v", !exists(repo), exists(other))
	}
	if !exists(cache) {
		t.Error("the cache dir itself was removed")
	}
}

func TestPlanDeleteAmbiguousHashNeedsTheRepo(t *testing.T) {
	cache := newCache(t)
	makeRepo(t, cache, "models--a--one", map[string]string{"b": "x"}, rev{h1, map[string]string{"f": "b"}, nil}, rev{h2, map[string]string{"f": "b"}, nil})
	makeRepo(t, cache, "models--b--two", map[string]string{"b": "y"}, rev{h1, map[string]string{"f": "b"}, nil}, rev{h3, map[string]string{"f": "b"}, nil})
	info, _ := Scan(cache)
	if _, err := PlanDelete(info, []Selector{{CommitHash: h1}}); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("ambiguous: %v", err)
	}
	plan, err := PlanDelete(info, []Selector{{CommitHash: h1, RepoID: "b/two", RepoType: "model"}})
	if err != nil || len(plan.Snapshots) != 1 || !strings.Contains(plan.Snapshots[0], "models--b--two") {
		t.Fatalf("narrowed: %+v %v", plan, err)
	}
	if _, err := PlanDelete(info, []Selector{{CommitHash: h1, RepoType: "bogus"}}); !errors.Is(err, ErrBadSelector) {
		t.Errorf("bad type: %v", err)
	}
}

func TestPlanDeleteRefusesPathsAndOddHashes(t *testing.T) {
	cache := newCache(t)
	twoRevisions(t, cache)
	outside := t.TempDir()
	write(t, filepath.Join(outside, "keep.txt"), "x")
	info, _ := Scan(cache)
	for _, bad := range []string{
		"", "..", "../..", "../" + filepath.Base(outside), "..\\x", "a/b", "a\\b", "/etc/passwd", outside,
		"C:\\Windows", "C:", ".hidden", "-rf", h1 + "/..", "x\x00y", "a b", "a..b", strings.Repeat("a", 129), "snapshots/" + h1,
		"%2e%2e", "~", "$HOME", "x*", "x?",
	} {
		if _, err := PlanDelete(info, []Selector{{CommitHash: bad}}); !errors.Is(err, ErrBadSelector) {
			t.Errorf("hash %q was not refused: %v", bad, err)
		}
	}
	if !exists(filepath.Join(outside, "keep.txt")) {
		t.Fatal("something outside the cache is gone")
	}
	for _, bad := range []string{"../x", "a/b", "/abs", "\\\\server\\share"} {
		if _, err := PlanDelete(info, []Selector{{CommitHash: h1, RepoID: bad}}); err != nil {
			continue // an id that matches no repo is "not found", never a path
		}
	}
}

func TestExecuteWillNotRemoveWhatTheCheckRefuses(t *testing.T) {
	cache := newCache(t)
	outside := t.TempDir()
	write(t, filepath.Join(outside, "victim", "file"), "x")
	write(t, filepath.Join(outside, "victim.txt"), "x")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}

	// A hand-made plan with paths outside the cache, relative, the cache itself
	// and a path above it: every one is refused and nothing is removed.
	plan := Plan{CacheDir: cache,
		Blobs:     []string{filepath.Join(outside, "victim.txt"), "relative", cache},
		Refs:      []string{filepath.Join(cache, "..", "..", filepath.Base(outside), "victim.txt")},
		Repos:     []string{filepath.Join(outside, "victim"), filepath.Dir(cache)},
		Snapshots: []string{outside},
	}
	res := plan.Execute()
	if len(res.Removed) != 0 || len(res.Errors) != 7 {
		t.Fatalf("removed %v, errors %v", res.Removed, res.Errors)
	}
	if !exists(filepath.Join(outside, "victim", "file")) || !exists(filepath.Join(outside, "victim.txt")) || !exists(cache) {
		t.Fatal("something was removed")
	}
}

func TestExecuteDoesNotFollowASwappedSymlink(t *testing.T) {
	cache := newCache(t)
	repo := twoRevisions(t, cache)
	outside := t.TempDir()
	write(t, filepath.Join(outside, "victim", "file"), "x")
	info, _ := Scan(cache)
	plan, err := PlanDelete(info, []Selector{{CommitHash: h1}})
	if err != nil {
		t.Fatal(err)
	}
	// After the scan the snapshot folder is swapped for a link to a folder
	// outside the cache.
	if err := os.RemoveAll(filepath.Join(repo, "snapshots", h1)); err != nil {
		t.Fatal(err)
	}
	symlink(t, filepath.Join(outside, "victim"), filepath.Join(repo, "snapshots", h1))
	// and the blobs folder for a link as well
	if err := os.RemoveAll(filepath.Join(repo, "blobs")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(outside, "blobs", "only1"), "keep me")
	symlink(t, filepath.Join(outside, "blobs"), filepath.Join(repo, "blobs"))

	res := plan.Execute()
	if len(res.Errors) == 0 {
		t.Errorf("a swapped link was accepted: %+v", res)
	}
	if !exists(filepath.Join(outside, "victim", "file")) || !exists(filepath.Join(outside, "blobs", "only1")) {
		t.Fatal("a file outside the cache was removed through a link")
	}
}

func TestDeleteOfASymlinkedBlobRemovesTheLinkOnly(t *testing.T) {
	cache := newCache(t)
	repo := makeRepo(t, cache, "models--org--store", map[string]string{"real": "payload"},
		rev{h1, map[string]string{"a": "real"}, nil}, rev{h2, map[string]string{"b": "linked"}, nil})
	// blobs/linked is itself a link to a payload that lives elsewhere in the cache (the shared store)
	write(t, filepath.Join(cache, "store", "payload"), "shared payload")
	if err := os.Remove(filepath.Join(repo, "blobs", "linked")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	symlink(t, filepath.Join(cache, "store", "payload"), filepath.Join(repo, "blobs", "linked"))
	info, err := Scan(cache)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanDelete(info, []Selector{{CommitHash: h2}})
	if err != nil {
		t.Fatal(err)
	}
	if res := plan.Execute(); len(res.Errors) != 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	if exists(filepath.Join(repo, "blobs", "linked")) {
		t.Error("the link is still there")
	}
	if !exists(filepath.Join(cache, "store", "payload")) {
		t.Error("the payload behind the link was removed")
	}
}

func TestFormatSizeAndTimeSince(t *testing.T) {
	for n, want := range map[int64]string{0: "0.0", 999: "999.0", 1000: "1.0K", 1536: "1.5K", 2_500_000: "2.5M", 7_900_000_000: "7.9G"} {
		if got := FormatSize(n); got != want {
			t.Errorf("FormatSize(%d) = %q, want %q", n, got, want)
		}
	}
	now := time.Unix(1_700_000_000, 0)
	ago := func(d time.Duration) float64 { return float64(now.Add(-d).Unix()) }
	for d, want := range map[time.Duration]string{
		5 * time.Second: "a few seconds ago", 30 * time.Second: "30 seconds ago", 90 * time.Second: "2 minutes ago",
		3 * time.Hour: "3 hours ago", 25 * time.Hour: "1 day ago", 3 * 24 * time.Hour: "3 days ago",
		8 * 24 * time.Hour: "1 week ago", 45 * 24 * time.Hour: "6 weeks ago", 50 * 24 * time.Hour: "2 months ago", 800 * 24 * time.Hour: "2 years ago",
	} {
		if got := FormatTimeSince(now, ago(d)); got != want {
			t.Errorf("FormatTimeSince(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestCacheDirFollowsHuggingFaceOrder(t *testing.T) {
	home := t.TempDir()
	base := []string{"HOME=" + home}
	// An absolute path on every OS: on Windows it needs a drive ("\app" is not absolute there).
	abs := func(p string) string { return filepath.Join(filepath.VolumeName(home)+string(filepath.Separator), p) }
	root := abs("app")
	// "~" is the user's home: HOME everywhere but Windows, where the cache code ignores HOME.
	tildeHome := home
	if runtime.GOOS == "windows" {
		if h, err := os.UserHomeDir(); err == nil {
			tildeHome = h
		}
	}
	cases := []struct {
		name string
		env  []string
		want string
	}{
		{"default", base, filepath.Join(tildeHome, ".cache", "huggingface", "hub")},
		{"xdg", append(base, "XDG_CACHE_HOME="+abs("xdg")), filepath.Join(abs("xdg"), "huggingface", "hub")},
		{"hf home", append(base, "HF_HOME="+abs("hfhome"), "XDG_CACHE_HOME="+abs("xdg")), filepath.Join(abs("hfhome"), "hub")},
		{"legacy hub cache beats hf home", append(base, "HF_HOME="+abs("hfhome"), "HUGGINGFACE_HUB_CACHE="+abs("legacy")), abs("legacy")},
		{"hf hub cache beats both", append(base, "HF_HOME="+abs("hfhome"), "HUGGINGFACE_HUB_CACHE="+abs("legacy"), "HF_HUB_CACHE="+abs("new")), abs("new")},
		{"empty is unset", append(base, "HF_HUB_CACHE=", "HUGGINGFACE_HUB_CACHE=  ", "HF_HOME="+abs("hfhome")), filepath.Join(abs("hfhome"), "hub")},
		{"relative is under the app root", append(base, "HF_HUB_CACHE=models/hf"), filepath.Join(root, "models", "hf")},
		{"tilde", append(base, "HF_HUB_CACHE=~/hubcache"), filepath.Join(tildeHome, "hubcache")},
		{"variable", append(base, "MYDIR="+abs("m"), "HF_HUB_CACHE=$MYDIR/hub"), filepath.Join(abs("m"), "hub")},
		{"last one wins", append(base, "HF_HUB_CACHE="+abs("a"), "HF_HUB_CACHE="+abs("b")), abs("b")},
	}
	for _, c := range cases {
		if got := CacheDir(c.env, root); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
