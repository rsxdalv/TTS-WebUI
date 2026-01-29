/**
 * Tests for the Unified Proxy and Process Manager
 *
 * These tests verify that the proxy correctly routes requests
 * to the appropriate backend services based on URL paths.
 */

const {
  getBackend,
  rewritePath,
  CONFIG,
} = require("../js/unifiedProxy.js");

describe("Unified Proxy", () => {
  describe("getBackend", () => {
    it("should route /gradio/* to Gradio backend", () => {
      const backend = getBackend("/gradio");
      expect(backend.port).toBe(CONFIG.gradioPort);
      expect(backend.prefix).toBe("/gradio");
    });

    it("should route /gradio/api/predict to Gradio backend", () => {
      const backend = getBackend("/gradio/api/predict");
      expect(backend.port).toBe(CONFIG.gradioPort);
      expect(backend.prefix).toBe("/gradio");
    });

    it("should route /update/* to Update backend", () => {
      const backend = getBackend("/update");
      expect(backend.port).toBe(CONFIG.updatePort);
      expect(backend.prefix).toBe("/update");
    });

    it("should route /update/poll to Update backend", () => {
      const backend = getBackend("/update/poll");
      expect(backend.port).toBe(CONFIG.updatePort);
      expect(backend.prefix).toBe("/update");
    });

    it("should route / to React backend", () => {
      const backend = getBackend("/");
      expect(backend.port).toBe(CONFIG.reactPort);
      expect(backend.prefix).toBe("");
    });

    it("should route /any-other-path to React backend", () => {
      const backend = getBackend("/some/random/path");
      expect(backend.port).toBe(CONFIG.reactPort);
      expect(backend.prefix).toBe("");
    });

    it("should route /_next/static to React backend", () => {
      const backend = getBackend("/_next/static/chunks/main.js");
      expect(backend.port).toBe(CONFIG.reactPort);
      expect(backend.prefix).toBe("");
    });
  });

  describe("rewritePath", () => {
    it("should rewrite /gradio to /", () => {
      const result = rewritePath("/gradio", "/gradio");
      expect(result).toBe("/");
    });

    it("should rewrite /gradio/ to /", () => {
      const result = rewritePath("/gradio/", "/gradio");
      expect(result).toBe("/");
    });

    it("should rewrite /gradio/api/predict to /api/predict", () => {
      const result = rewritePath("/gradio/api/predict", "/gradio");
      expect(result).toBe("/api/predict");
    });

    it("should rewrite /update/poll to /poll", () => {
      const result = rewritePath("/update/poll", "/update");
      expect(result).toBe("/poll");
    });

    it("should rewrite /update/stream-log to /stream-log", () => {
      const result = rewritePath("/update/stream-log", "/update");
      expect(result).toBe("/stream-log");
    });
  });

  describe("CONFIG", () => {
    it("should have default configuration values", () => {
      expect(CONFIG.proxyPort).toBeDefined();
      expect(CONFIG.gradioPort).toBeDefined();
      expect(CONFIG.reactPort).toBeDefined();
      expect(CONFIG.updatePort).toBeDefined();
    });

    it("should have correct default ports", () => {
      // These match the project defaults
      expect(CONFIG.gradioPort).toBe(7770);
      expect(CONFIG.reactPort).toBe(3000);
      expect(CONFIG.updatePort).toBe(7771);
    });
  });
});

describe("Process Manager", () => {
  // Import process manager functions
  const { startProcess, stopAllProcesses } = require("../js/processManager.js");

  it("should export required functions", () => {
    expect(typeof startProcess).toBe("function");
    expect(typeof stopAllProcesses).toBe("function");
  });
});
