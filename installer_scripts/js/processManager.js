/**
 * Unified Process Manager
 *
 * This script manages the lifecycle of all TTS WebUI components:
 * 1. Gradio UI (Python backend)
 * 2. React UI (Next.js frontend)
 * 3. Update status page (Node.js server)
 *
 * All services are proxied through a single port with path-based routing:
 * - / -> React UI
 * - /gradio -> Gradio UI
 * - /update -> Update status page
 */

const path = require("path");
const { spawn } = require("child_process");
const {
  createProxyServer,
  waitForServer,
  CONFIG,
} = require("./unifiedProxy.js");
const { openBrowser } = require("./openBrowser.js");

// Process registry
const processes = new Map();

/**
 * Log with timestamp and prefix
 */
function log(level, message, ...args) {
  const timestamp = new Date().toISOString();
  console[level](`[${timestamp}] [ProcessManager] ${message}`, ...args);
}

/**
 * Starts a child process with output streaming
 * @param {string} name - Process identifier
 * @param {string} command - Command to run
 * @param {string[]} args - Command arguments
 * @param {object} options - spawn options
 * @returns {ChildProcess}
 */
function startProcess(name, command, args = [], options = {}) {
  log("info", `Starting ${name}: ${command} ${args.join(" ")}`);

  const proc = spawn(command, args, {
    stdio: ["pipe", "pipe", "pipe"],
    shell: true,
    cwd: options.cwd || process.cwd(),
    env: { ...process.env, ...options.env },
  });

  proc.stdout.on("data", (data) => {
    const lines = data.toString().split("\n");
    lines.forEach((line) => {
      if (line.trim()) {
        console.log(`[${name}] ${line}`);
      }
    });
  });

  proc.stderr.on("data", (data) => {
    const lines = data.toString().split("\n");
    lines.forEach((line) => {
      if (line.trim()) {
        console.error(`[${name}:err] ${line}`);
      }
    });
  });

  proc.on("close", (code) => {
    log("info", `${name} exited with code ${code}`);
    processes.delete(name);
  });

  proc.on("error", (err) => {
    log("error", `${name} error: ${err.message}`);
  });

  processes.set(name, { proc, name, command });
  return proc;
}

/**
 * Stops all managed processes gracefully
 */
function stopAllProcesses() {
  log("info", "Stopping all processes...");
  for (const [name, { proc }] of processes) {
    log("info", `Stopping ${name}...`);
    try {
      proc.kill("SIGTERM");
    } catch (err) {
      log("error", `Error stopping ${name}: ${err.message}`);
    }
  }
}

/**
 * Starts the Gradio backend (Python)
 */
function startGradioBackend() {
  const rootDir = path.resolve(__dirname, "../..");
  return startProcess(
    "gradio",
    "python",
    ["server.py", "--no-react"],
    { cwd: rootDir }
  );
}

/**
 * Starts the React UI (Next.js)
 */
function startReactUI() {
  const reactDir = path.resolve(__dirname, "../../react-ui");
  return startProcess(
    "react",
    "npm",
    ["start", "--", "-p", String(CONFIG.reactPort)],
    {
      cwd: reactDir,
      env: {
        GRADIO_BACKEND_AUTOMATIC: `http://127.0.0.1:${CONFIG.gradioPort}/`,
        PORT: String(CONFIG.reactPort),
      },
    }
  );
}

/**
 * Starts the update status server
 * Note: This is typically started by init_app.js during initialization.
 * If running standalone, we start it ourselves.
 */
function startUpdateServer() {
  // The update server is started by the startServer function in server.js
  // during initialization. For the unified proxy, we assume it's already running
  // or we start it ourselves with skipBrowser=true since we'll open the proxy URL.
  try {
    const { startServer } = require("./server.js");
    return startServer({ skipBrowser: true });
  } catch (err) {
    log("warn", "Could not start update server:", err.message);
    return null;
  }
}

/**
 * Main entry point
 */
