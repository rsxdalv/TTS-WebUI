import platform from "./platform";
import { WEBUI_ROOT } from "./config";

/**
 * Check via the platform adapter if the git repository has updates (fetch + compare).
 * Returns true if up-to-date, false if updates available.
 */

export async function isGitUpdated(): Promise<boolean> {
  try {
    await platform.exec(`git fetch origin`);

    const localFile = await platform.readFile(
      WEBUI_ROOT + "\\installer_scripts\\.git_version",
    );
    const localHash = localFile.trim();

    const remoteResult = await platform.exec(`git rev-parse origin/main`);
    const remoteHash = remoteResult.stdout.trim();
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
    await platform.exec(`git fetch origin`);

    const localResult = await platform.readFile(
      WEBUI_ROOT + "\\installer_scripts\\.git_version",
    );
    const remoteResult = await platform.exec(`git rev-parse origin/main`);

    const localHash = localResult.trim();
    const remoteHash = remoteResult.stdout.trim();

    return { hasUpdates: localHash !== remoteHash, localHash, remoteHash };
  } catch {
    return { hasUpdates: false, localHash: "", remoteHash: "" };
  }
}
