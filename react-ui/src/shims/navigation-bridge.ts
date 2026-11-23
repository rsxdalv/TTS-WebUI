/**
 * The bridge between react-router and the modules that need to navigate
 * outside a component (the `next/router` default export).
 *
 * The router in src/main.tsx registers the navigate function here, so
 * `router.push("/somewhere")` from a helper module goes through react-router
 * and updates the URL like a real client-side navigation.
 */
type NavigateFn = (to: string, options?: { replace?: boolean }) => void;

let navigate: NavigateFn | null = null;

export const setNavigator = (fn: NavigateFn) => {
  navigate = fn;
};

export const navigateTo = (url: string, options?: { replace?: boolean }) => {
  if (navigate) {
    navigate(url, options);
    return;
  }
  // Before the router is mounted (or in a test): fall back to the history API.
  if (typeof window === "undefined") return;
  if (options?.replace) {
    window.history.replaceState(null, "", url);
  } else {
    window.history.pushState(null, "", url);
  }
  window.dispatchEvent(new PopStateEvent("popstate"));
};
