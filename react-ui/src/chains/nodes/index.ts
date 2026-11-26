import { registerConvertNodes } from "./convert";
import { registerGenerateNodes } from "./generate";
import { registerOutputNodes } from "./output";
import { registerProcessNodes } from "./process";
import { registerSourceNodes } from "./source";

let done = false;

/** Register every built-in node. Safe to call twice. */
export const registerBuiltinNodes = () => {
  if (done) return;
  done = true;
  registerSourceNodes();
  registerGenerateNodes();
  registerConvertNodes();
  registerProcessNodes();
  registerOutputNodes();
};
