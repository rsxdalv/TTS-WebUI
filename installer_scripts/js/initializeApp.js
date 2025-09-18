const fs = require("fs");
const { resolve } = require("path");
const { displayError, displayMessage } = require("./displayMessage.js");
const { processExit } = require("./processExit.js");
const { menu } = require("./menu.js");
const { $, $$, $sh } = require("./shell.js");
const { applyDatabaseConfig } = require("./applyDatabaseConfig.js");

const torchVersion = "2.7.0"; // 2.7.1 has no xformers
const cudaVersion = "12.8";
const cudaVersionTag = `cu128`;
const torch = `torch==${torchVersion}`;

const pythonVersion = `3.10.11`; // 3.11 has limited support, 3.12 is not yet supported

const ensurePythonVersion = async () => {
  try {
    displayMessage("Checking python version...");
    const version = await getPythonVersion();
    if (version !== `Python ${pythonVersion}`) {
      displayMessage(`Current python version is """${version}"""`);
      displayMessage(`Python version is not ${pythonVersion}. Reinstalling...`);
      const pythonPackage = `python=${pythonVersion}`;
      const conda = "micromamba";

      await $(`${conda} install -y -k -c conda-forge ${pythonPackage}`);
    }
  } catch (error) {
    displayError("Failed to check/install python version");
  }

  async function getPythonVersion() {
    await $sh(`python --version > installer_scripts/.python_version`);
    return fs.readFileSync("installer_scripts/.python_version", "utf8").trim();
  }
};

const rocmVersionTag = {
  "2.6.0": "rocm6.2.4",
  "2.7.0": "rocm6.3",
  "2.7.1": "rocm6.3",
};

const PyTorchChoice = {
  NVIDIA: "NVIDIA GPU",
  CUSTOM: "Custom (Manual torch install)",
  APPLE_M_SERIES: "Apple M Series Chip",
  AMD_ROCM: "AMD GPU (ROCM, Linux only)",
  INTEL_XPU: "Intel GPU (XPU)",
  CPU: "CPU",
  CANCEL: "Cancel",
  INTEGRATED_UNSUPPORTED: "Integrated GPU (unsupported)",
};

const installDependencies = async (gpuchoice) => {
  try {
    if (gpuchoice === PyTorchChoice.NVIDIA) {
      await $(
        `pip install -U ${torch}+${cudaVersionTag} torchvision torchaudio xformers --index-url https://download.pytorch.org/whl/${cudaVersionTag}`
      );
      // add torchao
      // pip install --dry-run torchao --index-url https://download.pytorch.org/whl/cu124
      // default version is already for 12.4 and has newer features
      // pip install --dry-run torchao
    } else if (gpuchoice === PyTorchChoice.CUSTOM) {
      displayMessage("Please install torch manually");
      displayMessage(
        `For example with CUDA ${cudaVersion} use: pip install ${torch}+${cudaVersionTag} torchvision torchaudio --index-url https://download.pytorch.org/whl/${cudaVersionTag}`
      );
    } else if (gpuchoice === PyTorchChoice.APPLE_M_SERIES) {
      await $(`pip install ${torch} torchvision torchaudio`);
    } else if (gpuchoice === PyTorchChoice.CPU) {
      await $(
        `pip install ${torch}+cpu torchvision torchaudio --index-url https://download.pytorch.org/whl/cpu`
      );
    } else if (gpuchoice === PyTorchChoice.AMD_ROCM) {
      await $(
        `pip install ${torch} torchvision torchaudio xformers --index-url https://download.pytorch.org/whl/${rocmVersionTag[torchVersion]}`
      );
    } else if (gpuchoice === PyTorchChoice.INTEL_XPU) {
      await $(
        `pip install ${torch} torchvision torchaudio --index-url https://download.pytorch.org/whl/test/xpu`
      );
    } else {
      displayMessage("Unsupported or cancelled. Exiting...");
      removeGPUChoice();
      processExit(1);
    }

    saveMajorVersion(majorVersion);
    displayMessage(
      `  Successfully installed ${torch} with CUDA ${cudaVersion} support`
    );
  } catch (error) {
    displayError(`Error during installation: ${error.message}`);
    throw error;
  }
};

