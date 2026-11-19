// client reads the per-run token from <meta name="tts-webui-token"> and
// attaches it as X-TTS-Token so API calls work even when the browser does
// not send the SameSite cookie (e.g. Chrome on bare IP addresses).
export function client(path: string, init?: RequestInit): Promise<Response> {
  const token =
    document
      .querySelector<HTMLMetaElement>('meta[name="tts-webui-token"]')
      ?.getAttribute("content") ?? "";

  const headers = new Headers(init?.headers);
  if (token) {
    headers.set("X-TTS-Token", token);
  }

  return fetch(path, { ...init, headers });
}
