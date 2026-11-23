import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import { errorText, goGetJson } from "../../functions/goData";

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

type Kind = "interface" | "decorator";

/**
 * The extension lists (interface tabs and decorators) from the Go catalog,
 * read only. Install and uninstall are in the Gradio tabs and the launcher.
 */
const ExtensionsPage = () => {
  const [kind, setKind] = React.useState<Kind>("interface");
  const [entries, setEntries] = React.useState<Entry[] | null>(null);
  const [filter, setFilter] = React.useState("");
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    setEntries(null);
    setError("");
    goGetJson<Entry[]>(`/api/go-data/extensions/${kind}`)
      .then(setEntries)
      .catch((e) => setError(errorText(e)));
  }, [kind]);

  const f = filter.trim().toLowerCase();
  const shown = (entries ?? []).filter((e) =>
    [e.name, e.package_name, e.description, e.extension_class]
      .join(" ")
      .toLowerCase()
      .includes(f)
  );

  return (
    <Template title="Extensions">
      <div className="flex flex-col gap-3 p-4">
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
        <p className="text-sm text-gray-600">
          Read only. Install, update and uninstall are done from the Extensions
          tab of the Gradio UI or the launcher.
        </p>
        {error && <div className="text-red-600">{error}</div>}
        {entries && (
          <table className="text-sm">
            <thead>
              <tr className="text-left border-b">
                <th className="py-1 pr-4">Name</th>
                <th className="py-1 pr-4">Class</th>
                <th className="py-1 pr-4">Package</th>
                <th className="py-1">Description</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((e) => (
                <tr key={e.package_name} className="border-b border-gray-100 align-top">
                  <td className="py-1 pr-4">
                    {e.website ? (
                      <a className="underline" href={e.website} target="_blank" rel="noreferrer">
                        {e.name}
                      </a>
                    ) : (
                      e.name
                    )}
                  </td>
                  <td className="py-1 pr-4">{e.extension_class}</td>
                  <td className="py-1 pr-4">
                    {e.package_name}
                    {e.proxy ? ` (${e.proxy})` : ""}
                  </td>
                  <td className="py-1">{e.description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Template>
  );
};

export default ExtensionsPage;
