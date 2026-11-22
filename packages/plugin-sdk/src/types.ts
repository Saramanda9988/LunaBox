export type Disposer = () => void;

export type LocalizedString = string | Record<string, string>;

export type SpacingToken = 'none' | 'xs' | 'sm' | 'md' | 'lg' | 'xl';
export type TextSizeToken = 'xs' | 'sm' | 'base' | 'lg' | 'xl';
export type FontWeightToken = 'normal' | 'medium' | 'semibold' | 'bold';
export type ToneToken = 'default' | 'muted' | 'strong' | 'brand' | 'success' | 'warning' | 'danger';
export type ButtonVariant = 'secondary' | 'primary' | 'danger' | 'ghost';

export interface Game {
  id: string;
  name: string;
  aliases?: string[];
  cover_url?: string;
  cover_source_url?: string;
  company?: string;
  summary?: string;
  rating?: number;
  release_date?: string;
  path?: string;
  game_directory?: string;
  status?: string;
  source_type?: string;
  source_id?: string;
  is_nsfw?: boolean;
  metadata_locked?: boolean;
}

export interface TagItem {
  name: string;
  source?: string;
  weight?: number;
  is_spoiler?: boolean;
}

export interface MetadataResult {
  game: Partial<Game>;
  tags?: TagItem[];
}

export interface Category {
  id: string;
  name: string;
  emoji: string;
  is_system?: boolean;
  game_count?: number;
}

export interface InstallRequestInput {
  name: string;
  downloadUrl: string;
  fileName?: string;
  archiveFormat?: string;
  startupPath?: string;
  installSubdir?: string;
  stripTopLevel?: boolean;
  headers?: Record<string, string>;
  archivePassword?: string;
  coverUrl?: string;
  source?: string;
  sourceId?: string;
  size?: number;
  silent?: boolean;
  metadata?: MetadataResult;
}

export interface ThemePaletteTokens {
  text?: string;
  textStrong?: string;
  textMuted?: string;
  textFaint?: string;
  appBg?: string;
  topbarBg?: string;
  topbarBorder?: string;
  controlIcon?: string;
  controlHover?: string;
  sidebarBg?: string;
  sidebarBorder?: string;
  navText?: string;
  navHover?: string;
  navActiveBg?: string;
  navActive?: string;
  inputBg?: string;
  inputBorder?: string;
  inputFocus?: string;
  primaryBtn?: string;
  primaryBtnHover?: string;
  cardBg?: string;
  cardBorder?: string;
  cardBorderHover?: string;
  panelBg?: string;
  panelBorder?: string;
  divider?: string;
}

export interface ThemeTokens {
  id?: string;
  light?: ThemePaletteTokens;
  dark?: ThemePaletteTokens;
  glass?: {
    enabled?: boolean;
    opacity?: number;
    blur?: number;
    backgroundImage?: string;
  };
}

export interface DecorationOptions {
  id: string;
  layer?: 'background' | 'foreground';
  anchor?: 'bottom-right' | 'bottom-left' | 'top-right' | 'top-left' | 'center';
  offset?: { x: number; y: number };
  size?: { width: number; height: number };
  opacity?: number;
  pointerEvents?: 'none' | 'auto';
  routes?: string[];
  image?: string;
  render?: (props: { route: string; dark: boolean }) => VNode;
}

export interface WindowOpenOptions {
  title?: string;
  width: number;
  height: number;
  x?: number;
  y?: number;
  frameless?: boolean;
  transparent?: boolean;
  alwaysOnTop?: boolean;
  skipTaskbar?: boolean;
  resizable?: boolean;
  render: (props: Record<string, unknown>) => VNode;
}

export interface PluginWindowHandle {
  id: string;
  setSize(w: number, h: number): void;
  setPosition(x: number, y: number): void;
  setAlwaysOnTop(v: boolean): void;
  show(): void;
  hide(): void;
  close(): void;
  onMove(fn: (pos: { x: number; y: number }) => void): void;
  onFocus(fn: () => void): void;
  onClose(fn: () => void): void;
}

export interface ConfigFieldSchema {
  key: string;
  type: 'string' | 'boolean' | 'number' | 'enum' | 'secret';
  label: string;
  description?: string;
  placeholder?: string;
  default?: unknown;
  options?: Array<{ value: string; label: string }>;
  min?: number;
  max?: number;
  step?: number;
}

