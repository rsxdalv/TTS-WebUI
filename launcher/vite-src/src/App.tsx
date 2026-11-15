import "./App.css";
import { WEBUI_ROOT } from "./config";
import { DarkThemeProvider, ThemeToggle } from "./DarkThemeProvider";
import { ProcessSpawner } from "./ProcessSpawner";
import { CondaSpawner } from "./CondaSpawner";
import { ReleaseDownloader } from "./ReleaseDownloader";
import { ReactUIStep } from "./ReactUIStep";
import { GPUSelectStep } from "./GPUSelectStep";
import { PreflightChecksStep } from "./PreflightChecksStep";
import {
  WaterfallInstaller,
  WaterfallStepDef,
  pathExistsInRoot,
} from "./WaterfallInstaller";
import { LogViewer } from "./LogViewer";
import { isGitUpdated } from "./isGitUpdated";

// ---------------------------------------------------------------------------
// Installation waterfall steps
// ---------------------------------------------------------------------------

const INSTALLATION_STEPS: WaterfallStepDef[] = [
  {
    id: "download",
    title: "Download & Extract Release",
    description: "Fetch the TTS WebUI installer package from GitHub",
    condition: () => pathExistsInRoot("installer_scripts"),
    render: (onComplete) => (
      <ReleaseDownloader
        name="TTS-WebUI Release v0.0.0"
        downloadUrl="https://github.com/rsxdalv/TTS-WebUI/releases/download/v0.0.0/tts-webui-installer.zip"
        zipName="tts-webui-installer.zip"
        cwd={WEBUI_ROOT}
        onSuccess={onComplete}
      />
    ),
  },
  {
    id: "preflight",
    title: "System Pre-flight Checks",
    description:
      "Verify system requirements: filesystem, paths, compiler tools",
    condition: () => pathExistsInRoot("installer_scripts/.preflight_ok"),
    render: (onComplete) => <PreflightChecksStep onComplete={onComplete} />,
  },
  {
    id: "mamba",
    title: "Init Mamba (Conda) Environment",
    description: "Download and set up the micromamba package manager",
    condition: () => pathExistsInRoot("installer_files/mamba"),
    render: (onComplete) => (
      <ProcessSpawner
        name="Init Mamba"
        command=""
        command_windows=".\\installer_scripts\\init_mamba.bat"
        command_mac="./installer_scripts/init_mamba.sh"
        command_linux="./installer_scripts/init_mamba.sh"
        cwd={WEBUI_ROOT}
        onSuccess={onComplete}
      />
    ),
  },
  {
    id: "gpu",
    title: "Select GPU / Accelerator",
    description: "Choose your hardware for PyTorch installation",
    condition: () => pathExistsInRoot("installer_scripts/.gpu"),
    render: (onComplete) => <GPUSelectStep onComplete={onComplete} />,
  },
  {
    id: "install",
    title: "Install WebUI",
    description: "Create the Python environment and install all dependencies",
    condition: () => pathExistsInRoot("installer_scripts/.git_version"),
    render: (onComplete) => (
      <div className="flex flex-col gap-6">
        <CondaSpawner
          name="Installer"
          command="node ./installer_scripts/init_app.js --silent"
          openUrl="http://localhost:7771"
          cwd={WEBUI_ROOT}
          onSuccess={onComplete}
        />
        <LogViewer />
      </div>
    ),
  },
  {
    id: "update",
    title: "Update WebUI",
    description: "Create the Python environment and install all dependencies",
    condition: isGitUpdated,
    render: (onComplete) => (
      <div className="flex flex-col gap-6">
        <CondaSpawner
          name="Update"
          command="node ./installer_scripts/init_app.js --silent"
          openUrl="http://localhost:7771"
          cwd={WEBUI_ROOT}
          onSuccess={onComplete}
        />
        <button
          className="px-4 py-2 bg-gray-200 dark:bg-gray-700 rounded-lg hover:bg-gray-300 dark:hover:bg-gray-600"
          onClick={onComplete}
        >
          Skip Update
        </button>
        <LogViewer />
      </div>
    ),
  },
  {
    id: "react-ui",
    title: "Start React UI",
    description: "Launch the web interface on http://localhost:3000",
    // condition: () => httpReachable("http://localhost:3000"),
    condition: async () => false,
    render: (onComplete) => <ReactUIStep onComplete={onComplete} />,
  },
];

// ---------------------------------------------------------------------------
// App
// ---------------------------------------------------------------------------

function App() {
  return (
    <DarkThemeProvider>
      <div className="min-h-screen bg-white dark:bg-gray-950">
        <div className="min-h-screen text-gray-900 dark:text-gray-100 p-8 max-w-2xl mx-auto">
          <div className="flex items-center justify-between mb-6">
            <h1 className="text-2xl font-bold">TTS WebUI Igniter</h1>
            <ThemeToggle />
          </div>

          <WaterfallInstaller steps={INSTALLATION_STEPS} />
        </div>
      </div>
    </DarkThemeProvider>
  );
}

export default App;
