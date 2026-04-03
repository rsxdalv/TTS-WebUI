// filepath: c:\Users\rob\Desktop\tts-webui-main\installer_scripts\tests\setup_react_ui.js
const { displayMessage, displayError } = require("../js/displayMessage.js");
const { processExit } = require("../js/processExit.js");
const { setupReactUI } = require("../js/initializeApp.js");

async function main() {
  displayMessage("\n\nSetting up React UI...\n\n");
  
  try {
    await setupReactUI();
    displayMessage("\n\nReact UI setup completed successfully.\n");
    processExit(0);
  } catch (error) {
    displayError("Failed to setup React UI: " + error.message);
    processExit(1);
  }
}

main();