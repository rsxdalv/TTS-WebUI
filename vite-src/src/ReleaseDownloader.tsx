import { useState, useEffect, useRef } from "react";
import { os, events, debug, filesystem } from "@neutralinojs/lib";
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
  const isWindows = (window as any).NL_OS === "Windows";
  if (isWindows) {
    return `powershell -Command "Expand-Archive -Path '${zipPath}' -DestinationPath '${destPath}' -Force"`;
  }
  return `unzip -o '${zipPath}' -d '${destPath}'`;
}

interface ProcessState {
  procId: number | null;
  pid: number | null;
  stdout: string[];
  stderr: string[];
  exitCode: number | null;
  isRunning: boolean;
}

export function ReleaseDownloader({
  name,
  downloadUrl,
  zipName = "release.zip",
  cwd = WEBUI_ROOT,
  onSuccess,
}: ReleaseDownloaderProps) {
  const [download, setDownload] = useState<ProcessState>({
    procId: null,
    pid: null,
    stdout: [],
    stderr: [],
    exitCode: null,
    isRunning: false,
  });
  const [unzip, setUnzip] = useState<ProcessState>({
    procId: null,
    pid: null,
    stdout: [],
    stderr: [],
    exitCode: null,
    isRunning: false,
  });
  const [gitConfig, setGitConfig] = useState<ProcessState>({
    procId: null,
    pid: null,
    stdout: [],
    stderr: [],
    exitCode: null,
    isRunning: false,
  });

  const downloadRef = useRef(download);
  const unzipRef = useRef(unzip);
  const gitConfigRef = useRef(gitConfig);
  const onSuccessRef = useRef(onSuccess);

  useEffect(() => {
    downloadRef.current = download;
  }, [download]);
  useEffect(() => {
    unzipRef.current = unzip;
  }, [unzip]);
  useEffect(() => {
    gitConfigRef.current = gitConfig;
  }, [gitConfig]);
  useEffect(() => {
    onSuccessRef.current = onSuccess;
  }, [onSuccess]);

  useEffect(() => {
    const handler = (evt: any) => {
      const { id, action, data } = evt.detail;
      const numId = typeof id === "string" ? parseInt(id, 10) : id;

      // Download process events
      if (
        downloadRef.current.procId !== null &&
        downloadRef.current.procId === numId
      ) {
        setDownload((prev) => {
          if (action === "stdOut")
            return { ...prev, stdout: [...prev.stdout, data] };
          if (action === "stdErr")
            return { ...prev, stderr: [...prev.stderr, data] };
          if (action === "exit")
            return { ...prev, exitCode: data, isRunning: false, procId: null };
          return prev;
        });
      }

      // Unzip process events
      if (
        unzipRef.current.procId !== null &&
        unzipRef.current.procId === numId
      ) {
        setUnzip((prev) => {
          if (action === "stdOut")
            return { ...prev, stdout: [...prev.stdout, data] };
          if (action === "stdErr")
            return { ...prev, stderr: [...prev.stderr, data] };
          if (action === "exit") {
            if (data === 0 && (window as any).NL_OS !== "Windows") onSuccessRef.current?.();
            return { ...prev, exitCode: data, isRunning: false, procId: null };
          }
          return prev;
        });
      }

      // Git config process events
      if (
        gitConfigRef.current.procId !== null &&
        gitConfigRef.current.procId === numId
      ) {
        setGitConfig((prev) => {
          if (action === "stdOut")
            return { ...prev, stdout: [...prev.stdout, data] };
          if (action === "stdErr")
            return { ...prev, stderr: [...prev.stderr, data] };
          if (action === "exit") {
            if (data === 0) onSuccessRef.current?.();
            return { ...prev, exitCode: data, isRunning: false, procId: null };
          }
          return prev;
        });
      }
    };

    events.on("spawnedProcess", handler);
    return () => {
      events.off("spawnedProcess", handler);
    };
  }, []);

  const spawnDownload = async () => {
    const zipPath = joinPath(cwd, zipName);
    await filesystem.createDirectory(cwd).catch(() => {}); // Neutralino always throws
    const command = getDownloadCommand(downloadUrl, zipPath);
    console.log(`[ReleaseDownloader] Downloading: ${command}`);
    const result = await os.spawnProcess(command, { cwd });
    setDownload({
      procId: result.id,
      pid: result.pid,
      stdout: [],
      stderr: [],
      exitCode: null,
      isRunning: true,
    });
  };

  const spawnUnzip = async () => {
    const zipPath = joinPath(cwd, zipName);
    const command = getUnzipCommand(zipPath, cwd);
    console.log(`[ReleaseDownloader] Unzipping: ${command}`);
    const result = await os.spawnProcess(command, { cwd });
    setUnzip({
      procId: result.id,
      pid: result.pid,
      stdout: [],
      stderr: [],
      exitCode: null,
      isRunning: true,
    });
  };

  const spawnGitConfig = async () => {
    const command = `git config core.fileMode false`;
    console.log(`[ReleaseDownloader] Fixing git core.fileMode: ${command}`);
    try {
      const result = await os.spawnProcess(command, { cwd });
      setGitConfig({
        procId: result.id,
        pid: result.pid,
        stdout: [],
        stderr: [],
        exitCode: null,
        isRunning: true,
      });
    } catch (error) {
      debug.log(
        `[ReleaseDownloader] Error spawning git config process: ${error}`,
      );
      console.error(`[ReleaseDownloader] Failed to fix git config: ${error}`);
      return;
    }
  };

  const terminate = async (_procId: number, pid: number) => {
    const isWindows = (window as any).NL_OS === "Windows";
    if (isWindows) {
      await os.execCommand(`taskkill /F /T /PID ${pid}`).catch(() => {});
    }
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
          download.isRunning
            ? terminate(download.procId!, download.pid!)
            : spawnDownload()
        }
        variant={download.isRunning ? "red" : "green"}
      />
      <Button
        label={unzip.isRunning ? "Cancel Extract" : "Extract ZIP"}
        isRunning={unzip.isRunning}
        onClick={() =>
          unzip.isRunning ? terminate(unzip.procId!, unzip.pid!) : spawnUnzip()
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
          gitConfig.isRunning
            ? terminate(gitConfig.procId!, gitConfig.pid!)
            : spawnGitConfig()
        }
        disabled={
          (window as any).NL_OS !== "Windows" ||
          unzip.isRunning ||
          unzip.exitCode !== 0
        }
        variant="blue"
      />
      <Status label="Download" exitCode={download.exitCode} running={download.isRunning} />
      <Status label="Extract" exitCode={unzip.exitCode} running={unzip.isRunning} />
      {(window as any).NL_OS === "Windows" && unzip.exitCode === 0 && (
        <Status label="Git config" exitCode={gitConfig.exitCode} running={gitConfig.isRunning} />
      )}
      <Log stdout={download.stdout} stderr={download.stderr} label="Download log" />
      <Log stdout={unzip.stdout} stderr={unzip.stderr} label="Extract log" />
      {(window as any).NL_OS === "Windows" && (
        <Log stdout={gitConfig.stdout} stderr={gitConfig.stderr} label="Git config log" />
      )}
    </div>
  );
}