const askForGPUChoice = () =>
  menu(
    [
      PyTorchChoice.NVIDIA,
      PyTorchChoice.APPLE_M_SERIES,
      PyTorchChoice.CPU,
      PyTorchChoice.AMD_ROCM,
      PyTorchChoice.INTEL_XPU,
      PyTorchChoice.CANCEL,
      PyTorchChoice.CUSTOM,
      PyTorchChoice.INTEGRATED_UNSUPPORTED,
    ],
    `
These are not yet automatically supported: AMD GPU, Intel GPU, Integrated GPU.
Select the device (GPU/CPU) you are using to run the application:
(use arrow keys to move, enter to select)
  `
  );

const getInstallerFilesPath = (...files) => resolve(__dirname, "..", ...files);

const gpuFile = getInstallerFilesPath(".gpu");
const majorVersionFile = getInstallerFilesPath(".major_version");
const pipPackagesFile = getInstallerFilesPath(".pip_packages");
const majorVersion = "5";

const versions = JSON.parse(
  fs.readFileSync(getInstallerFilesPath("versions.json"))
);
const newPipPackagesVersion = String(versions.pip_packages);

const readGeneric = (file) => {
  if (fs.existsSync(file)) {
    return fs.readFileSync(file, "utf8");
  }
  return -1;
};

const saveGeneric = (file, data) => fs.writeFileSync(file, data.toString());

const readMajorVersion = () => readGeneric(majorVersionFile);
const saveMajorVersion = (data) => saveGeneric(majorVersionFile, data);
const readPipPackagesVersion = () => readGeneric(pipPackagesFile);
const savePipPackagesVersion = (data) => saveGeneric(pipPackagesFile, data);
const readGPUChoice = () => readGeneric(gpuFile);
const saveGPUChoice = (data) => saveGeneric(gpuFile, data);

const removeGPUChoice = () => {
  if (fs.existsSync(gpuFile)) fs.unlinkSync(gpuFile);
};

async function pip_install_or_fail(
  requirements,
  name = "",
  pipFallback = false
) {
  displayMessage(`Installing ${name || requirements} dependencies...`);
  const pip = pipFallback ? "pip" : "uv pip";
  await $sh(`${pip} install ${requirements} ${torch}`);
  displayMessage(
    `Successfully installed ${name || requirements} dependencies\n`
  );
}

async function pip_install(requirements, name = "", pipFallback = false) {
  try {
    await pip_install_or_fail(requirements, name, pipFallback);
  } catch (error) {
    displayMessage(`Failed to install ${name || requirements} dependencies\n`);
  }
}

const extensions = [
  {
    name: "Bark Voice Clone",
    package:
      "git+https://github.com/rsxdalv/tts_webui_extension.bark_voice_clone@main",
  },
  {
    name: "RVC",
    package: "git+https://github.com/rsxdalv/tts_webui_extension.rvc@main",
  },
  {
    name: "Audiocraft",
    package:
      "git+https://github.com/rsxdalv/tts_webui_extension.audiocraft@main",
  },
  {
    name: "StyleTTS",
    package:
      "git+https://github.com/rsxdalv/tts_webui_extension.styletts2@main",
  },
  {
    name: "Vall-E-X",
    package: "git+https://github.com/rsxdalv/tts_webui_extension.vall_e_x@main",
  },
  // {
  //   name: "Maha TTS",
  //   package: "git+https://github.com/rsxdalv/tts_webui_extension.maha_tts@main",
  // },
  {
    name: "Stable Audio",
    package:
      "git+https://github.com/rsxdalv/tts_webui_extension.stable_audio@main",
  },
];

// The first install is a temporary safeguard due to mysterious issues with uv
async function pip_install_all(fi = false) {
  if (readPipPackagesVersion() === newPipPackagesVersion)
    return displayMessage(
      "Dependencies are already up to date, skipping pip installs..."
    );

  async function single_install() {
    displayMessage("Attempting single pip install of all dependencies...");

    await pip_install_or_fail(
      "-r requirements.txt " +
        extensions.map((ext) => ext.package).join(" ") +
        " hydra-core==1.3.2 nvidia-ml-py",
      "All dependencies",
      true
    );
    savePipPackagesVersion(newPipPackagesVersion);
    displayMessage("");
  }

  try {
    await single_install();
    return;
  } catch (error) {
    displayMessage(
      "Failed to install all dependencies, falling back to individual installs..."
    );
  }

  displayMessage("Updating dependencies...");

  try {
    await pip_install_or_fail("-r requirements.txt", "Core Packages", fi);
  } catch (error) {
    displayMessage("Failed to install core packages");
    displayMessage("Please check the log file for more information");
    displayMessage("Exiting...");
    throw error;
  }
  for (const ext of extensions) {
    await pip_install(ext.package, ext.name, fi);
  }
  await pip_install("hydra-core==1.3.2", "hydra-core fix due to fairseq", fi); // reinstall hydra-core==1.3.2 because of fairseq
  await pip_install("nvidia-ml-py", "nvidia-ml-py", fi);
  savePipPackagesVersion(newPipPackagesVersion);
  displayMessage("");
}

