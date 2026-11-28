import React from "react";
import { Template } from "./Template";
import { Button } from "./ui/button";
import { FullscreenFrame } from "./FullscreenFrame";
import { LauncherExtensionsLink } from "./LauncherExtensionsLink";
import { errorText, goGetJson } from "../functions/goData";

type Phase = "loading" | "down" | "starting" | "ready";

/**
 * A page that shows the Gradio UI (or one of its tabs) in a frame. The Gradio
 * app answers on the public port (server_port, 7770 by default), on the host
 * this page was opened from. `tab` is a path of tab names, for example
 * "Text-to-Speech/Kokoro"; the Gradio page opens that tab (see
 * tts_webui/gradio/blocks.py). The page is probed first, so a Gradio that is
 * not running or still starting shows a message and not an error page.
 */
export const GradioFrame = ({
  title,
  tab,
}: {
  title: string;
  tab?: string;
}) => {
  const [port, setPort] = React.useState<number | null>(null);
  const [phase, setPhase] = React.useState<Phase>("loading");
  const [error, setError] = React.useState("");
  const [attempt, setAttempt] = React.useState(0);

  React.useEffect(() => {
    let live = true;
    goGetJson<{ server_port: number | null }>("/api/go-data/server-settings")
      .then((s) => live && setPort(s.server_port ?? 7770))
      .catch(
        (e) =>
          live &&
          setError(
            `Could not reach the TTS WebUI server (${errorText(e)}). ` +
              "The app is probably not started."
          )
      );
    return () => {
      live = false;
    };
  }, [attempt]);

  const base = port
    ? `${window.location.protocol}//${window.location.hostname}:${port}`
    : "";

  React.useEffect(() => {
    if (!base) return;
    let live = true;
    let timer: ReturnType<typeof setTimeout>;
    const probe = async () => {
      let next: Phase;
      try {
        const r = await fetch(`${base}/config`, { cache: "no-store" });
        next = r.ok ? "ready" : "starting"; // the router answers 502 while Gradio starts
      } catch {
        next = "down"; // nothing listens: the app is not started
      }
      if (!live) return;
      setPhase(next);
      if (next !== "ready") timer = setTimeout(probe, 2000);
    };
    probe();
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [base, attempt]);

  const src = `${base}/?embed=1${tab ? `&tab=${encodeURIComponent(tab)}` : ""}`;

  return (
    <Template title={title}>
      <div className="flex w-full flex-col gap-3 p-4">
        {error && (
          <div className="flex flex-col gap-2">
            <div className="text-red-600">{error}</div>
            <div>
              Start it from the <LauncherExtensionsLink text="launcher" /> (Launch
              WebUI), then try again.
            </div>
            <div>
              <Button
                onClick={() => {
                  setError("");
                  setAttempt(attempt + 1);
                }}
              >
                Try again
              </Button>
            </div>
          </div>
        )}
        {!error && phase === "loading" && <div>Looking for Gradio…</div>}
        {phase === "starting" && (
          <div>Gradio is starting. The first start can take a minute.</div>
        )}
        {phase === "down" && (
          <div className="flex flex-col gap-2">
            <div>
              Gradio is not running on port {port}. Start the app from the{" "}
              <LauncherExtensionsLink text="launcher" />, then try again.
            </div>
            <div>
              <Button onClick={() => setAttempt(attempt + 1)}>Try again</Button>
            </div>
          </div>
        )}
        {phase === "ready" && <FullscreenFrame title={title} src={src} padded={!!tab} />}
      </div>
    </Template>
  );
};
