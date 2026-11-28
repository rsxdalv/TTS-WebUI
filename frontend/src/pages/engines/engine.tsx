import React from "react";
import { useSearchParams } from "react-router-dom";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import { FullscreenFrame } from "../../components/FullscreenFrame";
import { ExtensionManager, ExtensionActions } from "../../components/ExtensionManager";
import { goPostStream } from "../../functions/goStream";
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
type ExtState = {
  installed: boolean;
  version?: string;
  enabled: boolean;
  autostart: boolean;
};

const EnginePage = ({ pkg: fixedPkg, name: fixedName }: { pkg?: string; name?: string }) => {
  // The package name has a dot, so it is a query value: a path with a dot is a
  // missing asset to the static server.
  const queryPkg = useSearchParams()[0].get("pkg") ?? "";
  const pkg = fixedPkg ?? queryPkg;
  const [answer, setAnswer] = React.useState<Answer | null>(null);
  const [error, setError] = React.useState("");
  const [attempt, setAttempt] = React.useState(0);
  const base = `/api/go-data/engines/${encodeURIComponent(pkg)}`;
  const [info, setInfo] = React.useState<ExtState | null>(null);
  const [stoppedByUser, setStoppedByUser] = React.useState(false);

  const refreshInfo = React.useCallback(async () => {
    try {
      const all = await goGetJson<Record<string, ExtState>>(
        "/api/go-data/extensions/interface/state"
      );
      setInfo(all[pkg] ?? null);
    } catch {
      // the panel keeps what it knew
    }
  }, [pkg]);
  React.useEffect(() => {
    refreshInfo();
  }, [refreshInfo, attempt]);

  const config = (op: "enable" | "disable" | "autostart", extra: object = {}) =>
    goSendJson<{ info?: string }>("POST", `/api/go-data/extension-config/${op}`, {
      package_name: pkg,
      ...extra,
    });

  // Install and uninstall stream their output; the page opens the extension
  // again afterwards (it starts when it is installed, and says so when it is not).
  const streamed =
    (op: "install" | "uninstall"): ExtensionActions["install"] =>
    async (log) => {
      const ok = await goPostStream(`/api/go-data/extensions/${op}`, { package_name: pkg }, log);
      log(ok ? `${op === "install" ? "Installed" : "Uninstalled"}.` : `${op} failed.`);
      await refreshInfo();
      if (ok) {
        setStoppedByUser(false);
        setAttempt((a) => a + 1);
      }
    };

  const actions: ExtensionActions = {
    install: streamed("install"),
    uninstall: streamed("uninstall"),
    start: async (log) => {
      log("Starting…");
      setStoppedByUser(false);
      setAttempt((a) => a + 1);
    },
    shutdown: async (log) => {
      await goSendJson("POST", `${base}/stop`, {});
      setStoppedByUser(true);
      setAnswer(await goGetJson<Answer>(base));
      log("Stopped.");
    },
    setEnabled: async (enabled, log) => {
      const r = await config(enabled ? "enable" : "disable");
      log(r.info || (enabled ? "Enabled." : "Disabled.") + " It applies the next time the Gradio UI starts.");
      await refreshInfo();
    },
    setAutostart: async (autostart, log) => {
      await config("autostart", { enabled: autostart });
      log(autostart ? "Added to autostart." : "Removed from autostart.");
      await refreshInfo();
    },
  };

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
            {(answer.state === "stopped" || answer.state === "exited") && stoppedByUser
              ? "The extension is stopped. Press Start Extension."
              : answer.state === "starting" || answer.state === "stopped"
                ? "Starting the engine. The first start can take a minute."
                : `The engine is ${answer.state}. ${answer.error ?? ""}`}
          </div>
        )}
        {(error || answer?.state === "failed" || (answer?.state === "exited" && !stoppedByUser)) && (
          <div className="p-4">
            <Button onClick={() => setAttempt(attempt + 1)}>Try again</Button>
          </div>
        )}
        {pkg && (
          <ExtensionManager
            name={fixedName ?? answer?.name ?? pkg.split(".").pop() ?? pkg}
            installed={info ? info.installed : true}
            running={answer?.state === "ready"}
            starting={answer?.state === "starting"}
            enabled={info?.enabled ?? true}
            autostart={info?.autostart ?? false}
            actions={actions}
          />
        )}
        {src && <FullscreenFrame title={answer?.name || pkg} src={src} padded />}
      </div>
    </Template>
  );
};

export default EnginePage;
