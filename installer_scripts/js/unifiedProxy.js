/**
 * Unified Node.js Proxy
 *
 * This module provides a unified proxy that routes requests to different backends:
 *    - /gradio/* -> Gradio backend (port 7770)
 *    - /update/* -> Update status page (port 7771)
 *    - /* -> React UI (port 3000)
 *
 * Uses only native Node.js modules (no npm packages required).
 * 
 * Note: The update port (7771) is fixed in server.js and cannot be changed via environment variables.
 */

const http = require("http");
const net = require("net");
const url = require("url");

// Configuration
const CONFIG = {
  proxyPort: parseInt(process.env.PROXY_PORT, 10) || 7860,
  gradioPort: parseInt(process.env.GRADIO_PORT, 10) || 7770,
  reactPort: parseInt(process.env.REACT_PORT, 10) || 3000,
  // Note: updatePort is fixed at 7771 (hardcoded in server.js)
  updatePort: 7771,
  gradioHost: process.env.GRADIO_HOST || "127.0.0.1",
  reactHost: process.env.REACT_HOST || "127.0.0.1",
  updateHost: process.env.UPDATE_HOST || "127.0.0.1",
};

// Logging utility
function log(level, message, ...args) {
  const timestamp = new Date().toISOString();
  console[level](`[${timestamp}] [UnifiedProxy] ${message}`, ...args);
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
    // Build the HTTP upgrade request, filtering out 'host' header to avoid duplication
    const headers = Object.keys(req.headers)
      .filter((key) => key.toLowerCase() !== "host")
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
 * Main entry point - starts the proxy only
 */
async function main() {
  log("info", "Starting Unified Proxy");
  log("info", `Configuration: ${JSON.stringify(CONFIG)}`);

  // Set up signal handlers for graceful shutdown
  process.on("SIGINT", () => {
    log("info", "Received SIGINT, shutting down...");
    process.exit(0);
  });

  process.on("SIGTERM", () => {
    log("info", "Received SIGTERM, shutting down...");
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
  waitForServer,
  proxyRequest,
  proxyWebSocket,
  getBackend,
  rewritePath,
  CONFIG,
};

// Run main if executed directly
if (require.main === module) {
  main().catch((err) => {
    log("error", "Failed to start unified proxy:", err);
    process.exit(1);
  });
}
