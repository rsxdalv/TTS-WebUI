import type { AudioValue } from "./types";

type GradioFileLike = { url?: string; name?: string; orig_name?: string };

const readError = async (res: Response) => {
  const text = await res.text();
  try {
    const j = JSON.parse(text);
    const msg = j?.error ?? j?.data?.error;
    if (typeof msg === "string") return msg;
  } catch {
    // not JSON
  }
  const plain = text.trim();
  return plain && plain !== "{}" && plain !== "null"
    ? plain.slice(0, 300)
    : `${res.status} ${res.statusText || "request failed"}`;
};

/**
 * POST to a Go route that fronts the Gradio host. The Go side keeps the call
 * running if the browser leaves, so Stop only stops waiting for it.
 */
/** A call that gets no answer for this long is given up. */
export const REQUEST_TIMEOUT_MS = 15 * 60 * 1000;

export const postJson = async <T>(
  path: string,
  body: unknown,
  signal: AbortSignal,
  timeoutMs = REQUEST_TIMEOUT_MS
): Promise<T> => {
  // The Go route can wait forever when the Gradio tab rejects a parameter, so
  // stop waiting after a while, and when the user presses Stop.
  const ctl = new AbortController();
  let timedOut = false;
  const onAbort = () => ctl.abort();
  if (signal.aborted) ctl.abort();
  else signal.addEventListener("abort", onAbort, { once: true });
  const timer = setTimeout(() => {
    timedOut = true;
    ctl.abort();
  }, timeoutMs);
  try {
    const res = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal: ctl.signal,
    });
    if (!res.ok) throw new Error(await readError(res));
    return await res.json();
  } catch (e) {
    if (timedOut) {
      throw new Error(
        `No answer after ${Math.round(timeoutMs / 60000) || "<1"} min. The server may have rejected a setting.`
      );
    }
    throw e;
  } finally {
    clearTimeout(timer);
    signal.removeEventListener("abort", onAbort);
  }
};

export const gradio = <T>(name: string, body: unknown, signal: AbortSignal) =>
  postJson<T>(`/api/gradio/${name}`, body, signal);

export const asAudio = (f: GradioFileLike | undefined, name: string): AudioValue => {
  if (!f?.url) throw new Error("The server returned no audio");
  return { url: f.url, name: f.orig_name ?? f.name ?? name };
};
