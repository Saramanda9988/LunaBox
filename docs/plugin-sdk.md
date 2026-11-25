# LunaBox Plugin SDK 参考文档

> 本文档维护 `lunabox` 插件 SDK（`apiVersion: 1`）当前提供的**全部功能、UI 原语、组合组件、插槽协议与 TypeScript 类型**。
> 插件系统底层架构与线程模型见 [plugin-system.md](plugin-system.md)。

**源码真值位置：**
- TypeScript 类型与导出：[packages/plugin-sdk/src/types.ts](../packages/plugin-sdk/src/types.ts)、[packages/plugin-sdk/src/index.ts](../packages/plugin-sdk/src/index.ts)、[packages/plugin-sdk/src/jsx-runtime.ts](../packages/plugin-sdk/src/jsx-runtime.ts)
- 内嵌 JS 运行时：[packages/backend/plugin/sdk_runtime.go](../packages/backend/plugin/sdk_runtime.go)
- 描述树白名单与校验：[packages/backend/plugin/uitree.go](../packages/backend/plugin/uitree.go)
- 原生渲染器实现：[apps/desktop/ui/pluginui/renderer.go](../apps/desktop/ui/pluginui/renderer.go)
- CLI 打包工具：[packages/plugin-sdk/bin/lunabox-plugin.mjs](../packages/plugin-sdk/bin/lunabox-plugin.mjs)

---

## 1. 设计分层：原语 vs 组合组件

LunaBox 插件界面采用「**底层原语在 Go 渲染器、高阶模板在 SDK JS 层**」的分层设计：

1. **底层白名单原语（Go 渲染器）**：提供 32 个通用组件（布局、层叠定位 `Stack`、通用颜色/渐变/阴影、图片、表单、富文本、`Icon`、`NativeSlot` 等），宿主渲染器不包含任何特定页面或数据源的业务版式代码。
2. **SDK 组合组件（JS 层）**：如 `HeroBanner` 等标准模板完全由底层原语在 SDK 运行时中组合实现；插件既可以直接复用模板，也可以直接用 `Stack` + `Box` + `Image` + `NativeSlot` 等原语自由拼装全新版式。
3. **宿主原生嵌入（`NativeSlot`）**：页面宿主可通过 `NativeSlot` 暴露原生交互区（例如游戏详情页的返回按钮、封面、启动栏、标签、简介），让插件重构布局的同时保留宿主原生能力。

---

## 2. 插件包结构、清单与构建

### 2.1 最小目录结构

```text
my-plugin/
├── plugin.json        # 插件清单（必需）
├── main.js            # 入口 ESM 文件（或由 CLI 打包生成）
├── tsconfig.json      # 使用 TypeScript / TSX 时配置
├── src/index.tsx      # 源码入口
├── locales/           # 可选多语言 JSON
└── assets/            # 可选静态资源（图片、音频、字体等）
```

### 2.2 入口函数与 `tsconfig.json`

插件入口默认导出启动函数 `setup(ctx: Context)`，可返回清理函数（也可用 `ctx.effect` 与各注册 API 返回的 `Disposer` 自动在卸载/重载时回收）：

```tsx
import type { Context } from 'lunabox';
import { Column, Text } from 'lunabox';

export default function setup(ctx: Context) {
  ctx.ui.slot('game.detail.aside', {
    id: 'my-aside',
    render: ({ game }) => (
      <Column gap="sm" surface="card" padding="md" radius="md">
        <Text text={game.name} weight="semibold" />
      </Column>
    ),
  });
}
```

推荐 `tsconfig.json`：

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "jsxImportSource": "lunabox",
    "strict": true,
    "noEmit": true
  }
}
```

### 2.3 CLI 命令

```bash
# 使用 esbuild 打包 src/index.tsx|ts -> manifest.main（默认 main.js）
# 自动将 lunabox、lunabox/jsx-runtime、node:buffer 标为 external
# 默认 platform=browser（moejs 提供的是 Web API 而不是 Node 内置模块，OpenAI / Anthropic SDK 等会选用浏览器版实现）
# --minify 压缩输出；--platform=neutral 恢复旧行为
lunabox-plugin build [dir] [--minify] [--platform=browser|neutral]

