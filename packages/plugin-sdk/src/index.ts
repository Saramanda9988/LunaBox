export * from './types';
import type {
  AvatarProps,
  BadgeProps,
  BoxProps,
  ButtonProps,
  CardProps,
  CheckboxProps,
  CodeBlockProps,
  CollapsibleProps,
  ColumnProps,
  Context,
  DividerProps,
  EmptyProps,
  GridProps,
  HeroBannerProps,
  IconButtonProps,
  IconProps,
  ImageProps,
  InputProps,
  MarkdownProps,
  NativeSlotProps,
  NumberInputProps,
  ProgressProps,
  RowProps,
  ScrollAreaProps,
  SelectProps,
  SkeletonProps,
  SpacerProps,
  StackProps,
  SwitchProps,
  TableProps,
  TabsProps,
  TextareaProps,
  TextProps,
  ThemeTokens,
  TooltipProps,
  VNode,
} from './types';

export const Fragment = Symbol.for('lunabox.Fragment');

export declare function useState<T>(initial: T | (() => T)): [T, (next: T | ((prev: T) => T)) => void];
export declare function useEffect(setup: () => void | (() => void), deps?: readonly unknown[]): void;
export declare function useRef<T>(initial: T): { current: T };
export declare function useMemo<T>(factory: () => T, deps?: readonly unknown[]): T;

export declare function h(type: unknown, props?: Record<string, unknown> | null, ...children: unknown[]): VNode;
export declare function jsx(type: unknown, props?: Record<string, unknown> | null, key?: string): VNode;
export declare const jsxs: typeof jsx;

export type ComponentFactory<P = Record<string, unknown>> = (props?: P & { k?: string; key?: string; children?: unknown }) => VNode;

export declare const Column: ComponentFactory<ColumnProps>;
export declare const Row: ComponentFactory<RowProps>;
export declare const Box: ComponentFactory<BoxProps>;
export declare const Stack: ComponentFactory<StackProps>;
export declare const ScrollArea: ComponentFactory<ScrollAreaProps>;
export declare const Grid: ComponentFactory<GridProps>;
export declare const Text: ComponentFactory<TextProps>;
export declare const Markdown: ComponentFactory<MarkdownProps>;
export declare const CodeBlock: ComponentFactory<CodeBlockProps>;
export declare const Button: ComponentFactory<ButtonProps>;
export declare const IconButton: ComponentFactory<IconButtonProps>;
export declare const Input: ComponentFactory<InputProps>;
export declare const Textarea: ComponentFactory<TextareaProps>;
export declare const Switch: ComponentFactory<SwitchProps>;
export declare const Checkbox: ComponentFactory<CheckboxProps>;
export declare const Select: ComponentFactory<SelectProps>;
export declare const NumberInput: ComponentFactory<NumberInputProps>;
export declare const Tabs: ComponentFactory<TabsProps>;
export declare const Badge: ComponentFactory<BadgeProps>;
export declare const Card: ComponentFactory<CardProps>;
export declare const Collapsible: ComponentFactory<CollapsibleProps>;
export declare const Progress: ComponentFactory<ProgressProps>;
export declare const Image: ComponentFactory<ImageProps>;
export declare const Avatar: ComponentFactory<AvatarProps>;
export declare const Empty: ComponentFactory<EmptyProps>;
export declare const Skeleton: ComponentFactory<SkeletonProps>;
export declare const Tooltip: ComponentFactory<TooltipProps>;
export declare const Table: ComponentFactory<TableProps>;
export declare const Divider: ComponentFactory<DividerProps>;
export declare const Spacer: ComponentFactory<SpacerProps>;
export declare const Icon: ComponentFactory<IconProps>;
export declare const NativeSlot: ComponentFactory<NativeSlotProps>;
/** 基于 Stack / Box / Image / NativeSlot 等底层原语组合的标准 Hero 头图模板组件 */
export declare const HeroBanner: ComponentFactory<HeroBannerProps>;

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
