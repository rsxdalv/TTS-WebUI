const http = require("http");
const fs = require("fs");
const path = require("path");
const { getMessages, getLatestStateUpdate } = require("./webConsole");
const { getState } = require("./installerState");
const { openBrowser } = require("./openBrowser");

// Create a buffer to store output for new connections
let outputBuffer = [];
let clients = [];

// Function to start the unified server (combines log streaming and polling)
function startServer() {
  // Hook into stdout to capture output
  const originalStdoutWrite = process.stdout.write;
  process.stdout.write = function (chunk, encoding, callback) {
    // Convert chunk to string
    const data = chunk.toString();

    // Store in our buffer for new connections
    outputBuffer.push(data);
    // Keep the buffer at a reasonable size
    if (outputBuffer.length > 1000) {
      outputBuffer.shift();
    }

    // Send to all connected clients
    clients.forEach((client, index) => {
      try {
        client.write(data);
      } catch (error) {
        // If we can't write to the client, remove it from our list
        clients.splice(index, 1);
      }
    });

    // Pass through to original stdout
    return originalStdoutWrite.apply(process.stdout, arguments);
  };

  // Route handler functions
  function handleStreamLog(req, res) {
    res.writeHead(200, {
      "Content-Type": "text/plain",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    });

    // Send the current buffer first
    outputBuffer.forEach((chunk) => {
      res.write(chunk);
    });

    // Add this client to our list
    clients.push(res);

    // Handle client disconnect
    req.on("close", function () {
      const index = clients.indexOf(res);
      if (index !== -1) {
        clients.splice(index, 1);
      }
    });

    // Close the connection after 5 seconds
    setTimeout(() => {
      res.end();
      const index = clients.indexOf(res);
      if (index !== -1) {
        clients.splice(index, 1);
      }
    }, 5000);
  }

  function handlePoll(_, res) {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(
      JSON.stringify({
        messages: getMessages(),
        state: getState(),
        stateUpdate: getLatestStateUpdate(),
      })
    );
  }

  function handleIndex(_, res) {
    const filePath = path.join(__dirname, "index.html");
    fs.readFile(filePath, (err, data) => {
      if (err) {
        res.writeHead(500, { "Content-Type": "text/plain" });
        res.end("Server Error");
      } else {
        res.writeHead(200, { "Content-Type": "text/html" });
        res.end(data);
      }
    });
  }

  function handleNotFound(_, res) {
    res.writeHead(404, { "Content-Type": "text/plain" });
    res.end("404 Not Found");
  }

  // Helper to add CORS headers for cross-origin requests from vite dev server
  function setCorsHeaders(res) {
    res.setHeader("Access-Control-Allow-Origin", "*");
    res.setHeader("Access-Control-Allow-Methods", "GET, OPTIONS");
    res.setHeader("Access-Control-Allow-Headers", "Content-Type");
  }

  // Function to handle routing
  function handleRouting(req, res) {
    // Handle preflight OPTIONS requests
    if (req.method === "OPTIONS") {
      setCorsHeaders(res);
      res.writeHead(204);
      res.end();
      return;
    }

    setCorsHeaders(res);

    switch(req.url) {
      case "/stream-log":
        handleStreamLog(req, res);
        break;
      case "/poll":
        handlePoll(req, res);
        break;
      case "/":
      case "/index.html":
        handleIndex(req, res);
        break;
      default:
        handleNotFound(req, res);
        break;
    }
  }

  // Create HTTP server that routes to the appropriate handler
  const server = http
    .createServer(handleRouting)
    .listen(7771);

  const serverUrl = "http://localhost:7771";
  console.log("Unified server started on port 7771");
  console.log(`Web console and log viewer available at ${serverUrl}`);

  if (process.argv.includes("--silent")) {
    console.log("Silent mode enabled, not opening browser.");
  } else {
    openBrowser(serverUrl);
  }

  return server;
}

// For backward compatibility
function startLogStreamingServer() {
  return startServer();
}

// Export the functions
module.exports = {
  startServer,
  startLogStreamingServer, // for backward compatibility
};
