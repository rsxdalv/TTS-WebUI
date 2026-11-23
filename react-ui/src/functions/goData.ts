/**
 * Calls to the Go core routes under /api/go-data (core/react_ui_backend,
 * core_routes.go). The Go side adds the core token; the page sends none.
 * Changes are sent as application/json, which the routes require.
 */

const readError = async (response: Response): Promise<string> => {
  const text = await response.text();
  try {
    const parsed = JSON.parse(text);
    if (parsed && typeof parsed.error === "string") return parsed.error;
  } catch {
    // not JSON
  }
  return text || `${response.status} ${response.statusText}`;
};

export const goGetJson = async <T>(path: string): Promise<T> => {
  const response = await fetch(path);
  if (!response.ok) throw new Error(await readError(response));
  return response.json();
};

export const goGetText = async (path: string): Promise<string> => {
  const response = await fetch(path);
  if (!response.ok) throw new Error(await readError(response));
  return response.text();
};

export const goSendJson = async <T>(
  method: "POST" | "PUT",
  path: string,
  body: unknown
): Promise<T> => {
  const response = await fetch(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) throw new Error(await readError(response));
  return response.json();
};

export const errorText = (error: unknown): string =>
  error instanceof Error ? error.message : String(error);