# 打包为 <id>-<version>.zip（包含 plugin.json、main.js、locales/、assets/ 及自定义 icon）
lunabox-plugin pack [dir]
```

### 2.4 `PluginManifest`（`plugin.json`）

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `string` | 是 | 唯一插件 ID（小写字母开头，仅含小写字母、数字与连字符） |
| `version` | `string` | 是 | SemVer 版本号，如 `"1.0.0"` |
| `apiVersion` | `number` | 是 | 插件 API 主版本，当前为 `1` |
| `name` | `Record<string, string>` | 是 | 多语言插件名称（如 `{ "zh-CN": "...", "en-US": "..." }`） |
| `main` | `string` | 否 | 入口文件名（相对插件根目录，默认 `"main.js"`） |
| `engines` | `{ lunabox?: string }` | 否 | 兼容宿主版本范围 |
| `description` | `Record<string, string>` | 否 | 多语言简介 |
| `author` | `string` | 否 | 作者名 |
| `homepage` | `string` | 否 | 主页 URL |
| `repository` | `string` | 否 | 仓库 URL |
| `license` | `string` | 否 | 开源协议（如 `"MIT"`） |
| `icon` | `string` | 否 | 相对插件目录的图片路径（如 `"assets/icon.svg"`）或内置 MDI 图标名 |
| `iconBg` | `string` | 否 | 可选图标背景色（如 `"#fef2f2"`） |
| `keywords` | `string[]` | 否 | 关键词标签 |

---

## 3. 运行时导出（`'lunabox'` 与 `'lunabox/jsx-runtime'`）

### 3.1 JSX 与基础类型

- `Fragment: symbol`（`Symbol.for('lunabox.Fragment')`）
- `h(type, props?, ...children): VNode`
- `jsx(type, props?, key?): VNode`、`jsxs`（自动 JSX 运行时入口）
- `type Disposer = () => void`
- `type LocalizedString = string | Record<string, string>`
- `type ComponentFactory<P>`
- `type VNode`：描述树节点、函数组件、Fragment 或文本/数字/布尔/空值

### 3.2 Hooks

仅允许在组件 `render` 执行期间调用，同一微任务内多次 `setState` 自动合并为一次描述树提交；实例卸载时自动执行所有 `useEffect` 清理函数。

```ts
function useState<T>(initial: T | (() => T)): [T, (next: T | ((prev: T) => T)) => void];
function useEffect(setup: () => void | (() => void), deps?: readonly unknown[]): void;
function useRef<T>(initial: T): { current: T };
function useMemo<T>(factory: () => T, deps?: readonly unknown[]): T;
```

### 3.3 高阶辅助工具

#### `defineThemePreset(preset: ThemeTokens): ThemeTokens`
对主题配置提供类型校验与 IDE 自动补全。

#### `createPagination(fetcher, options?)`
配合 `ScrollArea` 的 `onReachBottom` 构建分页 Hook：

```ts
function createPagination<T>(
  fetcher: (args: { page: number; pageSize: number; query: string }) => Promise<T[] | { items: T[]; hasMore: boolean }>,
  options?: { pageSize?: number } // 默认 24
): (queryKey: string) => {
  items: T[];
  page: number;
  loading: boolean;
  hasMore: boolean;
  error: string | null;
  loadMore: () => void;
  refresh: () => void;
};
```

#### `createAIChatStore(ctx, options?)`
管理多轮 AI 流式对话与 Tool Call 状态机的轻量 Store：

```ts
function createAIChatStore(
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
  getState(): {
    messages: Array<{
      role: string;
      content: string;
      toolCalls?: Array<{ id: string; name: string; status: string; arguments?: unknown; result?: string }>;
    }>;
    streaming: boolean;
  };
  subscribe(fn: () => void): () => void;
  send(content: string): void;
  abort(): void;
  clear(): void;
};
```

### 3.4 SDK 组合模板组件：`HeroBanner`

`HeroBanner` 在 SDK JS 层完全使用 `Stack` / `Box` / `Column` / `Row` / `Image` / `Text` / `Icon` / `NativeSlot` 组合而成，提供标准全宽头图 + 双层渐变遮罩 + 返回胶囊 + 透明徽标自适应 + 元信息行 + 五星评分 + 启动栏 + 右侧封面与头图翻页器布局。遮罩与文字明暗自适应：浅色主题下头图向页面底色（偏白）融出、文字为深色，深色主题下为深色遮罩与白色文字。

```ts
interface HeroBannerMetaItem {
  icon?: string;
  text: string;
}

interface HeroBannerRating {
  score: number;  // 10 分制，内部折算为 5 颗星
  total?: number; // 评分人数
  played?: number; // 玩过人数
}

interface HeroBannerPager {
  current: number;
  total: number;
}

interface HeroBannerProps {
  height?: number;            // 默认 560（范围 280..760）
  contentMaxWidth?: number;   // 前景内容最大宽度，默认 1200（水平居中）
  safeTop?: number;           // 顶部安全区：透传槽位属性 safeTop，宿主隐藏顶栏时返回胶囊下移避开拖动区
  coverWidth?: number;        // 右侧封面宽度，默认 196（透传给 coverSlot）
  toolbarMaxWidth?: number;   // 启动栏最大宽度（默认 contentMaxWidth - coverWidth - 40）
  banner?: string;            // 背景头图 URL 或本地路径
  fallbackBanner?: string;    // 主图加载中或失败时的回退图，避免切换闪烁
  logo?: string;              // 透明徽标 URL/路径（自动按自然比例限制在 460×132 内）
  title?: string;             // 主标题（徽标加载成功后退居副行展示）
  subtitle?: string;          // 副标题
  meta?: Array<HeroBannerMetaItem | string> | string; // 带可选 MDI 图标的元信息行
  rating?: HeroBannerRating;  // 大字号评分 + 五星 + 统计人数
  blur?: boolean | number;    // 背景高斯模糊（true 默认 20）
  backSlot?: string;          // 顶部左上角胶囊原生槽位名（如 "back"）
  coverSlot?: string;         // 右下角封面原生槽位名（如 "cover"）
  toolbarSlot?: string;       // 左下角操作区原生槽位名（如 "toolbar"）
  pager?: HeroBannerPager;    // 头图切换页码
  onPrev?: () => void;        // 上一张头图
  onNext?: () => void;        // 下一张头图
  label?: string;             // 无障碍标签
}
```

---

## 4. 样式与设计令牌系统

### 4.1 间距、圆角、排版与表面材质

| 类型 | 可选值 | 实际映射 |
| --- | --- | --- |
| `SpacingToken` | `'none' \| 'xs' \| 'sm' \| 'md' \| 'lg' \| 'xl' \| number` | `0` / `4` / `8` / `12` / `16` / `24` / 任意像素值 |
| `RadiusToken` | `'sm' \| 'md' \| 'lg' \| 'full' \| number` | `6` / `10` / `14` / `999` / 任意像素值 |
| `SurfaceToken` | `'glass' \| 'glass-card' \| 'panel' \| 'card' \| 'chip' \| 'subtle'` | 跟随宿主明暗与毛玻璃模式的语义表面底色与描边 |
| `AlignToken` | `'start' \| 'center' \| 'end' \| 'stretch'` | 交叉轴对齐 |
| `JustifyToken` | `'start' \| 'center' \| 'end' \| 'between' \| 'space-between'` | 主轴分布 |
| `TextSizeToken` | `'xs' \| 'sm' \| 'base' \| 'lg' \| 'xl' \| '2xl' \| '3xl' \| '4xl' \| number` | 宿主主题字阶；`2xl`=24/32、`3xl`=30/38、`4xl`=38/44；数值时默认 `lineHeight = size * 1.25` |
| `FontWeightToken` | `'normal' \| 'medium' \| 'semibold' \| 'bold'` | 对应主题字重 |
| `ToneToken` | `'default' \| 'muted' \| 'faint' \| 'strong' \| 'brand' \| 'accent' \| 'success' \| 'warning' \| 'danger' \| 'light' \| 'light-muted'` | 文本与图标语义色（`light`/`light-muted` 用于深色背景/头图上） |

### 4.2 通用颜色系统（`ColorValue` / `ColorToken`）

支持 5 种写法，明暗主题与毛玻璃模式自动响应：

```ts
type ColorValue =
  | ColorToken                                              // 语义色板令牌
  | (string & {})                                           // "#hex" 或 "<token|#hex>@<0..1>"
  | { light?: ColorValue; dark?: ColorValue }               // 明暗主题自适应
  | { color?: ColorValue; token?: ColorValue; alpha?: number }; // 显式透明度
