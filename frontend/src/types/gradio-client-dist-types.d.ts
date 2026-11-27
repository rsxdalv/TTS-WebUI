/**
 * Ambient module declaration for @gradio/client/dist/types
 *
 * @gradio/client v1.x does not expose ./dist/types in its package.json "exports" field,
 * so Next.js 16's exports-aware TypeScript resolver cannot find the deep import.
 * This file provides the minimum types needed to satisfy TypeScript while the actual
 * runtime code correctly resolves the Node.js module.
 */
declare module "@gradio/client/dist/types" {
  export interface Payload {
    fn_index: number;
    data: unknown[];
    time?: Date;
    event_data?: unknown;
    trigger_id?: number | null;
  }

  export interface PayloadMessage extends Payload {
    type: "data";
    endpoint: string;
    fn_index: number;
  }

  export interface SubmitIterable<T> extends AsyncIterable<T> {
    [Symbol.asyncIterator](): AsyncIterator<T>;
    cancel: () => Promise<void>;
    event_id: () => string;
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export type GradioEvent = any;

  export type PredictFunction = (
    endpoint: string | number,
    data?: unknown[] | Record<string, unknown>,
    event_data?: unknown
  ) => Promise<{ type: string; time: Date; data: unknown; endpoint: string; fn_index: number }>;
}
