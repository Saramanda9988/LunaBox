export type Disposer = () => void;

export type LocalizedString = string | Record<string, string>;

/**
 * 插件清单（plugin.json）类型定义。
 */
export interface PluginManifest {
  /** 唯一插件 ID（小写字母开头，仅含小写字母、数字与连字符） */
  id: string;
  /** SemVer 版本号，例如 "1.0.0" */
  version: string;
  /** 插件 API 主版本（当前为 1） */
  apiVersion: number;
  /** 兼容的 LunaBox 宿主版本范围 */
  engines?: {
    lunabox?: string;
  };
  /** 插件入口文件（相对插件根目录，默认 "main.js"） */
  main?: string;
  /** 多语言插件名称（如 { "zh-CN": "...", "en-US": "..." }） */
  name: Record<string, string>;
  /** 多语言插件简介 */
  description?: Record<string, string>;
  /** 作者名称 */
  author?: string;
  /** 插件主页链接 */
  homepage?: string;
  /** 源码仓库链接 */
  repository?: string;
  /** 开源许可证标识（如 "MIT"） */
  license?: string;
  /**
   * 插件图标：支持相对插件目录的图片路径（如 "assets/icon.svg" / "icon.png"）
   * 或内置 MDI 图标名（如 "timer-outline"、"robot-happy"）。
   */
  icon?: string;
  /** 可选图标背景色（如 "#fef2f2"），为空时使用主题默认底色 */
  iconBg?: string;
  /** 插件关键词标签 */
  keywords?: string[];
}

export type SpacingToken = 'none' | 'xs' | 'sm' | 'md' | 'lg' | 'xl' | number;
export type RadiusToken = 'sm' | 'md' | 'lg' | 'full' | number;
export type SurfaceToken = 'glass' | 'glass-card' | 'panel' | 'card' | 'chip' | 'subtle';
export type AlignToken = 'start' | 'center' | 'end' | 'stretch';
export type JustifyToken = 'start' | 'center' | 'end' | 'between' | 'space-between';
export type TextSizeToken = 'xs' | 'sm' | 'base' | 'lg' | 'xl' | '2xl' | '3xl' | '4xl' | number;
export type FontWeightToken = 'normal' | 'medium' | 'semibold' | 'bold';
export type ToneToken =
  | 'default'
  | 'muted'
  | 'faint'
  | 'strong'
  | 'brand'
  | 'accent'
  | 'success'
  | 'warning'
  | 'danger'
  | 'light'
  | 'light-muted';
export type ButtonVariant = 'secondary' | 'primary' | 'danger' | 'ghost';
export type ButtonSize = 'sm' | 'md' | 'lg';

/** 宿主主题支持的语义色板令牌（亦可附加 "@0.5" 透明度后缀或传入十六进制色值） */
export type ColorToken =
  | 'transparent'
  | 'white'
  | 'black'
  | 'appBg'
  | 'fadeBg'
  | 'scrim'
  | 'panelBg'
  | 'panelBorder'
  | 'cardBg'
  | 'cardBorder'
  | 'chipBg'
  | 'divider'
  | 'text'
  | 'textStrong'
  | 'textMuted'
  | 'textFaint'
  | 'primary'
  | 'brand'
  | 'accent'
  | 'success'
  | 'warning'
  | 'warning400'
  | 'danger'
  | 'brand700'
  | 'brand800'
  | 'brand900';

/**
 * 通用颜色值：
 * - 语义色板令牌（如 "appBg"、"fadeBg"、"scrim"、"white"、"warning400"）
 * - 带透明度后缀（如 "black@0.42"、"white@0.86"、"#1a1b26@0.8"）
 * - 十六进制色值（"#rgb" / "#rgba" / "#rrggbb" / "#rrggbbaa"）
 * - 明暗主题自适应对象 `{ light, dark }`
 * - 显式透明度对象 `{ color | token, alpha }`
 */
export type ColorValue =
  | ColorToken
  | (string & {})
  | { light?: ColorValue; dark?: ColorValue }
  | { color?: ColorValue; token?: ColorValue; alpha?: number };

export interface GradientStop {
  color: ColorValue;
  /** 停靠位置 0..1，省略时按序号等分 */
  at?: number;
}

