/**
 * Unified Node.js Proxy and Process Manager
 *
 * This module provides a unified entry point that:
 * 1. Manages child processes for Gradio UI, React UI, and Update status page
 * 2. Proxies all requests through a single host with different paths:
 *    - /gradio/* -> Gradio backend (port 7770)
 *    - /update/* -> Update status page (port 7771)
 *    - /* -> React UI (port 3000)
 *
 * Uses only native Node.js modules (no npm packages required).
 */

const http = require("http");
const net = require("net");
const { spawn } = require("child_process");
const path = require("path");
const url = require("url");

// Configuration
const CONFIG = {
  proxyPort: parseInt(process.env.PROXY_PORT, 10) || 7860,
  gradioPort: parseInt(process.env.GRADIO_PORT, 10) || 7770,
  reactPort: parseInt(process.env.REACT_PORT, 10) || 3000,
  updatePort: parseInt(process.env.UPDATE_PORT, 10) || 7771,
  gradioHost: process.env.GRADIO_HOST || "127.0.0.1",
  reactHost: process.env.REACT_HOST || "127.0.0.1",
  updateHost: process.env.UPDATE_HOST || "127.0.0.1",
};

// Track managed processes
const managedProcesses = new Map();

// Logging utility
function log(level, message, ...args) {
  const timestamp = new Date().toISOString();
  console[level](`[${timestamp}] [UnifiedProxy] ${message}`, ...args);
}

/**
 * Starts a child process and manages its lifecycle
 * @param {string} name - Process name for identification
 * @param {string} command - Command to run
 * @param {string[]} args - Arguments for the command
 * @param {object} options - spawn options
 * @returns {ChildProcess}
 */
function startProcess(name, command, args, options = {}) {
  log("info", `Starting process: ${name} (${command} ${args.join(" ")})`);

  const proc = spawn(command, args, {
    stdio: ["pipe", "pipe", "pipe"],
    shell: true,
    ...options,
  });

  proc.stdout.on("data", (data) => {
    process.stdout.write(`[${name}] ${data}`);
  });

  proc.stderr.on("data", (data) => {
    process.stderr.write(`[${name}:err] ${data}`);
  });

  proc.on("close", (code) => {
    log("info", `Process ${name} exited with code ${code}`);
    managedProcesses.delete(name);
  });

  proc.on("error", (err) => {
    log("error", `Process ${name} error:`, err.message);
  });

  managedProcesses.set(name, proc);
  return proc;
}

/**
 * Stops all managed processes gracefully
 */
function stopAllProcesses() {
  log("info", "Stopping all managed processes...");
  for (const [name, proc] of managedProcesses) {
    log("info", `Stopping ${name}...`);
    proc.kill("SIGTERM");
  }
}

/**
 * Rewrites paths for proxied requests
 * @param {string} originalPath - The original request path
 * @param {string} prefix - The prefix to strip (e.g., '/gradio')
 * @returns {string} - The rewritten path
 */
function rewritePath(originalPath, prefix) {
  // Remove the prefix from the path
  let newPath = originalPath.slice(prefix.length);
  // Ensure path starts with /
  if (!newPath.startsWith("/")) {
    newPath = "/" + newPath;
  }
  return newPath;
}

/**
 * Proxies an HTTP request to a backend server
 * @param {http.IncomingMessage} req - Incoming request
 * @param {http.ServerResponse} res - Server response
 * @param {string} targetHost - Target host
 * @param {number} targetPort - Target port
 * @param {string} pathPrefix - Path prefix to strip (optional)
 */
function proxyRequest(req, res, targetHost, targetPort, pathPrefix = "") {
  // Rewrite the path if a prefix is provided
  let targetPath = req.url;
  if (pathPrefix && req.url.startsWith(pathPrefix)) {
    targetPath = rewritePath(req.url, pathPrefix);
  }

  // Build proxy request options
  const proxyOptions = {
    hostname: targetHost,
    port: targetPort,
    path: targetPath,
    method: req.method,
    headers: {
      ...req.headers,
      host: `${targetHost}:${targetPort}`,
    },
  };

  const proxyReq = http.request(proxyOptions, (proxyRes) => {
    // Rewrite Location headers for redirects
    if (proxyRes.headers.location) {
      // Handle relative and absolute redirects
      const location = proxyRes.headers.location;
      if (pathPrefix && !location.startsWith(pathPrefix)) {
        if (location.startsWith("/")) {
          proxyRes.headers.location = pathPrefix + location;
        }
      }
    }

    res.writeHead(proxyRes.statusCode, proxyRes.headers);
    proxyRes.pipe(res);
  });

  proxyReq.on("error", (err) => {
    log("error", `Proxy error to ${targetHost}:${targetPort}:`, err.message);
    if (!res.headersSent) {
      res.writeHead(502, { "Content-Type": "text/plain" });
      res.end(`Bad Gateway: Unable to reach backend at ${targetHost}:${targetPort}`);
    }
  });

  req.pipe(proxyReq);
}