```

- **透明度后缀**：`"black@0.42"`、`"white@0.86"`、`"scrim@0.84"`、`"#1a1b26@0.8"`
- **十六进制**：`#rgb`、`#rgba`、`#rrggbb`、`#rrggbbaa`
- **内置 `ColorToken`（26 个）**：
  - 基础：`transparent`、`white`、`black`
  - 表面：`appBg`、`fadeBg`（毛玻璃模式下自动为 `appBg@0.88`，适合头图底部自然融出）、`scrim`（浅色为 `brand900`、深色为 `appBg`）、`panelBg`、`panelBorder`、`cardBg`、`cardBorder`、`chipBg`、`divider`
  - 文本：`text`、`textStrong`、`textMuted`、`textFaint`
  - 语义：`primary`、`brand`、`accent`、`accentText`、`accentSoft`、`accentSoftText`、`success`、`warning`、`warning400`、`danger`、`brand700`、`brand800`、`brand900`

### 4.3 线性渐变（`LinearGradientSpec`）

```ts
interface GradientStop {
  color: ColorValue;
  at?: number; // 0..1，省略时按序号等分
}

interface LinearGradientSpec {
  angle?: number; // 角度（0=自下而上，90=自左而右，180=自上而下；默认 180）
  stops: GradientStop[]; // 至少 2 个停靠点
}
```

### 4.4 投影（`ShadowSpec`）

```ts
type ShadowSpec =
  | 'sm'
  | 'md'
  | 'lg'
  | { x?: number; y?: number; blur: number; spread?: number; color?: ColorValue };
```

### 4.5 通用布局与样式属性（`LayoutProps`）

适用于 `Column`、`Row`、`Box`、`Stack`（以及 `Image` 的大部分定位与尺寸属性）：

| 分组 | 属性 | 类型 | 说明 |
| --- | --- | --- | --- |
| 间距 | `gap`、`padding`、`paddingX`、`paddingY`、`paddingTop/Right/Bottom/Left` | `SpacingToken` | 子项间距与内边距 |
| 外边距 | `margin`、`marginY`、`marginTop/Right/Bottom/Left` | `SpacingToken` | 外边距 |
| 居中 | `marginX`、`centerX` | `SpacingToken \| 'auto'` / `boolean` | `centerX: true` 等价于 `marginX: 'auto'` |
| 尺寸 | `width`、`height` | `number \| 'full'` | 固定像素或撑满父容器 |
| 约束 | `minWidth`、`maxWidth`、`minHeight`、`maxHeight` | `number` | 最小/最大宽高 |
| 弹性 | `grow`、`shrink`、`basis`、`wrap` | `number` / `boolean` | Flex 伸缩与换行 |
| 对齐 | `align`、`alignSelf`、`justify` | `AlignToken` / `JustifyToken` | 轴对齐 |
| 定位 | `position`、`inset`、`top`、`right`、`bottom`、`left`、`zIndex` | `'absolute'` / `number` | 配合 `Stack` 实现层叠与绝对定位 |
| 指针 | `pointerEvents` | `'none' \| 'auto'` | `'none'` 使装饰/遮罩层不拦截点击 |
| 背景 | `surface`、`background`、`hoverBackground`、`gradient` | 见上 | 语义表面、纯色、悬停底色、线性渐变 |
| 边框 | `border`、`borderColor`、`borderBottom`、`borderLeft` | `boolean \| number` / `ColorValue` | 描边与单边分隔线 |
| 装饰 | `radius`、`clip`、`shadow`、`opacity` | 见上 | 圆角、裁切、阴影、不透明度 |
| 交互 | `onPress`、`onContextMenu`、`disabled`、`dragWindow`、`label` | 函数 / 布尔 / 字符串 | 点击、右键、禁用、窗口拖动区、无障碍名 |

---

## 5. UI 原语组件白名单（32 个）

描述树限制：最大深度 `64`、最大节点数 `2000`、单字符串上限 `64 KiB`（见 [uitree.go](../packages/backend/plugin/uitree.go)）。

### 5.1 布局容器

| 组件 | Props 接口 | 关键属性与事件 |
| --- | --- | --- |
| `Column` | `ColumnProps` (`LayoutProps`) | 纵向 Flex 容器 |
| `Row` | `RowProps` (`LayoutProps`) | 横向 Flex 容器（支持 `wrap: true` 自动换行） |
| `Box` | `BoxProps` (`LayoutProps` + 扩展) | 通用盒容器；支持原生帧率弹簧按压动画：`springPress`、`squishX`（默认 1.05）、`squishY`（默认 0.88）、`flipX`、`onPressIn`、`onPressOut` |
| `Stack` | `StackProps` (`LayoutProps`) | 层叠容器，子节点可通过 `position: 'absolute'` + `inset`/`top`/`right`/`bottom`/`left` + `zIndex` 自由叠放 |
| `ScrollArea` | `ScrollAreaProps` | `padding`、`gap`、`height`、`grow`、`autoScrollBottom`、`onReachBottom` |
| `Grid` | `GridProps` | `cols`、`minColWidth`、`gap`（自适应网格） |
| `Divider` | `DividerProps` | `subtle`、`marginY` |
| `Spacer` | `SpacerProps` | 弹性空白占位 |

