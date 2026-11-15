import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "../index.css";
import { CoreApp } from "./CoreApp";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <CoreApp />
  </StrictMode>,
);
