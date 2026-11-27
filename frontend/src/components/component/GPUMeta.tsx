import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { GPUInfo } from "../../types/GPUInfo";

export function GPUMeta({
  name,
  vram,
  cuda_capabilities,
  torch_version,
  cuda_version,
  multi_processor_count,
  temperature,
  power_draw,
  utilization,
}: GPUInfo) {
  return (
    <Card className="w-full max-w-md">
      <CardHeader className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="bg-muted rounded-full w-12 h-12 flex items-center justify-center">
            <CpuIcon className="w-6 h-6 text-muted-foreground" />
          </div>
          <div>
            <CardTitle>{name}</CardTitle>
            <CardDescription className="text-sm text-muted-foreground">
              Graphics Card
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="grid gap-4">
        <div className="grid grid-cols-2 gap-4">
          <DataPoint
            label="VRAM"
            value={`${Math.round(vram / 1024)}GB`}
            Icon={MemoryStickIcon}
          />
          <DataPoint
            label="Compute Capability"
            value={cuda_capabilities.join(".")}
            Icon={GaugeIcon}
          />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <DataPoint
            label="PyTorch Version"
            value={torch_version}
            Icon={CodeIcon}
          />
          <DataPoint
            label="CUDA Version"
            value={cuda_version}
            Icon={CodeIcon}
          />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <DataPoint
            label="Multi-Processor Count"
            value={multi_processor_count.toString()}
            Icon={CpuIcon}
          />
          <DataPoint
            label="Temperature"
            value={`${temperature}°C`}
            Icon={ThermometerIcon}
          />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <DataPoint
            label="Power Draw"
            value={`${power_draw}W`}
            Icon={PowerIcon}
          />
          <DataPoint
            label="Utilization"
            value={`${utilization}%`}
            Icon={GaugeIcon}
          />
        </div>
      </CardContent>
    </Card>
  );
}

function DataPoint({
  label,
  value,
  Icon,
}: {
  label: string;
  value: string;
  Icon: React.FC<React.SVGProps<SVGSVGElement>>;
}) {
  return (
    <div className="flex items-center gap-2">
      <Icon className="w-5 h-5 text-muted-foreground" />
      <div>
        <div className="text-sm text-muted-foreground">{label}</div>
        <div className="text-lg font-medium">{value}</div>
      </div>
    </div>
  );
}

function CodeIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  );
}

function CpuIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect width="16" height="16" x="4" y="4" rx="2" />
      <rect width="6" height="6" x="9" y="9" rx="1" />
      <path d="M15 2v2" />
      <path d="M15 20v2" />
      <path d="M2 15h2" />
      <path d="M2 9h2" />
      <path d="M20 15h2" />
      <path d="M20 9h2" />
      <path d="M9 2v2" />
      <path d="M9 20v2" />
    </svg>
  );
}

function GaugeIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="m12 14 4-4" />
      <path d="M3.34 19a10 10 0 1 1 17.32 0" />
    </svg>
  );
}

function MemoryStickIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M6 19v-3" />
      <path d="M10 19v-3" />
      <path d="M14 19v-3" />
      <path d="M18 19v-3" />
      <path d="M8 11V9" />
      <path d="M16 11V9" />
      <path d="M12 11V9" />
      <path d="M2 15h20" />
      <path d="M2 7a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v1.1a2 2 0 0 0 0 3.837V17a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-5.1a2 2 0 0 0 0-3.837Z" />
    </svg>
  );
}

function PowerIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 2v10" />
      <path d="M18.4 6.6a9 9 0 1 1-12.77.04" />
    </svg>
  );
}

function ThermometerIcon(props) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M14 4v10.54a4 4 0 1 1-4 0V4a2 2 0 0 1 4 0Z" />
    </svg>
  );
}
