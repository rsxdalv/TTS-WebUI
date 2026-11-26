# Race detector, flakiness and core-UI CI check

Run on an Ubuntu 24.04 cloud runner (gcc 13.3, `CGO_ENABLED=1`) against `main` at
`b844e0a`. Go is whatever `core/go.mod` asks for: `go 1.27.0` (the runner had 1.24.7;
`GOTOOLCHAIN=auto` fetched 1.27.0). Node 22.23.5 and 20.20.2 for the UI.

Summary:

- **No data races** in `core`. Nothing to fix there.
- `go vet`, `gofmt -l`, `go test` are clean and not flaky (3 runs).
- The failing CI check on #749-#755 and on main at `6eba853`, `71ae227`, `f41a69c` and
  `d2c464a` is real and has two causes (below). **main at `b844e0a` is clean.**
- Running the suite on GitHub's Windows and macOS runners (below) found **two real macOS
  bugs**, now fixed here. Windows was green from the first run.
- This branch adds: a robust `bundle-hash.sh`, a stricter CI asset check, a `-race` CI
  step, the two macOS fixes, and a manual-only cross-platform workflow.

## Task A: Go

All commands from `core/`.

| Command | Result |
|---|---|
| `go vet ./...` | clean |
| `gofmt -l .` | no files listed |
| `CGO_ENABLED=1 go test -race ./... -count=1` | 34 packages `ok`, 0 `DATA RACE`, 1m44s wall |
| `CGO_ENABLED=1 go test -race ./... -count=3` | 34 packages `ok`, 0 `DATA RACE` |
| `go test ./... -count=1`, three times | exit 0 all three, same packages `ok` each time |
| `go test -v ./... -count=1` | 928 `=== RUN` lines, 912 PASS, 16 SKIP, 0 FAIL |

**Flaky tests: none seen.** Per-package wall times were stable to about 1-2% across the
three plain runs (slowest: `installer_scripts` ~9.9s, `serverpy/extprocs` ~8.3s,
`platform/audiocpp` ~6.6s). Three runs cannot rule out a one-in-a-hundred flake.

**Races: none.** Nothing was fixed because nothing was found.

What a clean result does and does not tell you:

- The race detector only sees code that runs concurrently in a test. Of the non-test
  files that use goroutines or `sync`, these have **no test that uses them from more than
  one goroutine**: `internal/serverpy/history`, `logview`, `piplist`,
  `internal/platform/openai/store.go`, `internal/launcher/{hoststatus,handlers,log,browser,archive}.go`,
  `react_ui_backend/{claims,proxy,proxy_lan,go_gpu_info,core_client,gradio_client}.go`.
- I read the lock discipline of `openai/store.go`, `history`, `logview`, `piplist` and
  `claims.go`: every guarded field is accessed under its mutex, nothing to report.
  `piplist.Cached` holds `cacheMu` for the whole `pip list` (seconds), so `Invalidate`
  blocks behind it. That is latency, not a race.
- I added `TestStoreConcurrentUse` to `platform/openai` and `serverpy/history` (8
  goroutines, 25 iterations; run under `-race -count=5`: clean). The other packages
  above were **not** given concurrent tests.

### Skipped tests (what `go test` did not exercise)

All 16 skips are by design here:

- Windows-only: `TestSystem32InPath`, `TestExFAT`, `TestLongPaths`,
  `TestConflictingConda`, `TestMSVC` (`internal/preflight`),
  `TestInitMambaWindowsInvalidMicromambaFileLikeOriginal` (`cmd.exe`),
  `TestRootWritesOutputLog`.
- Need a real install or the network, gated on an env var: `LAUNCHER_REAL_ROOT` (4 tests
  in `internal/launcher`), `AUDIOCPP_REAL_ROOT`, `ONNX_REAL_ROOT`, `UV_REAL_ROOT` (2
  tests), `EXTPROCS_REAL_PYTHON` / `EXTPROCS_REAL_ROOT`.

**Not run (on Linux):** any of the `*_REAL_ROOT` tests (they download real runtimes and
Python packages), the Windows-only tests (they do run on the Windows runner, see the
Windows and macOS section), and anything needing a GPU or API keys. Races in the
real subprocess paths those tests cover are therefore unchecked.

## Task B: CI health

### State of GitHub Actions (read with `gh api`)

Workflows: `core.yaml`, `tests.yaml`, `test-react-ui.yaml`, `test-pip-install.yaml`,
`test-installer.yaml` (manual only), `publish-docker-image.yaml`,
`update-full-repo-zip.yaml`, `create-release.yaml`, `build-launcher-exe.yaml`.

Runs on `main`:

