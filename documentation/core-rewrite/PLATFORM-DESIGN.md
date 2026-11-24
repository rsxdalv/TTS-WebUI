# Platform seam: design (P1 gate)

Written 2026-11-24 for the P1 design gate of [PLAN.md](PLAN.md): the manifest fields (D5, D-open-2) and the disk
layout (D28), plus the seam they hang on (D23, D24). Nothing here is implemented yet. After the gate, P1 is built in
the order of the last section. Rob's answers to "For Rob" at the end decide what changes.

Everything below was read from the code on `platform/p1-seam`: `extensions.json` (66 entries, all `interface`, 20 with
`"proxy": "native"`, 10 `recommended`), `extcatalog` (`install.go`, `installed.go`, `torch.go`, `catalog.go`),
`supervise` (`config.go`, `supervisor.go`), `extprocs`, `piplist`, `launcher` (`extensions.go`, `serverpy.go`).

## 1. The seam

One Go package, `core/internal/platform`, holds the interface and the registry. Each implementation is a
subpackage. P1 has two: `platform/legacyenv` and `platform/system`. `uv` arrives in P2 as a third.

```go
// Platform is a shared runtime. Install, status, repair and uninstall are about the runtime;
// the rest is about one entry that runs on it.
type Platform interface {
    Name() string                                   // "legacy-env", "system", later "uv", "openai", ...
    Provides() []string                             // names an entry may require and this platform satisfies

    // Runtime level.
    Status(ctx context.Context) Status              // Ready | Missing, plus Detail (what the user is told)
    Install(ctx context.Context, emit Emit) error   // ErrNotInstallable for the two adopted platforms
    Repair(ctx context.Context, emit Emit) error    // same
    Uninstall(ctx context.Context, emit Emit) error // same: adopted runtimes are never removed by Go

    // Entry level.
    Create(ctx context.Context, e Entry, emit Emit) error  // install the entry onto this platform
    EntryStatus(ctx context.Context, e Entry) EntryStatus  // Installed | Partial | Absent, Version
    Remove(ctx context.Context, e Entry, emit Emit) error
    Command(e Entry, argv []string) (Command, error)        // see below
}

// Command is the resolved process: no shell, no prefix.
type Command struct {
    Program string
    Args    []string
    Env     []string // complete environment (the platform adds what it needs, e.g. MAMBA_ROOT_PREFIX)
    Dir     string
}
```

`argv[0]` is a **logical program**: `python`, `uv`, `npm`. The platform decides what that means. Today there are
three meanings, and the seam has to hold all three:

| Platform | `Command(e, ["python","-m","pip",...])` |
|---|---|
| `legacy-env` | `micromamba -p installer_files/env run python -m pip ...`; arguments caret-escaped on Windows; `MAMBA_ROOT_PREFIX` in `Env` |
| `system` | `python -m pip ...` from `PATH`, the launcher's environment |
| an own-venv entry (any platform) | `.venvs/<package>/{bin,Scripts}/python -m ...` directly; no shell, no escaping |

An own-venv entry's `Command("python")` does not depend on the platform that built the venv: it is the venv's own
python, which is what `extprocs.python()` already does. A platform is only consulted for `uv`, `npm` and the
main-env `python`.

### What moves where (no change in behaviour)

| Today | After P1 |
|---|---|
| `supervise.Config.CommandPrefix`, `micromambaPrefix`, `mambaRootEnv`, `CommandPrefix()`, `MambaRootEnv()` | `legacyenv` (the only place that knows `micromamba -p ... run`) |
| `supervise.Config.SystemPython` | `system` platform; the flag `--system-python` and `WithSystemPython` select it |
| `supervise.pythonAvailable` (prefix program exists, `-p` directory exists) | `Platform.Status`, same messages word for word |
| `piplist.hasPython` (same check, again) | `Platform.Status` |
| `extcatalog.Options.Run/Python/Env`, `run`, `runsThroughCmd`, `escapeForMambaRun` | `Platform.Command`; the escaping lives in `legacyenv` |
| `extcatalog.pipInstallTokens`, `pipUninstall` | `legacyenv`/`system` `Create`/`Remove` for an in-process entry |
| `extcatalog.venvSetupTokens` (`uv venv`, torch, torchcodec, xformers, requirements) | one shared helper `platform/venventry` that runs `uv` through `Platform.Command`; `Create` of an own-venv entry calls it. P2 swaps the `uv` it runs. |
| `extcatalog.Installs.Of` (pip list name, or `.venvs/<pkg>` exists) | `Platform.EntryStatus` |
| `extprocs.Options.MainPython`, `extprocs.python()` | `Platform.Command(e, ["python"])` |
| `launcher.react_ui_mode` npm via `micromamba` | `Platform.Command(nil, ["npm", ...])` |

