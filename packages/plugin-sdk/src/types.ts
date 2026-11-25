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
  /** 实心强调色（跟随主题插件的 accent；默认是宿主紫色） */
  | 'primary'
  | 'brand'
  /** 浅底上的强调文字 / 图标（跟随 accentText） */
  | 'accent'
  | 'accentText'
  /** 柔和强调底（选中、标签）与其上的文字 */
  | 'accentSoft'
  | 'accentSoftText'
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
  /** 顶部安全区（DIP），通常直接透传 game.detail.hero 槽位属性里的 safeTop：宿主隐藏顶栏时返回胶囊据此下移 */
  safeTop?: number;
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

export type GameStatus = 'not_started' | 'want_to_play' | 'playing' | 'completed' | 'on_hold' | 'dropped';
export type LaunchMode = 'normal' | 'admin' | 'steam' | 'compatibility';

export interface GameMetadataSource {
  game_id: string;
  source_type: string;
  source_id: string;
  cached_at?: string;
  created_at?: string;
  updated_at?: string;
}

export interface Game {
  id: string;
  name: string;
  aliases?: string[];
  cover_url?: string;
  /** 刮削封面的远程源地址（系统维护，只读） */
  cover_source_url?: string;
  company?: string;
  summary?: string;
  /** 评分，统一按 10 分制 */
  rating?: number;
  release_date?: string;
  /** 启动路径 */
  path?: string;
  /** 游戏根目录 */
  game_directory?: string;
  /** 存档目录 */
  save_path?: string;
  /** 实际监控的进程名（启动器与游戏进程不同时使用） */
  process_name?: string;
  /** macOS / Linux：兼容层类型、追加参数与 prefix */
  wine_runner?: string;
  wine_args?: string;
  wine_prefix?: string;
  launch_mode?: LaunchMode;
  /** Steam 关联（由 Steam 集成维护，只读） */
  steam_launch_id?: string;
  steam_launch_kind?: string;
  steam_user_id?: string;
  steam_launch_options?: string;
  status?: GameStatus | string;
  /** 默认元数据来源 */
  source_type?: string;
  source_id?: string;
  /** 全部元数据来源（只读；增删改用 ctx.games.sources） */
  metadata_sources?: GameMetadataSource[];
  use_locale_emulator?: boolean;
  use_magpie?: boolean;
  is_nsfw?: boolean;
  metadata_locked?: boolean;
  cached_at?: string;
  created_at?: string;
  updated_at?: string;
  last_played_at?: string;
}

/**
 * ctx.games.update / batchUpdate / create 可写的字段。
 * id、时间戳、cover_source_url、metadata_sources 与 Steam 关联字段会被忽略；未知字段会报错。
 */
export type GamePatch = Partial<
  Pick<
    Game,
    | 'name'
    | 'aliases'
    | 'cover_url'
    | 'company'
    | 'summary'
    | 'rating'
    | 'release_date'
    | 'path'
    | 'game_directory'
    | 'save_path'
    | 'process_name'
    | 'wine_runner'
    | 'wine_args'
    | 'wine_prefix'
    | 'launch_mode'
    | 'steam_launch_options'
    | 'status'
    | 'source_type'
    | 'source_id'
    | 'use_locale_emulator'
    | 'use_magpie'
    | 'is_nsfw'
    | 'metadata_locked'
  >
>;

export interface GameCreateInput extends GamePatch {
  name: string;
  /** 用户标签 */
  tags?: string[];
  /** 关联的元数据来源 */
  metadata_sources?: Array<{ source: string; source_id: string }>;
  /** 加入的收藏夹 */
  category_ids?: string[];
}

