import React from "react";
import { Link } from "react-router-dom";
import { Template } from "../components/Template";
import {
  errorText,
  goDelete,
  goGetJson,
  goSendJson,
} from "../functions/goData";

/** An endpoint as the Go core lists it: the key is never in it. */
type Endpoint = {
  name: string;
  url: string;
  model?: string;
  voice?: string;
  has_key: boolean;
};

type Clip = {
  folder: string;
  wav: string;
  png: string;
  seconds: number;
};

const EP = "/api/go-data/openai/endpoints";

const field = "border rounded px-2 py-1 w-full";
const button =
  "border rounded px-3 py-1 bg-gray-100 hover:bg-gray-200 disabled:opacity-50";
const primary =
  "rounded px-3 py-1 bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50";

/**
 * The openai platform: named endpoints that speak the OpenAI speech API, and a
 * page to generate with one. The key is typed here once, sent to Go and never
 * shown again; the clip is saved in outputs/ by Go and appears in History.
 */
const OpenAIPage = () => {
  const [endpoints, setEndpoints] = React.useState<Endpoint[] | null>(null);
  const [error, setError] = React.useState("");
  const load = React.useCallback(
    () =>
      goGetJson<Endpoint[]>(EP)
        .then((list) => {
          setEndpoints(list);
          setError("");
        })
        .catch((e) => setError(errorText(e))),
    []
  );
  React.useEffect(() => {
    load();
  }, [load]);

  return (
    <Template title="OpenAI-style endpoints">
      <div className="flex flex-col gap-6 p-4 w-full max-w-3xl">
        <h1 className="text-2xl font-bold">OpenAI-style endpoints</h1>
        <p className="text-sm text-gray-600">
          An endpoint is a server that answers{" "}
          <code>POST &lt;URL&gt;/audio/speech</code> like OpenAI does: OpenAI
          itself, or a local server such as the <code>openai_tts_api</code>{" "}
          extension. Nothing is installed. The key is kept by the TTS WebUI
          server and is never sent back to this page.
        </p>
        {error && <div className="text-red-600">{error}</div>}
        {endpoints && endpoints.length > 0 && (
          <Generate endpoints={endpoints} />
        )}
        {endpoints && <Endpoints endpoints={endpoints} reload={load} />}
      </div>
    </Template>
  );
};

