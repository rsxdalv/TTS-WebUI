# TTS WebUI Call Tree

## Overview

There are **two distinct servers** in this system:

| Server | Type | Port | Started by |
|--------|------|------|------------|
| Log Streaming Server | Node.js HTTP | 7771 | `init_app.js` → `server.js` → `startServer()` |
| Python App Server | Python | 7770 | `start_app.bat` / `start_app.sh` |

---

## Windows Call Tree

```
start_tts_webui.bat
└── root.ps1
      ├── init_mamba.bat                    (bootstrap micromamba)
      ├── init_app.bat
      │     ├── activate.bat
      │     │     └── condabin\activate.bat
      │     └── node init_app.js
      │           ├── checkConda()          (verifies conda)
      │           ├── syncRepo()            (git pull)
      │           ├── startServer()         ★ JS log server on :7771
      │           ├── initializeApp()        (torch/repo setup)
      │           ├── setupReactUI()
      │           └── repairTorch()
      └── start_app.bat
            ├── activate.bat
            │     └── condabin\activate.bat
            └── python server.py            ★ Python app server on :7770
```

---

## Linux/macOS Call Tree

```
start_tts_webui.sh
└── root.sh
      ├── init_mamba.sh                     (bootstrap micromamba)
      ├── $MICROMAMBA_EXE run -p ... node init_app.js
      │     ├── checkConda()
      │     ├── syncRepo()
      │     ├── startServer()               ★ JS log server on :7771
      │     ├── initializeApp()
      │     ├── setupReactUI()
      │     └── repairTorch()
      └── $MICROMAMBA_EXE run -p ... python server.py
                                                ★ Python app server on :7770
```

---

## Key Files

| File | Role |
|------|------|
| `start_tts_webui.bat` / `.sh` | Cross-platform entry points |
| `installer_scripts/root.ps1` / `.sh` | Platform-specific bootstrap (checks, env setup) |
| `installer_scripts/init_mamba.bat` / `.sh` | Micromamba bootstrap |
| `installer_scripts/activate.bat` / `.sh` | Conda env activation |
| `installer_scripts/init_app.bat` / `.js` | App initialization & JS log server |
| `installer_scripts/start_app.bat` / `.sh` | Final Python server launch |
| `installer_scripts/js/server.js` | JS log streaming HTTP server (port 7771) |
| `server.py` | Python Flask/app server (port 7770) |

---

## Data Flow

```
[Neutralino App]
       │                   ┌─────────────────────────────┐
       │                   │     JS Log Server :7771     │
       │                   │  (installer_scripts/js/     │
       │  http://localhost:7771/poll │   server.js)              │
       │◄──────────────────┤                             │
       │                   └──────────────┬──────────────┘
       │                                  │ stdout capture
       │                                  │
       ▼                                  ▼
[User's Browser]              ┌──────────────────────────┐
  :7770 (TTS WebUI)           │  python server.py        │
  :3000 (React UI)            │  (Flask, port 7770)     │
                               └──────────────────────────┘
```

---

## Port Mapping

| Port | Service | Protocol |
|------|---------|----------|
| 7770 | Python TTS WebUI | HTTP (Flask) |
| 7771 | JS Log Streaming Server | HTTP (Node.js) |
| 3000 | React UI (dev) | HTTP (Vite) |

---

## Observations for Refactoring

1. **Two-server architecture** — The JS log server and Python app server are independent processes
2. **Conda env activation is script-bound** — Currently only accessible via `activate.bat/sh` wrappers
3. **Platform divergence** — PowerShell (Windows) vs Bash (Linux/Mac) scripts have grown apart
4. **Double server launch (Windows)** — `init_app.js` spawns the JS server; `start_app.bat` separately spawns `python server.py`
5. **No direct JS→Python bridge** — The JS layer doesn't directly manage the Python process; it only monitors its stdout
