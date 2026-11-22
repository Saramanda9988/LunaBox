# 插件系统设计

> 状态：**已实现（P1–P4 核心与 SDK/示例均已落地；插件市场索引仓库为后期）**。本文是实现时的约定来源；实现过程中改了约定，先改这里。
> 路径约定同 [AGENTS.md](../AGENTS.md)：`packages/...` 相对仓库根，`ui/...`、`host/...`、`api/...` 相对 `apps/desktop/`。

## 1. 目标与非目标

**目标**

- 运行时加载、卸载、热重载插件，不重新编译 LunaBox。
- 插件用 JavaScript 编写（TypeScript 经打包后交付），由内嵌的 JS 引擎 [moejs](https://github.com/MoeForge/moejs) 执行。
- 借鉴 [Cordis](https://github.com/cordiverse/cordis) 的思想：插件通过 `apply(ctx)` 激活，**每次注册都返回撤销函数并挂在插件的作用域上**，卸载时整个作用域一次撤销，插件作者不写清理代码。
- 插件可以：提供元数据源、接入资源站下载并走宿主导入流程、提供外部站点浏览与扩展详情页、自定义主题（色板/字体/背景装饰水印）、打开独立悬浮窗（悬浮球）、构建 AI 助手（找资源/整理游戏库）、监听或改写业务事件（钩子）、往界面的插槽里放界面。
- 插件界面用**描述树**：JS 在状态变化时生成描述树，Go 缓存它，每帧用缓存的树调用对应的 `components.Xxx(c, ...)` 绘制，外观与原生界面一致；同一套协议同时用于主窗口插槽与插件创建的独立悬浮窗。

**非目标**

- **不做沙箱**。插件完全信任，与主程序同权限（文件、网络、进程）。安装时向用户明确提示。
- 不直接使用 Cordis 库，不做“一切皆插件”：核心仍是 Go，插件只能通过本文定义的宿主 API 扩展。
- 第一期不支持插件之间互相提供服务（Cordis 的跨插件 `inject`）。
- 不支持 npm 依赖解析与 CommonJS；插件交付时必须是打包好的单个 ESM 文件。
- 插件不能直接调用 MyGo 的 `ui` 包，只能使用白名单组件。

## 2. 总览

```text
┌──────────────────────────── packages/backend ────────────────────────────┐
│ plugin.Manager ── 发现 / 加载 / 卸载 / 热重载 / 状态                       │
│   └─ Plugin（每个插件一份）                                               │
│        ├─ 插件线程：moejs.Runtime + eventloop，独占一个 goroutine          │
│        ├─ Scope：撤销函数栈 + context（卸载时取消）                         │
│        └─ 宿主 API（ctx.*，Go 实现）                                      │
│ 注册表：metadata.Registry（元数据源）、hooks.Bus（事件）、plugin.Slots（插槽）│
└──────────────────────┬───────────────────────────────────────────────────┘
                       │ api.PluginService（界面只依赖接口）
┌──────────────────────▼──────────── apps/desktop ─────────────────────────┐
│ ui/pluginui：插槽实例、描述树缓存、输入框状态、渲染器（树 → components.Xxx） │
│ 插槽宿主点：设置页分区、详情页标签页、侧边栏页面……                          │
└──────────────────────────────────────────────────────────────────────────┘
```

**线程规则（MUST）**

1. 插件的 JS 只在它自己的插件线程上执行。
2. 界面线程**从不同步等待** JS：只读缓存的描述树、只投递事件消息。
3. 服务调用插件（元数据、钩子）时带 `context` 与超时，超时用 `Runtime.Interrupt` 打断，插件出错不影响主流程。

## 3. 插件包

### 目录结构

```text
<数据目录>/plugins/<id>/
  plugin.json        清单
  main.js            打包好的单个 ESM 文件
  locales/*.json     插件自己的语言包（可选）
  assets/            图标、图片（可选）
<数据目录>/plugins-data/<id>/
  storage.json       ctx.storage 的数据（见 §6）
```

插件代码与插件数据分开存放：升级插件时整个替换 `plugins/<id>/`，数据不丢；卸载时由用户选择是否删除数据。两者都在数据目录内，全量备份会带上。

### 清单 `plugin.json`

```jsonc
{
  "id": "vndb-cn-mirror",            // 小写字母、数字、连字符，字母开头；全局唯一
  "version": "0.3.1",                // semver
  "apiVersion": 1,                   // 插件 API 的主版本，见 §13
  "engines": { "lunabox": ">=2.1.0 <3" },
  "main": "main.js",
  "name": { "zh-CN": "VNDB 国内镜像", "en-US": "VNDB CN Mirror" },
  "description": { "zh-CN": "……", "en-US": "……" },
  "author": "someone",
  "homepage": "https://github.com/someone/lunabox-vndb-cn",
  "icon": "assets/icon.svg"
}
```

- 不声明权限（完全信任），也不静态声明贡献点：插件提供什么，全部在 `apply(ctx)` 里动态注册（同 Cordis）。
- `name` 和 `description` 按当前界面语言取值，缺失时依次回退到 `en-US`、第一个值。
- 加载前检查 `apiVersion` 与 `engines.lunabox`，不兼容的不加载，在插件管理页显示原因。

## 4. 生命周期

```text
discovered ──enable──▶ loading ──apply 完成──▶ active ──disable/reload/退出──▶ disposing ──▶ disabled
                          │                                                           
                          └──清单无效/不兼容/apply 抛错/超时──▶ error（已注册的部分立即撤销）
```

**加载**

1. 读清单并校验。
2. 新建 `moejs.Runtime`（见 §5），安装宿主全局对象。
3. 编译 `main.js`（开发模式下允许 `.ts`，用 `CompileTS`）。模块解析只接受虚拟模块 `lunabox`（SDK 的运行时部分）；其他 import 一律拒绝。
4. 调用导出的 `apply(ctx)`；返回 Promise 时等待它完成，超时 10s。
5. 成功则进入 `active`，并在界面线程上触发重绘。

**卸载**

1. 取消插件作用域的 `context`，进行中的请求和定时器随之停止。
2. 按 **后注册先撤销（LIFO）** 执行作用域里的全部撤销函数。
3. 停止事件循环，丢弃运行时。

**重载** = 卸载 + 加载。新版本加载失败时进入 `error`，不回退旧版本（旧作用域已撤销，回退只会让状态更复杂）。

**启动与退出**

- 服务就绪后在后台加载已启用的插件，**不阻塞首帧**。插件还没加载完时，它的元数据源视为不可用（与“该来源未启用”同样处理）。
- `OnShutdown` 里卸载全部插件，整体限时 2s，超时直接丢弃。这一步没有交互，符合 [backend.md](backend.md) 的退出生命周期约束。

## 5. 执行模型

### 插件线程

每个插件一个 `moejs.Runtime` 加 `eventloop`，固定在一个 goroutine 上。其他 goroutine 通过邮箱投递任务：

```go
// packages/backend/plugin/runtime.go（示意）
type thread struct {
	rt    *moejs.Runtime
	loop  *eventloop.Loop
	inbox chan func(rt *moejs.Runtime)
}

// Call 在插件线程上调用 JS 函数，等待结果；ctx 结束时 Interrupt。
func (p *Plugin) Call(ctx context.Context, fn moejs.Value, args ...any) (json.RawMessage, error)

// Post 投递任务后立即返回，界面事件用它。
func (p *Plugin) Post(task func(rt *moejs.Runtime))
```

moejs 的内置对象由所有运行时共享冻结，每个运行时约 81 KiB，开销可忽略。

### 运行时配置（完全信任）

| 能力 | 配置 |
| --- | --- |
| `fetch` | 安装 `webapi`。`Client` 用 `httputils.NewClient`，走 `proxyutils` 的代理；**默认 User-Agent 取 `version.UserAgent()`**（AGENTS.md 的 MUST），插件显式设置时才覆盖。`Allow` 为 nil（不限制目标）。 |
| 定时器 | 安装 `eventloop`（`setTimeout`/`setInterval`/`setImmediate`）。 |
| 文件访问与 Buffer | 安装 `node:buffer`（全局 `Buffer`）；因 `moejs/node` 的 `node:fs` 仅面向单根目录 `os.Root` 沙箱，完全信任的文件读写由 Go 直接实现为 `ctx.fs`（`readFile`/`writeFile`/`exists`/`readDir`/`mkdir`/`remove`）。 |
| `eval` / `new Function` | 不禁用。 |
| `MemoryLimit` | 256 MiB。只为防止插件 bug 拖垮主进程，不是安全边界。 |
| `console.*` | 映射到 `ctx.logger`。 |

### 错误处理

- JS 抛出的异常（`*moejs.Exception`）写入应用日志，前缀为 `[plugin:<id>]`，附带 JS 调用栈；插件管理页显示最近的错误。
- 宿主函数里的 Go panic 必须 `recover`，转成 JS 异常；moejs 的 `*InternalError` 同样处理，运行时继续可用。
- 超时与取消返回 `*moejs.InterruptedError`，调用方按“插件不可用”处理。

## 6. 作用域与可撤销注册（借鉴 Cordis）

### Go 侧

```go
// packages/backend/plugin/scope.go（示意）
type Disposer func()

type Scope struct {
	ctx      context.Context    // 卸载时取消
	cancel   context.CancelFunc
	mu       sync.Mutex
	effects  []*effect          // LIFO
	children []*Scope
	disposed bool
}

// Effect 登记撤销函数；返回的 Disposer 可提前单独撤销，撤销后从列表移除，不会执行第二次。
func (s *Scope) Effect(dispose func()) Disposer

// Fork 创建子作用域（如一个插槽实例），随父作用域一起撤销。
func (s *Scope) Fork() *Scope

func (s *Scope) Dispose()
```

**MUST**

- 插件能注册的一切都必须通过 `Scope.Effect` 登记，包括元数据源、事件监听、插槽、侧边栏项、命令、定时器、文件监听和后台任务。宿主侧的注册表都必须支持移除。
- 宿主 API 里的异步操作使用 `scope.ctx`，卸载时自动取消。
- 撤销函数必须幂等，并且不能阻塞（不能等待插件线程）。

### JS 侧

每个注册类 API 都返回撤销函数，另有 `ctx.effect` 用于插件自己的清理：

```ts
import type { Context } from 'lunabox'

export function apply(ctx: Context) {
  const off = ctx.on('game/added', (game) => ctx.logger.info('added', game.name))
  // off() 可提前撤销；不调用也没关系，卸载时自动撤销

  ctx.effect(() => {
    const ws = new WebSocket('…')       // 举例：插件自己持有的资源
    return () => ws.close()             // 卸载时执行
  })
}
```

作用域层级：插件根作用域 → 插槽实例作用域（§9.4）→ 组件里 `useEffect` 的清理函数。

## 7. 宿主 API（`ctx`，apiVersion 1）

| 服务 | 主要成员 | Go 实现 |
| --- | --- | --- |
| `ctx.logger` | `info/warn/error(...args)` | `applog`，前缀 `[plugin:<id>]` |
| `ctx.i18n` | `language`、`t(key, args?)`、`onChange(fn)` | 插件 `locales/*.json`，语言取 `AppConfig.Language` |
| `ctx.storage` | `get/set/delete/keys`（值为 JSON） | `plugins-data/<id>/storage.json`，写入时防抖落盘 |
| `ctx.config` | `define(schema)`、`get()`、`onChange(fn)` | 存在 `ctx.storage` 的保留键里；有 schema 时宿主自动生成设置分区（§10） |
| `ctx.games` | `list(query)`、`get(id)`、`findBySource(source, sourceId)`、`update(id, patch)`、`batchUpdate(patches)`、`setTags(id, tags)`、`start(id)`、`onChanged(fn)` | 包装 `GameService` / `StartService` 已有的公开方法；`update` 只开放白名单字段 |
| `ctx.categories` | `list()`、`create(input)`、`update(id, patch)`、`delete(id)`、`addGames(id, gameIds)`、`removeGames(id, gameIds)` | 包装 `CategoryService`（供 AI 整理游戏库等场景操作收藏夹） |
| `ctx.metadata` | `registerSource(def)`、`search(name, opts?)`、`fetch(source, sourceId, opts?)` | `metadata.Registry`（§8）+ 宿主元数据查询能力 |
| `ctx.downloads` | `requestInstall(req)`、`list()`、`getImportStates(reqs)`、`onProgress(fn)`、`onImported(fn)` | 包装 `DownloadService` + 唤起 `InstallConfirmModal`（走完整下载 → 解压 → 自动入库流程） |
| `ctx.import` | `openModal(path?)`、`fromPath(req)` | 包装 `ImportService` 与导入弹窗（供插件自定义下载后移交本地路径入库） |
| `ctx.theme` | `extend(tokens)`、`registerFont(def)`、`registerDecoration(def)` | 接入 `ui/theme`（色板覆盖、字体加载）与 `ui/app.go`（背景/前景装饰层，如右下角角色水印） |
| `ctx.window` | `open(id, opts)`、`main.show()/focus()/minimize()` | 包装 `mygo.NewWindow`（支持 `Frameless`、`Transparent`、`AlwaysOnTop`、`SkipTaskbar`），随作用域自动销毁 |
| `ctx.ai` | `getConfig()`、`complete(req)`、`stream(req, handlers)` | 复用宿主 `AiService` 配置或插件自定义模型配置，Go 侧处理 SSE 流式解析与 tool calls 回调 |
| `ctx.http` | `sse(url, init, handlers)` | 基于 `httputils.NewClient` 的流式 SSE 读取，按事件投递回插件线程 |
| `ctx.on` | 事件与钩子（§7.1） | `hooks.Bus` |
| `ctx.ui` | `slot(name, def)`、`toast(msg, type?)`、`navigate(path, params?)`、`openDrawer(id, props?)`、`closeDrawer()` | `plugin.Slots` + 界面（§9、§10） |
| `ctx.html` | `parse(html)` → `select(css)` → `text()/attr()/html()` | goquery（后端已经依赖） |
| `ctx.shell` | `openURL(url)`、`openPath(path)` | `appruntime.Runtime` / `apputils` |
| `ctx.paths` | `plugin`（插件目录）、`data`（插件数据目录） | |
| `ctx.effect` | `effect(setup)` | `Scope.Effect` |

**约定**

- 便宜的同步操作（`storage.get`、`i18n.t`、`logger`）直接返回值。涉及 IO 或数据库的操作返回 Promise：Go 侧用 `loop.NewPromise()`，在 goroutine 里完成后 `settle`。
- Go 与 JS 之间只传 JSON 兼容的值。Go 结构体的字段名用 `json` tag，与现有前端模型一致（`snake_case`）。
- `ctx` 不暴露 DuckDB 连接，也不暴露 service 指针：表结构变化不应该让插件坏掉。

### 7.1 事件与钩子

派发方式沿用 Cordis 的语义：

| 方式 | 语义 | 出错或超时 |
| --- | --- | --- |
| `emit` | 依次投递给监听者，不等返回值 | 记录日志，继续 |
| `waterfall` | 按顺序调用，每个监听者收到上一个的返回值；返回 `undefined` 表示不改 | 跳过该监听者，沿用上一个值 |
| `bail` | 按顺序调用，第一个返回非 `undefined` 的结果生效 | 视为 `undefined` |

顺序：带 `{ prepend: true }` 的先执行，其余按插件管理页里的插件顺序。

| 事件 | 方式 | 参数 → 返回 | 超时 | 触发点 |
| --- | --- | --- | --- | --- |
| `app/ready` | emit | — | — | 插件全部加载完成后 |
| `game/added` | emit | `Game` | — | 导入或添加游戏成功后（`ImportService`、`GameService`） |
| `game/updated` | emit | `Game` | — | `GameService` 保存后 |
| `game/deleted` | emit | `{ id }` | — | `GameService` 删除后 |
| `game/before-launch` | bail | `Game` → `{ cancel: true, reason } \| undefined` | 3s | `StartService.startGame` 解析启动策略之前 |
| `game/launched` | emit | `{ game, sessionId }` | — | 进入监控流程后 |
| `session/ended` | emit | `{ gameId, sessionId, duration }` | — | 游玩记录结束时 |
| `metadata/before-search` | waterfall | `{ name, source }` → `{ name }` | 2s | 按名称搜索元数据之前 |
| `metadata/after-fetch` | waterfall | `{ source, result }` → `result` | 3s | 拉到元数据之后、写入之前 |

**实现约束**

- 服务通过 `SetHookBus(...)` 注入 `hooks.Bus`（遵循 [backend.md](backend.md) 的 `SetXxxService` 约定）。接口定义在 `packages/backend/common/hooks`，服务不依赖 `plugin` 包。没有插件时注入 `hooks.Nop`。
- **没有监听者时必须直接返回**，不做 JSON 序列化：批量导入 2800 款游戏时钩子在热路径上。
- `emit` 在后台 goroutine 中投递，不阻塞调用方。

### 7.2 下载与导入 SDK（`ctx.downloads` / `ctx.import`）

面向**下载类插件**（如 TouchGal 等资源站浏览与一键下载入库），提供两条与宿主导入链路打通的路径：

1. **托管下载 + 宿主标准入库流程（推荐）**：插件通过站点 API 获取资源直链后调用 `ctx.downloads.requestInstall(req)`，宿主弹出内置的 `InstallConfirmModal`（或当 `silent: true` 且用户已授权默认目录时直接入队），走 `DownloadService` 下载 → 自动解压 → `ImportService` 自动入库并关联元数据：

   ```ts
   await ctx.downloads.requestInstall({
     name: '魔女的夜宴',
     downloadUrl: 'https://.../resource.7z',
     headers: { Referer: 'https://www.touchgal.us/' }, // 可选：站点防盗链 Header / Cookie
     archivePassword: '...',                           // 可选：解压密码，自动传给解压流程
     coverUrl: 'https://.../cover.jpg',
     source: 'plugin:touchgal/touchgal',               // 入库后自动绑定来源或内置源（如 'bangumi' / 'vndb'）
     sourceId: '12345',
     metadata: { /* 可选：预填 MetadataResult，入库时直接应用 */ },
   })
   ```

2. **自定义下载协议 + 本地路径移交入库**：若资源站使用私有分片协议或外部下载器，插件自行把文件下载到指定目录后：
   - 调用 `ctx.import.openModal({ path, metadata? })` 唤起宿主的添加/导入弹窗（预填本地路径与元数据，让用户确认可执行文件与分类）；
   - 或调用 `ctx.import.fromPath({ path, exePath?, metadata?, categoryIds? })` 直接走 `ImportService` 完成入库。
3. **在库与下载进度联动**：
   - `ctx.games.findBySource(source, sourceId)` / `ctx.downloads.getImportStates(reqs)`：插件在渲染资源站列表卡片时，可实时显示“已在库中（点击启动）”、“下载中（进度条）”或“下载并导入”。

### 7.3 主题与视觉装饰 SDK（`ctx.theme`）

面向**主题类插件**（自定义主题色、字体、右下角角色水印等背景装饰）：

1. **主题色板覆盖（`ctx.theme.extend`）**：
   - 按 `light` / `dark` 两套模式覆盖 `ui/theme/palette.go` 中的语义令牌（语义名与 `theme.Palette` 一一对应，如 `appBg`、`topbarBg`、`sidebarBg`、`cardBg`、`cardBorder`、`primaryBtn`、`primaryBtnHover`、`text`、`textStrong`、`textMuted`、`navActive`、`navActiveBg` 等），同时支持传入 `glass: { enabled, opacity, blur, backgroundImage }`。
   - 宿主在每帧构建 `*theme.Theme` 时按已启用插件顺序应用覆盖；插件停用或卸载时自动恢复原色板。
2. **自定义字体（`ctx.theme.registerFont`）**：
   - `ctx.theme.registerFont({ family: 'LXGW WenKai', file: 'assets/wenkai.ttf', target: 'ui' | 'mono' })`。
   - 从插件目录加载字体文件并注册到 MyGo 渲染上下文，可替换全局默认 `theme.UIFont` 或供插件组件使用。
3. **背景与前景装饰层（`ctx.theme.registerDecoration`）**：
   - 用于在窗口右下角等位置绘制角色立绘水印、动态挂件或氛围背景：

   ```ts
   ctx.theme.registerDecoration({
     id: 'character-watermark',
     layer: 'foreground',          // 'background'（背景图之上、主布局之下）| 'foreground'（主内容之上、顶栏与弹窗之下）
     anchor: 'bottom-right',       // 九宫格锚点：'bottom-right' | 'bottom-left' | 'top-right' | 'top-left' | 'center'
     offset: { x: 20, y: 12 },     // 距离锚点边缘的 DIP 偏移
     size: { width: 240, height: 340 },
     opacity: 0.22,
     pointerEvents: 'none',        // 'none' 时宿主用 PassThrough() 绘制，鼠标点击与滚轮完全穿透，不遮挡底部卡片与按钮
     routes: ['/', '/library'],    // 可选：仅在指定路由下显示
     image: 'assets/mascot.webp',  // 静态图片，也可传入 render: () => <Node> 用描述树绘制可交互挂件
   })
   ```

### 7.4 独立悬浮窗 SDK（`ctx.window`）

面向**悬浮球类插件**（打开另一个独立窗口作为桌面悬浮窗/快捷面板）：

```ts
const floatWin = ctx.window.open('float-ball', {
  title: 'LunaBox Float',
  width: 64,
  height: 64,
  x: 1680,
  y: 820,
  frameless: true,       // 无系统边框与标题栏
  transparent: true,     // 窗口底色透明，支持圆形悬浮球或异形角色立绘
  alwaysOnTop: true,     // 始终置顶
  skipTaskbar: true,     // 不在任务栏独立占位
  resizable: false,
  render: FloatBallApp,  // 复用 §9 的同一套 JSX 描述树协议！在根节点设置 dragWindow 即可按住拖动窗口
})
```

- **动态伸缩与窗口控制**：返回的窗口句柄提供 `setSize(w, h)`（例如悬浮球点击后从 `64×64` 原地展开为 `300×420` 的游戏快捷启动/计时器/攻略小窗）、`setPosition(x, y)`、`setAlwaysOnTop(v)`、`show()`、`hide()`、`close()`，以及 `onMove`/`onFocus`/`onClose` 事件。
- **主窗口协同**：`ctx.window.main.show()`、`ctx.window.main.focus()`、`ctx.window.main.minimize()`、`ctx.window.main.isVisible()`。所有由插件打开的子窗口均登记在插件 `Scope` 下，插件停用、重载或应用退出（`OnShutdown`）时由宿主自动销毁，绝不残留僵尸窗口。

### 7.5 AI 助手与流式能力 SDK（`ctx.ai` / `ctx.http.sse`）

面向**AI 类插件**（AI 帮搜资源、AI 帮整理游戏库，交互形式类似 Claude Code Desktop 的多轮 Agent 工作台）：

- **模型调用（`ctx.ai`）**：
  - `ctx.ai.getConfig()`：读取宿主设置页已配置的 AI 服务状态（是否已配置模型、当前模型名称，不暴露明文 API Key）。
  - `ctx.ai.stream(req, handlers)` / `ctx.ai.complete(req)`：在 Go 侧发起流式请求（复用宿主的 `AiService` 配置，或使用插件在 `ctx.config` 中自定义的 OpenAI / Anthropic 兼容端点），支持传入 `tools`（JSON Schema 工具定义）：

    ```ts
    const abort = ctx.ai.stream(
      {
        system: '你是 LunaBox 游戏库助手，可帮用户检索资源站或整理本地游戏库。',
        messages: history,
        tools: [searchResourceTool, organizeLibraryTool],
      },
      {
        onDelta: (text) => appendAssistantText(text),
        onToolCall: async (call) => { /* 执行 ctx.games / ctx.categories / 外部站点查询并返回结果 */ },
        onDone: (usage) => setStreaming(false),
        onError: (err) => ctx.ui.toast(err.message, 'error'),
      }
    )
    ```

  - Go 侧对高频 token 流做 30ms 节流合并后再投递到插件线程，配合描述树的 `ScrollArea(autoScrollBottom)`、`Markdown` 与 `Collapsible`（展示 AI 正在执行的工具调用和参数，像 Claude Code 一样可展开查看详情），保证流式输出时界面维持 60fps。
- **游戏库读写与批量整理原语**：
  - `ctx.games` 提供 `list(query)`（支持按未分类、缺失标签、来源、关键字筛选）、`batchUpdate(patches)`、`setTags(id, tags)`。
  - `ctx.categories` 提供 `list()`、`create()`、`update()`、`addGames(categoryId, gameIds)`、`removeGames(categoryId, gameIds)`。
  - 插件可在对话流里先用 `Card` / `Table` 渲染“待应用的整理方案预览（Diff）”，用户点击卡片上的“确认应用”按钮后再调用批量接口落库。

## 8. 元数据源插件

### 插件侧

```ts
ctx.metadata.registerSource({
  id: 'vndb-cn',                                   // 插件内唯一
  name: { 'zh-CN': 'VNDB 镜像', 'en-US': 'VNDB Mirror' },
  icon: 'assets/vndb-cn.svg',
  link: (sourceId) => `https://vndb.example.cn/${sourceId}`,
  normalizeId: (input) => /^v?\d+$/.test(input) ? input.replace(/^v?/, 'v') : null,
  rateLimit: { perSecond: 2 },
  async fetch(sourceId, { language, signal }) { /* → MetadataResult */ },
  async search(name, { language, limit, signal }) { /* → MetadataResult[] */ },
  async fetchBatch(ids, opts) { /* 可选 → Record<id, MetadataResult> */ },
})
```

- 来源类型（`enums.SourceType`）为 `plugin:<pluginId>/<id>`，例如 `plugin:vndb-cn-mirror/vndb-cn`。
- `MetadataResult` 的 JSON 形状与 `metadata.MetadataResult` 一致：`game` 的字段用 `models.Game` 的 json tag，另有 `tags: [{ name, weight, is_spoiler }]`；封面给 URL，由宿主下载。SDK 里提供完整的 TS 类型。

### 宿主侧（最小改动）

现有的来源分发写在多个 `switch` 里，只在各自的 `default` 分支里查询插件注册表：

| 位置 | 改动 |
| --- | --- |
| `packages/backend/utils/metadata/registry.go`（新增） | `RegisterExternal(source, ExternalSource) (dispose func())`、`LookupExternal(source)`、`ExternalSources()`；`ExternalSource` 实现 `Getter`/`CandidateGetter`/`BatchGetter` |
| `GameService.fetchMetadataResultBySource` 等按 ID 拉取的 `switch` | `default` 分支 → `LookupExternal` |
| 校验 ID 的 `switch`、`gamehelper.NormalizeMetadataSource` | `default` 分支 → `ExternalSource.NormalizeID` |
| `GameService` 与 `ImportService` 组装 `metadataSearchSource` 的 `switch` | `default` 分支 → 用 `ExternalSource` 组装 |
| `metadata_limiter.go` | 插件来源按 `rateLimit` 建限速器，没有声明时用默认值 |
| `appconf/normalize.go` 的 `normalizeMetadataSources` | 保留格式合法的 `plugin:` 条目，**即使插件当前没有加载**，这样暂时停用插件不会丢失用户的来源顺序；运行时跳过没有注册的来源 |
| `ui/metasource` | 名称、图标、链接的来源除了静态表，再加上 `ExternalSources()` |

`enums.MetadataSourceTypes` 保持只包含内置来源。插件来源不进枚举，统一通过注册表查询。

## 9. 界面协议

### 9.1 原则

- 插件状态变化 → 插件线程上执行 `render` → 生成描述树 → 在插件线程（**不是界面线程**）上解码、校验成 Go 结构 → `Win.Update` 把新树放进缓存并请求重绘。
- 每帧：渲染器读缓存里的树，调用对应的 `components.Xxx(c, ...)`，开销与原生界面相同。
- 用户操作 → 渲染器 `Post` 一条事件消息到插件线程 → 事件处理器修改状态 → 回到第一步。

### 9.2 描述树格式

```json
{ "t": "Column", "k": "root", "p": { "gap": "md" }, "c": [
  { "t": "Text", "p": { "text": "按名称搜索", "size": "sm", "tone": "muted" } },
  { "t": "Row", "p": { "gap": "sm" }, "c": [
    { "t": "Input", "k": "q", "p": { "value": "", "placeholder": "游戏名" },
      "on": { "change": "h1", "submit": "h2" } },
    { "t": "Button", "k": "go", "p": { "label": "搜索", "variant": "primary" },
      "on": { "press": "h2" } }
  ]}
]}
```

| 字段 | 含义 |
| --- | --- |
| `t` | 组件类型，必须在白名单中（§9.6） |
| `k` | 稳定的 key：作为 MyGo 元素的 key（`c.Key`），也用来索引 Go 侧状态（如输入框内容）。有状态的组件必须提供，同一层级内唯一 |
| `p` | 属性，只允许 JSON 值 |
| `on` | 事件名 → 处理器 ID |
| `c` | 子节点 |

**处理器 ID**：每次 render 重新分配，插件线程只保留最近一棵树的处理器表。界面线程投递事件时带上处理器 ID 和树的版本号，版本不匹配的事件直接丢弃。

**限制**（解码时校验，超出时整棵树作废，显示错误占位）：深度不超过 64，节点不超过 2000，单个字符串不超过 64 KiB。

### 9.3 JS SDK：`h` 与 hooks

SDK 提供 JSX 运行时（`jsxImportSource: "lunabox"`）和一组精简的 hooks：

```tsx
import { useState, Column, Row, Input, Button, Text } from 'lunabox'

