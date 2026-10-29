# Core Rewrite Run Log

Director settings: HEAVY_CHECKS=no · STOP_AFTER=all · started 2026-10-29

Integration branch: `core/rewrite` (from `core/c0-scaffold`)
Draft PR: pending first merge

Open decisions answered as `provisional (director)` in DECISIONS.md:
- D-open-1: port 7700
- D-open-2: accept C5.1 draft schema at review
- D-open-3: pinned uv package
- D-open-4: /setup route in ui/
- D-open-5: minisign, throwaway test key only
- D-open-6: Python 3.10.11 until P4

---

## Wave 1 — 2026-10-29

Checkpoints: C1.1 · C2.1 · C2.2 · C2.3 · C2.4 · P1 · P2 · C8.0
Branch: core/wave-1 | Merge commit on core/rewrite: 1de74b5
CI: ✅ Core (Go) green — all 6 cross-build targets, all unit tests

| Checkpoint | Status | Commit | Notes |
|---|---|---|---|
| C1.1 | done | 530576a | launcher/ subtree added; launcher.yaml workflow |
| C2.1 | done | 31e8e30 | internal/server: token, host/origin guards, SSE |
| C2.2 | done | 524351e | internal/procs: Job Objects (Win), process groups (Unix) |
| C2.3 | done | ada76ba | internal/paths + internal/state |
| C2.4 | done | d537823 | internal/gpu: label detection |
| P1 | done | ab1d8e0 | postgresql removed from conda env |
| P2 | done | c5767ce | tts-webui-sdk/ created; tts_webui/ re-exports |
| C8.0 | done | f5be989 | notes/C8.0.md written; **needs human verification** (rsxdalv/audio.cpp repo private) |

Needs human verification: C8.0 — verify audio.cpp cancel/streaming facts once repo is accessible.
Provisional decisions: all D-open-1..6 pre-answered; no new ones this wave.
Next: Wave 2 — C1.2 · C2.5 · C4.1 · C5.1 · C5.3 · C6.2

---

## Wave 2 — 2026-10-29

Checkpoints: C1.2 · C2.5 · C4.1 · C5.1 · C5.3 · C6.2
Branch: core/wave-2 | Merge commit on core/rewrite: 7bf5aa5
CI: ✅ Core (Go) green — all 6 cross-build targets

| Checkpoint | Status | Commit | Notes |
|---|---|---|---|
| C1.2 | done | b5b277d | Platform adapter; no Neutralino outside platform/ |
| C2.5 | done | ba71fc9 | internal/preflight: 7 checks, injectable runner |
| C4.1 | done | 3e92158 | internal/tools: uv/ffmpeg/MinGit pinned; SHA256 TODOs (needs human verification) |
| C5.1 | done | ebb45f5 | internal/engine/manifest: schema.json + Load() |
| C5.3 | done | b20c9d7 | internal/router: ReverseProxy, WS hijack, cancel |
| C6.2 | done | d3c8666 | ui/ Vite+React scaffold; --ui-dev flag |

Needs human verification: C4.1 SHA256 checksums; C1.2 full TS build; C5.3 HTTP parity.
Provisional decisions: none new.
Next: Wave 3 — join checkpoints C3.1 → C3.2

---

## Wave 3 — 2026-10-29 (join checkpoints, alone)

C3.1: core/c3.1-launch — Status: **in progress**

---
