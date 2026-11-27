import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { GPUInfo } from "../../types/GPUInfo";

export function GPUMemoryUsage({
  vram,
  used_vram,
  used_vram_total,
  cached_vram,
}: GPUInfo) {
  return (
    <Card className="w-full max-w-md">
      <CardHeader className="flex items-center justify-between">
        <CardTitle>GPU Memory Usage</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-4">
        <GPUMemoryUsageBar value={used_vram} max={vram} label="TTS WebUI used VRAM" />
        <GPUMemoryUsageBar value={cached_vram} max={vram} label="TTS WebUI Cached VRAM" />
        <GPUMemoryUsageBar value={used_vram_total} max={vram} label="Used VRAM System" />
      </CardContent>
    </Card>
  );
}

function GPUMemoryUsageBar({
  value,
  max,
  label,
}: {
  value: number;
  max: number;
  label: string;
}) {
  return (
    <div className="flex flex-col">
      <div className="flex items-center justify-between">
        <div className="text-sm text-muted-foreground">{label}</div>
        <div className="text-lg font-medium">{value.toFixed(0)} MB</div>
      </div>
      <Progress value={value} max={max} />
    </div>
  );
}


