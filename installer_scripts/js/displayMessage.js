const { sendToWebConsole } = require('./webConsole');

const displayMessage = async (...message) => {
  console.log(...message);
  sendToWebConsole(message.join(" "));
};
exports.displayMessage = displayMessage;

const displayError = async (...message) => {
  console.error(...message);
  sendToWebConsole(message.join(" "));
};
exports.displayError = displayError;