### 5.2 文本与富文本

| 组件 | Props 接口 | 关键属性与事件 |
| --- | --- | --- |
| `Text` | `TextProps` | `text`、`size`、`lineHeight`、`weight`、`tone`、`color`、`wrap`、`maxLines`、`selectable`、`label`、`margin*`、`centerX` |
| `Markdown` | `MarkdownProps` | `content`、`streaming`、`onLinkPress(href)`。由 MyGo `ui/markdown` 渲染：CommonMark + GitHub 扩展（表格、任务列表、删除线、自动链接）、代码高亮与复制；`streaming: true` 时按 AI 流式回复显示（增量解析、未写完的语法先按最终样子显示、逐词淡入）。不传 `onLinkPress` 时宿主直接打开 http/https/mailto 链接 |
| `CodeBlock` | `CodeBlockProps` | `code`、`language`、`copyable` |

### 5.3 表单与交互控件

| 组件 | Props 接口 | 关键属性与事件 |
| --- | --- | --- |
| `Button` | `ButtonProps` | `label`、`icon`、`variant`（`secondary`/`primary`/`danger`/`ghost`）、`size`（`sm`/`md`/`lg`）、`disabled`、`loading`、`onPress`/`onClick` |
| `IconButton` | `IconButtonProps` | `icon`、`label`、`size`（`sm`/`md`）、`active`、`disabled`、`onPress`/`onClick` |
| `Input` | `InputProps` | `value`、`placeholder`、`disabled`、`password`、`onChange(v)`（150ms 防抖）、`onSubmit(v)` |
| `Textarea` | `TextareaProps` | `value`、`placeholder`、`rows`、`autoResize`、`disabled`、`onChange(v)`、`onSubmit(v)` |
| `Switch` | `SwitchProps` | `checked`、`disabled`、`onChange(checked)` |
| `Checkbox` | `CheckboxProps` | `checked`、`label`、`disabled`、`onChange(checked)` |
| `Select` | `SelectProps` | `value`、`options: SelectOption[]`、`placeholder`、`disabled`、`onChange(v)` |
| `NumberInput` | `NumberInputProps` | `value`、`min`、`max`、`step`、`disabled`、`onChange(v)` |
| `Tabs` | `TabsProps` | `value`、`items: TabItem[]`（`{ key, label, badge? }`）、`onChange(key)` |

### 5.4 展示、媒体与反馈

| 组件 | Props 接口 | 关键属性与事件 |
| --- | --- | --- |
| `Image` | `ImageProps`（继承 `Omit<LayoutProps, 'onPress'>`） | `src`（本地路径或 http(s) URL，支持远程 `.svg` 与自动下采样）、`fallbackSrc`、`aspectRatio`、`fit`（`cover`/`contain`）、`blur`（`boolean \| number`）、`previewable`（点击打开原生图片查看器）、`transparent`、`hoverBorder`、`hideWhenEmpty`、`onPress(src)`、`onLoad({ src, width, height })`、`onError({ src })`。<br/>当未设 `width`/`height` 但设置了 `maxWidth`/`maxHeight` 时，自动按自然宽高比等比缩放且不放大超过原图。 |
| `Avatar` | `AvatarProps` | `src`、`fallback`、`size` |
| `Icon` | `IconProps` | `name`（内置 MDI 图标名）、`fallback`（空字符串可跳过渲染）、`size`、`tone`、`color` |
| `Badge` | `BadgeProps` | `label`、`tone`（`default`/`brand`/`success`/`warning`/`danger`）、`icon`、`closable`、`onPress`、`onClose` |
| `Card` | `CardProps` | `title`、`cover`、`padding`、`width`、`height`（`number \| 'full'`）、`hoverable`、`dragWindow`、`onPress` |
| `Collapsible` | `CollapsibleProps` | `title`、`icon`、`defaultOpen`、`status`（`running`/`done`/`error`） |
| `Progress` | `ProgressProps` | `value`（`0..1` 或 `0..100`）、`label`、`indeterminate`、`tone` |
| `Empty` | `EmptyProps` | `icon`、`title`、`description` |
| `Skeleton` | `SkeletonProps` | `lines`、`height` |
| `Tooltip` | `TooltipProps` | `content`（包裹单个子节点） |
| `Table` | `TableProps` | `columns: Array<TableColumn \| string>`、`rows`、`onRowPress({ index, row })` |

### 5.5 宿主原生嵌入：`NativeSlot`

```ts
interface NativeSlotProps {
  name: string;
  [prop: string]: unknown; // 透传给宿主原生插槽回调的自定义参数
}
```

当前游戏详情页（`game.detail.hero` / `game.detail.header` / `game.detail.tab` / `game.detail.aside`）暴露的原生插槽：

| `name` | 说明 | 支持透传参数 |
| --- | --- | --- |
| `"back"` | 宿主原生返回按钮 | 无 |
| `"cover"` | 宿主原生封面（含 NSFW 模糊遮罩与大图查看） | `width?: number`（覆盖默认封面宽度） |
| `"toolbar"` | 宿主原生启动栏与操作菜单 | `maxWidth?: number`（覆盖工具栏最大可用宽度） |
| `"tags"` | 宿主原生标签编辑与展示区 | 无 |
| `"summary"` | 宿主原生游戏简介区 | 无 |