export interface GameTag {
  id: string;
  game_id: string;
  name: string;
  /** 'user' 为用户标签，其余为刮削来源 */
  source: string;
  weight: number;
  is_spoiler: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface PlaySession {
  id: string;
  game_id: string;
  start_time: string;
  end_time?: string;
  /** 秒 */
  duration: number;
  updated_at?: string;
}

export interface GameProgress {
  id?: string;
  game_id: string;
  chapter?: string;
  route?: string;
  progress_note?: string;
  spoiler_boundary?: 'none' | 'chapter_end' | 'route_end' | 'full';
  updated_at?: string;
}

export interface GameReview {
  game_id: string;
  /** 1–10，null 表示未评分 */
  rating: number | null;
  content: string;
  is_spoiler: boolean;
  created_at?: string;
  updated_at?: string;
}

export type MetadataUpdateField = 'name' | 'aliases' | 'cover' | 'company' | 'summary' | 'rating' | 'release_date' | 'tags';

/** 启动选项：null / 缺省表示沿用游戏自身的配置 */
export interface LaunchOptions {
  use_locale_emulator?: boolean | null;
  use_magpie?: boolean | null;
  run_as_admin?: boolean | null;
  wine_runner?: string | null;
  wine_args?: string | null;
  wine_prefix?: string | null;
  use_steam?: boolean | null;
  use_compatibility?: boolean | null;
}

/** launch/plan 拦截器可以修改的启动计划 */
export interface LaunchPlan {
  /** 要启动的程序（Steam 游戏是 steam.exe） */
  file: string;
  args: string[];
  /** 工作目录 */
  dir: string;
  /** 追加的环境变量；Windows 经 ShellExecuteEx 启动，不生效 */
  env: Record<string, string>;
  run_as_admin: boolean;
}

/** 拦截器返回值：undefined 表示不改；cancel 为 true 时取消启动并停止后续拦截器 */
export interface InterceptCancel {
  cancel: true;
  /** 显示给用户的原因；缺省为“启动被插件取消” */
  reason?: string;
}

export type LaunchPrepareResult = { options?: LaunchOptions } | InterceptCancel | undefined;
export type LaunchPlanResult = { plan?: Partial<LaunchPlan> } | InterceptCancel | undefined;

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
  // 强调色：选中、当前状态与品牌点缀（插件 UI 的 primary / brand / accent / accentSoft 颜色令牌也取它们）
  accent?: string;
  accentText?: string;
  accentSoft?: string;
  accentSoftText?: string;
  // 游戏详情页标签栏：默认文字、悬停文字、选中文字、选中下划线
  tabText?: string;
  tabHover?: string;
  tabActive?: string;
  tabIndicator?: string;
  // 游戏详情页工具栏的图标按钮（状态、打开目录、攻略等）：底色、悬停底色、图标、悬停图标、当前状态外圈与外圈间隙
  actionBtnBg?: string;
  actionBtnHover?: string;
  actionBtnIcon?: string;
  actionBtnIconHover?: string;
  actionBtnRing?: string;
  actionBtnRingOffset?: string;
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
  /** 下拉菜单当前项的底色（可设为 "#00000000" 只用对勾标记当前项） */
  menuSelected?: string;
  /** 下拉菜单当前项的对勾颜色 */
  menuCheck?: string;
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
  // 开关：关闭态底色、滑块颜色（打开态取 primaryBtn）
  switchOff?: string;
  switchThumb?: string;
  // 提示气泡
  tooltipBg?: string;
  tooltipText?: string;
  tooltipBorder?: string;
  // 危险按钮（删除、清空等破坏性操作的实心按钮）
  dangerBtn?: string;
  dangerBtnHover?: string;
  dangerBtnText?: string;
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
  /**
   * 宿主全局圆角（DIP），作用于整个界面而不只是某个页面。未声明的项保留各组件自身的圆角。
   * - control：所有按钮、图标按钮、输入框、下拉选择、数字输入、分段切换、侧边栏导航项、菜单项与详情页工具栏 / 启动按钮
   *   （默认 4~8，详情页图标按钮为全圆）
   * - panel：所有浮层与容器面板——下拉菜单、选择列表、弹出层、提示气泡、日期选择、筛选抽屉、数据表格等（默认 6~12）
   * 卡片、封面、标签胶囊与头像等装饰性形状不受影响。
   */
  shape?: {
    control?: number;
    panel?: number;
  };
  /**
   * 替换侧边栏与顶栏的品牌标志。图片路径相对插件目录（如 'assets/wordmark.svg'），支持 svg / png / webp，仅限本地文件。
   * wordmarkDark / iconDark 为深色模式专用图，未设置时深浅色共用。
   * hideWordmark / hideIcon 为 true 时隐藏字标 / 图标（顶栏在字标隐藏时改放图标）。
   */
  brand?: {
    wordmark?: string;
    wordmarkDark?: string;
    icon?: string;
    iconDark?: string;
    hideWordmark?: boolean;
    hideIcon?: boolean;
  };
  /**
   * 窗口材质：让侧边栏透出系统的半透明磨砂材质，再叠一层有色半透明着色。
   * - sidebar：'acrylic' 磨砂（模糊窗口后面的内容）或 'mica' 云母（取桌面壁纸色调，不模糊），未声明时不使用。
   *   Windows 11 22H2 起支持两者；macOS 统一用系统侧边栏材质；Linux、旧版 Windows 没有材质，侧边栏保持不透明的 sidebarBg。
   * - tint / tintDark：叠在材质上的着色（#RRGGBBAA，透明度决定材质透出多少），tintDark 为深色模式专用；
   *   都未设置时取 sidebarBg 的 60% 不透明度。
   * 用户开启自定义背景（玻璃模式）时背景图盖住材质，不启用。Windows 上第一次启用需重启应用才显示
   * （窗口要在创建时带上材质），之后开关即时生效。多个插件声明时以后注册者为准。
   */
  material?: {
    sidebar?: 'acrylic' | 'mica';
    tint?: string;
    tintDark?: string;
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
  /**
   * 启动拦截器：解析启动方式之前调用，可改写启动选项或取消启动。game 只读。
   * 需要等用户操作（如弹出 ctx.ui.select）时，注册时传入 `{ timeout }` 放宽超时（默认 3 秒，最多 10 分钟）。
   */
  'launch/prepare': (payload: { game: Game; options: LaunchOptions }) => LaunchPrepareResult | Promise<LaunchPrepareResult>;
  /**
   * 启动拦截器：启动计划生成之后、启动进程之前调用，可改写程序、参数、工作目录、环境变量，或取消启动。game 只读。
   * 返回的对象会深合并进当前载荷（数组整体替换），多个插件按插件顺序依次执行。
   */
  'launch/plan': (payload: { game: Game; plan: LaunchPlan }) => LaunchPlanResult | Promise<LaunchPlanResult>;
  /** 通知：拦截器都已执行，进程即将启动 */
  'game/before-launch': (payload: { game: Game; plan: LaunchPlan }) => void;
  /** 通知：进程已启动，游玩记录已创建 */
  'game/launched': (payload: { game: Game; sessionId: string }) => void;
  /** 通知：识别到实际运行的游戏进程（或进程交接到新进程） */
  'game/running': (payload: { game: Game; sessionId: string; pid: number; processName: string; reason: string }) => void;
  'session/ended': (payload: { gameId: string; sessionId: string; duration: number }) => void;
  'metadata/before-search': (payload: { name: string; source: string }) => { name: string } | undefined;
  'metadata/after-fetch': (payload: { source: string; result: MetadataResult }) => MetadataResult | undefined;
}

export interface ProcessOptions {
  /** 工作目录 */
  cwd?: string;
  /** 追加的环境变量 */
  env?: Record<string, string>;
  /** 毫秒；exec 默认 60000，0 表示不限。spawn 不使用 */
  timeout?: number;
  /** 写入标准输入的内容 */
  input?: string;
  /** 为 true 时 command 是一整行命令，交给系统 shell（Windows 为 cmd.exe，其他平台为 /bin/sh） */
  shell?: boolean;
  /** 输出编码：auto（默认，非 UTF-8 时按系统代码页解码，如 GBK）| utf8 | gbk | big5 | shift_jis | euc-kr */
  encoding?: string;
}

export interface ExecResult {
  /** 退出码；未能取得时为 -1 */
  code: number;
  stdout: string;
  stderr: string;
  timedOut: boolean;
  /** 输出超过 8 MiB 被截断 */
  truncated: boolean;
}

export interface ProcessHandle {
  pid: number;
  onStdout(fn: (text: string) => void): void;
  onStderr(fn: (text: string) => void): void;
  onExit(fn: (result: { code: number; error?: string }) => void): void;
  write(data: string): void;
  closeStdin(): void;
  /** 结束进程及其子进程 */
  kill(): void;
}

export interface ConfirmOptions {
  title?: string;
  message?: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
}

export interface SelectOptions extends ConfirmOptions {
  options: Array<{ value: string; label: string; description?: string; icon?: string }>;
  /** 默认选中的 value */
  default?: string;
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
    /** 新增游戏（不走导入向导）；返回入库后的游戏 */
    create(input: GameCreateInput): Promise<Game>;
    update(id: string, patch: GamePatch): Promise<Game>;
    batchUpdate(patches: Array<{ id: string; patch: GamePatch }>): Promise<{ updated: number; failed?: Array<{ id: string; error: string }> }>;
    delete(id: string): Promise<void>;
    batchDelete(ids: string[]): Promise<{ deleted: number }>;
    /** 把用户标签设为给定列表（刮削标签不受影响） */
    setTags(id: string, tags: string[]): Promise<GameTag[]>;
    /** 重新拉取元数据：指定 source 时只用该来源，指定 fields 时只更新这些字段 */
    refreshMetadata(id: string, opts?: { source?: string; fields?: MetadataUpdateField[] }): Promise<Game>;
    /** 下载远程封面并设为本地封面，返回本地路径 */
    setCoverFromURL(id: string, url: string): Promise<string>;
    /** 启动游戏；传入 options 时覆盖游戏自身的启动配置 */
    start(id: string, options?: LaunchOptions): Promise<boolean>;
    onChanged(fn: (payload: unknown) => void): Disposer;
    tags: {
      list(gameId: string): Promise<GameTag[]>;
      add(gameId: string, name: string): Promise<GameTag[]>;
      remove(tagId: string): Promise<void>;
      /** 按关键字搜索库内已有的标签名 */
      search(query: string): Promise<string[]>;
      gameIds(tagName: string): Promise<string[]>;
    };
    sources: {
      list(gameId: string): Promise<GameMetadataSource[]>;
      upsert(gameId: string, source: string, sourceId: string): Promise<GameMetadataSource[]>;
      setDefault(gameId: string, source: string): Promise<void>;
      remove(gameId: string, source: string): Promise<void>;
    };
    sessions: {
      list(gameId: string): Promise<PlaySession[]>;
      add(gameId: string, input: { start_time: string; duration: number }): Promise<PlaySession>;
      update(session: PlaySession): Promise<void>;
      remove(sessionId: string): Promise<void>;
    };
    progress: {
      get(gameId: string): Promise<GameProgress | null>;
      list(gameId: string): Promise<GameProgress[]>;
      save(progress: GameProgress): Promise<GameProgress>;
      clear(gameId: string): Promise<void>;
    };
    review: {
      get(gameId: string): Promise<GameReview | null>;
      save(review: GameReview): Promise<GameReview>;
    };
  };
  categories: {
    list(): Promise<Category[]>;
    get(id: string): Promise<Category>;
    /** 游戏所在的收藏夹 */
    listByGame(gameId: string): Promise<Category[]>;
    /** 收藏夹里的游戏 */
    games(categoryId: string): Promise<Game[]>;
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
    /**
     * 宿主设置页配置的 AI 服务凭据（OpenAI 兼容端点），供插件自带的 agent 运行时（如 pi）直接使用。
     * 未配置时 configured 为 false、apiKey 为空。
     */
    getCredentials(): { configured: boolean; provider: string; baseUrl: string; apiKey: string; model: string };
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
  /**
   * 注册事件监听或拦截器。timeout（毫秒）只对拦截器（launch/*）生效：默认 3000，最多 600000。
   */
  on<K extends keyof EventMap>(event: K, listener: EventMap[K], opts?: { prepend?: boolean; timeout?: number }): Disposer;
  ui: {
    slot(name: SlotName, def: SlotDef | DecorationOptions): Disposer;
    toast(msg: string, type?: 'info' | 'success' | 'error' | 'loading'): void;
    navigate(path: string, params?: Record<string, unknown>): void;
    openDrawer(id: string, props?: Record<string, unknown>): void;
    closeDrawer(): void;
    /** 确认对话框：确认为 true，取消（或插件卸载）为 false。主窗口隐藏时会先显示主窗口 */
    confirm(opts: ConfirmOptions): Promise<boolean>;
    /** 单选对话框：返回选中项的 value，取消为 null */
    select(opts: SelectOptions): Promise<string | null>;
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
  /**
   * 执行本机命令（插件完全信任，与主程序同权限；宿主不做沙箱）。
   * 让 AI 调用命令的插件应先用 ctx.ui.confirm 让用户逐条确认。插件卸载时 spawn 的进程树会被结束。
   */
  process: {
    /** 当前系统：windows | darwin | linux */
    platform: string;
    exec(command: string, opts?: ProcessOptions): Promise<ExecResult>;
    exec(command: string, args: string[], opts?: ProcessOptions): Promise<ExecResult>;
    spawn(command: string, opts?: ProcessOptions): ProcessHandle;
    spawn(command: string, args: string[], opts?: ProcessOptions): ProcessHandle;
  };
  paths: {
    plugin: string;
    data: string;
  };
  effect(setup: () => void | Disposer): Disposer;
}