`torch.go`, the `.gpu` file and the GPU gate stay exactly as they are in P1. They move in P2/P3 (D29).

### Which platform an install gets

Chosen once at launcher start and shown on the Extensions page:

1. `--system-python` given: `system`.
2. else `installer_files/env` exists: `legacy-env`.
3. else: none. The core runs (as since #747); the status says `no Python environment`, the same text as today.

`legacy-env` is adopted, never installed: its `Install` returns `ErrNotInstallable` and the legacy installer step
(micromamba, gpu, install in the launcher) keeps creating it for a new install until P4. A new folder in P1 is
therefore on rule 3 until the user runs that step.

## 2. Manifest fields (D5, D-open-2)

`extensions.json` entries today carry `package_name`, `name`, `requirements`, `description`, `extension_type`,
`extension_class`, `author`, `extension_author`, `license`, `website`, `extension_website`,
`extension_platform_version`, and on some `proxy` and `recommended`. **P1 adds one field, `requires`.** The fuller D5
schema is listed so the later fields have a reserved place and name, but nothing is read before the item that needs
it (no field that nothing reads).

| Field | Type | Added in | Meaning |
|---|---|---|---|
| `requires` | list of strings | **P1** | Names of platforms and entries that must be present first. Optional; derived when absent (below). |
| `python` | string, e.g. `"3.11"` | P2 | Python version for an own-venv entry; default 3.10.11 (D25). |
| `torch` | string, a row name | P2 | Torch table row; default row per OS and GPU (D29). |
| `launch` | object: `command`, `args`, `env`, `cwd` | P6 | How the engine's process starts (replaces the proxy harness). |
| `health` | object: `path`, `timeout_s` | P6 | Readiness probe. |
| `api` | object: `kind` (`openai`\|`gradio`), `base_path` | P6 | What the process serves (D27). |
| `ui` | object: `kind` (`gradio-iframe`), `path` | P2 (iframe) | How the Vite engine page shows it. |
| `keep_alive` | duration string | P6 | Idle stop (D31). |
| `cancel` | string | P7 | How a running generation is cancelled (for `audiocpp`). |

`proxy: "native"` is not replaced in P1 or P2. It stays the marker of an own-venv engine (20 entries) and is the input
of the `requires` derivation. Whether it is renamed, and when, is for Rob (below).

### `requires`

- **Syntax.** A list of strings. A string is a **platform name**, the reserved entry id `gradio-host`, or another
  entry's `package_name`. No version ranges in P1: nothing needs them, and a range with no resolver is a placeholder.
- **Reserved names:** `openai`, `uv`, `audiocpp`, `onnx`, `legacy-env`, `system`, `gradio-host`. No catalog entry may
  take one of these as a `package_name` (checked when the catalog loads).
- **Derived** when the field is absent (the `extensions.json` file is not rewritten; PLAN "three kinds of entry"):

  | The entry has | Derived `requires` |
  |---|---|
  | `"proxy": "native"` | `["uv"]` |
  | otherwise (in-process tab, built-in tab, decorator) | `["gradio-host"]` |

  There is no derived `openai` yet (no remote entries exist). An explicit `requires` replaces the derived one.
- **Validation.** Each name must match `^[A-Za-z0-9][A-Za-z0-9._-]*$`. A name that is neither a platform, a reserved id
  nor a catalog entry is a **load error that names the entry and the name**, not a silent skip. The catalog from the
  online list and a pasted entry (the install-entry path) are untrusted: `requires` is only ever *looked up*, never
  executed, and a pasted entry's `requires` is validated the same way before anything runs.

### How a name is satisfied in P1

`gradio-host` and `uv` are not entries or platforms yet (P3 and P2). In P1:

- `legacy-env` and `system` **provide** both (`Provides() = ["gradio-host", "uv"]`): on a legacy install the main env
  *is* the Gradio host and its `uv` runs the native installs, which is exactly today's behaviour.
- With no platform (rule 3) neither is provided. `uv`'s answer is `would install: the uv platform (not available
  until P2)`; `gradio-host`'s is `would install: the Gradio host (not available until P3)`. The Extensions page says
  so and the Install button is disabled with that text. The message is a real statement of the current state, not a
  stub of a later feature.

## 3. Install and remove order (D30)

- **Plan before action.** `Resolve(entry) -> Plan` lists, in order, what is missing: each missing requirement
  (depth first, no repeats), then the entry. A cycle is a load error. The Extensions page shows the plan before the
  user confirms ("Chatterbox (requires uv): install uv, then Chatterbox").
- **Execution.** One step at a time under the existing install lock (the one in `extcatalog`, `s.install`); the stream
  gets one section per step. A failed step stops the plan and says which step failed. P1 adds the disk-space check
  and the "run again after an interrupted install" statement only where it already holds; the rest is P2 (it needs
  the venv state marker, see below).
- **Remove.** `Dependants(name)` lists installed entries whose (derived or explicit) `requires` contains `name`.
  A non-empty list refuses the removal and returns the list. In P1 this can only fire for entry-to-entry
  `requires`; the two adopted platforms are not removable at all (refusal text: "managed outside this app").
- **The two existing paths do not change shape.** `POST /api/extcatalog/install` (by name) and
  `.../install-entry` (pasted entry) keep their request and NDJSON answer. They gain the plan step before the
  install, and `requires` from the catalog file, not from the caller, for the by-name call.

## 4. What the Extensions page shows

Each row of `extItem` gains:

```json
"requires": ["uv"],
"requirements_state": [
  {"name": "uv", "kind": "platform", "state": "ready", "via": "legacy-env"}
]
```

`state` is `ready` (satisfied now; `via` names the platform or entry that does it), or `missing` with
`would_install` (the ordered plan, or the reason it cannot be done yet). The page prints "Requires: uv (provided by
legacy-env)" or "Requires: uv: not installed. Would install the uv platform: not available until P2". A short header
line says which platform this install is on (`legacy-env`, `system`, or `none`).

## 5. Disk layout (D28)

```
<install folder>/
  installer_files/            legacy only: mamba/, env/ (adopted in place, never moved or edited by Go)
  platforms/<name>/           one folder per installed platform (P2+; P1 creates none)
      platform.json           {"name","version","installed_at"}  written last, so absent = incomplete
  .venvs/<package_name>/      one venv per own-venv entry (exists today)
      entry.json              {"package","platform","installed_at"} written last   (P2)
  cache/                      the uv wheel cache, on the same volume as .venvs (P2; today ./cache by pyproject.toml)