export interface LinearGradientSpec {
  /** 渐变角度（度，0=自下而上，90=自左而右，180=自上而下；默认 180） */
  angle?: number;
  stops: GradientStop[];
}

export type ShadowSpec =
  | 'sm'
  | 'md'
  | 'lg'
  | {
      x?: number;
      y?: number;
      blur: number;
      spread?: number;
      color?: ColorValue;
    };

/** 容器类组件（Column / Row / Box / Stack）通用的布局、定位、背景、边框与交互属性 */
export interface LayoutProps {
  gap?: SpacingToken;
  padding?: SpacingToken;
  paddingX?: SpacingToken;
  paddingY?: SpacingToken;
  paddingTop?: SpacingToken;
  paddingRight?: SpacingToken;
  paddingBottom?: SpacingToken;
  paddingLeft?: SpacingToken;
  margin?: SpacingToken;
  marginX?: SpacingToken | 'auto';
  marginY?: SpacingToken;
  marginTop?: SpacingToken;
  marginRight?: SpacingToken;
  marginBottom?: SpacingToken;
  marginLeft?: SpacingToken;
  /** 水平居中（等价于 marginX: 'auto'） */
  centerX?: boolean;
  surface?: SurfaceToken;
  background?: ColorValue;
  hoverBackground?: ColorValue;
  gradient?: LinearGradientSpec;
  border?: boolean | number;
  borderColor?: ColorValue;
  borderBottom?: boolean;
  borderLeft?: boolean;
  shadow?: ShadowSpec;
  opacity?: number;
  radius?: RadiusToken;
  clip?: boolean;
  position?: 'absolute';
  inset?: number;
  top?: number;
  right?: number;
  bottom?: number;
  left?: number;
  zIndex?: number;
  pointerEvents?: 'none' | 'auto';
  align?: AlignToken;
  alignSelf?: AlignToken;
  justify?: JustifyToken;
  width?: number | 'full';
  height?: number | 'full';
  grow?: number;
  shrink?: number;
  basis?: number;
  minWidth?: number;
  maxWidth?: number;
  minHeight?: number;
  maxHeight?: number;
  wrap?: boolean;
  /** 独立悬浮窗中拖动窗口区域 */
  dragWindow?: boolean;
  /** 无障碍标签 */
  label?: string;
  disabled?: boolean;
  onPress?: () => void;
  onContextMenu?: () => void;
}

export type ColumnProps = LayoutProps;
export type RowProps = LayoutProps;
export type StackProps = LayoutProps;

export interface BoxProps extends LayoutProps {
  /** 开启原生帧率弹簧按压形变动画 */
  springPress?: boolean;
  /** springPress 按下时的横向缩放比例（默认 1.05） */
  squishX?: number;
  /** springPress 按下时的纵向缩放比例（默认 0.88） */
  squishY?: number;
  /** springPress 内容水平镜像翻转 */
  flipX?: boolean;
  onPressIn?: () => void;
  onPressOut?: () => void;
}

export interface ScrollAreaProps {
  padding?: SpacingToken;
  gap?: SpacingToken;
  height?: number;
  grow?: number;
  autoScrollBottom?: boolean;
  onReachBottom?: () => void;
}

export interface GridProps {
  cols?: number;
  minColWidth?: number;
  gap?: SpacingToken;
}

export interface TextProps {
  text?: string;
  size?: TextSizeToken;
  lineHeight?: number;
  weight?: FontWeightToken;
  tone?: ToneToken;
  color?: ColorValue;
  wrap?: boolean;
  maxLines?: number;
  selectable?: boolean;
  label?: string;
  margin?: SpacingToken;
  marginX?: SpacingToken | 'auto';
  marginY?: SpacingToken;
  marginTop?: SpacingToken;
  marginRight?: SpacingToken;
  marginBottom?: SpacingToken;
  marginLeft?: SpacingToken;
  centerX?: boolean;
}

export interface MarkdownProps {
  content: string;
  onLinkPress?: (href: string) => void;
}

export interface CodeBlockProps {
  code: string;
  language?: string;
  copyable?: boolean;
}

export interface ButtonProps {
  label?: string;
  icon?: string;
  variant?: ButtonVariant;
  size?: ButtonSize;
  disabled?: boolean;
  loading?: boolean;
  onPress?: () => void;
  onClick?: () => void;
}

