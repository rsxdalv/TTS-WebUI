const { exec } = require("child_process");

// Function to open the browser
function openBrowser(url) {
  // Determine the command based on the platform
  let command;
  switch (process.platform) {
    case "win32":
      command = `start ${url}`;
      break;
    case "darwin":
      command = `open ${url}`;
      break;
    default:
      command = `xdg-open ${url}`;
      break;
  }

  // Execute the command
  exec(command, (error) => {
    if (error) {
      console.error(`Failed to open browser: ${error.message}`);
    }
  });
}
exports.openBrowser = openBrowser;
