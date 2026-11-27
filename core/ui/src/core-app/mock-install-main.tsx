// A page that plays an install with made-up data, to look at the install
// dialog without installing anything. Served by `vite` (see mock-install.html);
// not part of the launcher build (the build only takes core.html).
import { StrictMode, useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import "../index.css";
import { DarkThemeProvider, ThemeToggle } from "../DarkThemeProvider";
import { InstallProgress } from "./InstallDialog";
import { applyStepMessage, type StepInfo, type StepMessage } from "./install-steps";

type Scenario = "fresh" | "cached" | "failure";

// The clock the page really has. While a scenario plays, Date.now runs faster
// (the dialog's timers show virtual time); stop() puts it back.
const realNow = Date.now.bind(Date);

const GB = 1 << 30;
const MB = 1 << 20;

interface Ev {
  at: number; // virtual seconds from the start
  msg?: StepMessage;
  line?: string;
}

const plan = (parallel: boolean): StepMessage => ({
  plan: [
    { id: "uv", label: "Download uv 0.13.6" },
    { id: "python", label: "Install Python 3.10.11" },
    { id: "git", label: "Download git 2.56.1" },
    { id: "ffmpeg", label: "Download ffmpeg 8.1.3" },
    { id: "venv", label: "Create the Gradio host environment" },
    { id: "torch", label: "Install torch 2.11.0 (cu126)" },
    ...(parallel ? [{ id: "wheels", label: "Fetch the requirements' wheels (alongside torch)" }] : []),
    { id: "requirements", label: "Install the 149 requirements packages" },
  ],
});

function download(id: string, name: string, size: number, start: number, secs: number): Ev[] {
  const out: Ev[] = [
    { at: start, msg: { step: { id, state: "running", done: 0, total: size, detail: `${name}: downloading` } } },
    { at: start, line: `Downloading ${name}` },
  ];
  for (let i = 1; i <= 20; i++) {
    out.push({
      at: start + (secs * i) / 20,
      msg: { step: { id, state: "running", done: Math.round((size * i) / 20), total: size } },
      line: i % 4 === 0 ? `${name}: ${i * 5}% of ${Math.round(size / MB)} MB` : undefined,
    });
  }
  out.push({ at: start + secs, line: "Checksum matches" });
  return out;
}

function build(scenario: Scenario, parallel: boolean): { events: Ev[]; end: number } {
  const ev: Ev[] = [{ at: 0, msg: plan(parallel) }];
  const done = (id: string, at: number, detail?: string): Ev => ({ at, msg: { step: { id, state: "done", detail } } });
  const skipped = (id: string, at: number): Ev => ({ at, msg: { step: { id, state: "skipped" } } });
  const cached = scenario === "cached";

  let t = 0;
  // uv
  if (cached) ev.push(skipped("uv", 0.2));
  else {
    ev.push(...download("uv", "uv", 14 * MB, 0, 5));
    ev.push(done("uv", 5.2));
    t = 5.2;
  }
  // python, git, ffmpeg
  const pyT = cached ? 0 : 11.6;
  const gitT = cached ? 0 : 10;
  const ffT = cached ? 0 : 25;
  const start = (id: string, at: number) => ({ at, msg: { step: { id, state: "running" as const } } });
  if (cached) {
    ev.push(skipped("python", 0.3), skipped("git", 0.3), skipped("ffmpeg", 0.3));
    t = 0.4;
  } else if (parallel) {
    ev.push(start("python", t), { at: t, line: "Installing Python 3.10.11 (uv downloads it)" });
    ev.push(done("python", t + pyT, "Python 3.10.11, 34.6 MB"));
    ev.push(...download("git", "git", 38 * MB, t, gitT), done("git", t + gitT + 0.5));
    ev.push(...download("ffmpeg", "ffmpeg", 85 * MB, t, ffT), done("ffmpeg", t + ffT + 1));
    t += ffT + 1;
  } else {
    ev.push(start("python", t), { at: t, line: "Installing Python 3.10.11 (uv downloads it)" });
    t += pyT;
    ev.push(done("python", t, "Python 3.10.11, 34.6 MB"));
    ev.push(...download("git", "git", 38 * MB, t, gitT), done("git", t + gitT + 0.5));
    t += gitT + 0.5;
    ev.push(...download("ffmpeg", "ffmpeg", 85 * MB, t, ffT), done("ffmpeg", t + ffT + 1));
    t += ffT + 1;
  }
  // venv
  ev.push(start("venv", t), { at: t, line: "Creating virtual environment at: .venvs\\gradio-host" });
  t += 1;
  ev.push(done("venv", t));
  // torch
  const torchSecs = cached ? 14 : 150; // the real one is about 10 minutes
  const unpacked = 4.3 * GB;
  ev.push(start("torch", t));
  ev.push({ at: t, line: "Downloading torch (2.4GiB)" });
  const wheelsStart = t;
  const failAt = scenario === "failure" ? t + torchSecs * 0.55 : Infinity;
  for (let i = 1; i <= torchSecs; i++) {
    const at = t + i;
    if (at > failAt) break;
    const written = Math.round((unpacked * i) / torchSecs);
    ev.push({
      at,
      msg: {
        step: {
          id: "torch",
          state: "running",
          done: written,
          total: Math.round(unpacked),
          estimated: true,
          detail: cached
            ? "unpacking from the cache (nothing to download)"
            : i < torchSecs * 0.8
              ? "downloading and unpacking: uv shows no progress, this is measured from the cache folder"
              : "unpacking the last files",
        },
      },
    });
  }
  if (scenario === "failure") {
    ev.push({
      at: failAt,
      msg: {
        step: {
          id: "torch",
          state: "failed",
          detail: "uv pip install: error sending request for url (https://download.pytorch.org/whl/cu126/torch…): connection reset",
        },
      },
    });
    ev.push({ at: failAt, line: "Failed to run uv pip install: exit status 2" });
    ev.push({ at: failAt + 0.1, line: "Install again to continue; downloads are kept." });
    return { events: ev, end: failAt + 0.2 };
  }
  t += torchSecs;
  ev.push(done("torch", t, cached ? "from the cache" : "2.4 GB downloaded, 4.1 GB unpacked"));
  if (parallel) {
    const w = 60;
    ev.push(start("wheels", wheelsStart));
    for (let i = 1; i <= 20; i++) {
      ev.push({
        at: wheelsStart + (w * i) / 20,
        msg: { step: { id: "wheels", state: "running", done: Math.round((300 * MB * i) / 20), total: 300 * MB, estimated: true } },
      });
    }
    ev.push(done("wheels", wheelsStart + w));
  }
  // requirements
  const reqSecs = parallel || cached ? 25 : 140;
  ev.push(start("requirements", t));
  ev.push({ at: t, line: "Resolved 162 packages in 28.54s" });
  ev.push({
    at: t + 1,
    msg: { step: { id: "requirements", state: "running", detail: parallel ? "installing from the cache" : "resolving, downloading and building 149 packages" } },
  });
  t += reqSecs;
  ev.push(done("requirements", t, "149 packages"));
  ev.push({ at: t, line: "Successfully set up the Gradio host" });
  return { events: ev, end: t + 0.1 };
}

function MockApp() {
  const [scenario, setScenario] = useState<Scenario>("fresh");
  const [parallel, setParallel] = useState(false);
  const [speed, setSpeed] = useState(10);
  const [steps, setSteps] = useState<StepInfo[]>([]);
  const [log, setLog] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState(false);
  const timer = useRef<ReturnType<typeof setInterval> | null>(null);

  const stop = useCallback(() => {
    if (timer.current) clearInterval(timer.current);
    timer.current = null;
    Date.now = realNow;
    setBusy(false);
  }, []);

  const play = useCallback(() => {
    stop();
    const { events, end } = build(scenario, parallel);
    events.sort((a, b) => a.at - b.at);
    let next = 0;
    // Virtual time runs `speed` times faster than the clock, and the step
    // timers in the dialog show virtual time too (Date.now is shifted).
    const realStart = realNow();
    const virtual = () => realStart + (realNow() - realStart) * speed;
    Date.now = virtual;
    setSteps([]);
    setLog([]);
    setOpen(true);
    setBusy(true);
    timer.current = setInterval(() => {
      const t = (realNow() - realStart) / 1000 * speed;
      while (next < events.length && events[next].at <= t) {
        const e = events[next++];
        if (e.msg) setSteps((s) => applyStepMessage(s, e.msg!, virtual()));
        if (e.line) setLog((l) => [...l, e.line!]);
      }
      if (t >= end) stop();
    }, 100);
  }, [scenario, parallel, speed, stop]);

  useEffect(() => () => stop(), [stop]);

  const cancel = () => {
    stop();
    setSteps((s) => s.map((x) => (x.state === "running" ? { ...x, state: "pending", detail: "cancelled" } : x)));
    setLog((l) => [...l, "Cancelled. Install again to continue; downloads are kept."]);
  };

  const label = "flex items-center gap-2 text-sm";
  const field = "px-2 py-1 rounded border border-gray-300 dark:border-gray-700 bg-transparent";

  return (
    <DarkThemeProvider>
      <div className="min-h-screen bg-gray-50 dark:bg-gray-950 text-gray-900 dark:text-gray-100">
        <header className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-800">
          <h1 className="text-lg font-semibold">Install dialog: mock data</h1>
          <ThemeToggle />
        </header>
        <main className="max-w-2xl mx-auto px-4 py-8 flex flex-col gap-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Nothing here installs anything. The events are made up from the log of a real install of the Gradio host
            (torch took about 10 minutes there; speed 10 plays it in about a minute).
          </p>
          <div className="flex flex-wrap items-center gap-4 p-3 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
            <label className={label}>
              Scenario
              <select className={field} value={scenario} onChange={(e) => setScenario(e.target.value as Scenario)}>
                <option value="fresh">Fresh install</option>
                <option value="cached">Reinstall (uv and wheels cached)</option>
                <option value="failure">Network failure in torch</option>
              </select>
            </label>
            <label className={label}>
              <input type="checkbox" checked={parallel} onChange={(e) => setParallel(e.target.checked)} />
              Parallel downloads
            </label>
            <label className={label}>
              Speed
              <select className={field} value={speed} onChange={(e) => setSpeed(Number(e.target.value))}>
                <option value={1}>1x (real time)</option>
                <option value={5}>5x</option>
                <option value={10}>10x</option>
                <option value={30}>30x</option>
              </select>
            </label>
            <button
              className="px-3 py-1 text-sm rounded bg-sky-600 text-white hover:bg-sky-700"
              onClick={play}
            >
              Play
            </button>
          </div>
          {open && (
            <div className="rounded-xl border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 p-4 shadow-xl">
              <InstallProgress
                steps={steps}
                log={log}
                busy={busy}
                onCancel={cancel}
                onClose={() => {
                  setOpen(false);
                  setSteps([]);
                  setLog([]);
                }}
              />
            </div>
          )}
        </main>
      </div>
    </DarkThemeProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MockApp />
  </StrictMode>,
);
