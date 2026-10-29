// Core platform adapter.
// Implements the Platform interface by calling Go step endpoints and /api/*.
// Used when VITE_PLATFORM=core (see vite.config.ts alias).
import type { Platform } from "./index";

// ---------------------------------------------------------------------------
// Token + health cache
// ---------------------------------------------------------------------------

let _token: string | null = null;
let _os: "windows" | "linux" | "darwin" | null = null;
let _cwd: string | null = null;

function getToken(): string {
  if (_token !== null) return _token;
  const meta = document.querySelector<HTMLMetaElement>(
    'meta[name="tts-webui-token"]',
  );
  _token = meta?.content ?? "";
  return _token;
}

function authHeaders(): Record<string, string> {
  return { "X-TTS-Token": getToken() };
}

async function fetchHealth(): Promise<void> {
  if (_os !== null) return;
  try {
    const r = await fetch("/api/health", { headers: authHeaders() });
    if (r.ok) {
      const j = await r.json();
      _os = j.os ?? "linux";
      _cwd = j.cwd ?? ".";
    }
  } catch {
    _os = "linux";
    _cwd = ".";
  }
}

// ---------------------------------------------------------------------------
// Build-time constants (required by index.ts re-exports)
// ---------------------------------------------------------------------------

/** OS detected at runtime via /api/health; defaults to "windows" until fetched. */
export let OS: "windows" | "linux" | "darwin" = "windows";

/** Working directory fetched from /api/health; defaults to "." until fetched. */
export let CWD: string = ".";

export async function initPlatform(): Promise<void> {
  await fetchHealth();
  if (_os !== null) OS = _os;
  if (_cwd !== null) CWD = _cwd;
}

// ---------------------------------------------------------------------------
// Step helpers (importable by step components)
// ---------------------------------------------------------------------------

export async function callStep(step: string, body?: object): Promise<void> {
  await fetch(`/api/steps/${step}`, {
    method: "POST",
    headers: { ...authHeaders(), "Content-Type": "application/json" },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

export function subscribeStep(
  step: string,
  onLine: (line: string) => void,
): EventSource {
  // tts-token cookie is set by the server on the first GET /; no header needed.
  const es = new EventSource(`/api/steps/${step}/stream`);
  es.onmessage = (e) => onLine(e.data);
  return es;
}

// ---------------------------------------------------------------------------
// Platform implementation
// ---------------------------------------------------------------------------

const CorePlatform: Platform = {
  async exec(_cmd) {
    return {
      exitCode: 1,
      stdout: "",
      stderr:
        "exec not supported in core mode — use step endpoints",
    };
  },

  async spawn(_cmd, _opts) {
    console.warn(
      "[core platform] spawn() is not supported; use step endpoints",
    );
    return 0;
  },

  async kill(_pid) {
    // no-op: Go manages processes
  },

  async readFile(_path) {
    // Generic file read not supported in core mode (D9).
    throw new Error("readFile not supported in core mode");
  },

  async writeFile(path, content) {
    if (path.endsWith(".gpu")) {
      await fetch("/api/steps/gpu", {
        method: "POST",
        headers: { ...authHeaders(), "Content-Type": "application/json" },
        body: JSON.stringify({ label: content.trim() }),
      });
      return;
    }
    throw new Error("writeFile not supported in core mode for non-.gpu paths");
  },

  async stat(_path) {
    // Deduce from /api/steps/status whether the step is done; no generic stat.
    // Return a safe default (exists: false) — callers check step status directly.
    return { exists: false, isDir: false };
  },

  async mkdir(_path) {
    // Go creates required dirs automatically; treat as success.
    return;
  },

  async getOS() {
    await fetchHealth();
    return _os ?? "linux";
  },

  async getCwd() {
    await fetchHealth();
    return _cwd ?? ".";
  },
};

export default CorePlatform;
