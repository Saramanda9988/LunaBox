export * from './types';
import type { Context, ThemeTokens, VNode } from './types';

export const Fragment = Symbol.for('lunabox.Fragment');

export declare function useState<T>(initial: T | (() => T)): [T, (next: T | ((prev: T) => T)) => void];
export declare function useEffect(setup: () => void | (() => void), deps?: readonly unknown[]): void;
export declare function useRef<T>(initial: T): { current: T };
export declare function useMemo<T>(factory: () => T, deps?: readonly unknown[]): T;

export declare function h(type: unknown, props?: Record<string, unknown> | null, ...children: unknown[]): VNode;
export declare function jsx(type: unknown, props?: Record<string, unknown> | null, key?: string): VNode;
export declare const jsxs: typeof jsx;

export type ComponentFactory<P = Record<string, unknown>> = (props?: P & { k?: string; children?: unknown }) => VNode;

export declare const Column: ComponentFactory;
export declare const Row: ComponentFactory;
export declare const Box: ComponentFactory;
export declare const ScrollArea: ComponentFactory;
export declare const Grid: ComponentFactory;
export declare const Text: ComponentFactory;
export declare const Markdown: ComponentFactory;
export declare const CodeBlock: ComponentFactory;
export declare const Button: ComponentFactory;
export declare const IconButton: ComponentFactory;
export declare const Input: ComponentFactory;
export declare const Textarea: ComponentFactory;
export declare const Switch: ComponentFactory;
export declare const Checkbox: ComponentFactory;
export declare const Select: ComponentFactory;
export declare const NumberInput: ComponentFactory;
export declare const Tabs: ComponentFactory;
export declare const Badge: ComponentFactory;
export declare const Card: ComponentFactory;
export declare const Collapsible: ComponentFactory;
export declare const Progress: ComponentFactory;
export declare const Image: ComponentFactory;
export declare const Avatar: ComponentFactory;
export declare const Empty: ComponentFactory;
export declare const Skeleton: ComponentFactory;
export declare const Tooltip: ComponentFactory;
export declare const Table: ComponentFactory;
export declare const Divider: ComponentFactory;
export declare const Spacer: ComponentFactory;

export declare function defineThemePreset(preset: ThemeTokens): ThemeTokens;
export declare function createPagination<T>(
  fetcher: (args: { page: number; pageSize: number; query: string }) => Promise<T[] | { items: T[]; hasMore: boolean }>,
  options?: { pageSize?: number }
): (queryKey: string) => {
  items: T[];
  page: number;
  loading: boolean;
  hasMore: boolean;
  error: string | null;
  loadMore: () => void;
  refresh: () => void;
};
export declare function createAIChatStore(
  ctx: Context,
  options?: {
    system?: string;
    tools?: unknown[];
    model?: string;
    baseUrl?: string;
    apiKey?: string;
    onToolCall?: (call: { id: string; name: string; arguments: Record<string, unknown> }) => unknown | Promise<unknown>;
    onError?: (err: { message: string }) => void;
  }
): {
  getState(): { messages: Array<{ role: string; content: string; toolCalls?: Array<{ id: string; name: string; status: string; arguments?: unknown; result?: string }> }>; streaming: boolean };
  subscribe(fn: () => void): () => void;
  send(content: string): void;
  abort(): void;
  clear(): void;
};