| Commit | Core (Go) | Tests | React UI | Docker | Zip | Test default install |
|---|---|---|---|---|---|---|
| `b844e0a` P8 (tip) | success (all 8 jobs) | success | success | success | success | cancelled |
| `4a28c22` P7 | success | success | success | success | success | cancelled |
| `d2c464a` P6 | **failure** | success | success | success | success | cancelled |
| `f41a69c` P5 | **failure** | success | success | success | success | cancelled |
| `71ae227` P4 | **failure** | success | success | success | success | cancelled |

In every failing Core run the `test` job and all six `build` jobs passed; only
`Build core UI and check committed assets` failed, in its last step.

"Test default install (gradio-host)" is `cancelled` on every push, including the tip.
Its macOS job dies instantly with `The configuration 'macos-14-arm64-us-default' is not
supported`; the ubuntu and windows jobs of the same run succeed. Unrelated to this work,
not changed.

PRs #749-#755 each have a failed `Core (Go)` run (push and `pull_request`) for the same
single job; #756 and #757 pass. #750 is docs-only and the workflow's path filter skips it.

### Why the check failed

The check rebuilds the UI and compares the recomputed `bundle.hash` (sha256 over
`core/ui/src/core-app/**` plus `core/ui/package-lock.json`) with the one committed.
I recomputed it from the git tree of every commit involved:

| Commit | committed `bundle.hash` | computed from the tree | committed assets vs fresh build |
|---|---|---|---|
| `1ab9437` and older | = | = | not checked |
| `6eba853` P1-P3 | `45467c52` | `16112e05` | **stale**: different JS chunk and `index.html` |
| `71ae227` P4 | `f9298e4f` | `9de2664e` | `index.html` and live chunks right, **2 orphaned files** (`index-CZJ8az9Q.js`, `index-CctKpNYq.css`) |
| `f41a69c` P5 | `f9298e4f` | `9de2664e` | same as P4 |
| `d2c464a` P6 | `518a73bf` | `c501229e` | identical, only the hash is wrong |
| `4a28c22` P7, `b844e0a` P8 | `32d98955` | `32d98955` | identical |

PR heads show the same: #749, #751, #752, #753, #754 and #755 each had a committed hash
that did not match their own sources.

**Cause 1, line endings in the hash (explains the wrong hashes).** `bundle-hash.sh`
hashed the working-tree bytes. `.gitattributes` has `* text=auto eol=lf`, so git stores
LF, but a working tree can hold CRLF. I brute-forced which files being CRLF would
reproduce a committed hash: for `38ba97a` (committed `518a73`) and `14f136d` (committed
`f9298e`), the committed hash is reproduced exactly when **only
`core/ui/src/core-app/ExtensionsView.tsx` is CRLF**. That is a file edited on the
author's Windows machine. Linux CI always sees LF and can never match. One more sampled
commit (`8640f53`, committed `45467c`) did not reproduce under that search, so it may
have a different cause; the first two are conclusive, a random match is out of the
question. I did not find evidence for the other suspects: not the Node version (a fresh
build is byte-identical on Node 20 and 22), not the lockfile (it is LF in git and the
build installs with `npm ci`), not the script (unchanged since `9b6d3b5`).

**Cause 2, squash-merged stacked branches (explains the stale assets).** `6eba853` and
`71ae227`/`f41a69c` carry assets that do not match a fresh build of their own sources
(stale chunk and `index.html`; leftover hashed chunks). The bundle filenames contain
content hashes, so when stacked PRs each regenerate `assets/` and are squashed in turn,
git keeps both sides' files. This is a plausible mechanism from the file listings; I
did not replay the merges.

**Fragility in the check itself.** It compared only `bundle.hash`. It never looked at
`index.html` or `assets/*`, so it could not see Cause 2 at all, and it covers only
`core-app/**` and the lockfile (not `vite.core.config.ts`, `package.json`, `core.html`,
`public/`). It also ran Node 20 while `core/ui/package.json` declares `engines.node >= 22`
(`npm ci` printed `EBADENGINE` warnings).

### Is main clean now?

Yes. At `b844e0a`, `bash core/scripts/build-webui.sh` followed by `diff -r` against the
committed `core/internal/webui/assets` is identical (`bundle.hash`, `index.html`,
`assets/index-Bm4Ca5wA.js`, `assets/index-CXExg_sU.css`), on both Node 22.23.5 and
Node 20.20.2. The Core (Go) run on main at the tip is green.

### Fixes on this branch (separate commits)

1. **`core/scripts/bundle-hash.sh`: strip CR before hashing.** For LF files the output is
   byte-identical to before, so the committed `bundle.hash` does not change (checked:
   unchanged at the tip; a CRLF copy of `ExtensionsView.tsx` and the lockfile gives the
   same hash; a one-line content edit changes it).
