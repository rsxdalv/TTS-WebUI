/**
 * A `dynamic` loader on top of `React.lazy`.
 *
 * The only call site (src/pages/extensions/[name].tsx) loads an extension
 * component by name with `ssr: false`. React.lazy does the same: the
 * component arrives in its own chunk and is loaded on demand.
 */
import React, { lazy, Suspense } from "react";

export interface DynamicOptions {
  ssr?: boolean;
  loading?: React.ComponentType<{ error?: Error | null; isLoading?: boolean }>;
}

const dynamic = <P extends object>(
  loader: () => Promise<{ default: React.ComponentType<P> } | React.ComponentType<P>>,
  options: DynamicOptions = {}
): React.ComponentType<P> => {
  const load = async () => {
    const loaded = await loader();
    if (typeof loaded === "function") return { default: loaded as React.ComponentType<P> };
    return loaded;
  };

  const Lazy = lazy(load);
  const Loading = options.loading;

  const Dynamic = (props: P) => (
    <Suspense fallback={Loading ? <Loading isLoading /> : null}>
      <Lazy {...props} />
    </Suspense>
  );

  Dynamic.displayName = "DynamicComponent";
  return Dynamic;
};

export default dynamic;