export interface ConfigSchema {
  title?: string;
  description?: string;
  icon?: string;
  fields: ConfigFieldSchema[];
}

export interface MetadataSourceDef {
  id: string;
  name: LocalizedString;
  icon?: string;
  link?: (sourceId: string) => string;
  normalizeId?: (input: string) => string | null;
  rateLimit?: { perSecond: number };
  fetch: (sourceId: string, opts: { language: string; signal?: AbortSignal }) => Promise<MetadataResult>;
  search: (name: string, opts: { language: string; limit: number; signal?: AbortSignal }) => Promise<MetadataResult[]>;
  fetchBatch?: (ids: string[], opts: { language: string; signal?: AbortSignal }) => Promise<Record<string, MetadataResult>>;
}

export interface AIMessage {
  role: 'system' | 'user' | 'assistant' | 'tool';
  content: string;
  tool_call_id?: string;
}

export interface AIStreamRequest {
  system?: string;
  messages: AIMessage[];
  tools?: unknown[];
  model?: string;
  baseUrl?: string;
  apiKey?: string;
  maxTurns?: number;
}

export interface AIStreamHandlers {
  onDelta?: (text: string) => void;
  onToolCall?: (call: { id: string; name: string; arguments: Record<string, unknown>; rawArgs: string }) => unknown | Promise<unknown>;
  onDone?: (usage?: Record<string, unknown>) => void;
  onError?: (err: { message: string }) => void;
}

export interface HTMLSelection {
  select(css: string): HTMLSelection;
  first(): HTMLSelection;
  eq(index: number): HTMLSelection;
  length(): number;
  text(): string;
  attr(name: string): string | undefined;
  html(): string;
  map<T>(fn: (el: HTMLSelection, index: number) => T): T[];
}

export type SlotName =
  | 'sidebar.page'
  | 'workspace.drawer'
  | 'topbar.item'
  | 'library.add_menu'
  | 'library.toolbar'
  | 'game.detail.tab'
  | 'game.detail.header'
  | 'game.detail.aside'
  | 'game.actions'
  | 'downloads.header'
  | 'app.decoration'
  | 'settings.section';

export interface SlotDef {
  id: string;
  title?: string;
  icon?: string;
  group?: 'main' | 'bottom';
  badge?: string;
  mode?: 'normal' | 'batch' | 'both';
  defaultWidth?: number;
  render?: (props: any) => VNode;
  onPress?: (payload?: any) => void | Promise<void>;
  run?: (game: Game) => void | Promise<void>;
  when?: (game: Game) => boolean;
}

export type VNode =
  | { t: string; k?: string; p?: Record<string, unknown>; on?: Record<string, string>; c?: VNode[] }
  | { type: (props: any) => VNode; props?: Record<string, unknown> }
  | { __fragment: true; children?: unknown }
  | string
  | number
  | boolean
  | null
  | undefined;

export interface EventMap {
  'app/ready': () => void;
  'game/added': (game: Game) => void;
  'game/updated': (game: Game) => void;
  'game/deleted': (payload: { id: string }) => void;
  'game/before-launch': (game: Game) => { cancel: true; reason?: string } | undefined | Promise<{ cancel: true; reason?: string } | undefined>;
  'game/launched': (payload: { game: Game; sessionId: string }) => void;
  'session/ended': (payload: { gameId: string; sessionId: string; duration: number }) => void;
  'metadata/before-search': (payload: { name: string; source: string }) => { name: string } | undefined;
  'metadata/after-fetch': (payload: { source: string; result: MetadataResult }) => MetadataResult | undefined;
}

