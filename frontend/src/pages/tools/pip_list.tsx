import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import { errorText, goGetJson } from "../../functions/goData";

type Package = { name: string; version: string };

/** Installed packages of the main environment (pip list, done by the Go core). */
const PipListPage = () => {
  const [packages, setPackages] = React.useState<Package[] | null>(null);
  const [filter, setFilter] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");

  const refresh = async () => {
    setLoading(true);
    setError("");
    try {
      setPackages(await goGetJson<Package[]>("/api/go-data/pip-list"));
    } catch (e) {
      const message = errorText(e);
      setError(
        message.includes("no Python environment")
          ? "There is no Python environment (not installed yet, or no Python on PATH). The package list needs one; everything else in the core works without it."
          : message
      );
    } finally {
      setLoading(false);
    }
  };

  const shown = (packages ?? []).filter((p) =>
    p.name.toLowerCase().includes(filter.trim().toLowerCase())
  );

  return (
    <Template title="Installed Packages">
      <div className="flex flex-col gap-3 p-4 max-w-2xl">
        <div className="flex gap-2 items-center">
          <Button onClick={refresh} disabled={loading}>
            {loading ? "Loading..." : packages ? "Refresh" : "Load"}
          </Button>
          <input
            className="border rounded px-2 py-1 flex-1"
            placeholder="Filter by name"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          {packages && (
            <span className="text-sm text-gray-500">
              {shown.length} of {packages.length}
            </span>
          )}
        </div>
        {error && <div className="text-red-600">{error}</div>}
        {packages && (
          <table className="text-sm">
            <thead>
              <tr className="text-left border-b">
                <th className="py-1 pr-4">Package</th>
                <th className="py-1">Version</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((p) => (
                <tr key={p.name} className="border-b border-gray-100">
                  <td className="py-1 pr-4">{p.name}</td>
                  <td className="py-1">{p.version}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Template>
  );
};

export default PipListPage;
