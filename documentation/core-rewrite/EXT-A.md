# EXT-A: Huggingface Cache Manager gets a pure-Go backend

Branch `cloud/ext-hf-cache-go`, cut from `main` (b844e0a). Companion jobs: EXT-D (cleanup) and EXT-C (Model Downloader
as a native entry), each on its own branch from `main`.

## What changed

| Piece | Where |
|---|---|
| Scan and prune of the hub cache | `core/internal/hfcache` (`hfcache.go` scan, `rows.go` the rows and the two format helpers, `delete.go` plan and execute, `cachedir.go` which folder, `atime_*.go` st_atime per OS) |
| Routes | `core/react_ui_backend/go_hf_cache.go`, claim `go-hf-cache`: `GET /api/go-data/hf-cache/scan`, `POST /api/go-data/hf-cache/delete` |
| Vite page | `react-ui/src/components/ExtensionHuggingFaceCacheManager.tsx` calls those two routes (it called `/api/gradio/scan_huggingface_cache_api` and `/api/gradio/delete_huggingface_cache_revisions`); shows delete errors and scan warnings; `functions/hostStatus.ts` no longer counts `/extensions/huggingface_cache_manager` as a Gradio page, so it stays in the menu without the Gradio host |
| Unchanged | the Gradio tab (`extensions/builtin/extension_huggingface_cache_manager/`), its api names, and the `scan_huggingface_cache_api` / `delete_huggingface_cache_revisions` entries in `react_ui_backend/gradio.go` |

### API

`GET /api/go-data/hf-cache/scan` answers `{headers, rows, cache_dir, exists, size_on_disk, size_on_disk_str, warnings}`.
`headers` and `rows` are what the Gradio endpoint answered: the 13 keys of `scan_cache.get_rows_json`, one row per revision.

`POST /api/go-data/hf-cache/delete` (JSON only, unknown fields refused) takes `{"revisions": [{"commit_hash", "repo_id"?,
"repo_type"?}]}` or, like the Gradio endpoint, `{"commit_hash": "..."}`. It answers `{deleted, not_found,
expected_freed_size, expected_freed_size_str, removed, errors}`. 400 for a bad hash or body, 409 for an ambiguous hash.
The page now sends `commit_hash`, `repo_id` and `repo_type` of the row.

### Which folder

`hfcache.CacheDir` follows huggingface_hub's order: `HF_HUB_CACHE`, `HUGGINGFACE_HUB_CACHE`, `$HF_HOME/hub`
(`HF_HOME` defaults to `$XDG_CACHE_HOME/huggingface`, then `~/.cache/huggingface`). The environment is
`appenv.LoadEnv(root, os.Environ())`, the one `server.py` has after `.env` is loaded, so what the Model Location tab saves in
`.env` is used (a process variable wins, like `load_dotenv` without override). `~` and `$VAR` are expanded, a relative path is
relative to the install folder, an empty variable counts as unset (the app writes empty settings commented out).

### Semantics kept from huggingface_hub 1.33

