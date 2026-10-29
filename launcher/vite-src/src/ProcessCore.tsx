import { useState, useEffect, useRef, useCallback } from "react";
import { os, events } from "@neutralinojs/lib";

interface ProcessOutput {
  stdout: string[];
  stderr: string[];
  exitCode: number | null;
}

interface ProcInfo {
  id: number;
  pid: number;
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
  const isWindows = (window as any).NL_OS === "Windows";
  const isMac = (window as any).NL_OS === "Darwin";
  if (isWindows && props.command_windows) return props.command_windows;
  if (isMac && props.command_mac) return props.command_mac;
  if (!isWindows && !isMac && props.command_linux) return props.command_linux;
  return props.command;
}

export interface ProcessCoreState {
  procId: ProcInfo | null;
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
  const [procId, setProcId] = useState<ProcInfo | null>(null);
  const [outputs, setOutputs] = useState<ProcessOutput>({
    stdout: [],
    stderr: [],
    exitCode: null,
  });
  const procIdRef = useRef<ProcInfo | null>(null);
  const handlerRef = useRef<((evt: any) => void) | null>(null);
  const onSuccessRef = useRef(props.onSuccess);
  useEffect(() => {
    onSuccessRef.current = props.onSuccess;
  }, [props.onSuccess]);

  useEffect(() => {
    const handler: typeof handlerRef.current = (evt: any) => {
      const { id, action, data } = evt.detail;
      const numId = typeof id === "string" ? parseInt(id, 10) : id;

      if (
        (action === "stdOut" || action === "stdErr") &&
        procIdRef.current?.id === numId
      ) {
        setOutputs((prev) => {
          const updated = { ...prev };
          if (action === "stdOut") updated.stdout = [...updated.stdout, data];
          else updated.stderr = [...updated.stderr, data];
          return updated;
        });
      } else if (action === "exit" && procIdRef.current?.id === numId) {
        setOutputs((prev) => ({ ...prev, exitCode: data }));
        setProcId(null);
        if (data === 0) onSuccessRef.current?.();
      }
    };

    handlerRef.current = handler;
    events.on("spawnedProcess", handler);

    return () => {
      if (handlerRef.current) {
        events.off("spawnedProcess", handlerRef.current);
        handlerRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    procIdRef.current = procId;
  }, [procId]);

  const spawn = useCallback(() => {
    console.log(`[ProcessCore] Spawning: ${command} (cwd: ${cwd})`);
    os.spawnProcess(command, { cwd }).then((result) => {
      const info: ProcInfo = { id: result.id, pid: result.pid };
      procIdRef.current = info;
      setProcId(info);
      setOutputs({ stdout: [], stderr: [], exitCode: null });
    });
  }, [command, cwd]);

  const terminate = useCallback(() => {
    if (procIdRef.current === null) return;

    const isWindows = (window as any).NL_OS === "Windows";
    if (isWindows) {
      os.execCommand(`taskkill /F /T /PID ${procIdRef.current.pid}`).catch(
        () => {
          os.updateSpawnedProcess(procIdRef.current!.id, "exit").catch(
            () => {},
          );
        },
      );
    } else {
      os.updateSpawnedProcess(procIdRef.current.id, "exit").catch(() => {});
    }

    procIdRef.current = null;
    setProcId(null);
  }, []);

  return {
    procId,
    outputs,
    isRunning: procId !== null,
    spawn,
    terminate,
  };
}
