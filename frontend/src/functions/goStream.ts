/**
 * POST JSON to a Go route that answers with NDJSON ({"line": "..."} per output
 * line, then {"done": true, "ok": bool}), as the install and uninstall routes
 * do. Each line goes to onLine; the result is the `ok` of the last message.
 */
export const goPostStream = async (
  path: string,
  body: unknown,
  onLine: (line: string) => void
): Promise<boolean> => {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok || !response.body) {
    const text = await response.text().catch(() => "");
    let message = text || `${response.status} ${response.statusText}`;
    try {
      const parsed = JSON.parse(text);
      if (parsed && typeof parsed.error === "string") message = parsed.error;
    } catch {
      // not JSON
    }
    throw new Error(message);
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let ok = false;
  const handle = (text: string) => {
    if (!text.trim()) return;
    const msg = JSON.parse(text);
    if (typeof msg.line === "string") onLine(msg.line.replace(/<br\s*\/?>/gi, "\n"));
    if (msg.done) ok = !!msg.ok;
  };
  for (;;) {
    const { value, done } = await reader.read();
    if (value) buffer += decoder.decode(value, { stream: !done });
    let nl = buffer.indexOf("\n");
    while (nl >= 0) {
      handle(buffer.slice(0, nl));
      buffer = buffer.slice(nl + 1);
      nl = buffer.indexOf("\n");
    }
    if (done) {
      handle(buffer);
      return ok;
    }
  }
};
