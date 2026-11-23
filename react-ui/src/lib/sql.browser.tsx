/**
 * The browser version of src/lib/sql.tsx.
 *
 * The `postgres` package needs Node's TCP stack, so it only works on the
 * server. The database page (src/pages/db.tsx) shows the generations of the
 * server build; in the browser the list is empty.
 *
 * The Vite build swaps the module for this one (see the alias in
 * vite.config.ts). The Next build keeps the original.
 */
export const sql = (() =>
  Promise.resolve([])) as unknown as (
  strings: TemplateStringsArray,
  ...values: unknown[]
) => Promise<unknown[]>;
