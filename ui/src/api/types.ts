// Types matching the Go structs in core/internal/api/api.go.

/** Response from POST /api/upload */
export type UploadResponse = {
  /** Relative path from the install root, e.g. "outputs/uploads/hello.wav" */
  path: string;
};

/** Response from GET /api/version */
export type VersionResponse = {
  /** Human-readable build version string, e.g. "dev (commit none, built unknown)" */
  version: string;
};

/** Response from GET /api/generations/outputs */
export type GenerationsResponse = {
  files: string[];
};

/** Running state of a single engine */
export type EngineStatus = {
  running: boolean;
  port: number;
  pid: number;
};

/** Entry from GET /api/engines */
export type EngineListEntry = {
  id: string;
  status: EngineStatus;
};
