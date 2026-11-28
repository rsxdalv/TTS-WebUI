import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import { ExtensionRunDialog } from "../../components/ExtensionRunDialog";
import { errorText, goSendJson } from "../../functions/goData";
import { useExtensionRuns } from "../../functions/extensionOps";

type Entry = {
  package_name: string;
  name?: string;
  extension_class?: string;
  extension_type?: string;
  proxy?: string;
};

type Parsed = { entries: Entry[]; info: string };
type Added = { status: string; json: string };

/**
 * Add extension entries by pasting their JSON, and install them. The same
 * steps as the Gradio tab: Go validates the text (Parse), appends the valid
 * entries to extensions.external.json (Add) and installs them by name. The page
 * sends only the pasted text; Go does the parsing again before it writes.
 */
const CustomExtensionsPage = () => {
  const [text, setText] = React.useState("");
  const [parsed, setParsed] = React.useState<Parsed | null>(null);
  const [message, setMessage] = React.useState("");
  const [external, setExternal] = React.useState("");
  const [working, setWorking] = React.useState(false);
  const { run, view, busy, start, close } = useExtensionRuns();

  const guard = async (fn: () => Promise<void>) => {
    setWorking(true);
    setMessage("");
    try {
      await fn();
    } catch (e) {
      setMessage(errorText(e));
    } finally {
      setWorking(false);
    }
  };

  const parse = () =>
    guard(async () => {
      const p = await goSendJson<Parsed>("POST", "/api/go-data/extensions/parse", { text });
      setParsed(p);
      setMessage(p.info);
    });

  const add = async (): Promise<boolean> => {
    try {
      const a = await goSendJson<Added>("POST", "/api/go-data/extensions/add", { text });
      setMessage(a.status);
      setExternal(a.json);
      return true;
    } catch (e) {
      setMessage(errorText(e));
      return false;
    }
  };

  const addOnly = () => guard(async () => void (await add()));

  // Added first (an entry already there is skipped), then installed one by one.
  const addAndInstall = () =>
    guard(async () => {
      const p = await goSendJson<Parsed>("POST", "/api/go-data/extensions/parse", { text });
      setParsed(p);
      if (!(await add())) return;
      start(
        p.entries.map((e) => ({
          op: "install" as const,
          name: e.name || e.package_name,
          package_name: e.package_name,
          proxy: e.proxy,
        }))
      );
    });

  const disabled = working || busy || !text.trim();

  return (
    <Template title="Custom Extensions Installer">
      <div className="flex w-full flex-col gap-3 p-4">
        <p className="text-sm">
          Paste one or more extension JSON objects below. They are validated and previewed, and
          can be added to <code>extensions.external.json</code> and installed right away.
        </p>
        <textarea
          className="border rounded p-2 font-mono text-sm min-h-[16rem]"
          placeholder="Paste JSON object or array of objects here"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <div className="flex gap-2">
          <Button disabled={disabled} onClick={parse}>
            Parse JSON
          </Button>
          <Button variant="outline" disabled={disabled} onClick={addOnly}>
            Add to external list
          </Button>
          <Button disabled={disabled} onClick={addAndInstall}>
            Add and install
          </Button>
        </div>
        {message && <div className="text-sm whitespace-pre-wrap">{message}</div>}
        {parsed && parsed.entries.length > 0 && (
          <table className="text-sm">
            <thead>
              <tr className="text-left border-b">
                <th className="py-1 pr-4">Name</th>
                <th className="py-1 pr-4">Package</th>
                <th className="py-1 pr-4">Class</th>
                <th className="py-1">Type</th>
              </tr>
            </thead>
            <tbody>
              {parsed.entries.map((e) => (
                <tr key={e.package_name} className="border-b border-gray-100">
                  <td className="py-1 pr-4">{e.name}</td>
                  <td className="py-1 pr-4">
                    {e.package_name}
                    {e.proxy ? ` (${e.proxy})` : ""}
                  </td>
                  <td className="py-1 pr-4">{e.extension_class}</td>
                  <td className="py-1">{e.extension_type}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {external && (
          <details>
            <summary className="cursor-pointer text-sm">Current extensions.external.json</summary>
            <pre className="mt-2 max-h-[40vh] overflow-auto rounded bg-gray-100 p-2 text-xs dark:bg-gray-900">
              {external}
            </pre>
          </details>
        )}
      </div>
      <ExtensionRunDialog run={run} view={view} onClose={close} />
    </Template>
  );
};

export default CustomExtensionsPage;
