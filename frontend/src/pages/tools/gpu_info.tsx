import React from "react";
import { Template } from "../../components/Template";
import { GPUInfoWidget } from "../../components/GPUInfoWidget";

const GPUInfoPage = () => {
  return (
    <Template title="GPU Info">
      <div className="gap-y-4 p-4 flex w-full flex-col items-center">
        <GPUInfoWidget />
      </div>
    </Template>
  );
};

export default GPUInfoPage;
