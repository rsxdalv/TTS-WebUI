// Platform abstracts all host-native capabilities so that step components
// never import from @neutralinojs/lib directly.
export interface Platform {
  exec(cmd: string): Promise<{ exitCode: number; stdout: string; stderr: string }>;
  /** Spawn a long-running process. Returns the OS pid. */
  spawn(
    cmd: string,
    opts?: {
      cwd?: string;
      onData?: (data: string) => void;
      onStdout?: (data: string) => void;
      onStderr?: (data: string) => void;
      onExit?: (code: number) => void;
    },
  ): Promise<number>;
  kill(pid: number): Promise<void>;
  readFile(path: string): Promise<string>;
  writeFile(path: string, content: string): Promise<void>;
  stat(path: string): Promise<{ exists: boolean; isDir: boolean }>;
  mkdir(path: string): Promise<void>;
  getOS(): Promise<"windows" | "linux" | "darwin">;
  getCwd(): Promise<string>;
}

// Synchronous constants derived from NL_* globals.
// These are exported so config.ts and other initialisation code can read them
// without re-reading window.NL_* (which must stay confined to this package).
export { OS, CWD, initPlatform } from "./neutralino";

export { default as platform } from "./neutralino";
export { default } from "./neutralino";
