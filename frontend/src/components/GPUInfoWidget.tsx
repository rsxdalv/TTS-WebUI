import React from "react";
import { GPUMemoryUsage } from "./component/GPUMemoryUsage";
import { GPUInfo } from "../types/GPUInfo";
import { GPUMeta } from "./component/GPUMeta";
import { cn } from "../lib/utils";

/** GPU data from the Go core (nvidia-smi); needs no Python or Gradio. */
export const GPUInfoWidget = ({ className = "", refreshInterval = 2000 }) => {
  const [gpuDatas, setGPUDatas] = React.useState<GPUInfo[]>([]);
  const [loaded, setLoaded] = React.useState<boolean>(false);
  const [error, setError] = React.useState<string>("");

  const fetchGPUData = async () => {
    try {
      const response = await fetch("/api/go-data/gpu-info");
      if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
      setGPUDatas(await response.json());
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoaded(true);
    }
  };

  React.useEffect(() => {
    fetchGPUData();
    const interval = setInterval(fetchGPUData, refreshInterval);
    return () => clearInterval(interval);
  }, [refreshInterval]);

  if (!loaded) {
    return <div>Loading GPU info...</div>;
  }
  if (error) {
    return <div className="text-red-600">GPU info is not available: {error}</div>;
  }
  if (!gpuDatas.length) {
    return (
      <div className="text-gray-600">
        No NVIDIA GPU found (nvidia-smi is missing or reports no device).
      </div>
    );
  }

  return (
    <div className={cn("flex gap-2 w-3/4", className)}>
      {gpuDatas.map((gpuData) => (
        <React.Fragment key={gpuData.idx}>
          <GPUMeta {...gpuData} />
          <GPUMemoryUsage {...gpuData} />
        </React.Fragment>
      ))}
    </div>
  );
};