---

## 6. 插槽系统（`SlotName` & `SlotDef`）

通过 `ctx.ui.slot(name, def)` 注册，返回 `Disposer`。

### 6.1 `SlotDef`

```ts
interface SlotDef {
  id: string;
  title?: string;
  icon?: string;
  group?: 'main' | 'bottom';            // sidebar.page 分组
  badge?: string;
  mode?: 'normal' | 'batch' | 'both';   // library.toolbar 显示模式
  defaultWidth?: number;                // workspace.drawer 默认宽度
  order?: number;                       // game.detail.tab 排序权重（<0 可成为首屏默认 Tab；>0 靠前）
  hideTabs?: string[];                  // game.detail.* 隐藏的内置 Tab key（如 ["progress"]）
  render?: (props: any) => VNode;
  onPress?: (payload?: any) => void | Promise<void>;
  run?: (game: Game) => void | Promise<void>;
  when?: (game: Game) => boolean;
}
```

### 6.2 全部 13 个插槽（`SlotName`）

| 插槽名 | 位置与用途 | 关键 props / 行为 |
| --- | --- | --- |
| `sidebar.page` | 侧边栏独立页面 | `title`、`icon`、`group`、`badge`、`render()` |
| `workspace.drawer` | 全局右侧伴随抽屉 | `defaultWidth`、`render(props)`，通过 `ctx.ui.openDrawer(id, props)` 打开 |
| `topbar.item` | 主窗口顶栏快捷入口 | `title`、`icon`、`onPress()` 或 `render()` |
| `library.add_menu` | 游戏库「添加游戏」下拉菜单项 | `title`、`icon`、`onPress()` |
| `library.toolbar` | 游戏库工具栏动作 | `mode`（`normal`/`batch`/`both`）、`title`、`icon`、`onPress(payload)` |
| `game.detail.hero` | 游戏详情页全宽顶栏接管 | `render({ game, gameId, contentWidth })`；支持 `hideTabs` 与全部 `NativeSlot`；无注册时回退原生顶栏 |
| `game.detail.header` | 游戏详情页原有页头扩展区 | `render({ game, gameId })` |
| `game.detail.tab` | 游戏详情页新增标签页 | `title`、`icon`、`order`、`hideTabs`、`render({ game, gameId })` |
| `game.detail.aside` | 游戏详情页常驻右侧信息栏 | 宽屏（`contentW >= 920`）右侧分栏、窄屏自动堆叠到底部；`render({ game, gameId })` |
| `game.actions` | 游戏卡片/详情右键及更多操作菜单 | `title`、`icon`、`when(game)`、`run(game)` |
| `downloads.header` | 下载管理页顶部扩展区 | `render()` |
| `app.decoration` | 应用全局前景/背景装饰层 | 传入 `DecorationOptions`（同 `ctx.theme.registerDecoration`） |
| `settings.section` | 插件详情页自定义设置区 | `title`、`icon`、`render()`（若只需表单推荐直接用 `ctx.config.define`） |

---

## 7. 宿主上下文 API（`Context` / `ctx`）

### 7.1 日志、多语言与生命周期

```ts
ctx.logger: {
  info(...args: unknown[]): void;
  warn(...args: unknown[]): void;
  error(...args: unknown[]): void;
}

ctx.i18n: {
  language: string;
  getLanguage(): string;
  t(key: string, args?: Record<string, unknown>): string;
  onChange(fn: (lang: string) => void): Disposer;
}

ctx.paths: {
  plugin: string; // 插件安装目录绝对路径
  data: string;   // 插件专属数据目录（plugins-data/<id>/）
}

ctx.effect(setup: () => void | Disposer): Disposer; // 注册随作用域自动回收的副作用
```

### 7.2 持久化存储：`ctx.storage` 与独立 SQLite `ctx.db`

- **`ctx.storage`**（轻量键值存储，适合小型状态与配置缓存）：
  ```ts
  ctx.storage.get<T = unknown>(key: string): T | undefined;
  ctx.storage.set(key: string, value: unknown): void;
  ctx.storage.delete(key: string): void;
  ctx.storage.keys(): string[];
  ```
- **`ctx.db`**（每插件隔离的独立 SQLite 数据库 `plugins-data/<id>/plugin.db`，支持 WAL 与事务，适合缓存接口数据、索引与结构化记录）：
  ```ts
  ctx.db.exec(sql: string, params?: unknown[]): Promise<{ rowsAffected: number; lastInsertId: number }>;
  ctx.db.query<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T[]>;
  ctx.db.queryOne<T = Record<string, unknown>>(sql: string, params?: unknown[]): Promise<T | null>;
  ctx.db.transaction(statements: Array<{ sql: string; params?: unknown[] }>): Promise<Array<{ rowsAffected: number; lastInsertId: number }>>;
  ```

### 7.3 声明式插件配置：`ctx.config`

注册后自动在「插件详情页」渲染原生设置表单并持久化：

```ts
interface ConfigFieldSchema {
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

interface ConfigSchema {
  title?: string;
  description?: string;
  icon?: string;
  fields: ConfigFieldSchema[];
}

ctx.config.define(schema: ConfigSchema | Record<string, Omit<ConfigFieldSchema, 'key'>>): Disposer;
ctx.config.get<T = Record<string, unknown>>(): T;
ctx.config.onChange<T = Record<string, unknown>>(fn: (next: T) => void): Disposer;
```

### 7.4 游戏库与分类：`ctx.games` / `ctx.categories`

类型 `Game`、`GamePatch`、`GameCreateInput`、`GameTag`、`PlaySession`、`GameProgress`、`GameReview`、`GameMetadataSource`、`LaunchOptions` 见 `src/types.ts`。

