// Neutralino platform adapter.
// This is the ONLY file in the launcher that imports @neutralinojs/lib or
// reads NL_* globals. All other files use the Platform interface from ./index.
import { os, events, filesystem, init } from "@neutralinojs/lib";
import type { Platform } from "./index";

// ---------------------------------------------------------------------------
// Sync constants — the only place NL_* globals are read.
// ---------------------------------------------------------------------------

const _rawOS = (window as any).NL_OS as string | undefined;
const _rawCWD = (window as any).NL_CWD as string | undefined;

/** Current OS in lower-case form ('windows' | 'linux' | 'darwin'). */
export const OS: "windows" | "linux" | "darwin" = (() => {
  if (_rawOS === "Windows") return "windows";
  if (_rawOS === "Darwin") return "darwin";
  return "linux";
})();

/** Current working directory as set by Neutralino at launch. */
export const CWD: string = _rawCWD ?? ".";

/** Re-exported Neutralino init function for main.tsx. */
export { init as initPlatform };

// ---------------------------------------------------------------------------
// pid → Neutralino process id mapping (required for updateSpawnedProcess).
// ---------------------------------------------------------------------------

const _pidToNlId = new Map<number, number>();

// ---------------------------------------------------------------------------
// Platform implementation
// ---------------------------------------------------------------------------

const NeutralinoPlatform: Platform = {
  async exec(cmd) {
    try {
      const r = await os.execCommand(cmd);
      return { exitCode: r.exitCode, stdout: r.stdOut, stderr: r.stdErr };
    } catch {
      return { exitCode: 1, stdout: "", stderr: "Command failed to execute" };
    }
  },

  async spawn(cmd, opts) {
    const spawnOpts: Parameters<typeof os.spawnProcess>[1] = opts?.cwd
      ? { cwd: opts.cwd }
      : {};
    const result = await os.spawnProcess(cmd, spawnOpts);
    const { id, pid } = result;
    _pidToNlId.set(pid, id);

    if (opts?.onData || opts?.onStdout || opts?.onStderr || opts?.onExit) {
      const handler = (evt: any) => {
        const evtId: number =
          typeof evt.detail.id === "string"
            ? parseInt(evt.detail.id, 10)
            : evt.detail.id;
        if (evtId !== id) return;
        const { action, data } = evt.detail as {
          action: "stdOut" | "stdErr" | "exit";
          data: string | number;
        };
        if (action === "stdOut") {
          opts?.onData?.(data as string);
          opts?.onStdout?.(data as string);
        } else if (action === "stdErr") {
          opts?.onData?.(data as string);
          opts?.onStderr?.(data as string);
        } else if (action === "exit") {
          _pidToNlId.delete(pid);
          events.off("spawnedProcess", handler);
          opts?.onExit?.(data as number);
        }
      };
      events.on("spawnedProcess", handler);
    }

    return pid;
  },

  async kill(pid) {
    const nlId = _pidToNlId.get(pid);
    if (nlId !== undefined) {
      await os.updateSpawnedProcess(nlId, "exit").catch(() => {});
      _pidToNlId.delete(pid);
    }
    if (OS === "windows") {
      await os.execCommand(`taskkill /F /T /PID ${pid}`).catch(() => {});
    }
  },

  async readFile(path) {
    return filesystem.readFile(path);
  },

  async writeFile(path, content) {
    await filesystem.writeFile(path, content);
  },

  async stat(path) {
    try {
      const info = await filesystem.getStats(path);
      return { exists: true, isDir: info.isDirectory };
    } catch {
      return { exists: false, isDir: false };
    }
  },

  async mkdir(path) {
    await filesystem.createDirectory(path);
  },

  async getOS() {
    return OS;
  },

  async getCwd() {
    return CWD;
  },
};

export default NeutralinoPlatform;