- Rows: repos sorted by folder name, revisions by hash; `size_on_disk_str` is `"{:>12}".format(...)`; the sizes are powers
  of 1000 with one decimal; "x ago" uses `format_timesince` (Python's round-half-even, the same limits); repo
  `last_accessed` is the newest blob atime, revision `last_modified` the newest blob mtime (the folder's own when empty);
  `nb_files` is the number of distinct blobs, the revision size counts each blob once.
- A repo with a missing snapshots folder, a missing blob, an unknown type prefix or a ref naming a missing hash is left out with a
  warning, like `CorruptedCacheException`.
- Delete is `delete_revisions`: a repo whose every revision is selected is removed whole (`rmtree`); otherwise the selected
  snapshot folders and their refs go, and a blob goes only when no remaining revision of that repo lists it (a copy that
  lives inside the snapshot goes with it). Order: blobs, refs, repos, snapshots. The expected freed size is returned.
  The scan is redone at delete time, so the page needs no earlier scan (Python kept a global from the last scan).

### Security

The request never carries a path. A revision is named by hash (and repo id/type); `ValidHash` accepts only letters,
digits, `.`, `-`, `_` (at most 128, not starting with `.` or `-`, no `..`), which refuses `..`, `/`, `\`, drive letters,
absolute paths, `%2e`, `~`, `$` and NUL. Even a valid hash is only compared with the names found by the scan; it is never
joined to a path. On top of that:

- The cache folder itself may be a symlink (it is the user's setting); it is resolved once and everything is checked
  against the resolved path.
- A repo folder, snapshot folder, refs folder or ref file that is a symlink is **not followed** (warning).
- A snapshot symlink is read one hop (as Python 1.33 does) and must point **inside the same repo's `blobs/`**, and what it
  resolves to must stay inside the cache. Otherwise the file is skipped with a warning: not counted, not deleted.
  (`blobs/<x>` may itself be a link into a shared store inside the cache; deleting it removes the link only.)
- Before each removal: the path must be absolute and strictly below the cache, its parent chain must contain no symlink
  (`EvalSymlinks(parent) == parent`, which catches a folder swapped for a link after the scan), a folder must be a real
  folder (not a link), a file must not be a folder. A failure is an entry in `errors`; the rest continues.
- `os.RemoveAll` does not follow symlinks inside what it removes. Delete needs `Content-Type: application/json` (the
  repo's CSRF rule for go-data writes) and is serialised with a mutex.
- An ambiguous hash (the same commit in two repos) deletes **nothing** and answers 409. Python deletes it from whichever
  repo it meets first (the loop removes the hash from its set after the first match).

### Deviations from Python (on purpose)

| Python | Go |
|---|---|
| Follows symlinked repo folders, refs folders, refs and snapshot folders | Skips them with a warning |
| Counts a snapshot symlink that points anywhere | Only into the repo's `blobs/` and inside the cache |
| `CacheNotFound` error for a missing folder (the page broke on it) | `{"exists": false, rows: []}` and a warning |
| Ambiguous hash: arbitrary repo | 409, name the repo |
| `HUGGINGFACE_HUB_CACHE=""` set but empty means the current folder | empty means unset |
| `%VAR%` expansion on Windows | not expanded (`~` and `$VAR` are) |

## Evidence (Linux x86-64 CPU runner, 2026-11-26)

| Check | Result |
|---|---|
| `gofmt -l .`, `go vet ./...`, `go test ./...` in `core` (Go 1.27.0) | pass |
| Go tests with a temp cache (`internal/hfcache`, `react_ui_backend/go_hf_cache_test.go`) | scan sizes/refs/rows/sorting; missing and file cache dir; corrupt repos; shared blob kept; whole-repo removal; ambiguity; **24 refused hashes** (`..`, `../x`, `/etc/passwd`, absolute temp dir, `C:\`, `a/b`, `a\b`, `%2e%2e`, NUL, ...); symlinks out of the cache (absolute, relative, repo folder, refs folder) are not scanned and the target is untouched; a hand-made plan with outside, relative and parent paths removes nothing; a snapshot and a blobs folder swapped for links after the scan remove nothing; a blob that is a link removes the link only; the route's 415/400/405/409; cache dir order of precedence |
| Parity with Python: `core/internal/hfcache/testdata/parity.py` builds a fake cache under `/tmp` (model with shared blob, nested `refs/refs/pr/7`, detached revision, `--` in the repo name, dataset, space, empty revision, a copy instead of a symlink, `.locks`, `CACHEDIR.TAG`; times set with `os.utime` days to years back), runs the **extension's own** `scan_cache_json()` (huggingface_hub 1.33.0) and the Go scan, and compares all 13 keys of every row (floats to 1 ms, snapshot paths relative to the cache) | 9 scenarios, 60 rows, **python and go agree** |
| Same script, delete: for 8 selections (one revision with a shared blob, one with a ref, detached, all revisions of a repo, unknown plus real hash, a copy, an empty revision, three repos) it runs `scan_cache_dir().delete_revisions(...).execute()` on one copy and the Go plan on the other and compares the two folder trees (paths, sizes, link targets), the expected freed size and the rows afterwards | identical in all 8 |
| Negative control: the script with the Go side deleting another hash | reported `DIFFERENT` |
| Real launcher (`launch --start`) on a scratch folder with **no Gradio host**, `.env` holding `HF_HUB_CACHE=/tmp/a-run/hf/hub` | `GET :3000/api/go-data/hf-cache/scan` listed the 8 revisions of the fake cache (`cache_dir` from `.env`); `POST :3000/api/gradio/scan_huggingface_cache_api` answered 500 there (what the page used to call) |
| The page in Chromium (Playwright) against that launcher | loaded, 8 rows; clicked Delete on `acme/tiny 11111111`: requests were `GET scan`, `POST delete`, `GET scan`; 7 rows; on disk the snapshot, `refs/main` and the blob only that revision used are gone, the blob shared with the other revision is kept |
| `react-ui`: `tsc --noEmit`, `npm run build:static` | pass |
| `core/ui` and `core/internal/webui/assets` | unchanged; `bundle.hash` depends on `core/ui/src/core-app` and the lockfile only, so the check 'Build core UI and check committed assets' is not affected |

## Not run

- Windows and macOS: not run. They are only cross-checked: `GOOS=windows|darwin GOARCH=amd64|arm64 go vet ./internal/hfcache ./react_ui_backend`
  is clean and the Windows test binary builds, which is all that says. `TestCacheDirFollowsHuggingFaceOrder` uses POSIX absolute paths and would need adjusting on Windows;
  Windows junctions and symlinks without privileges (huggingface_hub then stores copies, which the code handles as "a copy
  in the snapshot", but that was only run on Linux).
- A huggingface_hub **0.x** comparison (the parity ran against 1.33.0, the version installed with Gradio 5.49.1 in the
  scratch venv; older versions format "x ago" with slightly different limits).
- The 1.x **shared blobs store** (`<cache>/blobs/` with `.huggingface-shared-blobs`) is skipped as Python skips it, but a
  cache that really uses it was not built.
- A cache on a network share, very large caches (the scan stats every blob once; no timing was taken), and a delete that
  hits a permission error on a real read-only file (covered only by the "refused" paths).
- Gradio tab against this branch: not started; nothing in its code or routes changed.
