// client is a thin fetch wrapper that reads the per-run token from the
// <meta name="tts-webui-token"> element and attaches it as X-TTS-Token.
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