```

- **P1 changes nothing on disk.** `platforms/` is reserved and not created; `legacy-env` and `system` leave no
  marker (they are adopted, there is nothing to say was installed). No existing install is moved or rewritten.
- **Names.** Platform folder names are the platform names and are at most 12 characters (Windows path length, see
  PLAN "Risks"). `.venvs/<package_name>` keeps the entry's full package name (e.g.
  `tts_webui_extension.bark`, 24 characters). Whether to shorten it is P2's decision, when path depth is
  measured on a real install (see "For Rob" 3).
- **Entry state.** Today an own-venv entry is "installed" when `.venvs/<package>` exists, so it already says
  installed while the download is running (P0 baseline). The fix is the `entry.json` written last: the entry is
  `Installed` when it exists, `Partial` when only the directory does. This is P2 because it needs the new install
  path; until then P1 keeps today's answer (stated, not hidden).
- **Never touched by Go:** `installer_files/` beyond the existing installer step, `outputs/`, `config.json`, `.env`,
  `data/`, `voices/`, `favorites/`.

## 6. The grep guard

Demo line of P1: `micromamba` and `installer_files/env` appear in no Go package except `legacy-env` and the installer
step. Made concrete as a Go test, `core/internal/platform/guard_test.go`, that walks `core/**/*.go` and **fails** if
any non-test file matches `(?i)micromamba|installer_files|MAMBA_ROOT_PREFIX` outside this exact allow-list:

| Allowed | Why |
|---|---|
| `internal/platform/legacyenv/` | the platform itself |
| `installer_scripts/` | the installer step: it creates the env (removed for a default install in P4) |
| `internal/launcher/steps.go` and the step handlers in `handlers.go` | the launcher's mamba/gpu/install steps and their markers (P4) |
| `internal/paths/paths.go` | defines the folder (`InstallerFiles()`); no behaviour |
| `cmd/tts-webui/cmd_launch.go` | the flag's help text names the micromamba env |
| `react_ui_backend/go_ffmpeg_metadata.go` | ffprobe search in the env's folders; no seam method for it in P1 (decided), removed in P4 |
| `internal/launcher/procscan_other.go` | orphan sweep matches the word `micromamba`; no seam method in P1 (decided), removed in P3 |

Everything else the earlier grep found outside that list (supervise, extcatalog, piplist, extprocs wiring,
`react_ui_mode.go`, the preflight message) moves behind the seam or is reworded. The test is allowed to fail on a new file; adding a file to the allow-list is a reviewed change.
Smoke and test fakes (`scripts/smoke`, `_test.go`) are exempt: the rule is about shipped code.

## 7. How "no change in behaviour" is checked

- The existing Go tests keep passing without edits to what they assert (fake `micromamba`, `python`, `uv` stubs
  stay, as the Verification rules allow; they stop at the boundary).
- Windows and WSL: the P0 lines are rerun on the P1 binary and compared with [P0-BASELINE.md](P0-BASELINE.md): start
  with the real env (112 tabs, none `(!)`, clean stop), `go test ./internal/launcher -run TestReal` with
  `LAUNCHER_REAL_ROOT` (3 pass on Windows, 2 on Linux), and the Piper native install through the launcher API.
- New: the Extensions page answer for a legacy install (every entry shows `requires` and `ready`), and for a new
  folder (shows what would be installed), checked through the real launcher and the page in the browser.

## 8. Build order after the gate (each ends in a commit with gofmt, vet and test green)

1. `internal/platform`: interface, `Command`, `Status`, registry, detection (rules 1 to 3), tests for the three
   `Command` shapes against the strings used today.
2. `legacyenv` and `system`: move the prefix, the escaping and the status messages; supervise and piplist use
   `Platform.Status`; supervisor tests unchanged.
3. `extcatalog` and `extprocs` take a `Platform` instead of `Run`/`Python`/`Env`/`MainPython`; `venventry` helper.
4. `requires`: derivation, validation, `Resolve`, `Dependants`, the install plan.
5. The Extensions page and API fields (section 4); Vite and launcher UI.
6. `react_ui_mode` npm move; the grep guard test.
7. Real runs on Windows and WSL against the baseline; `INTEGRATION.md` for `platform`.

## 9. Not decided here / not in P1

- The `uv` platform, pinned uv, torch table, the `entry.json` state marker (P2).
- `gradio-host` as an entry that can be installed or removed (P3).
- A version syntax in `requires`; `python`/`torch` fields (P2); `launch`/`health`/`api`/`keep_alive` (P6).
- Anything under `platforms/` on disk.

## Gate answers (Rob, 2026-11-24)

1. `legacy-env` and `system` provide `uv` and `gradio-host` in P1: **accepted**.
2. `proxy: "native"` stays and `requires` is derived from it: **accepted**.
3. Shorten venv folder names in P2 after measuring depth: **accepted**.
4. The grep guard keeps the launcher step files and `paths.go` allowed until P4: **stay allowed**.
5. `ToolDirs()` and `ProcessMarkers()`: **left out for now**. `go_ffmpeg_metadata.go` and `procscan_other.go` are on the
   allow-list instead (section 6).