export interface Context {
  logger: {
    info(...args: unknown[]): void;
    warn(...args: unknown[]): void;
    error(...args: unknown[]): void;
  };
  i18n: {
    language: string;
    getLanguage(): string;
    t(key: string, args?: Record<string, unknown>): string;
    onChange(fn: (lang: string) => void): Disposer;
  };
  storage: {
    get<T = unknown>(key: string): T | undefined;
    set(key: string, value: unknown): void;
    delete(key: string): void;
    keys(): string[];
  };
  config: {
    define(schema: ConfigSchema | Record<string, Omit<ConfigFieldSchema, 'key'>>): Disposer;
    get<T = Record<string, unknown>>(): T;
    onChange<T = Record<string, unknown>>(fn: (next: T) => void): Disposer;
  };
  games: {
    list(query?: Record<string, unknown>): Promise<{ games: Game[]; total: number }>;
    get(id: string): Promise<Game>;
    findBySource(source: string, sourceId: string): Promise<Game | null>;
    update(id: string, patch: Partial<Game>): Promise<Game>;
    batchUpdate(patches: Array<{ id: string; patch: Partial<Game> }>): Promise<{ updated: number }>;
    setTags(id: string, tags: string[]): Promise<unknown[]>;
    start(id: string): Promise<boolean>;
    onChanged(fn: (payload: unknown) => void): Disposer;
  };
  categories: {
    list(): Promise<Category[]>;
    create(input: { name: string; emoji?: string }): Promise<Category | null>;
    update(id: string, patch: { name?: string; emoji?: string }): Promise<void>;
    delete(id: string): Promise<void>;
    addGames(categoryId: string, gameIds: string[]): Promise<void>;
    removeGames(categoryId: string, gameIds: string[]): Promise<void>;
  };
  metadata: {
    registerSource(def: MetadataSourceDef): Disposer;
    search(name: string, opts?: Record<string, unknown>): Promise<unknown[]>;
    fetch(source: string, sourceId: string, opts?: Record<string, unknown>): Promise<unknown>;
  };
  downloads: {
    requestInstall(req: InstallRequestInput): Promise<{ pending?: boolean; taskId?: string; queued?: boolean }>;
    list(): Promise<unknown[]>;
    getImportStates(reqs: Array<{ task_id?: string; file_path?: string; meta_source?: string; meta_id?: string }>): Promise<unknown[]>;
    onProgress(fn: (ev: unknown) => void): Disposer;
    onImported(fn: (taskId: string) => void): Disposer;
  };
  import: {
    openModal(opts?: string | { path?: string; metadata?: MetadataResult }): void;
    fromPath(req: { path: string; exePath?: string; name?: string; source?: string; sourceId?: string; metadata?: MetadataResult; categoryIds?: string[] }): Promise<unknown>;
  };
  theme: {
    extend(tokens: ThemeTokens): Disposer;
    registerFont(def: { family: string; file: string; target?: 'ui' | 'mono' }): Disposer;
    registerDecoration(def: DecorationOptions): Disposer;
  };
  window: {
    open(id: string, opts: WindowOpenOptions): PluginWindowHandle;
    main: {
      show(): void;
      focus(): void;
      minimize(): void;
      isVisible(): boolean;
    };
  };
  ai: {
    getConfig(): { configured: boolean; provider: string; model: string };
    complete(req: AIStreamRequest): Promise<{ content: string; toolCalls?: unknown[]; usage?: Record<string, unknown> }>;
    stream(req: AIStreamRequest, handlers: AIStreamHandlers): Disposer;
  };
  http: {
    sse(
      url: string,
      init: { method?: string; headers?: Record<string, string>; body?: string },
      handlers: { onEvent?: (ev: { event: string; id: string; data: string }) => void; onDone?: () => void; onError?: (err: { message: string }) => void }
    ): Disposer;
  };
  on<K extends keyof EventMap>(event: K, listener: EventMap[K], opts?: { prepend?: boolean }): Disposer;
  ui: {
    slot(name: SlotName, def: SlotDef | DecorationOptions): Disposer;
    toast(msg: string, type?: 'info' | 'success' | 'error' | 'loading'): void;
    navigate(path: string, params?: Record<string, unknown>): void;
    openDrawer(id: string, props?: Record<string, unknown>): void;
    closeDrawer(): void;
  };
  html: {
    parse(html: string): HTMLSelection;
  };
  shell: {
    openURL(url: string): Promise<void>;
    openPath(path: string): Promise<void>;
  };
  fs: {
    readFile(path: string): Promise<string>;
    writeFile(path: string, content: string): Promise<void>;
    exists(path: string): Promise<boolean>;
    readDir(path: string): Promise<Array<{ name: string; is_dir: boolean }>>;
    mkdir(path: string): Promise<void>;
    remove(path: string): Promise<void>;
  };
  paths: {
    plugin: string;
    data: string;
  };
  effect(setup: () => void | Disposer): Disposer;
}