- `ctx.games`
  - 查询：`list(query?)`、`get(id)`、`findBySource(source, sourceId)`
  - 增删改：`create(input)`、`update(id, patch)`、`batchUpdate(patches)`（返回 `{ updated, failed }`）、`delete(id)`、`batchDelete(ids)`
  - 标签：`setTags(id, tags)`（只替换用户标签）、`tags.list/add/remove/search/gameIds`
  - 元数据：`sources.list/upsert/setDefault/remove`、`refreshMetadata(id, { source?, fields? })`、`setCoverFromURL(id, url)`
  - 游玩记录：`sessions.list/add/update/remove`；进度：`progress.get/list/save/clear`；评价：`review.get/save`
  - 其他：`start(id, options?)`、`onChanged(fn)`
- `ctx.categories`：`list()`、`get(id)`、`listByGame(gameId)`、`games(categoryId)`、`create({ name, emoji? })`、`update(id, patch)`、`delete(id)`、`addGames(categoryId, gameIds)`、`removeGames(categoryId, gameIds)`

可写字段与规则见 [plugin-system.md §7.5.1](plugin-system.md)：只读字段静默忽略，未知字段报错。写操作后游戏库页与首页自动刷新。

```ts
const game = await ctx.games.create({
  name: '星之终途',
  company: 'Key',
  path: 'D:/Games/Stella/Stella.exe',
  tags: ['已通关'],
  metadata_sources: [{ source: 'vndb', source_id: 'v26437' }],
});
await ctx.games.setCoverFromURL(game.id, 'https://example.com/cover.jpg');
await ctx.games.update(game.id, { status: 'playing', use_locale_emulator: true });
```

### 7.5 元数据与认证令牌：`ctx.metadata`

```ts
interface TagItem {
  name: string;
  source?: string;
  weight?: number;
  is_spoiler?: boolean;
}

interface MetadataResult {
  game: Partial<Game>;
  tags?: TagItem[];
}

interface MetadataSourceDef {
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
```

- `ctx.metadata.registerSource(def: MetadataSourceDef): Disposer`
- `ctx.metadata.search(name, opts?): Promise<unknown[]>`
- `ctx.metadata.fetch(source, sourceId, opts?): Promise<unknown>`
- `ctx.metadata.getAuthToken(source: string): Promise<string>`（由宿主代管应用凭证与用户 OAuth Token，不向插件暴露客户端密钥；当前内置支持 `"hikarinagi"`）

### 7.6 下载与导入：`ctx.downloads` / `ctx.import`

- `ctx.downloads.requestInstall(req: InstallRequestInput)`：走宿主确认弹窗 → 下载解压 → 自动入库流程
- `ctx.downloads.list()`、`getImportStates(reqs)`、`onProgress(fn)`、`onImported(fn)`
- `ctx.import.openModal(opts?)`、`fromPath(req)`

`InstallRequestInput` 字段：`name`、`downloadUrl`、`fileName?`、`archiveFormat?`、`startupPath?`、`installSubdir?`、`stripTopLevel?`、`headers?`、`archivePassword?`、`coverUrl?`、`source?`、`sourceId?`、`size?`、`silent?`、`metadata?`。

### 7.7 主题、字体与装饰：`ctx.theme`

- `ctx.theme.extend(tokens: ThemeTokens): Disposer`（按 `light` / `dark` 覆盖 `ThemePaletteTokens` 共 78 项宿主语义色板，以及 `glass` 毛玻璃参数、`shape` 控件圆角、`brand` 品牌标志与 `material` 窗口材质）
  - 色板覆盖作用于读取语义令牌的宿主组件（窗口骨架、侧边栏、输入框、主/次/危险按钮、开关、卡片、面板、菜单、筛选标签、下拉、数据表格、提示气泡等）；宿主中的状态色（成功 / 警告 / 错误）、封面上的玻璃按钮与阴影不在覆盖范围内。
  - 宿主的表单组件全部读取语义令牌：`inputBg` / `inputBorder` / `inputFocus` / `placeholder` 驱动所有输入框、文本域、数字输入、下拉选择与编辑页字段；`menuCheck` / `menuSelected` / `menuText` / `menuHover` 同时驱动下拉菜单与下拉选择列表；`switchOff` / `switchThumb` 驱动开关；`dangerBtn` / `dangerBtnHover` / `dangerBtnText` 驱动删除等危险操作按钮。未覆盖的令牌保持宿主原有配色，不会因为覆盖了其他令牌而改变。
  - `primaryBtn` / `primaryBtnHover` / `primaryBtnText` 同时驱动 `Button variant="primary"`、开关打开态与宿主各处的实心主按钮（启动按钮、弹窗确认、卡片悬停的开始游戏等）；`toolBtn*` 驱动次要按钮。
  - `accent` / `accentText` / `accentSoft` / `accentSoftText` 是强调色（选中、当前状态、加载指示、链接等）；`tabText` / `tabHover` / `tabActive` / `tabIndicator` 驱动游戏详情页的标签栏；`actionBtn*` 驱动详情页工具栏的图标按钮与当前状态外圈；`menuSelected` / `menuCheck` 是下拉菜单当前项的底色与对勾。
  - `shape: { control?, panel? }`（DIP）：**全局**圆角。`control` 覆盖所有按钮、图标按钮、输入框、下拉选择、分段切换、侧边栏导航项、菜单项与详情页工具栏 / 启动按钮；`panel` 覆盖所有浮层与容器面板（下拉菜单、选择列表、弹出层、提示气泡、日期选择、筛选抽屉、数据表格等）。卡片、封面、标签胶囊与头像不受影响；未声明时保持宿主默认。
  - `brand: { wordmark?, wordmarkDark?, icon?, iconDark?, hideWordmark?, hideIcon? }`：替换侧边栏与顶栏的 LunaBox 字标 / 应用图标（路径相对插件目录，支持 svg / png / webp，仅本地文件；`*Dark` 为深色模式专用图），或用 `hideWordmark` / `hideIcon` 隐藏；顶栏在字标隐藏时改放图标。多个插件声明时以后注册者为准，图片读取失败时回退到内置标志。
  - `material: { sidebar?: 'acrylic' | 'mica', tint?, tintDark? }`：侧边栏透出系统的半透明磨砂材质（Windows 11 22H2+ 的 Acrylic 磨砂 / Mica 云母，macOS 统一用系统侧边栏材质），上面再叠 `tint`（`#RRGGBBAA`，`tintDark` 为深色专用；未设置时取 `sidebarBg` 的 60% 不透明度）。顶栏、主内容区保持各自的不透明底色。Linux、旧版 Windows 与开启自定义背景时不生效，侧边栏回退到 `sidebarBg`；Windows 上第一次启用需重启应用（窗口要在创建时带上材质），之后开关即时生效。侧边栏的 `navHover` / `navActiveBg` / `sidebarBorder` 建议用半透明色，在着色上也能看清。
  - `glass.enabled: false`：**锁定配色**——插件启用期间宿主不使用用户的自定义背景与玻璃模式，设置页「背景」分区显示提示，停用插件后恢复；`glass.enabled: true` 则开启玻璃模式（可配合 `glass.backgroundImage` 使用插件自带背景）。多个插件声明时以后注册者为准。
  - 插件自身 UI 中的 `primary` / `brand`（取 `accent`）、`accent` / `accentText`（取 `accentText`）、`accentSoft` / `accentSoftText` 颜色令牌，以及 `tone: 'brand' | 'accent'`、`Badge tone="brand"`、`Tabs` 下划线、`Spinner`、`Progress` 与 Markdown 链接都跟随色板覆盖，因此详情页类插件用这些令牌即可适配主题插件；`brand700`/`brand800`/`brand900` 等固定色阶不跟随。
