/**
 * A `next/navigation` compatible shim on top of react-router.
 *
 * Only `useRouter().push` is used (src/components/RouteCommands.tsx).
 */
import { useCallback, useMemo } from "react";
import { useLocation, useNavigate } from "react-router-dom";

export interface AppRouterInstance {
  push(url: string): void;
  replace(url: string): void;
  back(): void;
  forward(): void;
  refresh(): void;
  prefetch(): Promise<void>;
}

export const useRouter = (): AppRouterInstance => {
  const navigate = useNavigate();
  const location = useLocation();

  const push = useCallback((url: string) => navigate(url), [navigate]);
  const replace = useCallback(
    (url: string) => navigate(url, { replace: true }),
    [navigate]
  );

  return useMemo(
    () => ({
      push,
      replace,
      back: () => navigate(-1),
      forward: () => navigate(1),
      refresh: () => window.location.reload(),
      prefetch: async () => {},
    }),
    [push, replace, navigate, location]
  );
};

export const usePathname = () => useLocation().pathname;
