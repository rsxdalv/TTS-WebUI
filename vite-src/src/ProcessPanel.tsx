import { useProcessCore, ProcessCoreProps } from "./ProcessCore";
import { ProcessDisplay } from "./ProcessDisplay";

interface ProcessPanelProps extends ProcessCoreProps {
  openUrl?: string;
}

export function ProcessPanel(props: ProcessPanelProps) {
  const core = useProcessCore(props);
  return <ProcessDisplay {...props} core={core} />;
}
