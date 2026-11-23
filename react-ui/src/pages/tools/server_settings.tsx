import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import { errorText, goGetJson, goSendJson } from "../../functions/goData";

type Settings = {
  server_name: string;
  server_port: number | null;
  ssl_keyfile: string;
  ssl_certfile: string;
  ssl_verify: boolean | null;
  auth_users: string[];
  ssl_keyfile_password_set: boolean;
};

/**
 * Server options the Go core reads from config.json (server_name,
 * server_port, auth, ssl_*). Secrets are never shown: leave a password field
 * empty to keep it. The Gradio-only options of the old tab are not offered.
 */
const ServerSettingsPage = () => {
  const [s, setS] = React.useState<Settings | null>(null);
  const [authUser, setAuthUser] = React.useState("");
  const [authPassword, setAuthPassword] = React.useState("");
  const [removeAuth, setRemoveAuth] = React.useState(false);
  const [keyPassword, setKeyPassword] = React.useState("");
  const [removeKeyPassword, setRemoveKeyPassword] = React.useState(false);
  const [message, setMessage] = React.useState("");
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    goGetJson<Settings>("/api/go-data/server-settings")
      .then(setS)
      .catch((e) => setError(errorText(e)));
  }, []);

  const save = async () => {
    if (!s) return;
    setError("");
    setMessage("");
    const body: Record<string, unknown> = {
      server_name: s.server_name,
      ssl_keyfile: s.ssl_keyfile,
      ssl_certfile: s.ssl_certfile,
    };
    if (s.server_port !== null) body.server_port = s.server_port;
    if (s.ssl_verify !== null) body.ssl_verify = s.ssl_verify;
    if (removeAuth) body.auth = null;
    else if (authUser || authPassword) {
      body.auth = { user: authUser, password: authPassword };
    }
    if (removeKeyPassword) body.ssl_keyfile_password = null;
    else if (keyPassword) body.ssl_keyfile_password = keyPassword;
    try {
      const saved = await goSendJson<Settings>(
        "PUT",
        "/api/go-data/server-settings",
        body
      );
      setS(saved);
      setAuthPassword("");
      setKeyPassword("");
      setRemoveAuth(false);
      setRemoveKeyPassword(false);
      setMessage("Saved. Restart the app to apply.");
    } catch (e) {
      setError(errorText(e));
    }
  };

  const text = (label: string, value: string, set: (v: string) => void, type = "text") => (
    <label className="flex flex-col gap-1">
      <span className="font-medium">{label}</span>
      <input
        className="border rounded px-2 py-1"
        type={type}
        value={value}
        onChange={(e) => set(e.target.value)}
        autoComplete="off"
      />
    </label>
  );

  return (
    <Template title="Server Settings">
      <div className="flex flex-col gap-3 p-4 max-w-2xl">
        <p className="text-sm text-gray-600">
          Written to config.json. Changes apply after the app is restarted
          (there is no restart button here: restart from the launcher).
        </p>
        {error && <div className="text-red-600">{error}</div>}
        {s && (
          <>
            {text("server_name (0.0.0.0 makes the app reachable on the network)", s.server_name, (v) => setS({ ...s, server_name: v }))}
            <label className="flex flex-col gap-1">
              <span className="font-medium">server_port</span>
              <input
                className="border rounded px-2 py-1"
                type="number"
                min={1}
                max={65535}
                value={s.server_port ?? ""}
                onChange={(e) =>
                  setS({ ...s, server_port: e.target.value === "" ? null : Number(e.target.value) })
                }
              />
            </label>
            {text("ssl_keyfile", s.ssl_keyfile, (v) => setS({ ...s, ssl_keyfile: v }))}
            {text("ssl_certfile", s.ssl_certfile, (v) => setS({ ...s, ssl_certfile: v }))}
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={s.ssl_verify ?? false}
                onChange={(e) => setS({ ...s, ssl_verify: e.target.checked })}
              />
              <span>ssl_verify</span>
            </label>
            {text(
              `ssl_keyfile_password (${s.ssl_keyfile_password_set ? "set, leave empty to keep" : "not set"})`,
              keyPassword,
              setKeyPassword,
              "password"
            )}
            {s.ssl_keyfile_password_set && (
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={removeKeyPassword}
                  onChange={(e) => setRemoveKeyPassword(e.target.checked)}
                />
                <span>Remove the key password</span>
              </label>
            )}
            <div className="border-t pt-3 flex flex-col gap-2">
              <span className="font-medium">
                Login (auth):{" "}
                {s.auth_users.length ? `set for ${s.auth_users.join(", ")}` : "off"}
              </span>
              {text("user", authUser, setAuthUser)}
              {text("password", authPassword, setAuthPassword, "password")}
              {s.auth_users.length > 0 && (
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={removeAuth}
                    onChange={(e) => setRemoveAuth(e.target.checked)}
                  />
                  <span>Turn the login off</span>
                </label>
              )}
            </div>
          </>
        )}
        <div className="flex gap-3 items-center">
          <Button onClick={save} disabled={!s}>
            Save
          </Button>
          {message && <span className="text-sm text-green-700">{message}</span>}
        </div>
      </div>
    </Template>
  );
};

export default ServerSettingsPage;