async function main() {
  log("info", "=".repeat(60));
  log("info", "TTS WebUI - Unified Process Manager");
  log("info", "=".repeat(60));
  log("info", `Proxy Port: ${CONFIG.proxyPort}`);
  log("info", `Gradio Port: ${CONFIG.gradioPort}`);
  log("info", `React Port: ${CONFIG.reactPort}`);
  log("info", `Update Port: ${CONFIG.updatePort}`);
  log("info", "=".repeat(60));

  // Set up signal handlers
  process.on("SIGINT", () => {
    log("info", "Received SIGINT, shutting down...");
    stopAllProcesses();
    process.exit(0);
  });

  process.on("SIGTERM", () => {
    log("info", "Received SIGTERM, shutting down...");
    stopAllProcesses();
    process.exit(0);
  });

  // Check command line arguments
  const args = process.argv.slice(2);
  const skipGradio = args.includes("--no-gradio");
  const skipReact = args.includes("--no-react");
  const skipUpdate = args.includes("--no-update");
  const proxyOnly = args.includes("--proxy-only");

  // Start services unless in proxy-only mode
  if (!proxyOnly) {
    // Start Update Status Server first (if not skipped)
    if (!skipUpdate) {
      log("info", "Starting Update Status Server...");
      startUpdateServer();
      // Wait for it to be available
      await waitForServer(CONFIG.updateHost, CONFIG.updatePort, 5000, 500);
    }

    // Start Gradio Backend (if not skipped)
    if (!skipGradio) {
      log("info", "Starting Gradio Backend...");
      startGradioBackend();
      // Wait for it to be available
      log("info", `Waiting for Gradio backend at ${CONFIG.gradioHost}:${CONFIG.gradioPort}...`);
      const gradioReady = await waitForServer(
        CONFIG.gradioHost,
        CONFIG.gradioPort,
        120000,
        2000
      );
      if (gradioReady) {
        log("info", "Gradio backend is ready!");
      } else {
        log("warn", "Gradio backend did not become available in time, continuing anyway...");
      }
    }

    // Start React UI (if not skipped)
    if (!skipReact) {
      log("info", "Starting React UI...");
      startReactUI();
      // Wait for it to be available
      log("info", `Waiting for React UI at ${CONFIG.reactHost}:${CONFIG.reactPort}...`);
      const reactReady = await waitForServer(
        CONFIG.reactHost,
        CONFIG.reactPort,
        60000,
        2000
      );
      if (reactReady) {
        log("info", "React UI is ready!");
      } else {
        log("warn", "React UI did not become available in time, continuing anyway...");
      }
    }
  }

  // Start the unified proxy server
  log("info", "Starting Unified Proxy Server...");
  const proxyServer = createProxyServer();

  proxyServer.listen(CONFIG.proxyPort, () => {
    log("info", "=".repeat(60));
    log("info", "Unified Proxy Server is running!");
    log("info", `Access the application at: http://localhost:${CONFIG.proxyPort}`);
    log("info", "Routes:");
    log("info", `  http://localhost:${CONFIG.proxyPort}/           -> React UI`);
    log("info", `  http://localhost:${CONFIG.proxyPort}/gradio/    -> Gradio UI`);
    log("info", `  http://localhost:${CONFIG.proxyPort}/update/    -> Update Status`);
    log("info", "=".repeat(60));

    // Open browser
    if (!args.includes("--no-browser")) {
      openBrowser(`http://localhost:${CONFIG.proxyPort}`);
    }
  });

  proxyServer.on("error", (err) => {
    log("error", "Proxy server error:", err.message);
    if (err.code === "EADDRINUSE") {
      log("error", `Port ${CONFIG.proxyPort} is already in use. Please choose a different port.`);
      process.exit(1);
    }
  });

  return proxyServer;
}

// Export for use as a module
module.exports = {
  main,
  startProcess,
  stopAllProcesses,
  startGradioBackend,
  startReactUI,
  startUpdateServer,
};

// Run main if executed directly
if (require.main === module) {
  main().catch((err) => {
    log("error", "Failed to start process manager:", err);
    process.exit(1);
  });
}
