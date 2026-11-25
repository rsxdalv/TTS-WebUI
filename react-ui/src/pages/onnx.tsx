import React from "react";
import { Link } from "react-router-dom";
import { Template } from "../components/Template";
import { errorText, goGetJson } from "../functions/goData";

/** An engine of the onnx platform as the Go core lists it. */
type Engine = {
  package: string;
  name: string;
  installed: boolean;
  voices: number;
  language: string;
  license: string;
  platform_ready: boolean;
  detail?: string;
};

type Clip = {
  folder: string;
  wav: string;
  png: string;
  seconds: number;
};

const field = "border rounded px-2 py-1 w-full";
const button =
  "border rounded px-3 py-1 bg-gray-100 hover:bg-gray-200 disabled:opacity-50";
const primary =
  "rounded px-3 py-1 bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50";

/**
 * The onnx platform: engines run by a native ONNX runtime that TTS WebUI
 * downloads on demand (no Python). Installing one is done on the launcher's
 * Extensions page (port 7700); generating is here. The clip is saved by Go in
 * outputs/ and appears in History. Cancel stops the program.
 */
const OnnxPage = () => {
  const [engines, setEngines] = React.useState<Engine[] | null>(null);
  const [error, setError] = React.useState("");
  React.useEffect(() => {
    goGetJson<Engine[]>("/api/go-data/onnx/engines")
      .then(setEngines)
      .catch((e) => setError(errorText(e)));
  }, []);
  const ready = (engines ?? []).filter((e) => e.installed);

  return (
    <Template title="ONNX engines">
      <div className="flex flex-col gap-6 p-4 w-full max-w-3xl">
        <h1 className="text-2xl font-bold">ONNX engines (no Python)</h1>
        <p className="text-sm text-gray-600">
          These engines run on a native ONNX runtime that TTS WebUI downloads
          once (checked against a pinned checksum). Install one on the
          launcher&apos;s Extensions page (port 7700); then generate here. CPU
          only.
        </p>
        {error && <div className="text-red-600">{error}</div>}
        {engines && engines.length === 0 && <p>No ONNX engine is known.</p>}
        {engines && ready.length === 0 && engines.length > 0 && (
          <p>
            No ONNX engine is installed
            {engines[0].detail ? ` (${engines[0].detail})` : ""}. Open the
            launcher&apos;s Extensions page (port 7700) and install{" "}
            <b>{engines[0].name}</b>.
          </p>
        )}
        {ready.length > 0 && <Generate engines={ready} />}
      </div>
    </Template>
  );
};

const Generate = ({ engines }: { engines: Engine[] }) => {
  const [pkg, setPkg] = React.useState(engines[0].package);
  const [text, setText] = React.useState("");
  const [voice, setVoice] = React.useState(0);
  const [speed, setSpeed] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");
  const [clip, setClip] = React.useState<Clip | null>(null);
  const abort = React.useRef<AbortController | null>(null);
  const engine = engines.find((e) => e.package === pkg) ?? engines[0];

  const generate = async () => {
    const ctl = new AbortController();
    abort.current = ctl;
    setBusy(true);
    setError("");
    setClip(null);
    try {
      const response = await fetch("/api/go-data/onnx/speech", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        signal: ctl.signal,
        body: JSON.stringify({
          package: engine.package,
          input: text,
          voice,
          speed: speed === "" ? 0 : Number(speed),
        }),
      });
      const body = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(body.error ?? `${response.status}`);
      setClip(body as Clip);
    } catch (e) {
      if (ctl.signal.aborted) setError("Cancelled: nothing was saved.");
      else setError(errorText(e));
    } finally {
      abort.current = null;
      setBusy(false);
    }
  };

  return (
    <section className="flex flex-col gap-3 border rounded p-4">
      <h2 className="text-lg font-semibold">Generate</h2>
      <label className="flex flex-col gap-1">
        Engine
        <select
          className={field}
          value={engine.package}
          onChange={(e) => setPkg(e.target.value)}
        >
          {engines.map((e) => (
            <option key={e.package} value={e.package}>
              {e.name} ({e.license})
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
      <div className="grid grid-cols-2 gap-2">
        <label className="flex flex-col gap-1">
          Voice
          <select
            className={field}
            value={voice}
            onChange={(e) => setVoice(Number(e.target.value))}
          >
            {Array.from({ length: engine.voices }, (_, i) => (
              <option key={i} value={i}>
                Voice {i}
              </option>
            ))}
          </select>
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
      <div className="flex gap-2">
        <button
          className={primary}
          disabled={busy || text.trim() === ""}
          onClick={generate}
        >
          {busy ? "Generating..." : "Generate"}
        </button>
        {busy && (
          <button className={button} onClick={() => abort.current?.abort()}>
            Cancel
          </button>
        )}
      </div>
      {error && <div className="text-red-600 break-words">{error}</div>}
      {clip && (
        <div className="flex flex-col gap-2">
          <audio controls src={`/api/webui-generations/${clip.wav}`} className="w-full" />
          <img src={`/api/webui-generations/${clip.png}`} alt="waveform" className="w-full" />
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

export default OnnxPage;