const checkIfTorchInstalled = async () => {
  try {
    await $$([
      "python",
      "-c",
      'import importlib.util; import sys; package_name = "torch"; spec = importlib.util.find_spec(package_name); sys.exit(0) if spec else sys.exit(1)',
    ]);
    return true;
  } catch (error) {
    return false;
  }
};

const getGPUChoice = async () => {
  if (fs.existsSync(gpuFile)) {
    const gpuchoice = readGPUChoice();
    displayMessage(`  Using saved GPU choice: ${gpuchoice}`);
    return gpuchoice;
  } else {
    const gpuchoice = await askForGPUChoice();
    displayMessage(`  You selected: ${gpuchoice}`);
    saveGPUChoice(gpuchoice);
    return gpuchoice;
  }
};

async function applyCondaConfig() {
  displayMessage("Applying conda config...");
  displayMessage("  Checking if Torch is installed...");
  if (readMajorVersion() === majorVersion) {
    if (await checkIfTorchInstalled()) {
      displayMessage("  Torch is already installed. Skipping installation...");
      await pip_install_all();
      return;
    } else {
      displayMessage("  Torch is not installed. Starting installation...\n");
    }
  } else {
    displayMessage(
      "  Major version update detected. Upgrading base environment"
    );
  }

  const gpuchoice = await getGPUChoice();
  await installDependencies(gpuchoice);
  await pip_install_all(true); // approximate first install
}

const extensionsToInstall = [
  "bark",
  "bark_voice_clone",
  "rvc",
  "audiocraft",
  "styletts2",
  "vall_e",
  "maha_tts",
  "stable_audio",
];

async function chooseExtensions() {
  displayMessage("Choose extensions to install...");
}

exports.initializeApp = async () => {
  displayMessage("Ensuring that python has the correct version...");
  await ensurePythonVersion();
  displayMessage("");
  await applyCondaConfig();
  displayMessage("");
  await chooseExtensions();
  displayMessage("");
  try {
    await applyDatabaseConfig();
    displayMessage("");
  } catch (error) {
    displayError("Failed to apply database config");
  }
};

const checkIfTorchHasCuda = async () => {
  try {
    displayMessage("Checking if torch has CUDA...");
    await $$([
      "python",
      "-c",
      "import torch; exit(0 if torch.cuda.is_available() else 1)",
    ]);
    return true;
  } catch (error) {
    return false;
  }
};

exports.repairTorch = async () => {
  const gpuChoice = readGPUChoice();
  $sh("pip show torch torchvision torchaudio");
  if (!checkIfTorchHasCuda() && gpuChoice === "NVIDIA GPU") {
    displayMessage("Backend is NVIDIA GPU, fixing PyTorch");
    try {
      await installDependencies(gpuChoice);
    } catch (error) {
      displayError("Failed to fix torch");
    }
  }
};

function setupReactUIExtensions() {
  try {
    displayMessage("Initializing extensions...");
    const packageJSONpath = getInstallerFilesPath(
      "../react-ui/src/extensions/package.json"
    );

    if (!fs.existsSync(packageJSONpath)) {
      fs.writeFileSync(packageJSONpath, "{}");
    }
    // $sh("cd react-ui/src/extensions && npm install");
    // displayMessage("Successfully installed extensions");
  } catch (error) {
    displayMessage("Failed to install extensions");
    throw error;
  }
}

exports.setupReactUI = async () => {
  try {
    setupReactUIExtensions();
    if (!fs.existsSync("outputs")) fs.mkdirSync("outputs");
    if (!fs.existsSync("favorites")) fs.mkdirSync("favorites");
    displayMessage("Installing node_modules...");
    await $sh("cd react-ui && npm install");
    displayMessage("Successfully installed node_modules");
    displayMessage("Building react-ui...");
    await $sh("cd react-ui && npm run build");
    displayMessage("Successfully built react-ui");
  } catch (error) {
    displayMessage("Failed to install node_modules or build react-ui");
    throw error;
  }
};
