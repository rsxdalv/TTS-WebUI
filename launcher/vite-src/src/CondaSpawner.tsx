import { ProcessSpawner } from "./ProcessSpawner";
import { joinPath, WEBUI_ROOT } from "./config";

export interface CondaSpawnerProps {
  /** Display name for the process */
  name: string;
  /** Python command to run inside the conda env, e.g. "python server.py" */
  command: string;
  /** Additional args to pass to the python command, e.g. ["--no-react", "--no-database"] */
  args?: string[];
  /** URL to open when the process is ready */
  openUrl?: string;
  /** Working directory for the process */
  cwd?: string;
  /** Called when the process exits successfully (exit code 0) */
  onSuccess?: () => void;
}

/**
 * Cross-platform wrapper around ProcessSpawner that runs commands inside
 * the micromamba-managed conda environment.
 *
 * Uses the pattern:
 *   Windows: micromamba.bat -p <env> run <command>
 *   Linux:   micromamba    -p <env> run <command>
 *   Mac:     micromamba    -p <env> run <command>
 */
export function CondaSpawner({
  name,
  command,
  args = [],
  openUrl,
  cwd,
  onSuccess,
}: CondaSpawnerProps) {
  const pythonArgs = args.length > 0 ? ` ${args.join(" ")}` : "";

  const condaCommand = `${command}${pythonArgs}`;

  // micromamba is at installer_files/mamba/condabin/micromamba.bat (Windows)
  // or installer_files/mamba/condabin/micromamba (Linux/Mac)
  const micromambaBin = joinPath(
    WEBUI_ROOT,
    "installer_files",
    "mamba",
    "condabin",
    "micromamba",
  );

  const environmentPath = joinPath(WEBUI_ROOT, "installer_files", "env");
  return (
    <ProcessSpawner
      key={name}
      name={name}
      command={""}
      command_windows={`${micromambaBin}.bat -p ${environmentPath} run ${condaCommand}`}
      command_linux={`${micromambaBin} -p ${environmentPath} run ${condaCommand}`}
      command_mac={`${micromambaBin} -p ${environmentPath} run ${condaCommand}`}
      openUrl={openUrl}
      cwd={cwd ?? WEBUI_ROOT}
      onSuccess={onSuccess}
    />
  );
}
