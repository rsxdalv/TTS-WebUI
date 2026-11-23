/**
 * Router hooks (`useRouter`, `asPath`, `query`) on top of react-router.
 *
 * The 19 call sites of the app use `router.push`, `router.asPath` and
 * `router.query`. This shim gives them the same shape, so the shared sources
 * stay unchanged.
 */
import { useCallback, useMemo } from "react";
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
  type NavigateOptions,
} from "react-router-dom";
import { navigateTo } from "./navigation-bridge";

export type RouterQuery = Record<
  string,
  string | string[] | undefined
>;

export interface AppRouter {
  /** The current path, with the search string, as `asPath`. */
  asPath: string;
  /** The route parameters and the search parameters, as `query`. */
  query: RouterQuery;
  push(url: string, options?: NavigateOptions): Promise<void>;
  replace(url: string, options?: NavigateOptions): Promise<void>;
  back(): void;
  forward(): void;
  reload(): void;
  prefetch(): Promise<void>;
  beforePopState(): void;
  events: {
    on(): void;
    off(): void;
    emit(): void;
  };
}

const toQuery = (params: Record<string, string | string[] | undefined>) => {
  const query: RouterQuery = {};
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined) query[key] = value;
  }
  return query;
};

/**
 * The router of the current route. `query` holds the dynamic segment
 * parameters (as `params` in react-router) and the search parameters.
 */
export const useRouter = (): AppRouter => {
  const location = useLocation();
  const navigate = useNavigate();
  const params = useParams();
  const [searchParams] = useSearchParams();

  const query = useMemo(() => {
    const search: RouterQuery = {};
    for (const key of new Set(searchParams.keys())) {
      const values = searchParams.getAll(key);
      search[key] = values.length > 1 ? values : values[0];
    }
    return { ...search, ...toQuery(params) };
  }, [params, searchParams]);

  const push = useCallback(
    async (url: string, options?: NavigateOptions) => {
      navigate(url, options);
    },
    [navigate]
  );

  const replace = useCallback(
    async (url: string, options?: NavigateOptions) => {
      navigate(url, { ...options, replace: true });
    },
    [navigate]
  );

  return useMemo(
    () => ({
      asPath: location.pathname + location.search,
      query,
      push,
      replace,
      back: () => navigate(-1),
      forward: () => navigate(1),
      reload: () => window.location.reload(),
      prefetch: async () => {},
      beforePopState: () => {},
      events: { on: () => {}, off: () => {}, emit: () => {} },
    }),
    [location.pathname, location.search, query, push, replace, navigate]
  );
};

/**
 * The default export is the router itself. The call sites
 * that use it outside a component (the tab helpers) get a proxy that reads
 * the current location and navigates through the history API.
 */
const routerProxy = new Proxy({} as AppRouter, {
  get(_target, prop) {
    if (prop === "asPath") {
      return typeof window === "undefined" ? "/" : window.location.pathname + window.location.search;
    }
    if (prop === "query") {
      if (typeof window === "undefined") return {};
      const query: RouterQuery = {};
      new URLSearchParams(window.location.search).forEach((value, key) => {
        query[key] = value;
      });
      return query;
    }
    if (prop === "push" || prop === "replace") {
      return (url: string) => {
        navigateTo(url, { replace: prop === "replace" });
        return Promise.resolve();
      };
    }
    if (prop === "back") return () => window.history.back();
    if (prop === "forward") return () => window.history.forward();
    if (prop === "reload") return () => window.location.reload();
    if (prop === "prefetch") return () => Promise.resolve();
    if (prop === "beforePopState") return () => {};
    if (prop === "events") return { on: () => {}, off: () => {}, emit: () => {} };
    return undefined;
  },
});

export default routerProxy;