export interface IconButtonProps {
  icon: string;
  label?: string;
  size?: 'sm' | 'md';
  active?: boolean;
  disabled?: boolean;
  onPress?: () => void;
  onClick?: () => void;
}

export interface InputProps {
  value?: string;
  placeholder?: string;
  disabled?: boolean;
  password?: boolean;
  onChange?: (value: string) => void;
  onSubmit?: (value: string) => void;
}

export interface TextareaProps {
  value?: string;
  placeholder?: string;
  rows?: number;
  autoResize?: boolean;
  disabled?: boolean;
  onChange?: (value: string) => void;
  onSubmit?: (value: string) => void;
}

export interface SwitchProps {
  checked?: boolean;
  disabled?: boolean;
  onChange?: (checked: boolean) => void;
}

export interface CheckboxProps {
  checked?: boolean;
  label?: string;
  disabled?: boolean;
  onChange?: (checked: boolean) => void;
}

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps {
  value?: string;
  options: SelectOption[];
  placeholder?: string;
  disabled?: boolean;
  onChange?: (value: string) => void;
}

export interface NumberInputProps {
  value?: number;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  onChange?: (value: number) => void;
}

export interface TabItem {
  key: string;
  label: string;
  badge?: string;
}

export interface TabsProps {
  value?: string;
  items: TabItem[];
  onChange?: (key: string) => void;
}

export interface BadgeProps {
  label: string;
  tone?: 'default' | 'brand' | 'success' | 'warning' | 'danger';
  icon?: string;
  closable?: boolean;
  onPress?: () => void;
  onClose?: () => void;
}

export interface CardProps {
  title?: string;
  cover?: string;
  padding?: SpacingToken;
  width?: number;
  height?: number | 'full';
  hoverable?: boolean;
  dragWindow?: boolean;
  onPress?: () => void;
}

export interface CollapsibleProps {
  title: string;
  icon?: string;
  defaultOpen?: boolean;
  status?: 'running' | 'done' | 'error';
}

export interface ProgressProps {
  /** 0..1 或 0..100（>1 自动按百分比除以 100） */
  value?: number;
  label?: string;
  indeterminate?: boolean;
  tone?: 'default' | 'success' | 'warning' | 'danger';
}

export interface ImageProps extends Omit<LayoutProps, 'onPress'> {
  src?: string;
  /** 主图加载中或加载失败时的回退图（避免切换闪烁） */
  fallbackSrc?: string;
  aspectRatio?: number;
  fit?: 'cover' | 'contain';
  blur?: boolean | number;
  previewable?: boolean;
  /** 透明背景，不绘制默认封面占位底色 */
  transparent?: boolean;
  /** 可交互时绘制浅色悬停描边 */
  hoverBorder?: boolean;
  /** 图片尚未加载或加载失败时不占布局空间 */
  hideWhenEmpty?: boolean;
  onPress?: (src: string) => void;
  onLoad?: (info: { src: string; width: number; height: number }) => void;
  onError?: (info: { src: string }) => void;
}

export interface AvatarProps {
  src?: string;
  fallback?: string;
  size?: number;
}

export interface EmptyProps {
  icon?: string;
  title?: string;
  description?: string;
}

export interface SkeletonProps {
  lines?: number;
  height?: number;
}

export interface TooltipProps {
  content: string;
}

export interface TableColumn {
  key: string;
  label: string;
}

export interface TableProps {
  columns: Array<TableColumn | string>;
  rows: Array<Record<string, unknown>>;
  onRowPress?: (ev: { index: number; row: Record<string, unknown> }) => void;
}

export interface DividerProps {
  subtle?: boolean;
  marginY?: SpacingToken;
}

export interface SpacerProps {}

export interface IconProps {
  name: string;
  /** 图标不存在时的回退名称，传空字符串可跳过渲染 */
  fallback?: string;
  size?: number;
  tone?: ToneToken;
  color?: ColorValue;
}

export interface NativeSlotProps {
  /** 宿主页面暴露的原生插槽名（如游戏详情页的 "back" / "cover" / "toolbar" / "tags" / "summary"） */
  name: string;
  [prop: string]: unknown;
}

export interface HeroBannerMetaItem {
  icon?: string;
  text: string;
}