2. **`.github/workflows/core.yaml`: compare the whole assets directory.** After
   `build-webui.sh` (which wipes and rebuilds `core/internal/webui/assets`), fail if
   `git status --porcelain --untracked-files=all -- core/internal/webui/assets` is
   non-empty, and print the diff. Node 20 becomes Node 22 to match `engines`.
   Dry run of exactly that logic locally: **passes** on the tip with Node 22 and with
   Node 20; **fails** on `71ae227` (shows the 2 orphans and the hash), `d2c464a` (hash),
   `6eba853` (stale chunk, `index.html`, hash). (The old step would have passed `6eba853`
   if its hash had matched.)
3. **`.github/workflows/core.yaml`: add `go test -race ./...`** to the Linux `test` job
   (`CGO_ENABLED=1`), plus two small concurrent-use tests. Costs roughly 2 minutes
   extra per run (the race run took 1m44s here).

Trade-offs to be aware of:

- The stricter check depends on Vite output being byte-deterministic for a given
  lockfile. It was identical across Node 20 and 22 here; a Vite/Rollup bump in the
  lockfile that changes output will now (correctly) require rebuilding and committing
  the assets.
- Stacked PRs can still conflict on `assets/`. The check does not prevent that; it makes
  the stale result fail on the PR instead of landing. Rebuilding after the final rebase
  is the remedy.
- `bundle.hash` is now redundant as a gate (the directory diff covers it). I left it in
  place: no Go code reads it (grep: only `build-webui.sh`, `bundle-hash.sh`, the workflow
  and `core/README.md` mention it), so removing it is a safe follow-up if you want.

### Windows and macOS (GitHub runners)

Linux cannot run these, so `.github/workflows/core-cross-platform.yaml` (manual
`workflow_dispatch`, plus PRs that edit it) runs gofmt, `go vet`, `go test`,
`go test -race` and the core UI build on `windows-latest` and `macos-latest`
(macOS runner is arm64). Three runs on this branch:

| Run | windows go | windows UI | macOS go | macOS UI |
|---|---|---|---|---|
| 1 | pass (incl. `-race`) | pass | **2 test failures** | step failed (harness bug) |
| 2 (macOS fixes) | pass | step failed (harness bug) | pass | pass |
| 3 (harness fixed) | pass | pass | pass | pass |

Findings:

- **Real bug, macOS: `procs.Tree.Assign` (`internal/procs/kill_unix.go`).** The contract
  (`tree.go`, relied on by `runner.go` and `proc.go`) is to return `ErrProcessExited`
  when the child ended before it was attached. The Unix code returned the raw `getpgid`
  error instead. On macOS `getpgid` of an ended child is `ESRCH` (on Linux a zombie keeps
  its group), so a short command was reported as `assigning process tree: getpgid: no
  such process` and the step failed. Seen in
  `TestCommandThatExitsBeforeAttachIsANormalFinish`. Fixed by mapping `ESRCH` to
  `ErrProcessExited`; regression test `internal/procs/kill_unix_test.go` (it passes on
  Linux either way, it is the macOS run that proves the fix).
- **Wrong test assumption, macOS:** `TestInstallMakesTheVenvAndHoldsTorch` hard-coded
  `torch==2.11.0+cpu`. The only macOS torch row is PyPI (`torch==2.11.0`), so the product
  code was right and the test now expects the pin of the platform's row.
- **No data races on macOS or Windows** (`go test -race` ran on both).
- **CRLF check:** with 443 CR bytes written into `ExtensionsView.tsx`, `bundle.hash` is
  unchanged (`32d98955...`) on both Windows and macOS, and a fresh UI build on both
  matches the committed assets byte for byte. That confirms the LF-normalising hash and
  shows the Vite output is the same on Linux, Windows and macOS.
- The first harness failures were mine: BSD `sed -i` on macOS, and MSYS `grep` hiding CR
  on Windows.

Should this stay? It costs two slower runners per run, so it is manual-only and not in
`core.yaml`. It found real bugs the Linux checks cannot see; run it before releases or
after touching process, path or installer code. Dropping the file from the PR is also
fine; the fixes do not depend on it.

### Not run

- Nothing about the edited `core.yaml` is unverified any more: it ran green on this PR
  (test job incl. `-race`, the asset check on Node 22, all six cross-builds).
- Windows and macOS were run only on GitHub's runners (section above), not on a developer
  machine. Wine 9.0 crashes here (`free(): invalid pointer` creating a prefix), so there
  was no local emulation; locally I only ran `go vet` for `windows/*`, `darwin/*` and
  `linux/arm64`. The `*_REAL_ROOT` and Windows-only tests that need a real install were
  still skipped on every OS. The CRLF explanation for the stale hashes is inferred from
  reproducing them, not observed on the author's machine.
- `*_REAL_ROOT` tests, GPU and API-key paths, the board tool.
