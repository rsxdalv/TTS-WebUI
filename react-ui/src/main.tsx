import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { CacheProvider } from "@emotion/react";
import { cache } from "@emotion/css";

import "@/app/globals.css";

import { AppRoutes } from "./routes";
import { NavigatorRegistration } from "./shims/NavigatorRegistration";

/**
 * The Vite entry point.
 *
 * The pages are the same files Next uses (src/pages), so the two builds show
 * the same UI. Next keeps its own entry (_app.tsx / _document.tsx); this
 * file replaces them for the Vite build.
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

const container = document.getElementById("__next");
if (!container) {
  throw new Error("The #__next element is missing from index.html");
}

// The title of the page comes from its <Head> (see src/shims/next-head.tsx).
// A page without one has no title, like in Next, so the title of index.html
// is cleared here.
document.title = "";

ReactDOM.createRoot(container).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
