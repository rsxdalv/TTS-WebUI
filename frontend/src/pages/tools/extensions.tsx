import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import { ExtensionRunDialog } from "../../components/ExtensionRunDialog";
import { errorText, goGetJson } from "../../functions/goData";
import { Op, useExtensionRuns } from "../../functions/extensionOps";

type Entry = {
  package_name?: string;
  name?: string;
  description?: string;
  extension_class?: string;
  author?: string;
  website?: string;
  requirements?: string;
  proxy?: string;
};

/** What the control API says about an entry (installed, version, enabled). */
type State = { installed: boolean; version?: string; enabled: boolean; proxy?: string };

type Kind = "interface" | "decorator";

/** Built-in tabs are part of the host (as in interface_extensions._is_builtin_tab): no install or uninstall. */
const isBuiltin = (e: Entry) =>
  e.requirements === "builtin" ||
  ((e.package_name ?? "").includes("builtin") && e.proxy !== "native");

/**
 * The extension lists (interface tabs and decorators) from the Go catalog, with
 * install and uninstall. The same operations the launcher's Extensions page
 * runs; the Gradio list tabs are gone.
 */
const ExtensionsPage = () => {
  const [kind, setKind] = React.useState<Kind>("interface");
  const [entries, setEntries] = React.useState<Entry[] | null>(null);
  const [states, setStates] = React.useState<Record<string, State>>({});
  const [filter, setFilter] = React.useState("");
  const [error, setError] = React.useState("");

  const loadStates = React.useCallback(
    () =>
      goGetJson<Record<string, State>>(`/api/go-data/extensions/${kind}/state`)
        .then(setStates)
        .catch(() => undefined),
    [kind]
  );
  const { run, view, busy, start, close } = useExtensionRuns(loadStates);

  React.useEffect(() => {
    setEntries(null);
    setStates({});
    setError("");
    goGetJson<Entry[]>(`/api/go-data/extensions/${kind}`)
      .then(setEntries)
      .catch((e) => setError(errorText(e)));
    loadStates();
  }, [kind, loadStates]);

  const begin = (e: Entry, op: Op) => {
    const name = e.name || e.package_name || "";
    if (op === "uninstall" && !window.confirm(`Uninstall ${name}?`)) return;
    start([{ op, name, package_name: e.package_name ?? "", proxy: e.proxy }]);
  };

  const f = filter.trim().toLowerCase();
  const shown = (entries ?? []).filter((e) =>
    [e.name, e.package_name, e.description, e.extension_class]
      .join(" ")
      .toLowerCase()
      .includes(f)
  );

  const statusOf = (e: Entry) => {
    if (isBuiltin(e)) return "built in";
    const s = states[e.package_name ?? ""];
    if (!s) return "";
    const base = s.installed
      ? `installed${s.version ? ` ${s.version}` : ""}`
      : "not installed";
    return s.enabled ? base : `${base}, disabled`;
  };

  return (
    <Template title="Extensions">
      <div className="flex w-full flex-col gap-3 p-4">
        <div className="flex gap-2 items-center">
          <Button
            variant={kind === "interface" ? "default" : "outline"}
            onClick={() => setKind("interface")}
          >
            Extensions
          </Button>
          <Button
            variant={kind === "decorator" ? "default" : "outline"}
            onClick={() => setKind("decorator")}
          >
            Decorators
          </Button>
          <input
            className="border rounded px-2 py-1 flex-1"
            placeholder="Filter"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
        {error && <div className="text-red-600">{error}</div>}
        {entries && (
          <table className="text-sm">
            <thead>
              <tr className="text-left border-b">
                <th className="py-1 pr-4">Name</th>
                <th className="py-1 pr-4">Class</th>
                <th className="py-1 pr-4">Status</th>
                <th className="py-1 pr-4">Description</th>
                <th className="py-1" />
              </tr>
            </thead>
            <tbody>
              {shown.map((e) => {
                const s = states[e.package_name ?? ""];
                return (
                  <tr key={e.package_name} className="border-b border-gray-100 align-top">
                    <td className="py-1 pr-4">
                      {e.website ? (
                        <a className="underline" href={e.website} target="_blank" rel="noreferrer">
                          {e.name}
                        </a>
                      ) : (
                        e.name
                      )}
                      <div className="text-xs text-gray-500">
                        {e.package_name}
                        {e.proxy ? ` (${e.proxy})` : ""}
                      </div>
                    </td>
                    <td className="py-1 pr-4">{e.extension_class}</td>
                    <td className="py-1 pr-4 whitespace-nowrap">{statusOf(e)}</td>
                    <td className="py-1 pr-4">{e.description}</td>
                    <td className="py-1 text-right whitespace-nowrap">
                      {!isBuiltin(e) && s && (
                        <Button
                          size="sm"
                          variant={s.installed ? "outline" : "default"}
                          disabled={busy}
                          onClick={() => begin(e, s.installed ? "uninstall" : "install")}
                        >
                          {s.installed ? "Uninstall" : "Install"}
                        </Button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
      <ExtensionRunDialog run={run} view={view} onClose={close} />
    </Template>
  );
};

export default ExtensionsPage;
