const fs = require("fs");
const { resolve } = require("path");
const { $, $sh } = require("./js/shell");
const { displayError, displayMessage } = require("./js/displayMessage.js");
const { processExit } = require("./js/processExit.js");
const { startServer } = require("./js/server.js");
const { updateState } = require("./js/installerState.js");

const checkConda = async () => {
  try {
    updateState({ status: "checking_dependencies", currentStep: 1 });

    displayMessage("Checking conda installation...");

    displayMessage("");
    // verify conda paths
    $sh("conda info --envs");

    // expect
    // # conda environments:
    // #
    // base                 * .. ..\tts-webui-main\installer_files\env
    $sh("node --version");
    $sh("python --version");
    $sh("pip --version");

    await $sh("conda --version");

    updateState({ condaReady: true });
  } catch (error) {
    updateState({ status: "error", lastError: "Conda installation not found" });

    displayError(
      "Please install conda from https://docs.conda.io/projects/conda/en/latest/user-guide/install/index.html"
    );
    processExit(1);
  }
};

const getGitCommitHash = () =>
  fs.readFileSync("./.git/refs/heads/main", "utf8");

const AppliedGitVersion = {
  file: resolve(__dirname, ".git_version"),
  get: () =>
    fs.existsSync(AppliedGitVersion.file)
      ? fs.readFileSync(AppliedGitVersion.file, "utf8")
      : null,
  save: () => fs.writeFileSync(AppliedGitVersion.file, getGitCommitHash()),
};

const syncRepo = async () => {
  updateState({ status: "updating_repo", currentStep: 2 });

  if (!fs.existsSync(".git")) {
    displayMessage("CRITICAL: No git repository found, skipping update...");
    throw new Error("No git repository found");
  }

  displayMessage("Pulling updates from tts-webui");
  try {
    await $sh("git pull");
    const newHash = getGitCommitHash();
    updateState({ gitHash: newHash });
    if (AppliedGitVersion.get() === newHash) {
      displayMessage("Current git version: " + newHash);
      displayMessage("No updates found, skipping...");
      return false;
    }
    return true;
  } catch (error) {
    updateState({ lastError: "Problem pulling updates from git" });
    displayMessage(
      "There was a problem while pulling updates. Warning: missing updates might cause issues. Continuing..."
    );
    displayMessage("Error:");
    displayError(error);
    return false;
  }
};

async function main() {
  startServer();

  const version = "0.2.0";
  displayMessage("\n\nStarting init app (version: " + version + ")...\n\n");

  updateState({ status: "initializing", currentStep: 0, totalSteps: 5 });

  try {
    await checkConda();
    const isUpdated = await syncRepo();
    if (!isUpdated) {
      updateState({ status: "ready", currentStep: 5, totalSteps: 5 });
      $sh("pip show torch torchvision torchaudio");
      return true;
    }

    updateState({ status: "installing", currentStep: 3 });
    const {
      initializeApp,
      setupReactUI,
      repairTorch,
    } = require("./js/initializeApp.js");
    await initializeApp();
    updateState({ torchReady: true, currentStep: 4 });
    await setupReactUI();
    updateState({ reactUIReady: true, currentStep: 5 });
    await repairTorch();

    AppliedGitVersion.save();
    updateState({ status: "ready", currentStep: 5, totalSteps: 5 });
    return true;
  } catch (error) {
    updateState({ status: "error", lastError: error.message });
    displayError(error.message);
    return false;
  }
}

main().then((result) => {
  displayMessage("\n\nFinished init app.\n");
  return setTimeout(() => processExit(result ? 0 : 1), 100);
});