- `ctx.theme.registerFont({ family, file, target?: 'ui' | 'mono' }): Disposer`
- `ctx.theme.registerDecoration(def: DecorationOptions): Disposer`（`layer`: `'background' | 'foreground'`，`anchor`: `'bottom-right' | 'bottom-left' | 'top-right' | 'top-left' | 'center'`，`offset`、`size`、`opacity`、`pointerEvents`、`routes`、`image`、`render`）

### 7.8 独立窗口与主窗口控制：`ctx.window`

- `ctx.window.open(id, opts: WindowOpenOptions): PluginWindowHandle`
  - `WindowOpenOptions`：`title?`、`width`、`height`、`x?`、`y?`、`frameless?`、`transparent?`、`alwaysOnTop?`、`skipTaskbar?`、`resizable?`、`render`
  - `PluginWindowHandle`：`id`、`setSize(w, h)`、`setPosition(x, y)`、`setAlwaysOnTop(v)`、`show()`、`hide()`、`close()`、`onMove(fn)`、`onFocus(fn)`、`onClose(fn)`
- `ctx.window.main`：`show()`、`focus()`、`minimize()`、`isVisible()`

### 7.9 AI 与网络：`ctx.ai` / `ctx.http` / `fetch`

- **全局 `fetch`**：遵循宿主代理配置，默认注入 LunaBox 统一 `User-Agent`；只限制等待响应头 30 秒，流式响应体不限时长，用 `AbortSignal` 中止。
- **Web 兼容层**：宿主补齐了 `FinalizationRegistry`、`crypto.getRandomValues` / `crypto.randomUUID`、`Blob` / `File` / `FormData`（`fetch` 暂不支持 multipart 上传）。
- **`ctx.http.sse(url, init, handlers): Disposer`**：Server-Sent Events 流式订阅（`onEvent` / `onDone` / `onError`）。
- **`ctx.ai`**：
  - `getConfig(): { configured, provider, model }`
  - `getCredentials(): { configured, provider, baseUrl, apiKey, model }`：宿主设置页配置的凭据，给插件自带的 agent 运行时使用
  - `complete(req)` / `stream(req, handlers): Disposer`：宿主 Go 侧的 OpenAI 兼容调用（支持 `tools`、`maxTurns` 与 `onToolCall`）

