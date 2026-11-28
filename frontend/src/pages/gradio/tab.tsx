import React from "react";
import { Link, useParams } from "react-router-dom";
import { Template } from "../../components/Template";
import { GradioFrame } from "../../components/GradioFrame";
import EnginePage from "../engines/engine";
import { goGetJson } from "../../functions/goData";
import { tabSlug, useGradioTabs } from "../../functions/gradioTabs";

type CatalogEntry = { package_name: string; name: string; proxy?: string };
type Native = { pkg: string; name: string };

/**
 * The extensions that run as their own process in their own venv ("proxy":
 * "native" in the catalog), by the slug of their name, which is the title of
 * their Gradio tab. The catalog lists them whether or not they are installed:
 * an extension that is not installed gets its own page too, with the Install
 * button. Asked once per page load.
 */
let natives: Promise<Map<string, Native>> | null = null;
const nativeBySlug = () =>
  (natives ??= goGetJson<CatalogEntry[]>("/api/go-data/extensions/interface")
    .then((l) => {
      const m = new Map<string, Native>();
      for (const e of l) {
        const slug = e.proxy === "native" ? tabSlug(e.name || "") : "";
        if (slug && !m.has(slug)) m.set(slug, { pkg: e.package_name, name: e.name });
      }
      return m;
    })
    .catch(() => {
      natives = null;
      return new Map<string, Native>();
    }));

/**
 * One tab of the Gradio UI, /gradio/<group> or /gradio/<group>/<tab> (the
 * links of the Gradio menu). A tab whose name is an extension that runs in its
 * own venv (Kokoro, Chatterbox, ...) shows that extension's page directly,
 * installed or not; any other tab (they run inside the Gradio app itself) is
 * the Gradio UI opened on it.
 */
const GradioTabPage = () => {
  const { group: groupSlug = "", tab: tabSlugParam } = useParams();
  const tabs = useGradioTabs();
  const [native, setNative] = React.useState<Native | null | undefined>(undefined);

  const group = tabs?.find((g) => g.slug === groupSlug);
  const tab = tabSlugParam ? group?.children.find((t) => t.slug === tabSlugParam) : undefined;
  const label = tab?.label ?? group?.label ?? "";
  const known = !!tabs && !!group && (!tabSlugParam || !!tab);
  const direct = tab ? tabSlug(tab.label) : "";

  React.useEffect(() => {
    let live = true;
    setNative(undefined);
    if (!direct) {
      setNative(null);
      return;
    }
    nativeBySlug().then((m) => live && setNative(m.get(direct) ?? null));
    return () => {
      live = false;
    };
  }, [direct]);

  if (!tabs) {
    return (
      <Template title="Gradio (legacy)">
        <div className="p-4">Looking for Gradio…</div>
      </Template>
    );
  }
  if (!known) {
    return (
      <Template title="Gradio (legacy)">
        <div className="p-4">
          There is no such tab in the Gradio UI. <Link className="underline" to="/gradio">Open Gradio</Link>
        </div>
      </Template>
    );
  }
  if (native === undefined) {
    return (
      <Template title={label}>
        <div className="p-4">Loading…</div>
      </Template>
    );
  }
  if (native) return <EnginePage pkg={native.pkg} name={native.name} />;
  return <GradioFrame title={`${label} (Gradio)`} tab={group ? (tab ? `${group.label}/${tab.label}` : group.label) : undefined} />;
};

export default GradioTabPage;
