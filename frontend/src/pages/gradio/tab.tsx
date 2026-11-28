import React from "react";
import { Link, useParams } from "react-router-dom";
import { Template } from "../../components/Template";
import { GradioFrame } from "../../components/GradioFrame";
import EnginePage from "../engines/engine";
import { goGetJson } from "../../functions/goData";
import { tabSlug, useGradioTabs } from "../../functions/gradioTabs";

type Engine = { package: string; name: string };

/** The extensions that run in their own venv, by the slug of their name (asked once). */
let engines: Promise<Map<string, string>> | null = null;
const engineBySlug = () =>
  (engines ??= goGetJson<Engine[]>("/api/go-data/engine-list")
    .then((l) => new Map(l.map((e) => [tabSlug(e.name || e.package), e.package])))
    .catch(() => new Map<string, string>()));

/**
 * One tab of the Gradio UI, /gradio/<group> or /gradio/<group>/<tab> (the
 * links of the Gradio menu). A tab whose name is an extension that runs in its
 * own venv (Kokoro) shows that extension's page directly; any other tab is the
 * Gradio UI opened on it.
 */
const GradioTabPage = () => {
  const { group: groupSlug = "", tab: tabSlugParam } = useParams();
  const tabs = useGradioTabs();
  const [engine, setEngine] = React.useState<string | null | undefined>(undefined);

  const group = tabs?.find((g) => g.slug === groupSlug);
  const tab = tabSlugParam ? group?.children.find((t) => t.slug === tabSlugParam) : undefined;
  const label = tab?.label ?? group?.label ?? "";
  const known = !!tabs && !!group && (!tabSlugParam || !!tab);
  const direct = tab ? tabSlug(tab.label) : "";

  React.useEffect(() => {
    let live = true;
    setEngine(undefined);
    if (!direct) {
      setEngine(null);
      return;
    }
    engineBySlug().then((m) => live && setEngine(m.get(direct) ?? null));
    return () => {
      live = false;
    };
  }, [direct]);

  if (!tabs) {
    return (
      <Template title="Gradio">
        <div className="p-4">Looking for Gradio…</div>
      </Template>
    );
  }
  if (!known) {
    return (
      <Template title="Gradio">
        <div className="p-4">
          There is no such tab in the Gradio UI. <Link className="underline" to="/gradio">Open Gradio</Link>
        </div>
      </Template>
    );
  }
  if (engine === undefined) {
    return (
      <Template title={label}>
        <div className="p-4">Loading…</div>
      </Template>
    );
  }
  if (engine) return <EnginePage pkg={engine} />;
  return <GradioFrame title={`${label} (Gradio)`} tab={group ? (tab ? `${group.label}/${tab.label}` : group.label) : undefined} />;
};

export default GradioTabPage;