export interface HeroBannerRating {
  score: number;
  total?: number;
  played?: number;
}

export interface HeroBannerPager {
  current: number;
  total: number;
}

/** SDK 内置基于底层原语组合的标准 Hero 头图模板属性 */
export interface HeroBannerProps {
  height?: number;
  contentMaxWidth?: number;
  coverWidth?: number;
  toolbarMaxWidth?: number;
  banner?: string;
  fallbackBanner?: string;
  logo?: string;
  title?: string;
  subtitle?: string;
  meta?: Array<HeroBannerMetaItem | string> | string;
  rating?: HeroBannerRating;
  blur?: boolean | number;
  backSlot?: string;
  coverSlot?: string;
  toolbarSlot?: string;
  pager?: HeroBannerPager;
  onPrev?: () => void;
  onNext?: () => void;
  label?: string;
}

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

/**
 * ctx.theme.extend 可覆盖的宿主语义色板（取值为 "#rgb" / "#rgba" / "#rrggbb" / "#rrggbbaa"）。
 * 只影响读取这些语义令牌的宿主组件；宿主中直接引用固定色阶的少量装饰色不受影响。
 */
export interface ThemePaletteTokens {
  // 文本
  text?: string;
  textStrong?: string;
  textMuted?: string;
  textFaint?: string;
  // 窗口骨架：主区域底色、顶栏、窗口按钮
  appBg?: string;
  topbarBg?: string;
  topbarBorder?: string;
  controlIcon?: string;
  controlHover?: string;
  // 侧边栏导航
  sidebarBg?: string;
  sidebarBorder?: string;
  navText?: string;
  navHover?: string;
  navActiveBg?: string;
  navActive?: string;
  // 输入框
  inputBg?: string;
  inputBorder?: string;
  inputFocus?: string;
  inputIcon?: string;
  placeholder?: string;
  // 次要按钮（secondary）与工具栏按钮
  toolBtnBg?: string;
  toolBtnBorder?: string;
  toolBtnHover?: string;
  toolBtnIcon?: string;
  toolBtnText?: string;
  toolBtnActiveBg?: string;
  toolBtnActiveBorder?: string;
  toolBtnActiveText?: string;
  // 主按钮（primary）与开关打开态
  primaryBtn?: string;
  primaryBtnHover?: string;
  primaryBtnText?: string;
  // 卡片与封面占位
  cardBg?: string;
  cardBorder?: string;
  cardBorderHover?: string;
  coverBg?: string;
  // 滚动条
  scrollThumb?: string;
  scrollThumbHover?: string;
  // 面板、菜单、分隔线
  panelBg?: string;
  panelBorder?: string;
  divider?: string;
  menuHover?: string;
  menuText?: string;
  sectionLabel?: string;
  // 筛选标签（chip）
  chipBg?: string;
  chipText?: string;
  chipHover?: string;
  chipActiveBg?: string;
  chipActiveText?: string;
  chipRing?: string;
  // 下拉选择、列表
  selectBg?: string;
  selectBorder?: string;
  selectText?: string;
  listFocus?: string;
  listSelected?: string;
  switchOff?: string;
  // 提示气泡
  tooltipBg?: string;
  tooltipText?: string;
  tooltipBorder?: string;
}

export interface ThemeTokens {
  id?: string;
  light?: ThemePaletteTokens;
  dark?: ThemePaletteTokens;
  glass?: {
    /**
     * true：开启玻璃模式（可配合 backgroundImage 使用插件自带背景）。
     * false：锁定配色，本插件启用期间不使用用户的自定义背景与玻璃模式（设置页会提示），适合对配色有严格要求的设计主题。
     * 多个插件同时声明时以后注册者为准。
     */
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
  | 'game.detail.hero'
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
  order?: number;
  hideTabs?: string[];
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
  db: {
    exec(sql: string, params?: unknown[]): Promise<{ rowsAffected: number; lastInsertId: number }>;
    query<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T[]>;
    queryOne<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T | null>;
    transaction(statements: Array<{ sql: string; params?: unknown[] }>): Promise<Array<{ rowsAffected: number; lastInsertId: number }>>;
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
    getAuthToken(source: string): Promise<string>;
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
  audio: {
    play(src: string, opts?: { volume?: number } | number): void;
    stop(src?: string): void;
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