**自带 agent 运行时（推荐）**：直接打包 [pi](https://github.com/earendil-works/pi)，模型适配与 agent 循环都在插件里，支持 OpenAI 兼容与 Anthropic 端点：

```ts
import { Agent } from '@earendil-works/pi-agent-core';

const cred = ctx.ai.getCredentials();
// 用 cred.baseUrl / cred.apiKey / cred.model 创建 provider，
// 工具实现调用 ctx.games.* / fetch + ctx.html.parse / ctx.process（先 ctx.ui.confirm）
```

pi 自带的 bash / read / edit 工具依赖 Node 接口，在插件里不可用，用 `ctx.process` / `ctx.fs` 实现同名工具。


### 7.10 宿主事件钩子与启动拦截器：`ctx.on`（`EventMap`）

```ts
ctx.on<K extends keyof EventMap>(event: K, listener: EventMap[K], opts?: { prepend?: boolean; timeout?: number }): Disposer;
```

| 事件名 | 类型 | 载荷 | 返回值作用 |
| --- | --- | --- | --- |
| `app/ready` | 通知 | 无 | — |
| `game/added` | 通知 | `Game` | — |
| `game/updated` | 通知 | `Game` | — |
| `game/deleted` | 通知 | `{ id: string }` | — |
| `launch/prepare` | 拦截器 | `{ game, options: LaunchOptions }` | 返回 `{ options }` 改启动选项，`{ cancel: true, reason? }` 取消启动 |
| `launch/plan` | 拦截器 | `{ game, plan: LaunchPlan }` | 返回 `{ plan }` 改 `file` / `args` / `dir` / `env` / `run_as_admin`，`{ cancel: true, reason? }` 取消启动 |
| `game/before-launch` | 通知 | `{ game, plan }` | —（拦截器都已执行，进程即将启动） |
| `game/launched` | 通知 | `{ game: Game; sessionId: string }` | — |
| `game/running` | 通知 | `{ game, sessionId, pid, processName, reason }` | —（识别到实际游戏进程） |
| `session/ended` | 通知 | `{ gameId: string; sessionId: string; duration: number }` | — |
| `metadata/before-search` | waterfall | `{ name: string; source: string }` | 返回 `{ name }` 可改写搜索关键词 |
| `metadata/after-fetch` | waterfall | `{ source: string; result: MetadataResult }` | 返回新的 `MetadataResult` 可加工/补全抓取结果 |

拦截器的返回值会**深合并**进载荷（数组整体替换），多个插件按插件顺序依次执行，任一返回 `cancel: true` 即停止。默认超时 3 秒，需要等用户操作时用 `{ timeout }` 放宽（最多 10 分钟），例如启动前选择 Steam 账号：

```ts
ctx.on('launch/plan', async ({ game, plan }) => {
  if (!plan.file.toLowerCase().endsWith('steam.exe')) return;
  const login = await ctx.ui.select({
    title: `用哪个账号启动《${game.name}》？`,
    options: accounts.map((a) => ({ value: a.login, label: a.name })),
  });
  if (login === null) return { cancel: true, reason: '已取消' };
  await ctx.process.exec('taskkill /IM steam.exe /F', { shell: true });
  await ctx.process.exec('reg', ['add', 'HKCU\\Software\\Valve\\Steam', '/v', 'AutoLoginUser', '/d', login, '/f']);
}, { timeout: 120_000 });
```


### 7.11 界面操作、HTML 解析、系统与文件

- **`ctx.ui`**：`slot(name, def)`、`toast(msg, type?)`、`navigate(path, params?)`、`openDrawer(id, props?)`、`closeDrawer()`、`confirm(opts): Promise<boolean>`、`select(opts): Promise<string | null>`（原生对话框；主窗口隐藏时会先显示主窗口；插件卸载时自动按取消关闭）
- **`ctx.html.parse(html): HTMLSelection`**：提供类 jQuery / CSS 选择器链式查询（`select`、`first`、`eq`、`length`、`text`、`attr`、`html`、`map`）
- **`ctx.shell`**：`openURL(url)`、`openPath(path)`
- **`ctx.audio`**：`play(src, opts?: { volume?: number } | number)`、`stop(src?)`
- **`ctx.fs`**：`readFile(path)`、`writeFile(path, content)`、`exists(path)`、`readDir(path)`、`mkdir(path)`、`remove(path)`
- **`ctx.process`**（完全信任，宿主不做沙箱）：
  - `platform`：`windows` / `darwin` / `linux`
  - `exec(command, args?, opts?) → Promise<{ code, stdout, stderr, timedOut, truncated }>`：非零退出码不 reject，命令无法启动才 reject；默认超时 60 秒
  - `spawn(command, args?, opts?) → { pid, onStdout, onStderr, onExit, write, closeStdin, kill }`：插件卸载时结束整个进程树
  - `opts`：`cwd`、`env`、`timeout`、`input`、`shell`（整行命令交给 `cmd.exe` / `/bin/sh`）、`encoding`（默认 `auto`，非 UTF-8 输出按系统代码页解码）
  - 让 AI 调用命令时，MUST 先用 `ctx.ui.confirm` 展示完整命令让用户确认

---

## 8. 内置示例插件索引

所有示例位于 [packages/plugin-sdk/examples/](../packages/plugin-sdk/examples/)：

| 示例目录 | 演示核心能力 |
| --- | --- |
| `hikarinagi-detail` | TypeScript/TSX 构建、`game.detail.hero` + `HeroBanner`、`NativeSlot`、`game.detail.tab`（`order`/`hideTabs`）、`game.detail.aside`、`ctx.db` SQLite 缓存、`ctx.metadata.getAuthToken` |
| `hikarinagi-theme` | 按 Hina UI 设计令牌映射全部宿主语义色板（含 accent 与详情页标签栏）、`glass.enabled: false` 锁定配色、`ctx.config.onChange` 动态重新注册主题 |
| `metadata-source` | `ctx.metadata.registerSource` 自定义元数据源 |
| `resource-hub` | `sidebar.page`、`Grid`/`Card`、分页加载与 `ctx.downloads.requestInstall` |
| `site-detail` | `game.detail.tab`、`Markdown`、`ctx.html.parse` |
| `theme-mascot` | `ctx.theme.extend`、`defineThemePreset`、字体与角色水印装饰 |
| `float-ball` | `ctx.window.open` 无边框透明置顶悬浮窗、`dragWindow` |
| `pomodoro-overlay` | 独立悬浮计时器与插件自定义 SVG 图标 |
| `whale-pet` | `Box(springPress)` 原生弹簧按压动画与 `ctx.audio.play` 音效 |
| `ai-assistant` | 最简 AI 对话：`workspace.drawer`、宿主 Go 侧的 `ctx.ai.stream`、`createAIChatStore` 与 Tool Call |
| `ai-agent` | 完整的 AI 游戏库助手（类 Claude / Codex 桌面端）：打包 pi（`@earendil-works/pi-ai` + `pi-agent-core`）自带 agent 循环，支持 OpenAI 兼容与 Anthropic；`sidebar.page`（会话列表 + 对话）、`workspace.drawer`、`topbar.item`、`library.toolbar`、`game.actions`；流式 `Markdown`、可展开的工具调用卡片、`ctx.ui.confirm` 写操作确认；工具覆盖游戏库查询与增删改、收藏夹、元数据搜索入库、`web_fetch` 刮削网页、`run_command`（`ctx.process`）；会话持久化到 `ctx.storage` |
