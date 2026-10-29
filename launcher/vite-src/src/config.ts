import path from "node:path";

const isWindows = (window as any).NL_OS === "Windows";

const normalize = (p: string) =>
  isWindows ? path.normalize(p).replace(/\//g, "\\") : path.normalize(p);

const toUnix = (p: string) => p.replace(/\\/g, "/");

export const joinPath = (...parts: string[]) =>
  normalize(path.join(...parts.map(toUnix)));

export const PWD = normalize((window as any).NL_CWD || ".");
export const WEBUI_ROOT = normalize(import.meta.env.VITE_WEBUI_ROOT || PWD);
export const OUTPUTS_PATH = joinPath(WEBUI_ROOT, "outputs");