const Generate = ({ endpoints }: { endpoints: Endpoint[] }) => {
  const [name, setName] = React.useState(endpoints[0].name);
  const [text, setText] = React.useState("");
  const [model, setModel] = React.useState("");
  const [voice, setVoice] = React.useState("");
  const [speed, setSpeed] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");
  const [clip, setClip] = React.useState<Clip | null>(null);
  const ep = endpoints.find((e) => e.name === name) ?? endpoints[0];

  const generate = async () => {
    setBusy(true);
    setError("");
    setClip(null);
    try {
      setClip(
        await goSendJson<Clip>("POST", "/api/go-data/openai/speech", {
          endpoint: ep.name,
          input: text,
          model,
          voice,
          speed: speed === "" ? 0 : Number(speed),
        })
      );
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="flex flex-col gap-3 border rounded p-4">
      <h2 className="text-lg font-semibold">Generate</h2>
      <label className="flex flex-col gap-1">
        Endpoint
        <select
          className={field}
          value={ep.name}
          onChange={(e) => setName(e.target.value)}
        >
          {endpoints.map((e) => (
            <option key={e.name} value={e.name}>
              {e.name} ({e.url})
            </option>
          ))}
        </select>
      </label>
      <label className="flex flex-col gap-1">
        Text
        <textarea
          className={field}
          rows={4}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </label>
      <div className="grid grid-cols-3 gap-2">
        <label className="flex flex-col gap-1">
          Model
          <input
            className={field}
            value={model}
            placeholder={ep.model || "tts-1"}
            onChange={(e) => setModel(e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1">
          Voice
          <input
            className={field}
            value={voice}
            placeholder={ep.voice || "alloy"}
            onChange={(e) => setVoice(e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1">
          Speed
          <input
            className={field}
            type="number"
            min="0.25"
            max="4"
            step="0.05"
            value={speed}
            placeholder="1"
            onChange={(e) => setSpeed(e.target.value)}
          />
        </label>
      </div>
      <div>
        <button
          className={primary}
          disabled={busy || text.trim() === ""}
          onClick={generate}
        >
          {busy ? "Generating..." : "Generate"}
        </button>
      </div>
      {error && <div className="text-red-600 break-words">{error}</div>}
      {clip && (
        <div className="flex flex-col gap-2">
          <audio controls src={`/${clip.wav}`} className="w-full" />
          <img src={`/${clip.png}`} alt="waveform" className="w-full" />
          <div className="text-sm text-gray-600">
            {clip.seconds.toFixed(1)} s, saved as <code>{clip.folder}</code>.{" "}
            <Link className="underline" to="/outputs/outputs">
              Open History
            </Link>
          </div>
        </div>
      )}
    </section>
  );
};

const emptyForm = { name: "", url: "", model: "", voice: "", key: "" };

const Endpoints = ({
  endpoints,
  reload,
}: {
  endpoints: Endpoint[];
  reload: () => Promise<void>;
}) => {
  const [form, setForm] = React.useState(emptyForm);
  const [editing, setEditing] = React.useState(false);
  const [message, setMessage] = React.useState<{ ok: boolean; text: string }>();
  const [busy, setBusy] = React.useState("");
  const set =
    (k: keyof typeof emptyForm) => (e: React.ChangeEvent<HTMLInputElement>) =>
      setForm({ ...form, [k]: e.target.value });
  const hasKey = endpoints.find((e) => e.name === form.name)?.has_key;

  const run = async (what: string, fn: () => Promise<void>) => {
    setBusy(what);
    setMessage(undefined);
    try {
      await fn();
    } catch (e) {
      setMessage({ ok: false, text: errorText(e) });
    } finally {
      setBusy("");
    }
  };

  const save = () =>
    run("save", async () => {
      const body: Record<string, string> = {
        url: form.url,
        model: form.model,
        voice: form.voice,
      };
      // No key typed keeps the stored one.
      if (form.key !== "") body.key = form.key;
      await goSendJson("PUT", `${EP}/${encodeURIComponent(form.name)}`, body);
      setForm(emptyForm);
      setEditing(false);
      setMessage({ ok: true, text: `Saved ${form.name}.` });
      await reload();
    });

  const clearKey = () =>
    run("key", async () => {
      await goSendJson("PUT", `${EP}/${encodeURIComponent(form.name)}`, {
        url: form.url,
        model: form.model,
        voice: form.voice,
        key: "",
      });
      setMessage({ ok: true, text: `The key of ${form.name} was removed.` });
      await reload();
    });

  const test = (name: string) =>
    run("test " + name, async () => {
      const r = await goSendJson<{ ok: boolean; message: string }>(
        "POST",
        `${EP}/${encodeURIComponent(name)}/test`,
        {}
      );
      setMessage({ ok: r.ok, text: `${name}: ${r.message}` });
    });

  const remove = (name: string) =>
    run("delete " + name, async () => {
      await goDelete(`${EP}/${encodeURIComponent(name)}`);
      setMessage({ ok: true, text: `Deleted ${name}.` });
      await reload();
    });

  return (
    <section className="flex flex-col gap-3 border rounded p-4">
      <h2 className="text-lg font-semibold">Endpoints</h2>
      {endpoints.length === 0 && (
        <p>No endpoint yet. Add one below to generate with it.</p>
      )}
      <ul className="flex flex-col gap-2">
        {endpoints.map((e) => (
          <li
            key={e.name}
            className="flex items-center justify-between gap-2 border rounded p-2"
          >
            <div className="min-w-0">
              <div className="font-semibold">{e.name}</div>
              <div className="text-sm text-gray-600 break-all">
                {e.url} {e.model && `· ${e.model}`} {e.voice && `· ${e.voice}`}{" "}
                · {e.has_key ? "key stored" : "no key"}
              </div>
            </div>
            <div className="flex gap-2 shrink-0">
              <button
                className={button}
                disabled={busy !== ""}
                onClick={() => test(e.name)}
              >
                {busy === "test " + e.name ? "Testing..." : "Test"}
              </button>
              <button
                className={button}
                onClick={() => {
                  setForm({
                    name: e.name,
                    url: e.url,
                    model: e.model ?? "",
                    voice: e.voice ?? "",
                    key: "",
                  });
                  setEditing(true);
                }}
              >
                Edit
              </button>
              <button
                className={button}
                disabled={busy !== ""}
                onClick={() => remove(e.name)}
              >
                Delete
              </button>
            </div>
          </li>
        ))}
      </ul>
      {message && (
        <div
          className={`${
            message.ok ? "text-green-700" : "text-red-600"
          } break-words`}
        >
          {message.text}
        </div>
      )}
      <h3 className="font-semibold">{editing ? `Edit ${form.name}` : "Add"}</h3>
      <div className="grid grid-cols-2 gap-2">
        <label className="flex flex-col gap-1">
          Name
          <input
            className={field}
            value={form.name}
            disabled={editing}
            placeholder="openai"
            onChange={set("name")}
          />
        </label>
        <label className="flex flex-col gap-1">
          URL
          <input
            className={field}
            value={form.url}
            placeholder="https://api.openai.com/v1"
            onChange={set("url")}
          />
        </label>
        <label className="flex flex-col gap-1">
          Model
          <input
            className={field}
            value={form.model}
            placeholder="tts-1"
            onChange={set("model")}
          />
        </label>
        <label className="flex flex-col gap-1">
          Voice
          <input
            className={field}
            value={form.voice}
            placeholder="alloy"
            onChange={set("voice")}
          />
        </label>
        <label className="flex flex-col gap-1 col-span-2">
          Key
          <input
            className={field}
            type="password"
            autoComplete="off"
            value={form.key}
            placeholder={hasKey ? "A key is stored: leave empty to keep it" : ""}
            onChange={set("key")}
          />
        </label>
      </div>
      <div className="flex gap-2">
        <button
          className={primary}
          disabled={busy !== "" || form.name === "" || form.url === ""}
          onClick={save}
        >
          Save
        </button>
        {editing && hasKey && (
          <button className={button} disabled={busy !== ""} onClick={clearKey}>
            Remove the key
          </button>
        )}
        {editing && (
          <button
            className={button}
            onClick={() => {
              setForm(emptyForm);
              setEditing(false);
            }}
          >
            Cancel
          </button>
        )}
      </div>
    </section>
  );
};

export default OpenAIPage;
