export type GPUInfo = {
  vram: number;
  name: string;
  cuda_capabilities: number[];
  used_vram: number;
  used_vram_total: number;
  cached_vram: number;
  torch_version: string;
  cuda_version: string;
  idx: number;
  multi_processor_count: number;
  temperature: number;
  power_draw: number;
  utilization: number;
};
