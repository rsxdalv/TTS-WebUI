import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { CacheProvider } from "@emotion/react";
import { cache } from "@emotion/css";

import "@/app/globals.css";

import { AppRoutes } from "./routes";
import { NavigatorRegistration } from "./shims/NavigatorRegistration";

/**
 * The entry point: the router around the pages listed in src/routes.tsx.
 */
const App = () => (
  <CacheProvider value={cache}>
    <BrowserRouter
      // Opt in to the v7 behaviour now, so the console stays clean.
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <NavigatorRegistration />
      <AppRoutes />
    </BrowserRouter>
  </CacheProvider>
);

const container = document.getElementById("root");
if (!container) {
  throw new Error("The #root element is missing from index.html");
}

// The title of the page comes from its <Head> (see src/shims/head.tsx).
// A page without one has no title, so the title of index.html is cleared here.
document.title = "";

ReactDOM.createRoot(container).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
