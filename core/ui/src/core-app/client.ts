// client reads the per-run token from <meta name="tts-webui-token"> and
// attaches it as X-TTS-Token so API calls work even when the browser does
// not send the SameSite cookie (e.g. Chrome on bare IP addresses).
export async function client(path: string, init?: RequestInit): Promise<Response> {
  const token =
    document
      .querySelector<HTMLMetaElement>('meta[name="tts-webui-token"]')
      ?.getAttribute("content") ?? "";

  const headers = new Headers(init?.headers);
  if (token) {
    headers.set("X-TTS-Token", token);
  }

  const res = await fetch(path, { ...init, headers });
  // Each launcher run has its own token. A page that was loaded before a
  // restart is refused; say that instead of letting callers fail on the text.
  if (res.status === 403) {
    const text = await res.clone().text();
    if (text.startsWith("forbidden: missing or invalid token")) {
      throw new Error(
        "The launcher was restarted after this page loaded. Reload the page."
      );
    }
  }
  return res;
}
