import { os, filesystem } from "@neutralinojs/lib";
import { WEBUI_ROOT } from "./config";

/**
 * Check via Neutralino if the git repository has updates (fetch + compare).
 * Returns true if up-to-date, false if updates available.
 */

export async function isGitUpdated(): Promise<boolean> {
  try {
    await os.execCommand("git fetch origin", { cwd: WEBUI_ROOT });

    const localFile = await filesystem.readFile(
      WEBUI_ROOT + "\\installer_scripts\\.git_version",
    );
    const localHash = localFile.trim();

    const remoteResult = await os.execCommand("git rev-parse origin/main", {
      cwd: WEBUI_ROOT,
    });
    const remoteHash = remoteResult.stdOut.trim();
    console.log("Local Git Hash:", localHash);
    console.log("Remote Git Hash:", remoteHash);

    return localHash === remoteHash;
  } catch (error) {
    console.error("Git update check failed:", error);
    // If anything fails (no git, no origin, etc.), assume it's updated
    return true;
  }
}

/**
 * Detailed update check - returns info for display purposes.
 */

export async function checkGitUpdateStatus(): Promise<{
  hasUpdates: boolean;
  localHash: string;
  remoteHash: string;
}> {
  try {
    await os.execCommand("git fetch origin", { cwd: WEBUI_ROOT });

    const localResult = await filesystem.readFile(
      WEBUI_ROOT + "\\installer_scripts\\.git_version",
    );
    const remoteResult = await os.execCommand("git rev-parse origin/main", {
      cwd: WEBUI_ROOT,
    });

    const localHash = localResult.trim();
    const remoteHash = remoteResult.stdOut.trim();

    return { hasUpdates: localHash !== remoteHash, localHash, remoteHash };
  } catch {
    return { hasUpdates: false, localHash: "", remoteHash: "" };
  }
}
