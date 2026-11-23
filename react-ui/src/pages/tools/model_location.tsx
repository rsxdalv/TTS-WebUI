import React from "react";
import { Template } from "../../components/Template";
import { Button } from "../../components/ui/button";
import { errorText, goGetJson, goSendJson } from "../../functions/goData";

type Locations = {
  huggingface_hub_cache: string;
  hf_home: string;
  torch_home: string;
  xdg_cache_home: string;
};

const FIELDS: [keyof Locations, string, string][] = [
  ["huggingface_hub_cache", "HUGGINGFACE_HUB_CACHE", "Hugging Face cache (takes precedence over HF_HOME)"],
  ["hf_home", "HF_HOME", "Hugging Face home"],
  ["torch_home", "TORCH_HOME", "Torch Hub cache, default ~/.cache/torch/"],
  ["xdg_cache_home", "XDG_CACHE_HOME", "General cache, default ~/.cache/"],
];

/** Model cache locations, saved to .env by the Go core. Needs a restart. */
const ModelLocationPage = () => {
  const [values, setValues] = React.useState<Locations | null>(null);
  const [message, setMessage] = React.useState("");
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    goGetJson<Locations>("/api/go-data/model-locations")
      .then(setValues)
      .catch((e) => setError(errorText(e)));
  }, []);

  const save = async () => {
    if (!values) return;
    setError("");
    setMessage("");
    try {
      await goSendJson("PUT", "/api/go-data/model-locations", values);
      setMessage("Saved to .env. Restart the app to apply.");
    } catch (e) {
      setError(errorText(e));
    }
  };

  return (
    <Template title="Model Location Settings">
      <div className="flex flex-col gap-3 p-4 max-w-2xl">
        <p className="text-sm text-gray-600">
          Experimental. Changes are written to .env and apply after a restart.
          Empty means unset. The tts-webui model directory
          (./data/models/) cannot be changed here.
        </p>
        {error && <div className="text-red-600">{error}</div>}
        {values &&
          FIELDS.map(([key, label, hint]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="font-medium">{label}</span>
              <input
                className="border rounded px-2 py-1"
                placeholder={hint}
                value={values[key]}
                onChange={(e) => setValues({ ...values, [key]: e.target.value })}
              />
            </label>
          ))}
        <div className="flex gap-3 items-center">
          <Button onClick={save} disabled={!values}>
            Save
          </Button>
          {message && <span className="text-sm text-green-700">{message}</span>}
        </div>
      </div>
    </Template>
  );
};

export default ModelLocationPage;
