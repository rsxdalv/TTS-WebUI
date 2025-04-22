const fs = require("fs");
const { resolve } = require("path");
const { displayError, displayMessage } = require("./displayMessage.js");
const { processExit } = require("./processExit.js");
const { menu } = require("./menu.js");
const { $, $$, $sh } = require("./shell.js");
const { applyDatabaseConfig } = require("./applyDatabaseConfig.js");

const DEBUG_DRY_RUN = false;

const torchVersion = "2.6.0";
const cudaVersion = "12.4";
const cudaVersionTag = `cu124`;

const pythonVersion = `3.10.11`;
const pythonPackage = `python=${pythonVersion}`;

const ensurePythonVersion = async () => {
  try {
    displayMessage("Checking python version...");
    const version = await getPythonVersion();
    if (version !== `Python ${pythonVersion}`) {
      displayMessage(`Current python version is """${version}"""`);
      displayMessage(`Python version is not ${pythonVersion}. Reinstalling...`);
      await $(`conda install -y -k -c conda-forge ${pythonPackage}`);
    }
  } catch (error) {
    displayError("Failed to check/install python version");
  }

  async function getPythonVersion() {
    await $sh(`python --version > installer_scripts/.python_version`);
    return fs.readFileSync("installer_scripts/.python_version", "utf8").trim();
  }
};

const installDependencies = async (gpuchoice) => {
  try {
    if (gpuchoice === "NVIDIA GPU") {
      await $(
        `pip install -U torch==${torchVersion}+${cudaVersionTag} torchvision torchaudio --index-url https://download.pytorch.org/whl/${cudaVersionTag}`
      );

      await pip_install(
        `-U xformers==0.0.29.post3 --index-url https://download.pytorch.org/whl/${cudaVersionTag}`,
        "xformers",
        true
      );
    } else if (gpuchoice === "Apple M Series Chip") {
      await $(`pip install torch==${torchVersion} torchvision torchaudio`);
    } else if (gpuchoice === "CPU") {
      await $(
        `pip install torch==${torchVersion}+cpu torchvision torchaudio --index-url https://download.pytorch.org/whl/cpu`
      );
    } else if (gpuchoice === "AMD GPU (ROCM, Linux only)") {
      displayMessage(
        "ROCM is experimental and not well supported yet, installing..."
      );
      displayMessage("Linux only!");
      await $(
        `pip install torch==${torchVersion} torchvision torchaudio --index-url https://download.pytorch.org/whl/rocm6.0`
      );
    } else {
      displayMessage("Unsupported or cancelled. Exiting...");
      removeGPUChoice();
      processExit(1);
    }

    saveMajorVersion(majorVersion);
    displayMessage("  Successfully installed torch");
  } catch (error) {
    displayError(`Error during installation: ${error.message}`);
    throw error;
  }
};

const askForGPUChoice = () =>
  menu(
    [
      "NVIDIA GPU",
      "Apple M Series Chip",
      "CPU",
      "Cancel",
      "AMD GPU (ROCM, Linux only)",
      "Intel GPU (unsupported)",
      "Integrated GPU (unsupported)",
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

const dry_run_flag = DEBUG_DRY_RUN ? "--dry-run " : "";

async function pip_install_or_fail(
  requirements,
  name = "",
  pipFallback = false
) {
  displayMessage(`Installing ${name || requirements} dependencies...`);
  await $sh(
    `${
      pipFallback ? "pip" : "uv pip"
    } install ${dry_run_flag}${requirements} torch==${torchVersion}`
  );
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

// The first install is a temporary safeguard due to mysterious issues with uv
async function pip_install_all(fi = false) {
  if (readPipPackagesVersion() === newPipPackagesVersion)
    return displayMessage(
      "Dependencies are already up to date, skipping pip installs..."
    );

  const pip_install_all_choice = await menu(
    ["Yes", "No"],
    `Attempt single pip install of all dependencies (potentially faster)?
    (use arrow keys to move, enter to select)`
  );

  if (pip_install_all_choice === "Yes") {
    try {
      displayMessage("Attempting single pip install of all dependencies...");

      await pip_install_or_fail(
        "-r requirements.txt -r requirements_bark_hubert_quantizer.txt -r requirements_rvc.txt -r requirements_audiocraft.txt -r requirements_styletts2.txt -r requirements_vall_e.txt -r requirements_maha_tts.txt -r requirements_stable_audio.txt hydra-core==1.3.2 nvidia-ml-py",
        "All dependencies",
        // first_install
        true
      );
      savePipPackagesVersion(newPipPackagesVersion);
      displayMessage("");
      return;
    } catch (error) {
      displayMessage(
        "Failed to install all dependencies, falling back to individual installs..."
      );
    }
  }

  displayMessage("Updating dependencies...");
  // pip_install_all(false); // potential speed optimization

  try {
    await pip_install_or_fail(
      "-r requirements.txt",
      "Core Packages, Bark, Tortoise",
      fi
    );
  } catch (error) {
    displayMessage("Failed to install core packages");
    displayMessage("Please check the log file for more information");
    displayMessage("Exiting...");
    throw error;
  }
  await pip_install("-r requirements_bark_hubert_quantizer.txt", "Bark Voice Clone", fi); // prettier-ignore
  await pip_install("-r requirements_rvc.txt", "RVC", fi);
  await pip_install("-r requirements_audiocraft.txt", "Audiocraft", fi);
  await pip_install("-r requirements_styletts2.txt", "StyleTTS", fi);
  await pip_install("-r requirements_vall_e.txt", "Vall-E-X", fi);
  await pip_install("-r requirements_maha_tts.txt", "Maha TTS", fi);
  await pip_install("-r requirements_stable_audio.txt", "Stable Audio", true);
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

const FORCE_REINSTALL = process.env.FORCE_REINSTALL ? true : false;

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
  if (readMajorVersion() === majorVersion && !FORCE_REINSTALL) {
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
      "../react-ui/src/extensions/package.json".split("/")
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