/**
 * Handles WebSocket upgrade requests
 * @param {http.IncomingMessage} req - Incoming request
 * @param {net.Socket} socket - Client socket
 * @param {Buffer} head - First packet of the upgraded stream
 * @param {string} targetHost - Target host
 * @param {number} targetPort - Target port
 * @param {string} pathPrefix - Path prefix to strip (optional)
 */
function proxyWebSocket(req, socket, head, targetHost, targetPort, pathPrefix = "") {
  // Rewrite the path if a prefix is provided
  let targetPath = req.url;
  if (pathPrefix && req.url.startsWith(pathPrefix)) {
    targetPath = rewritePath(req.url, pathPrefix);
  }

  const serverSocket = net.connect(targetPort, targetHost, () => {
    // Build the HTTP upgrade request
    const headers = Object.keys(req.headers)
      .map((key) => `${key}: ${req.headers[key]}`)
      .join("\r\n");

    const upgradeRequest =
      `${req.method} ${targetPath} HTTP/${req.httpVersion}\r\n` +
      `host: ${targetHost}:${targetPort}\r\n` +
      `${headers}\r\n` +
      "\r\n";

    serverSocket.write(upgradeRequest);
    if (head && head.length) {
      serverSocket.write(head);
    }

    // Pipe data between client and server
    socket.pipe(serverSocket);
    serverSocket.pipe(socket);
  });

  serverSocket.on("error", (err) => {
    log("error", `WebSocket proxy error:`, err.message);
    socket.end();
  });

  socket.on("error", (err) => {
    log("error", `Client socket error:`, err.message);
    serverSocket.end();
  });
}

/**
 * Determines which backend to route to based on the request path
 * @param {string} pathname - The request pathname
 * @returns {{ host: string, port: number, prefix: string }}
 */
function getBackend(pathname) {
  if (pathname.startsWith("/gradio")) {
    return {
      host: CONFIG.gradioHost,
      port: CONFIG.gradioPort,
      prefix: "/gradio",
    };
  }
  if (pathname.startsWith("/update")) {
    return {
      host: CONFIG.updateHost,
      port: CONFIG.updatePort,
      prefix: "/update",
    };
  }
  // Default to React UI
  return {
    host: CONFIG.reactHost,
    port: CONFIG.reactPort,
    prefix: "",
  };
}

/**
 * Creates and starts the unified proxy server
 * @returns {http.Server}
 */
function createProxyServer() {
  const server = http.createServer((req, res) => {
    const parsedUrl = url.parse(req.url);
    const backend = getBackend(parsedUrl.pathname);

    log("info", `${req.method} ${req.url} -> ${backend.host}:${backend.port}`);

    proxyRequest(req, res, backend.host, backend.port, backend.prefix);
  });

  // Handle WebSocket upgrade requests
  server.on("upgrade", (req, socket, head) => {
    const parsedUrl = url.parse(req.url);
    const backend = getBackend(parsedUrl.pathname);

    log("info", `WebSocket upgrade ${req.url} -> ${backend.host}:${backend.port}`);

    proxyWebSocket(req, socket, head, backend.host, backend.port, backend.prefix);
  });

  return server;
}

/**
 * Waits for a server to be available at the specified port
 * @param {string} host - Host to check
 * @param {number} port - Port to check
 * @param {number} timeout - Timeout in milliseconds
 * @param {number} interval - Check interval in milliseconds
 * @returns {Promise<boolean>}
 */
function waitForServer(host, port, timeout = 30000, interval = 1000) {
  return new Promise((resolve) => {
    const startTime = Date.now();

    function check() {
      const socket = net.connect(port, host);

      socket.on("connect", () => {
        socket.destroy();
        resolve(true);
      });

      socket.on("error", () => {
        socket.destroy();
        if (Date.now() - startTime < timeout) {
          setTimeout(check, interval);
        } else {
          resolve(false);
        }
      });
    }

    check();
  });
}

/**
 * Main entry point - starts all services and the proxy
 */
async function main() {
  log("info", "Starting Unified Proxy and Process Manager");
  log("info", `Configuration: ${JSON.stringify(CONFIG)}`);

  // Set up signal handlers for graceful shutdown
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

  // Create and start the proxy server
  const proxyServer = createProxyServer();

  proxyServer.listen(CONFIG.proxyPort, () => {
    log("info", `Unified proxy server listening on port ${CONFIG.proxyPort}`);
    log("info", `Routes:`);
    log("info", `  /           -> React UI (${CONFIG.reactHost}:${CONFIG.reactPort})`);
    log("info", `  /gradio/*   -> Gradio UI (${CONFIG.gradioHost}:${CONFIG.gradioPort})`);
    log("info", `  /update/*   -> Update Status (${CONFIG.updateHost}:${CONFIG.updatePort})`);
  });

  return proxyServer;
}

// Export for use as a module
module.exports = {
  main,
  createProxyServer,
  startProcess,
  stopAllProcesses,
  waitForServer,
  proxyRequest,
  proxyWebSocket,
  getBackend,
  CONFIG,
};

// Run main if executed directly
if (require.main === module) {
  main().catch((err) => {
    log("error", "Failed to start unified proxy:", err);
    process.exit(1);
  });
}
