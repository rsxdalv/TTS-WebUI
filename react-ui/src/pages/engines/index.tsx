import React from "react";
import { Link } from "react-router-dom";
import { Template } from "../../components/Template";
import { errorText, goGetJson } from "../../functions/goData";

type Engine = { package: string; name: string; description: string };

/**
 * The engines installed in their own venv (the launcher's Extensions page
 * installs them). Opening one starts it and shows its own page.
 */
const EnginesPage = () => {
  const [engines, setEngines] = React.useState<Engine[] | null>(null);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    goGetJson<Engine[]>("/api/go-data/engine-list")
      .then(setEngines)
      .catch((e) => setError(errorText(e)));
  }, []);

  return (
    <Template title="Engines">
      <div className="flex flex-col gap-3 p-4">
        {error && <div className="text-red-600">{error}</div>}
        {engines && engines.length === 0 && (
          <p>
            No engine is installed. Install one from the Extensions page of the
            launcher (port 7700).
          </p>
        )}
        <ul className="flex flex-col gap-2">
          {(engines ?? []).map((e) => (
            <li key={e.package} className="border rounded p-3">
              <Link
                className="font-semibold underline"
                to={`/engine?pkg=${encodeURIComponent(e.package)}`}
              >
                {e.name || e.package}
              </Link>
              <div className="text-sm text-gray-600">{e.description}</div>
            </li>
          ))}
        </ul>
      </div>
    </Template>
  );
};

export default EnginesPage;
