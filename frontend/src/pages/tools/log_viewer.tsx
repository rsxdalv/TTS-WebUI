import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import {
  errorText,
  goGetJson,
  goSendJson,
} from "../../functions/goData";

type Kind = "all" | "install" | "uninstall";
type LogFile = { name: string; file: string };

const BASE = "/api/go-data/logs";
const q = (params: Record<string, string | number>) =>
  new URLSearchParams(
    Object.entries(params).map(([k, v]) => [k, String(v)])
  ).toString();

/** The logs of installer_scripts/logs: list, read, search, analyse, delete. */
const LogViewerPage = () => {
  const [kind, setKind] = React.useState<Kind>("all");
  const [files, setFiles] = React.useState<LogFile[]>([]);
  const [stats, setStats] = React.useState("");
  const [selected, setSelected] = React.useState("");
  const [info, setInfo] = React.useState("");
  const [analysis, setAnalysis] = React.useState("");
  const [content, setContent] = React.useState("");
  const [search, setSearch] = React.useState("");
  const [maxLines, setMaxLines] = React.useState(1000);
  const [status, setStatus] = React.useState("");
  const [error, setError] = React.useState("");

  const run = async (fn: () => Promise<void>) => {
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(errorText(e));
    }
  };

  const loadList = (k: Kind) =>
    run(async () => {
      if (k === "all") {
        const names = await goGetJson<string[]>(`${BASE}/files`);
        setFiles(names.map((n) => ({ name: n, file: n })));
        setStats(await goGetJson<string>(`${BASE}/stats`));
      } else {
        setFiles(await goGetJson<LogFile[]>(`${BASE}/pip-logs?${q({ kind: k })}`));
        setStats(await goGetJson<string>(`${BASE}/summary?${q({ kind: k })}`));
      }
      setSelected("");
      setInfo("");
      setAnalysis("");
      setContent("");
    });

  React.useEffect(() => {
    loadList(kind);
  }, [kind]);

  const readFile = (file: string) =>
    run(async () => {
      setContent(
        await goGetJson<string>(
          `${BASE}/read?${q({ file, search, max_lines: maxLines })}`
        )
      );
    });

  const select = (file: string) => {
    setSelected(file);
    setInfo("");
    setAnalysis("");
    setContent("");
    if (!file) return;
    run(async () => {
      setInfo(await goGetJson<string>(`${BASE}/info?${q({ file })}`));
      if (kind === "all") {
        await readFile(file);
      } else {
        setAnalysis(await goGetJson<string>(`${BASE}/analyze?${q({ file })}`));
      }
    });
  };

  const afterDelete = (res: { status: string }) => {
    setStatus(res.status);
    return loadList(kind);
  };

  const deleteSelected = () => {
    if (!selected || !window.confirm(`Delete ${selected}?`)) return;
    run(async () =>
      afterDelete(await goSendJson("POST", `${BASE}/delete`, { file: selected }))
    );
  };

  const deleteAll = () => {
    if (!window.confirm("Delete all log files?")) return;
    run(async () => afterDelete(await goSendJson("POST", `${BASE}/delete-all`, {})));
  };

  const tabs: [Kind, string][] = [
    ["all", "All Logs"],
    ["install", "Pip Install"],
    ["uninstall", "Pip Uninstall"],
  ];

  return (
    <Template title="Log Viewer">
      <div className="flex flex-col gap-3 p-4">
        <div className="flex gap-2">
          {tabs.map(([k, label]) => (
            <Button
              key={k}
              variant={k === kind ? "default" : "outline"}
              onClick={() => setKind(k)}
            >
              {label}
            </Button>
          ))}
          <Button variant="outline" onClick={() => loadList(kind)}>
            Refresh
          </Button>
        </div>
        {error && <div className="text-red-600">{error}</div>}
        {status && <div className="text-sm text-gray-600">{status}</div>}
        <div className="flex gap-4 flex-wrap md:flex-nowrap">
          <div className="flex flex-col gap-2 w-full md:w-1/3">
            <pre className="whitespace-pre-wrap text-sm">{stats}</pre>
            <select
              className="border rounded px-2 py-1"
              value={selected}
              onChange={(e) => select(e.target.value)}
            >
              <option value="">Select a log file</option>
              {files.map((f) => (
                <option key={f.file} value={f.file}>
                  {f.name}
                </option>
              ))}
            </select>
            <pre className="whitespace-pre-wrap text-sm">{info}</pre>
            {kind === "all" && (
              <div className="flex gap-2">
                <Button
                  variant="destructive"
                  disabled={!selected}
                  onClick={deleteSelected}
                >
                  Delete selected
                </Button>
                <Button variant="destructive" onClick={deleteAll}>
                  Delete all
                </Button>
              </div>
            )}
          </div>
          <div className="flex flex-col gap-2 w-full md:w-2/3">
            {kind !== "all" && analysis && (
              <pre className="whitespace-pre-wrap text-sm border rounded p-2">
                {analysis}
              </pre>
            )}
            <div className="flex gap-2">
              <input
                className="border rounded px-2 py-1 flex-1"
                placeholder="Search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && selected && readFile(selected)}
              />
              <input
                className="border rounded px-2 py-1 w-28"
                type="number"
                min={100}
                max={10000}
                step={100}
                value={maxLines}
                onChange={(e) => setMaxLines(Number(e.target.value) || 1000)}
              />
              <Button disabled={!selected} onClick={() => readFile(selected)}>
                View
              </Button>
            </div>
            <textarea
              className="border rounded p-2 font-mono text-xs h-[32rem] w-full"
              readOnly
              value={content}
            />
          </div>
        </div>
      </div>
    </Template>
  );
};

export default LogViewerPage;
