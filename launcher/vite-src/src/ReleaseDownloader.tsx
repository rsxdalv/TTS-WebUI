import { useState, useRef } from "react";
import platform, { OS } from "./platform";
import { WEBUI_ROOT, joinPath } from "./config";
import { Button } from "./components/ReleaseDownloader/Button";
import { Status } from "./components/ReleaseDownloader/Status";
import { Log } from "./components/ReleaseDownloader/Log";

export interface ReleaseDownloaderProps {
  name: string;
  downloadUrl: string;
  zipName?: string;
  cwd?: string;
  /** Called when extraction completes successfully (exit code 0) */
  onSuccess?: () => void;
}

function getDownloadCommand(url: string, outputPath: string): string {
  return `curl -Lk "${url}" -o "${outputPath}"`;
}

function getUnzipCommand(zipPath: string, destPath: string): string {
  if (OS === "windows") {
    return `powershell -Command "Expand-Archive -Path '${zipPath}' -DestinationPath '${destPath}' -Force"`;
  }
  return `unzip -o '${zipPath}' -d '${destPath}'`;
}

interface ProcessState {
  pid: number | null;
  stdout: string[];
  stderr: string[];
  exitCode: number | null;
  isRunning: boolean;
}

const idle: ProcessState = {
  pid: null,
  stdout: [],
  stderr: [],
  exitCode: null,
  isRunning: false,
};

export function ReleaseDownloader({
  name,
  downloadUrl,
  zipName = "release.zip",
  cwd = WEBUI_ROOT,
  onSuccess,
}: ReleaseDownloaderProps) {
  const [download, setDownload] = useState<ProcessState>(idle);
  const [unzip, setUnzip] = useState<ProcessState>(idle);
  const [gitConfig, setGitConfig] = useState<ProcessState>(idle);
  const onSuccessRef = useRef(onSuccess);
  onSuccessRef.current = onSuccess;

  const spawnDownload = async () => {
    const zipPath = joinPath(cwd, zipName);
    await platform.mkdir(cwd).catch(() => {});
    const command = getDownloadCommand(downloadUrl, zipPath);
    console.log(`[ReleaseDownloader] Downloading: ${command}`);
    setDownload({ ...idle, isRunning: true });
    const pid = await platform.spawn(command, {
      cwd,
      onStdout: (data) =>
        setDownload((prev) => ({ ...prev, stdout: [...prev.stdout, data] })),
      onStderr: (data) =>
        setDownload((prev) => ({ ...prev, stderr: [...prev.stderr, data] })),
      onExit: (code) =>
        setDownload((prev) => ({
          ...prev,
          exitCode: code,
          isRunning: false,
          pid: null,
        })),
    });
    setDownload((prev) => ({ ...prev, pid }));
  };

  const spawnUnzip = async () => {
    const zipPath = joinPath(cwd, zipName);
    const command = getUnzipCommand(zipPath, cwd);
    console.log(`[ReleaseDownloader] Unzipping: ${command}`);
    setUnzip({ ...idle, isRunning: true });
    const pid = await platform.spawn(command, {
      cwd,
      onStdout: (data) =>
        setUnzip((prev) => ({ ...prev, stdout: [...prev.stdout, data] })),
      onStderr: (data) =>
        setUnzip((prev) => ({ ...prev, stderr: [...prev.stderr, data] })),
      onExit: (code) => {
        if (code === 0 && OS !== "windows") onSuccessRef.current?.();
        setUnzip((prev) => ({
          ...prev,
          exitCode: code,
          isRunning: false,
          pid: null,
        }));
      },
    });
    setUnzip((prev) => ({ ...prev, pid }));
  };

  const spawnGitConfig = async () => {
    const command = `git config core.fileMode false`;
    console.log(`[ReleaseDownloader] Fixing git core.fileMode: ${command}`);
    try {
      setGitConfig({ ...idle, isRunning: true });
      const pid = await platform.spawn(command, {
        cwd,
        onStdout: (data) =>
          setGitConfig((prev) => ({ ...prev, stdout: [...prev.stdout, data] })),
        onStderr: (data) =>
          setGitConfig((prev) => ({ ...prev, stderr: [...prev.stderr, data] })),
        onExit: (code) => {
          if (code === 0) onSuccessRef.current?.();
          setGitConfig((prev) => ({
            ...prev,
            exitCode: code,
            isRunning: false,
            pid: null,
          }));
        },
      });
      setGitConfig((prev) => ({ ...prev, pid }));
    } catch (error) {
      console.error(`[ReleaseDownloader] Failed to fix git config: ${error}`);
    }
  };

  const terminate = async (pid: number) => {
    await platform.kill(pid).catch(() => {});
    setDownload((prev) => ({ ...prev, isRunning: false }));
    setUnzip((prev) => ({ ...prev, isRunning: false }));
    setGitConfig((prev) => ({ ...prev, isRunning: false }));
  };

  return (
    <div className="bg-gray-100 dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex flex-col gap-2">
      <h2 className="text-base font-semibold border-b border-gray-200 dark:border-gray-700 pb-2 text-gray-900 dark:text-gray-100">
        {name}
      </h2>
      <Button
        label={download.isRunning ? "Cancel Download" : "Download Release"}
        isRunning={download.isRunning}
        onClick={() =>
          download.isRunning && download.pid != null
            ? terminate(download.pid)
            : spawnDownload()
        }
        variant={download.isRunning ? "red" : "green"}
      />
      <Button
        label={unzip.isRunning ? "Cancel Extract" : "Extract ZIP"}
        isRunning={unzip.isRunning}
        onClick={() =>
          unzip.isRunning && unzip.pid != null
            ? terminate(unzip.pid)
            : spawnUnzip()
        }
        disabled={!download.isRunning && download.exitCode === null}
        variant="purple"
      />
      <Button
        label={
          gitConfig.isRunning ? "Cancel Git Config Fix" : "Fix Git Permissions (Windows)"
        }
        isRunning={gitConfig.isRunning}
        onClick={() =>
          gitConfig.isRunning && gitConfig.pid != null
            ? terminate(gitConfig.pid)
            : spawnGitConfig()
        }
        disabled={
          OS !== "windows" ||
          unzip.isRunning ||
          unzip.exitCode !== 0
        }
        variant="blue"
      />
      <Status label="Download" exitCode={download.exitCode} running={download.isRunning} />
      <Status label="Extract" exitCode={unzip.exitCode} running={unzip.isRunning} />
      {OS === "windows" && unzip.exitCode === 0 && (
        <Status label="Git config" exitCode={gitConfig.exitCode} running={gitConfig.isRunning} />
      )}
      <Log stdout={download.stdout} stderr={download.stderr} label="Download log" />
      <Log stdout={unzip.stdout} stderr={unzip.stderr} label="Extract log" />
      {OS === "windows" && (
        <Log stdout={gitConfig.stdout} stderr={gitConfig.stderr} label="Git config log" />
      )}
    </div>
  );
}
