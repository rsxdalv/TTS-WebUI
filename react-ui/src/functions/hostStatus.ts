import React from "react";
import { goGetJson } from "./goData";

/**
 * Whether the Gradio host is installed (P3). The pages that call /api/gradio
 * need it; the menu shows them only when it is, and otherwise links to the
 * launcher's Extensions page, where it is installed.
 */
export type HostStatus = {
  installed: boolean;
  installable: boolean;
  detail?: string;
  launcherPort?: number;
};

/** What the menu knows: nothing yet, the answer, or that asking failed. */
export type HostState =
  | { kind: "loading" }
  | { kind: "known"; status: HostStatus }
  | { kind: "error"; message: string };

/** The paths of the pages that call the Gradio host. */
const gradioPrefixes = [
  "/pipeline",
  "/text-to-speech",
  "/audio-music-generation",
  "/audio-conversion",
  "/extensions/",
  "/tools/api_config",
];

export const isGradioPath = (path: string): boolean =>
  gradioPrefixes.some((p) => path === p || path.startsWith(p.endsWith("/") ? p : p + "/"));

/** The launcher's page, on the host the browser used for this one. */
export const launcherUrl = (status?: HostStatus): string =>
  status?.launcherPort
    ? `${window.location.protocol}//${window.location.hostname}:${status.launcherPort}/`
    : "";

/**
 * The menu and the command palette both ask; one request serves them for a
 * page load. A reload asks again, which is how an install shows up.
 */
let pending: Promise<HostStatus> | null = null;
const askOnce = () =>
  (pending ??= goGetJson<HostStatus>("/api/go-data/host-status"));

export const useHostStatus = (): HostState => {
  const [state, setState] = React.useState<HostState>({ kind: "loading" });
  React.useEffect(() => {
    let live = true;
    askOnce()
      .then((status) => live && setState({ kind: "known", status }))
      .catch(
        (e) =>
          live &&
          setState({ kind: "error", message: e instanceof Error ? e.message : String(e) })
      );
    return () => {
      live = false;
    };
  }, []);
  return state;
};

/** False only when the host is known not to be installed (loading and errors show the pages). */
export const useGradioAvailable = (): boolean => {
  const host = useHostStatus();
  return !(host.kind === "known" && !host.status.installed);
};
