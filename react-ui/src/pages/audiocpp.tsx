import React from "react";
import { Link } from "react-router-dom";
import { Template } from "../components/Template";
import { errorText, goGetJson, goSendJson } from "../functions/goData";
import { launcherUrl, type HostStatus } from "../functions/hostStatus";

type Model = {
  id: string;
  package: string;
  name: string;
  installed: boolean;
  voices: string[];
  default_voice: string;
  size_mb: number;
  license: string;
};

type Status = {
  supported: boolean;
  ready: boolean;
  detail: string;
  version: string;
  row?: string;
  models: Model[];
  server: { running: boolean; busy: boolean; models: string[] };
};

type Clip = {
  folder: string;
  wav: string;
  png: string;
  seconds: number;
  model: string;
  voice: string;
};

const STATUS = "/api/go-data/audiocpp/status";

const field = "border rounded px-2 py-1 w-full";
const button =
  "border rounded px-3 py-1 bg-gray-100 hover:bg-gray-200 disabled:opacity-50";
const primary =
  "rounded px-3 py-1 bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50";

/**
 * The audiocpp platform (P7): the audio.cpp server and its models, installed
 * from the launcher's Extensions page. This page generates with an installed
 * model and can cancel the generation; the clip is saved in outputs/ by Go and
 * appears in History.
 */
const AudioCppPage = () => {
  const [status, setStatus] = React.useState<Status | null>(null);
  const [error, setError] = React.useState("");
  const [launcher, setLauncher] = React.useState("");
  const load = React.useCallback(
    () =>
      goGetJson<Status>(STATUS)
        .then((s) => {
          setStatus(s);
          setError("");
        })
        .catch((e) => setError(errorText(e))),
    []
  );
  React.useEffect(() => {
    load();
    goGetJson<HostStatus>("/api/go-data/host-status")
      .then((h) => setLauncher(launcherUrl(h)))
      .catch(() => undefined);
  }, [load]);

  const installed = status?.models.filter((m) => m.installed) ?? [];

  return (
    <Template title="audio.cpp">
      <div className="flex flex-col gap-6 p-4 w-full max-w-3xl">
        <h1 className="text-2xl font-bold">audio.cpp</h1>
        <p className="text-sm text-gray-600">
          audio.cpp runs speech models from GGUF files with no Python. The
          server and the models are installed from the launcher&apos;s
          Extensions page, which checks every download against its sha256.
        </p>
        {error && <div className="text-red-600">{error}</div>}
        {status && !status.supported && (
          <div className="text-amber-700">{status.detail}</div>
        )}
        {status && status.supported && !status.ready && (
          <div className="border rounded p-3">
            The audio.cpp platform is not installed ({status.detail}).{" "}
            {launcher ? (
              <a className="underline" href={launcher}>
                Install it on the Extensions page
              </a>
            ) : (
              "Install it on the launcher's Extensions page."
            )}
          </div>
        )}
        {status && status.ready && installed.length === 0 && (
          <div className="border rounded p-3">
            No audio.cpp model is installed.{" "}
            {launcher && (
              <a className="underline" href={launcher}>
                Install one on the Extensions page
              </a>
            )}
          </div>
        )}
        {status && installed.length > 0 && (
          <Generate models={installed} status={status} reload={load} />
        )}
        {status && (
          <Details status={status} />
        )}
      </div>
    </Template>
  );
};

const Generate = ({
  models,
  status,
  reload,
}: {
  models: Model[];
  status: Status;
  reload: () => void;
}) => {
  const [id, setId] = React.useState(models[0].id);
  const model = models.find((m) => m.id === id) ?? models[0];
  const [voice, setVoice] = React.useState("");
  const [text, setText] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [cancelling, setCancelling] = React.useState(false);
  const [note, setNote] = React.useState("");
  const [error, setError] = React.useState("");
  const [clip, setClip] = React.useState<Clip | null>(null);

  const generate = async () => {
    setBusy(true);
    setError("");
    setNote(
      status.server.running
        ? "Generating..."
        : "Starting the audio.cpp server and loading the model, then generating..."
    );
    setClip(null);
    try {
      setClip(
        await goSendJson<Clip>("POST", "/api/go-data/audiocpp/speech", {
          model: model.id,
          input: text,
          voice: voice || model.default_voice,
        })
      );
      setNote("");
    } catch (e) {
      const msg = errorText(e);
      if (/cancelled/i.test(msg)) {
        setNote("Cancelled. Nothing was saved.");
      } else {
        setNote("");
        setError(msg);
      }
    } finally {
      setBusy(false);
      setCancelling(false);
      reload();
    }
  };

  const cancel = async () => {
    setCancelling(true);
    try {
      await goSendJson<{ cancelled: boolean; server_stopped: boolean }>(
        "POST",
        "/api/go-data/audiocpp/cancel",
        {}
      );
    } catch (e) {
      setError(errorText(e));
    }
  };

  return (
    <section className="flex flex-col gap-3 border rounded p-4">
      <h2 className="text-lg font-semibold">Generate</h2>
      <div className="grid grid-cols-2 gap-2">
        <label className="flex flex-col gap-1">
          Model
          <select
            className={field}
            value={model.id}
            onChange={(e) => setId(e.target.value)}
          >
            {models.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1">
          Voice
          <select
            className={field}
            value={voice || model.default_voice}
            onChange={(e) => setVoice(e.target.value)}
          >
            {model.voices.map((v) => (
              <option key={v} value={v}>
                {v}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label className="flex flex-col gap-1">
        Text
        <textarea
          className={field}
          rows={5}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </label>
      <div className="flex gap-2">
        <button
          className={primary}
          disabled={busy || text.trim() === ""}
          onClick={generate}
        >
          {busy ? "Generating..." : "Generate"}
        </button>
        <button className={button} disabled={!busy || cancelling} onClick={cancel}>
          {cancelling ? "Cancelling..." : "Cancel"}
        </button>
      </div>
      {note && <div className="text-sm text-gray-600">{note}</div>}
      {error && <div className="text-red-600 break-words">{error}</div>}
      {clip && (
        <div className="flex flex-col gap-2">
          <audio
            controls
            src={`/api/webui-generations/${clip.wav}`}
            className="w-full"
          />
          <img
            src={`/api/webui-generations/${clip.png}`}
            alt="waveform"
            className="w-full"
          />
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

const Details = ({ status }: { status: Status }) => (
  <section className="text-sm text-gray-600 flex flex-col gap-1">
    <div>
      Platform:{" "}
      {status.ready
        ? `audio.cpp ${status.version}, ${status.row} build`
        : "not installed"}
      . Server:{" "}
      {status.server.running
        ? `running (${status.server.models.join(", ")})${status.server.busy ? ", generating" : ""}`
        : "stopped (it starts with the first generation and stops with the app)"}
      .
    </div>
    <ul className="list-disc pl-5">
      {status.models.map((m) => (
        <li key={m.id}>
          {m.name}: {m.installed ? "installed" : "not installed"}, about{" "}
          {m.size_mb} MB, license {m.license}
        </li>
      ))}
    </ul>
  </section>
);

export default AudioCppPage;
