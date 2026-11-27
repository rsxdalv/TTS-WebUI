import React from "react";
import { Template } from "../components/Template";
import { ChainsEditor } from "../chains/ui/ChainsEditor";

/**
 * Chains: a node graph for text, speech and audio steps. The editor is
 * lazy-loaded with this page, so the rest of the app does not pay for it.
 * See documentation/NODE-GRAPH-SPEC.md.
 */
const ChainsPage = () => (
  <Template title="Chains">
    <div className="flex w-full flex-col gap-2">
      <h1 className="text-2xl font-bold">Chains</h1>
      <ChainsEditor />
    </div>
  </Template>
);

export default ChainsPage;
