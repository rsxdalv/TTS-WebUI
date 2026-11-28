// SystemChecks: the pre-flight checks as a panel of the Extensions page. They
// matter when something is installed or built, not when the launcher starts.
import { PreflightView } from "./PreflightView";

export function SystemChecks() {
  return <PreflightView variant="panel" />;
}
