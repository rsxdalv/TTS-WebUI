import { ProcessCoreProps, useProcessCore } from "./ProcessCore";
import { ProcessDisplay } from "./ProcessDisplay";

interface SpawnerProps extends ProcessCoreProps {
  openUrl?: string;
}

export function ProcessSpawner(props: SpawnerProps) {
  const core = useProcessCore(props);
  return <ProcessDisplay {...props} core={core} />;
}
