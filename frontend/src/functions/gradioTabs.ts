import React from "react";
import { goGetJson } from "./goData";

/**
 * The tabs of the Gradio UI, read from Gradio's own /config, so the menu lists
 * exactly what is installed: a group per top-level tab ("💬 Text-to-Speech")
 * with the tabs inside it ("Kokoro", "Bark (uv)", ...). Deeper tabs stay inside
 * the frame.
 */
export type GradioTab = {
  label: string;
  slug: string;
  children: GradioTab[];
};

/** "💬 Text-to-Speech" -> "text-to-speech"; the same words Gradio's head script matches on. */
export const tabSlug = (label: string): string =>
  label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .replace(/ /g, "-");

type Node = { id: number; children?: Node[] };
type Config = {
  components: { id: number; type: string; props: { label?: string } }[];
  layout: Node;
};

const unique = (tabs: GradioTab[]): GradioTab[] => {
  const seen = new Map<string, number>();
  return tabs.map((t) => {
    const n = (seen.get(t.slug) ?? 0) + 1;
    seen.set(t.slug, n);
    return n === 1 ? t : { ...t, slug: `${t.slug}-${n}` };
  });
};

/** The two levels of tabs in a Gradio /config (components "goodtabs" and "goodtab"). */
export const parseTabs = (config: Config): GradioTab[] => {
  const type = new Map(config.components.map((c) => [c.id, c]));
  const kind = (n: Node) => type.get(n.id)?.type;
  const firstTabs = (n: Node): Node | undefined => {
    if (kind(n) === "goodtabs") return n;
    for (const c of n.children ?? []) {
      const found = firstTabs(c);
      if (found) return found;
    }
    return undefined;
  };
  const tabsOf = (row: Node | undefined, depth: number): GradioTab[] =>
    unique(
      (row?.children ?? [])
        .filter((n) => kind(n) === "goodtab")
        .map((n) => {
          const label = type.get(n.id)?.props.label ?? "";
          return {
            label,
            slug: tabSlug(label),
            children: depth < 2 ? tabsOf(firstTabs({ id: -1, children: n.children }), depth + 1) : [],
          };
        })
        .filter((t) => t.slug)
    );
  return tabsOf(firstTabs(config.layout), 1);
};

/** Where the Gradio page is: the public port of this host, under the prefix the server reports. */
const gradioBase = async (): Promise<string> => {
  const s = await goGetJson<{ server_port: number | null; gradio_prefix?: string }>(
    "/api/go-data/server-settings"
  );
  const port = s.server_port ?? 7770;
  const here = window.location.port || (window.location.protocol === "https:" ? "443" : "80");
  const origin =
    String(port) === here
      ? window.location.origin
      : `${window.location.protocol}//${window.location.hostname}:${port}`;
  return origin + (s.gradio_prefix ?? "");
};

/** One request per page load; a failure (Gradio not up yet) is asked again by the next page. */
let pending: Promise<GradioTab[]> | null = null;
const askOnce = (): Promise<GradioTab[]> => {
  if (!pending) {
    pending = (async () => {
      const r = await fetch(`${await gradioBase()}/config`, { cache: "no-store" });
      if (!r.ok) throw new Error(`Gradio answered ${r.status}`);
      return parseTabs((await r.json()) as Config);
    })();
    pending.catch(() => {
      pending = null;
    });
  }
  return pending;
};

/** The tab tree, or null while unknown (not asked, loading, or Gradio is not running). */
export const useGradioTabs = (enabled = true): GradioTab[] | null => {
  const [tabs, setTabs] = React.useState<GradioTab[] | null>(null);
  React.useEffect(() => {
    if (!enabled) return;
    let live = true;
    askOnce()
      .then((t) => live && setTabs(t))
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, [enabled]);
  return tabs;
};
