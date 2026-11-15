import { useState, useRef, useCallback } from "react";
import platform, { OS } from "./platform";

interface ProcessOutput {
  stdout: string[];
  stderr: string[];
  exitCode: number | null;
}

export interface ProcessCoreProps {
  name: string;
  command: string;
  command_windows?: string;
  command_mac?: string;
  command_linux?: string;
  cwd: string;
  /** Called when the process exits with code 0 */
  onSuccess?: () => void;
}

export function resolveCommand(props: ProcessCoreProps): string {
  const isWindows = OS === "windows";
  const isMac = OS === "darwin";
  if (isWindows && props.command_windows) return props.command_windows;
  if (isMac && props.command_mac) return props.command_mac;
  if (!isWindows && !isMac && props.command_linux) return props.command_linux;
  return props.command;
}

export interface ProcessCoreState {
  pid: number | null;
  outputs: ProcessOutput;
  isRunning: boolean;
}

export interface ProcessCoreActions {
  spawn: () => void;
  terminate: () => void;
}

export function useProcessCore(
  props: ProcessCoreProps,
): ProcessCoreState & ProcessCoreActions {
  const { cwd } = props;
  const command = resolveCommand(props);
  const [pid, setPid] = useState<number | null>(null);
  const [outputs, setOutputs] = useState<ProcessOutput>({
    stdout: [],
    stderr: [],
    exitCode: null,
  });
  const pidRef = useRef<number | null>(null);
  const onSuccessRef = useRef(props.onSuccess);
  onSuccessRef.current = props.onSuccess;

  const spawn = useCallback(() => {
    console.log(`[ProcessCore] Spawning: ${command} (cwd: ${cwd})`);
    setOutputs({ stdout: [], stderr: [], exitCode: null });
    platform
      .spawn(command, {
        cwd,
        onStdout: (data) =>
          setOutputs((prev) => ({ ...prev, stdout: [...prev.stdout, data] })),
        onStderr: (data) =>
          setOutputs((prev) => ({ ...prev, stderr: [...prev.stderr, data] })),
        onExit: (code) => {
          setOutputs((prev) => ({ ...prev, exitCode: code }));
          setPid(null);
          pidRef.current = null;
          if (code === 0) onSuccessRef.current?.();
        },
      })
      .then((newPid) => {
        pidRef.current = newPid;
        setPid(newPid);
      });
  }, [command, cwd]);

  const terminate = useCallback(() => {
    if (pidRef.current === null) return;
    platform.kill(pidRef.current).catch(() => {});
    pidRef.current = null;
    setPid(null);
  }, []);

  return {
    pid,
    outputs,
    isRunning: pid !== null,
    spawn,
    terminate,
  };
}
