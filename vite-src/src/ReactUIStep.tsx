import { os } from "@neutralinojs/lib";
import { CondaSpawner } from "./CondaSpawner";
import { WEBUI_ROOT, OUTPUTS_PATH, joinPath } from "./config";

interface ReactUIStepProps {
  onComplete: () => void;
}

interface ActionButtonProps {
  label: string;
  onClick: () => void;
}

function ActionButton({ label, onClick }: ActionButtonProps) {
  return (
    <button
      onClick={onClick}
      className="py-2 px-4 rounded-lg bg-gray-200 hover:bg-gray-300 dark:bg-gray-800 dark:hover:bg-gray-700 text-gray-900 dark:text-white font-medium transition-colors cursor-pointer text-sm"
    >
      {label}
    </button>
  );
}

export function ReactUIStep({ }: ReactUIStepProps) {
  const isWindows = (window as any).NL_OS === "Windows";

  const openTerminal = async () => {
    const cmd = isWindows
      ? `start ${joinPath(WEBUI_ROOT, "installer_scripts/tools/open_terminal.bat")}`
      : `${joinPath(WEBUI_ROOT, "installer_scripts/tools/open_terminal.sh")}`;
    console.log(`[ReactUIStep] Opening terminal with command: ${cmd}`);
    try {
      await os.spawnProcess(cmd);
    } catch (error) {
      console.error(`[ReactUIStep] Failed to open terminal: ${error}`);
    }
  };

  // Check disk usage while taking hardlinks into account
  // const checkDiskUsage = async () => {
  //   const cmd = isWindows
  //     ? `start cmd /k "dir ${OUTPUTS_PATH}"`
  //     : `open -a Terminal "du -sh ${OUTPUTS_PATH}"`;
  //   try {
  //     await os.execCommand(cmd);
  //   } catch (error) {
  //     console.error(`[ReactUIStep] Failed to check disk usage: ${error}`);
  //   }
  // };

  const openOutputs = async () => {
    const cmd = isWindows
      ? `explorer "${OUTPUTS_PATH}"`
      : `open "${OUTPUTS_PATH}"`;
    console.log(`[ReactUIStep] Opening outputs folder: ${cmd}`);
    try {
      await os.spawnProcess(cmd);
    } catch (error) {
      console.error(`[ReactUIStep] Failed to open outputs folder: ${error}`);
    }
  };

  const openInstallFolder = async () => {
    const cmd = isWindows ? `explorer "${WEBUI_ROOT}"` : `open "${WEBUI_ROOT}"`;
    console.log(`[ReactUIStep] Opening install folder: ${cmd}`);
    try {
      await os.spawnProcess(cmd);
    } catch (error) {
      console.error(`[ReactUIStep] Failed to open install folder: ${error}`);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      {/* Server launcher */}
      <CondaSpawner
        name="Start Gradio Server"
        command="python server.py --no-react"
        openUrl="http://localhost:7770"
        cwd={WEBUI_ROOT}
        onSuccess={() => {}}
      />
      <CondaSpawner
        name="Start React UI"
        command="npm --prefix react-ui start -- -p 3000"
        openUrl="http://localhost:3000"
        cwd={WEBUI_ROOT}
        onSuccess={() => {}}
      />
      <CondaSpawner
        name="Manage Extensions"
        command="python .\installer_scripts\tools\extension_manager.py"
        openUrl="http://localhost:7771"
        cwd={WEBUI_ROOT}
        onSuccess={() => {}}
      />
      <CondaSpawner
        name="Diagnostic"
        command={
          isWindows
            ? `installer_scripts\\tools\\diagnostic.bat`
            : `installer_scripts/tools/diagnostic.sh`
        }
        openUrl=""
        cwd={WEBUI_ROOT}
        onSuccess={() => {}}
      />

      {/* Quick-access actions */}
      {/* ISSUE - closing app does not close terminal. Then reopening the app does not work until it is closed */}
      <div className="border-t border-gray-200 dark:border-gray-800 pt-3">
        <p className="text-xs text-gray-500 dark:text-gray-400 mb-2 font-medium uppercase tracking-wide">
          Quick actions
        </p>
        <div className="flex flex-wrap gap-2">
          <ActionButton label="Open Terminal" onClick={openTerminal} />
          <ActionButton
            label="Open Install Folder"
            onClick={openInstallFolder}
          />
          {/* <ActionButton label="Manage Extensions" onClick={manageExtensions} /> */}
          <ActionButton label="Open Outputs" onClick={openOutputs} />
          {/* <ActionButton label="Check Disk Usage" onClick={checkDiskUsage} /> */}
        </div>
      </div>
    </div>
  );
}
