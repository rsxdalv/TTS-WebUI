import React from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./ui/card";
import {
  MINIMAX_SETTINGS,
  MiniMaxSettings,
  getMiniMaxSettings,
} from "../functions/generateWithMiniMaxTTS";
import { errorText, goSendJson } from "../functions/goData";

const field = "w-full p-2 border rounded bg-background text-foreground";
const button =
  "border rounded px-3 py-1 bg-gray-100 hover:bg-gray-200 disabled:opacity-50 text-black";

const keyStatus = (s: MiniMaxSettings) =>
  s.key_source === "stored"
    ? "A key is saved on this machine."
    : s.key_source === "env"
    ? "Using the MINIMAX_API_KEY environment variable."
    : "No key yet.";

/**
 * The MiniMax settings: the key goes to the TTS WebUI server and is never sent
 * back, so the field is empty whatever is stored. Saving and testing work from
 * the machine the app runs on only.
 */
export const MiniMaxKeyCard = () => {
  const [settings, setSettings] = React.useState<MiniMaxSettings | null>(null);
  const [key, setKey] = React.useState("");
  const [baseUrl, setBaseUrl] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [message, setMessage] = React.useState("");
  const [error, setError] = React.useState("");

  const load = React.useCallback(
    () =>
      getMiniMaxSettings()
        .then((s) => {
          setSettings(s);
          setBaseUrl(s.base_url);
        })
        .catch((e) => setError(errorText(e))),
    []
  );
  React.useEffect(() => {
    load();
  }, [load]);

  const save = async (newKey: string | undefined) => {
    if (!settings) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const body: Record<string, string> = {
        base_url: baseUrl,
        model: settings.model,
        voice: settings.voice,
      };
      if (newKey !== undefined) body.key = newKey;
      const s = await goSendJson<MiniMaxSettings>("PUT", MINIMAX_SETTINGS, body);
      setSettings(s);
      setBaseUrl(s.base_url);
      setKey("");
      setMessage(newKey === "" ? "Key removed." : "Saved.");
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  const test = async () => {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const r = await goSendJson<{ ok: boolean; message: string }>(
        "POST",
        "/api/go-data/minimax/test",
        {}
      );
      if (r.ok) setMessage(r.message);
      else setError(r.message);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle>API Configuration</CardTitle>
        <CardDescription>
          The key is kept by the TTS WebUI server and is never sent back to this
          page. It can also come from the MINIMAX_API_KEY environment variable.
          Saving and testing work from the machine TTS WebUI runs on only.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="text-sm">
          {settings ? keyStatus(settings) : "Loading…"}
        </div>
        <input
          type="password"
          autoComplete="off"
          name="minimax_key"
          placeholder="Enter MiniMax API key..."
          value={key}
          onChange={(e) => setKey(e.target.value)}
          className={field}
        />
        <label className="flex flex-col gap-1 text-sm">
          API host
          <input
            className={field}
            value={baseUrl}
            onChange={(e) => setBaseUrl(e.target.value)}
          />
        </label>
        <div className="flex gap-2 flex-wrap">
          <button
            className={button}
            disabled={busy || !settings || (!key && baseUrl === settings.base_url)}
            onClick={() => save(key ? key : undefined)}
          >
            Save
          </button>
          <button
            className={button}
            disabled={busy || !settings || settings.key_source !== "stored"}
            onClick={() => save("")}
          >
            Remove key
          </button>
          <button
            className={button}
            disabled={busy || !settings || !settings.has_key}
            onClick={test}
            title="Makes a one-word speech request (uses a little of the quota)"
          >
            Test
          </button>
        </div>
        {message && <div className="text-sm text-green-600">{message}</div>}
        {error && <div className="text-sm text-red-600">{error}</div>}
      </CardContent>
    </Card>
  );
};