function SearchPanel({ gameId }: { gameId: string }) {
  const [q, setQ] = useState('')
  const [result, setResult] = useState<string | null>(null)
  const run = async () => setResult(await search(q))
  return (
    <Column gap="md">
      <Row gap="sm">
        <Input k="q" value={q} placeholder={ctx.i18n.t('search.placeholder')} onChange={setQ} onSubmit={run} />
        <Button k="go" label={ctx.i18n.t('search.go')} variant="primary" onPress={run} />
      </Row>
      {result && <Text text={result} tone="muted" />}
    </Column>
  )
}

export function apply(ctx: Context) {
  ctx.ui.slot('game.detail.tab', { id: 'search', title: ctx.i18n.t('tab.title'), render: SearchPanel })
}
```

- hooks 只提供 `useState`、`useEffect`（清理函数挂在插槽实例的作用域上）、`useRef`、`useMemo`。
- 同一轮微任务内的多次 `setState` 合并为一次 render。
- 第一版每次都发送整棵树（插件界面一般很小）。将来实测有瓶颈时，再改为按 key 发送补丁，协议里给这个留了 `"v"`（版本）字段。

### 9.4 插槽实例与生命周期

插槽实例 = 一次挂载，由 `(插槽名, 插件 ID, 注册 ID, props)` 确定，例如游戏详情页的标签页按 `gameId` 区分。

1. 界面在宿主点**进入**时挂载实例：Go 新建子作用域，`Post` `mount(instanceId, props)` 到插件线程，插件执行第一次 render。
2. 第一棵树到达之前，渲染器画占位（不占高度，或使用该插槽规定的骨架）。
3. 宿主点**离开**时卸载实例：撤销实例作用域（其中包括 `useEffect` 的清理函数），丢弃缓存的树和输入框状态。
4. props 变化（如切换到另一款游戏）= 先卸载，再挂载。

MyGo 的界面每帧重建，没法从帧上判断“组件卸载了”，所以**进入和离开绑定到页面已有的 `Open`/`Leave`**（`ui/routes.go` 里 `entered` 时调用的那组方法）。常驻的插槽（如侧边栏项）生命周期等于注册本身。

### 9.5 缓存与重绘

- 界面侧持有 `map[instanceID]*Instance`，每个 `Instance` 包含：树、树版本、输入框状态、错误。只在界面线程上读写。
- 插件线程产出新树后：解码、校验（插件线程）→ `env.Win.Update(func(){ inst.tree = t; inst.version++ })`。`Update` 本身会请求下一帧。
- **合并**：同一实例在一帧内收到多棵树时只保留最新的一棵（在插件线程用“待发送槽”实现，不排队）。

### 9.6 组件白名单（apiVersion 1）

| `t` | 映射 | 主要属性 | 事件 |
| --- | --- | --- | --- |
| `Column` / `Row` / `Box` | `ui.Column` / `ui.Row` / `ui.Box` | `gap`、`padding`、`align`、`justify`、`width`、`height`、`grow`、`dragWindow`（悬浮窗拖动） | `press`、`contextMenu` |
| `ScrollArea` | `components.ScrollArea` | `direction: "vertical" \| "horizontal"`、`padding`、`autoScrollBottom`（AI 对话流自动贴底） | `ReachBottom`（分页加载） |
| `Grid` | 响应式网格容器 | `minColWidth`、`cols`、`gap`（用于资源站游戏卡片网格） | — |
| `Text` | `components.Text` | `text`、`size`、`weight`、`tone`、`wrap`、`maxLines`、`selectable` | — |
| `Markdown` | 结构化富文本 / Markdown 渲染器 | `content`（支持标题、列表、代码块、引用、表格、图片、链接，用于站点详情文章与 AI 回复流） | `linkPress` |
| `CodeBlock` | 代码 / 结构化块 | `code`、`language`、`copyable` | — |
| `Button` | `better.Button` | `label`、`icon`、`variant`、`size`、`disabled`、`loading` | `press` |
| `IconButton` | `better.IconButton` | `icon`、`label`（无障碍名称）、`size`、`active` | `press` |
| `Input` | `better.Input` | `value`、`placeholder`、`password`、`disabled`、`prefixIcon` | `change`、`submit` |
| `Textarea` | `better.Textarea` | `value`、`placeholder`、`rows`、`autoResize`（AI 聊天输入框） | `change`、`submit`（Ctrl/Cmd+Enter 或 Enter） |
| `Switch` | `better.Switch` | `checked`、`disabled` | `change` |
| `Checkbox` | `components.Checkbox` | `checked`、`label` | `change` |
| `Select` | `better.Select` | `value`、`options: [{ value, label }]` | `change` |
| `NumberInput` | `better.NumberInput` | `value`、`min`、`max`、`step` | `change` |
| `Tabs` | 标签切换栏 | `value`、`items: [{ key, label, badge? }]` | `change` |
| `Badge` | `components.Tag` / 徽章 | `label`、`tone`、`icon`、`closable` | `press`、`close` |
| `Card` | 卡片容器（同 `components/card` 的样式） | `padding`、`title`、`cover`、`hoverable` | `press` |
| `Collapsible` | `components.CollapsibleSection` | `title`、`icon`、`defaultOpen`、`status`（AI 工具调用折叠卡片） | — |
| `Progress` | `components.ProgressBar`；`indeterminate` 时用 `components.Spinner` | `value`（0–1）、`indeterminate`、`label` | — |
| `Image` | 图片元素（复用封面/HTTP 图片缓存） | `src`（插件资源路径或 http URL）、`width`、`height`、`aspectRatio`、`fit`、`radius`、`blur`、`previewable`（点击打开 `better.ImageViewer`） | `press` |
| `Avatar` | 圆形/圆角头像 | `src`、`fallback`、`size`（站点评论作者、AI 角色头像） | — |
| `Empty` / `Skeleton` | 空状态 / 骨架占位 | `title`、`description`、`icon` / `lines`、`height` | — |
| `Tooltip` | `better.Tooltip` | `content`，单个子节点 | — |
| `Table` | `better.DataTable` | `columns`、`rows` | `rowPress` |
| `Divider` / `Spacer` | `components.Divider` / 弹性空白 | — | — |

未知的 `t`：开发模式下画红色占位并写明类型，正式版跳过该节点。

### 9.7 输入框：状态由 Go 持有

`better.Input` 本来就接收 `*string`。渲染器为每个 `(实例, k)` 保存一份 `*string`，用户输入直接改这份数据，不需要往返插件线程，输入法候选和光标都不受影响。

- **初值与程序设置**：渲染器记住插件上次下发的 `value`（`lastSent`）。新树里的 `value` 与 `lastSent` 不同时，说明插件主动改了值，用它覆盖本地；相同时保留本地内容，正在输入的文字不会被冲掉。
- **通知插件**：`change` 防抖 150ms 后带最新文本投递；`submit`（回车）立即投递，并带上当前文本。
- `Textarea`、`NumberInput` 按同样规则处理。

### 9.8 样式只用主题变量

| 属性 | 可选值 | 映射 |
| --- | --- | --- |
| `gap` / `padding` | `none` `xs` `sm` `md` `lg` `xl` | 0 / 2 / 4 / 8 / 12 / 16 / 24 |
| `size` | `xs` `sm` `base` `lg` `xl` | `theme.TextXS` … `theme.TextXL` |
| `weight` | `normal` `medium` `semibold` `bold` | `theme.WeightNormal` … `theme.WeightBold` |
| `tone` | `default` `muted` `strong` `brand` `success` `warning` `danger` | 当前 `*theme.Theme` 上对应的颜色 |
| `variant`（按钮） | `secondary`（默认）`primary` `danger` `ghost` | `better.ButtonSecondary` … `better.ButtonGhost` |

不接受像素值和颜色值。深色模式、玻璃模式、缩放全部自动跟随。

### 9.9 错误边界

- render 抛错、树校验失败、插件处于 `error` 状态时，该实例显示统一的错误占位：插件名 + “界面出错”，开发模式下附带错误信息。新增的提示文字放进 `ui/locales`。
- 一个实例出错不影响页面其他部分，也不影响同一插件的其他实例。

## 10. 插槽（apiVersion 1）

| 插槽 | 位置 | 注册参数 | props | 生命周期 | 宿主点 |
| --- | --- | --- | --- | --- | --- |
| `sidebar.page` | 侧边栏一级导航项 + 独立路由整页（`/plugin/{pluginId}/{id}`） | `id`、`title`、`icon`、`group?: "main" \| "bottom"`、`badge?`、`render` | `{ params, query }` | 进入/离开该路由 | `ui/components/bar/sidebar.go` 的 `navItems`、`ui/routes.go` |
| `workspace.drawer` | 主窗口右侧常驻可折叠副栏（抽屉），跨页面保留会话状态 | `id`、`title`、`icon`、`defaultWidth?`、`render` | `{ route, gameId?, categoryId? }` | 抽屉打开/关闭 | `ui/app.go` 主行布局右侧（通过 `ctx.ui.openDrawer` 或顶栏按钮唤起） |
| `topbar.item` | 窗口顶栏右侧（最小化按钮左侧）快捷图标按钮 | `id`、`title`、`icon`、`badge?`、`onPress` | — | 注册期间 | `ui/components/bar/topbar.go`（不用描述树，只是图标按钮） |
| `library.add_menu` | 游戏库右上角“+ 添加游戏”下拉菜单项 | `id`、`title`、`icon`、`onPress` | — | 注册期间 | `ui/pages/library/view.go` 的 `addMenu` |
| `library.toolbar` | 游戏库筛选栏右侧扩展按钮 / 批量模式动作栏按钮 | `id`、`title`、`icon`、`mode?: "normal" \| "batch" \| "both"`、`onPress` | `{ selectedGameIds }` | 注册期间 | `ui/components/bar/filterbar.go`、`ui/pages/library/batch.go` |
| `game.detail.tab` | 游戏详情页标签页，排在内置标签页之后 | `id`、`title`、`badge?`、`render` | `{ gameId, game }` | 详情页 `Open`/`Leave` | `ui/pages/game/game.go` 的 `tabs` |
| `game.detail.header` | 游戏详情页页头扩展区（外部站点徽章、资源下载快捷按钮） | `id`、`render` | `{ gameId, game }` | 详情页 `Open`/`Leave` | `ui/pages/game/view.go` 页头信息栏 |
| `game.detail.aside` | 游戏详情页“统计”标签页右侧信息列附加卡片 | `id`、`title`、`render` | `{ gameId, game }` | 详情页 `Open`/`Leave` | `ui/pages/game/stats.go` |
| `game.actions` | 游戏卡片右键菜单、详情页“更多”菜单 | `id`、`title`、`icon`、`when?(game)`、`run(game)` | — | 注册期间 | `ui/components/card/gamecard.go`、`ui/pages/game/view.go` |
| `downloads.header` | 下载管理页顶部工具栏扩展区（如站点配额、登录状态、网盘入口） | `id`、`render` | — | 下载页 `Open`/离开 | `ui/pages/downloads/view.go` |
| `app.decoration` | 主窗口全局背景层 / 前景层装饰位点（等价于 `ctx.theme.registerDecoration`） | `id`、`layer`、`anchor`、`offset`、`size`、`opacity`、`pointerEvents`、`routes?`、`image` 或 `render` | `{ route, dark }` | 插件启用期间 | `ui/app.go` 的 `View`（`root.Draw` 后与 `layout` 后） |
| `settings.section` | 设置页，排在内置分区之后，每次注册一个 `CollapsibleSection` | `id`、`title`、`icon`、`render` | — | 设置页 `Open`/离开 | `ui/pages/settings/view.go` 的 `sections()` |

`ctx.config.define(schema)` 会自动生成一个 `settings.section`，插件不需要为简单设置写界面。schema 支持 `string`、`boolean`、`number`、`enum`、`secret`（密码输入框）。

插槽规格（名称、props、尺寸约束）属于 API 的一部分，变更规则见 §13。

### 10.1 六大典型场景与插槽 / SDK 组合对照

1. **下载类插件**（如浏览 TouchGal 资源站并走宿主下载导入流程）
   - **位点（Slots）**：`sidebar.page`（独立资源站浏览页：搜索、分类筛选、卡片网格）、`library.add_menu`（“从 TouchGal 下载导入”入口）、`game.detail.tab` / `game.detail.header`（已入库游戏关联资源或汉化补丁下载）、`downloads.header`（站点账号配额与节点状态）。
   - **SDK 与组件**：`fetch`（调用 TouchGal API `/api/game-resources`）、`ctx.downloads.requestInstall`（唤起宿主 `InstallConfirmModal` → `DownloadService` 下载解压 → `ImportService` 自动入库）、`ctx.import.openModal` / `fromPath`（自定义下载器完成后移交本地路径入库）、`ctx.games.findBySource` / `ctx.downloads.getImportStates`（卡片显示“已在库中 / 下载中”）；组件使用 `Grid`、`Card`、`Image`、`Badge`、`ScrollArea(ReachBottom)`。

2. **站点浏览插件**（如查看 KunGal `/galgame/3817` 完整详情内容，不止于宿主当前的游戏详情）
   - **位点（Slots）**：`game.detail.tab`（在游戏详情页新增“KunGal 详情”标签页，展示长文介绍、汉化组说明、多平台资源列表、评分与评论区）、`game.detail.header` / `game.actions`（页头徽章与右键“在 KunGal 查看”）、`sidebar.page`（KunGal 社区广场与新作发现页，通过 `ctx.ui.navigate` 与本地库游戏双向跳转）。
   - **SDK 与组件**：`fetch` + `ctx.html.parse`、`ctx.games.get` / `update`（写回关联 ID 或补充信息）、`ctx.downloads.requestInstall` / `ctx.shell.openURL`；组件使用 `Markdown`（富文本、评测、剧透折叠）、`Tabs`、`Table`、`Image(previewable)`、`Avatar`、`Collapsible`。

3. **主题类插件**（自定义主题色、字体、右下角角色水印等背景装饰）
   - **位点（Slots）**：`app.decoration` / `ctx.theme.registerDecoration`（在 `bottom-right` 等九宫格锚点挂载角色立绘水印或动态挂件，`pointerEvents: 'none'` 点击完全穿透）、`settings.section`（或 `ctx.config.define`，切换配色预设、调节水印角色与不透明度）。
   - **SDK 与组件**：`ctx.theme.extend`（按 `light` / `dark` 覆盖 `theme.Palette` 语义色板与毛玻璃参数）、`ctx.theme.registerFont`（加载插件 TTF/OTF 字体替换 `theme.UIFont`）、`ctx.theme.registerDecoration`（前景/背景装饰层）。

4. **元数据类插件**（自定义接入元数据源）
   - **位点（Slots）**：注册后自动接入宿主全部元数据位点（添加/导入向导、详情页“编辑 → 获取元数据”、批量获取元数据弹窗、设置页“元数据来源优先级与策略”），配合 `settings.section` 配置镜像域名或 API Key。
   - **SDK 与组件**：`ctx.metadata.registerSource`（`search`、`fetch`、`fetchBatch`、`normalizeId`、`rateLimit`）、`ctx.on('metadata/before-search')` / `ctx.on('metadata/after-fetch')` 钩子。

5. **悬浮球类插件**（打开另一个独立窗口作为桌面悬浮窗）
   - **位点（Slots）**：通过 `ctx.window.open` 创建独立无边框透明置顶窗口（同样使用 JSX 描述树渲染），配合 `topbar.item`（主窗口顶栏一键开关悬浮球）与 `settings.section`（配置尺寸、透明度、贴边隐藏、随游戏启动自动显示）。
   - **SDK 与组件**：`ctx.window.open`（`frameless`、`transparent`、`alwaysOnTop`、`skipTaskbar`）、`win.setSize` / `setPosition`（点击悬浮球从 `64×64` 原地展开为快捷控制面板）、`ctx.window.main`（唤醒/最小化主窗口）、`ctx.on('game/launched')` / `ctx.on('session/ended')`（展示当前游玩计时）；组件使用 `Box(dragWindow: true)`、`Image`、`Button`。

6. **AI 类插件**（AI 帮搜资源、AI 帮整理游戏库，交互形式类似 Claude Code Desktop）
   - **位点（Slots）**：`sidebar.page`（独立 AI 工作台页面：左侧会话列表 + 中间流式对话与 Tool Call 折叠卡片 + 右侧游戏库变更 Diff 预览）、`workspace.drawer` + `topbar.item`（在任意页面呼出右侧 AI 伴随抽屉）、`library.toolbar`（批量模式下“让 AI 整理选中游戏 / 自动分类打标签”）、`game.actions`（右键游戏“让 AI 找资源/总结攻略”）。
   - **SDK 与组件**：`ctx.ai.stream` / `ctx.http.sse`（流式输出与 Function Calling 工具调用）、`ctx.games`（`list`、`batchUpdate`、`setTags`）+ `ctx.categories`（`list`、`create`、`addGames`）、`ctx.metadata.search` + `ctx.downloads.requestInstall`（在对话流内渲染资源卡片并一键下载入库）；组件使用 `ScrollArea(autoScrollBottom)`、`Markdown`、`CodeBlock`、`Collapsible(status)`、`Textarea(autoResize)`。

## 11. 插件管理

### 配置

在 `appconf.AppConfig` 中新增：

```go
Plugins        []PluginEntry `json:"plugins,omitempty"`          // 顺序 = 钩子执行顺序
PluginDevPaths []string      `json:"plugin_dev_paths,omitempty"` // 开发中的插件目录，直接从源目录加载

