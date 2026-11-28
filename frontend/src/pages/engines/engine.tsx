import React from "react";
import { useSearchParams } from "react-router-dom";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import { FullscreenFrame } from "../../components/FullscreenFrame";
import { errorText, goGetJson, goSendJson } from "../../functions/goData";

type Answer = {
  package: string;
  name: string;
  state: "stopped" | "starting" | "ready" | "failed" | "exited";
  error?: string;
  public_port: number;
  path: string;
};

/**
 * Opening the page starts the engine (Go starts it only when it is installed)
 * and shows the engine's own Gradio page. That page is served by the router on
 * the Gradio port, so the frame points there, on the host this page was
 * opened from.
 */
const EnginePage = ({ pkg: fixedPkg }: { pkg?: string }) => {
  // The package name has a dot, so it is a query value: a path with a dot is a
  // missing asset to the static server.
  const queryPkg = useSearchParams()[0].get("pkg") ?? "";
  const pkg = fixedPkg ?? queryPkg;
  const [answer, setAnswer] = React.useState<Answer | null>(null);
  const [error, setError] = React.useState("");
  const [attempt, setAttempt] = React.useState(0);
  const base = `/api/go-data/engines/${encodeURIComponent(pkg)}`;

  React.useEffect(() => {
    let live = true;
    setError("");
    setAnswer(null);
    const poll = async (first: boolean) => {
      try {
        const a = first
          ? await goSendJson<Answer>("POST", `${base}/open`, {})
          : await goGetJson<Answer>(base);
        if (!live) return;
        setAnswer(a);
        if (a.state === "starting" || (first && a.state !== "ready")) {
          setTimeout(() => poll(false), 750);
        }
      } catch (e) {
        if (live) setError(errorText(e));
      }
    };
    poll(true);
    return () => {
      live = false;
    };
  }, [base, attempt]);

  const src =
    answer?.state === "ready"
      ? `${window.location.protocol}//${window.location.hostname}:${answer.public_port}${answer.path}`
      : "";

  return (
    <Template title={answer?.name || "Engine"}>
      <div className="flex w-full flex-col gap-3">
        {error && <div className="p-4 text-red-600">{error}</div>}
        {!error && answer && answer.state !== "ready" && (
          <div className="p-4">
            {answer.state === "starting" || answer.state === "stopped"
              ? "Starting the engine. The first start can take a minute."
              : `The engine is ${answer.state}. ${answer.error ?? ""}`}
          </div>
        )}
        {(error || answer?.state === "failed" || answer?.state === "exited") && (
          <div className="p-4">
            <Button onClick={() => setAttempt(attempt + 1)}>Try again</Button>
          </div>
        )}
        {src && <FullscreenFrame title={answer?.name || pkg} src={src} padded />}
      </div>
    </Template>
  );
};

export default EnginePage;