type PluginEntry struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}
```

按 [backend.md](backend.md) 的配置同步约束：同步修改 `config_service.go` 的 `UpdateAppConfig(...)`，规范化函数写在 `appconf/normalize.go`。

### 插件管理页

设置页新增内置的“插件”分区（原生 Go 界面，不是插件界面），列出：名称、版本、状态（加载中 / 运行中 / 已停用 / 出错及原因）、启用开关、重新加载、打开插件目录、卸载（可选是否删除数据）、拖动排序。

### 安装、升级与卸载

- **安装**：选择或拖入 `.zip` → 解压到临时目录 → 校验清单与兼容性 → 弹出确认框（**明确提示插件可以访问全部文件与网络，只安装信任的插件**）→ 移动到 `plugins/<id>/` → 加载。
- **升级**：版本更高的同 ID 插件 → 卸载旧版 → 替换目录 → 加载。
- **卸载**：卸载运行时 → 删除 `plugins/<id>/` → 按用户选择删除 `plugins-data/<id>/`。

### 开发模式热重载

`PluginDevPaths` 中的目录由 `host` 用 `mygo.WatchFiles` 监听（后端不依赖 MyGo），文件变化防抖 200ms 后调用 `Manager.Reload(id)`。正式安装的插件不监听，只能手动重新加载。

### 插件市场（后期）

GitHub 上维护一个索引仓库（`index.json`：id、版本、下载地址、sha256、`engines`），插件仓库加 topic `lunabox-plugin` 便于发现。客户端查询索引、检查更新，下载后走同样的安装流程。

## 12. SDK 与开发体验

- `packages/plugin-sdk/`：独立的 pnpm 项目（遵循 monorepo 约定，不加入根 workspace），发布为 npm 包 `lunabox`（或 `@lunabox/plugin-sdk`，在 tsconfig 的 `paths` 中映射为 `lunabox`）。内容：
  - `ctx`（含 `games`、`categories`、`metadata`、`downloads`、`import`、`theme`、`window`、`ai`、`http`、`ui`）和全部事件、插槽、`MetadataResult`、`ThemeTokens` 的 TS 类型。
  - JSX 运行时、hooks（`useState`、`useEffect`、`useRef`、`useMemo`）、组件工厂（`Column`、`Grid`、`ScrollArea`、`Markdown`、`Button`……只是返回描述节点的函数）。
  - 高阶辅助工具（SDK 内置纯 JS 封装）：`createAIChatStore`（管理多轮消息、流式拼接与 Tool Call 状态）、`createPagination`（配合 `ScrollArea(ReachBottom)` 的分页器）、`defineThemePreset`（主题色板与水印装饰类型校验）。
  - CLI：`lunabox-plugin build` 用 esbuild 打包（`format: esm`、`platform: neutral`、`target: es2022`，把依赖全部打进去，`lunabox` 标记为 external），`lunabox-plugin pack` 打成 zip。
- `packages/plugin-sdk/examples/`：
  - `metadata-source`：自定义元数据源（§8）
  - `resource-hub`：资源站浏览与一键下载入库（TouchGal 场景，`sidebar.page` + `ctx.downloads.requestInstall`）
  - `site-detail`：扩展站点详情页（KunGal 场景，`game.detail.tab` + `Markdown`）
  - `theme-mascot`：自定义主题色板、字体与右下角角色立绘水印（`ctx.theme`）
  - `float-ball`：桌面置顶透明悬浮球（`ctx.window.open` + `dragWindow`）
  - `ai-assistant`：AI 找资源与整理游戏库工作台（`sidebar.page` + `workspace.drawer` + `ctx.ai.stream`）
- 调试：`console.log` 写入应用日志；开发模式下界面错误直接显示在占位里。

## 13. 版本与兼容

- `apiVersion` 是插件 API 的主版本。同一主版本内只做**新增**（新服务、新事件、新组件、新属性），不改变已有成员的行为。
- 删除或修改已有成员时 `apiVersion + 1`。宿主同时支持当前版本和前一个版本，期间旧版本的 API 由适配层实现。
- `engines.lunabox` 用于要求宿主的最低版本（比如依赖某个新事件）。
- 宿主侧依赖的 moejs 锁定到具体版本。moejs 的 API 只在 `packages/backend/plugin` 的适配层中使用，换引擎时只改这一层。

## 14. 代码落点

| 路径 | 内容 |
| --- | --- |
| `packages/backend/plugin/` | `manifest.go`、`manager.go`（发现、加载、卸载、状态）、`runtime.go`（moejs 适配、插件线程）、`scope.go`、`host_*.go`（`games`/`categories`/`metadata`/`downloads`/`import`/`theme`/`window`/`ai`/`http` 等 `ctx` 服务）、`slots.go`（插槽注册表）、`uitree.go`（描述树类型、解码与校验，不依赖 MyGo） |
| `packages/backend/common/hooks/` | `Bus` 接口、`Nop`、事件名常量与 payload 类型 |
| `packages/backend/utils/metadata/registry.go` | 外部元数据源注册表 |
| `packages/backend/bootstrap/services.go` | 创建 `plugin.Manager`，向服务注入 `hooks.Bus` |
| `packages/backend/appconf/` | `Plugins`、`PluginDevPaths` 及规范化 |
| `api/` | `PluginService` 接口：列表、启用、重载、安装、卸载、插槽实例的挂载与卸载、事件投递、订阅新树、主题覆盖与子窗口请求 |
| `host/` | 开发目录监听；把插件线程产出的树、主题变更与 `ctx.window.open` 请求桥接到界面线程 |
| `ui/pluginui/` | `Instance`、缓存、渲染器（树 → 组件，含 `Markdown`/`Grid`/`ScrollArea`）、输入框状态、错误占位、`window.go`（插件独立悬浮窗管理）、`decoration.go`（前景/背景水印装饰层） |
| `ui/theme/` | 支持叠加插件注册的 `Palette` 覆盖与自定义字体 |
| `ui/app.go`、`ui/pages/*`、`ui/components/bar`、`ui/routes.go` | 插槽宿主点（侧边栏页、右侧抽屉、顶栏按钮、游戏库菜单与批量栏、详情页标签与页头、下载页头、全局装饰层）、插件管理分区 |
| `ui/metasource` | 加上插件来源 |
| `ui/locales/*.json` | 插件管理页与错误占位的文字（改完运行 `pnpm i18n`） |
| `packages/plugin-sdk/` | SDK、CLI、六大场景示例插件 |

## 15. 分期

| 阶段 | 内容 | 验收 |
| --- | --- | --- |
| **P1 运行时 + 元数据源 + 主题扩展** | 清单、Manager、插件线程、Scope、基础 `ctx`（`logger/i18n/storage/config/html/shell/paths`）、`ctx.metadata`、`ctx.theme`（色板覆盖、自定义字体、`app.decoration` 右下角角色水印）、插件管理分区、开发热重载 | 示例元数据源插件可搜索与拉取；主题插件可切换配色、字体并在右下角显示点击穿透的角色水印；热重载无泄漏 |
| **P2 界面协议 + 站点浏览与下载入库** | `pluginui` 渲染器与组件白名单（含 `Grid`、`ScrollArea`、`Markdown`、`Tabs`）、`settings.section`、`game.detail.tab`、`game.detail.header`、`sidebar.page`、`library.add_menu`、`downloads.header`、`ctx.downloads.requestInstall`、`ctx.import` | TouchGal 资源浏览插件可搜索并拉起宿主 `InstallConfirmModal` 完成下载解压入库；KunGal 详情插件可在游戏详情页渲染完整站点图文；中文输入法与 60fps 滚动正常 |
| **P3 钩子、独立悬浮窗与 AI 工作台** | `hooks.Bus` 及 §7.1 事件、`ctx.window.open`（无边框透明置顶悬浮球）、`ctx.ai.stream` / `ctx.http.sse`、`ctx.games` 批量写与 `ctx.categories`、`workspace.drawer`、`topbar.item`、`library.toolbar`、`game.actions` | 悬浮球插件可桌面置顶拖拽、点击展开面板并联动主窗口；AI 插件可在独立页面或右侧抽屉流式对话、搜资源并批量整理收藏夹与标签 |
| **P4 分发** | zip 安装、升级与卸载、索引仓库、检查更新 | |

P2 风险最高，可以先用一个只有 `Text`/`Input`/`Button` 的 `settings.section` 做原型，验证输入法、延迟和热重载，再补全组件。

## 16. 风险与待定问题

| 问题 | 说明 / 倾向 |
| --- | --- |
| moejs 尚处早期（2026-09 首次提交，2026-11 发布 v0.1.0） | 锁定版本，只在适配层使用；准备好退路（goja/Sobek） |
| `node:fs` 的 `Mount` 能否挂载各盘符（完全信任） | P1 开始时验证；不行就只提供 `ctx.fs` |
| 插件数据放 JSON 文件还是 DuckDB 表 | 本文倾向 JSON 文件（不需要迁移，卸载时直接删除）；**需要确认**是否要随云同步 |
| 插件和插件数据是否参与云同步与备份 | 全量备份会自然带上；云同步**待定** |
| 高级元数据策略（`metadata_strategy.go`）对插件来源的处理 | 来源是字符串，原则上可以直接参与；P1 时检查策略代码里有没有只认内置来源的地方 |
| 热路径上的 waterfall 钩子 | 没有监听者时直接返回；必要时增加批量版本的钩子 |
| 完全信任的安全性 | 不做技术隔离；只靠安装确认、来源展示和索引仓库审核 |
