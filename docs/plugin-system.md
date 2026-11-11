# 插件系统设计（草案）

> 状态：**设计草案，尚未实现**。本文是实现时的约定来源；实现过程中改了约定，先改这里。
> 路径约定同 [AGENTS.md](../AGENTS.md)：`packages/...` 相对仓库根，`ui/...`、`host/...`、`api/...` 相对 `apps/desktop/`。

## 1. 目标与非目标

**目标**

- 运行时加载、卸载、热重载插件，不重新编译 LunaBox。
- 插件用 JavaScript 编写（TypeScript 经打包后交付），由内嵌的 JS 引擎 [moejs](https://github.com/MoeForge/moejs) 执行。
- 借鉴 [Cordis](https://github.com/cordiverse/cordis) 的思想：插件通过 `apply(ctx)` 激活，**每次注册都返回撤销函数并挂在插件的作用域上**，卸载时整个作用域一次撤销，插件作者不写清理代码。
- 插件可以：提供元数据源、监听或改写业务事件（钩子）、往界面的插槽里放界面。
- 插件界面用**描述树**：JS 在状态变化时生成描述树，Go 缓存它，每帧用缓存的树调用对应的 `components.Xxx(c, ...)` 绘制，外观与原生界面一致。

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
| `node:fs` | 安装 `node`，可写。Windows 下把各盘符作为 `Mount` 挂载，Unix 下 `Root` 为 `/`。**待验证**：moejs 的 `Mount` 能否覆盖这种用法；不行就只提供 `ctx.fs`（Go 直接实现）。 |
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
| `ctx.games` | `list(query)`、`get(id)`、`update(id, patch)`、`onChanged(fn)` | 包装 `GameService` 已有的公开方法；`update` 只开放白名单字段 |
| `ctx.metadata` | `registerSource(def)` | `metadata.Registry`（§8） |
| `ctx.on` | 事件与钩子（§7.1） | `hooks.Bus` |
| `ctx.ui` | `slot(name, def)`、`toast(msg, type?)`、`navigate(path)` | `plugin.Slots` + 界面（§9） |
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
| `Column` / `Row` | `ui.Column` / `ui.Row` | `gap`、`padding`、`align`、`justify`、`width: "fill" \| "auto"`、`grow` | — |
| `Text` | `components.Text` | `text`、`size`、`weight`、`tone`、`wrap` | — |
| `Button` | `better.Button` | `label`、`icon`、`variant`、`disabled`、`loading` | `press` |
| `IconButton` | `better.IconButton` | `icon`、`label`（无障碍名称）、`active` | `press` |
| `Input` | `better.Input` | `value`、`placeholder`、`password`、`disabled` | `change`、`submit` |
| `Textarea` | `better.Textarea` | `value`、`placeholder`、`rows` | `change` |
| `Switch` | `better.Switch` | `checked`、`disabled` | `change` |
| `Checkbox` | `components.Checkbox` | `checked`、`label` | `change` |
| `Select` | `better.Select` | `value`、`options: [{ value, label }]` | `change` |
| `NumberInput` | `better.NumberInput` | `value`、`min`、`max`、`step` | `change` |
| `Card` | 卡片容器（同 `components/card` 的样式） | `padding`、`title` | — |
| `Collapsible` | `components.CollapsibleSection` | `title`、`icon`、`defaultOpen` | — |
| `Progress` | `components.ProgressBar`；`indeterminate` 时用 `components.Spinner` | `value`（0–1）、`indeterminate` | — |
| `Image` | 图片元素 | `src`（插件资源路径或 http URL）、`width`、`height`、`fit` | `press` |
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
| `settings.section` | 设置页，排在内置分区之后，每次注册一个 `CollapsibleSection` | `id`、`title`、`icon`、`render` | — | 设置页 `Open`/离开 | `ui/pages/settings/view.go` 的 `sections()` |
| `game.detail.tab` | 游戏详情页，排在内置标签页之后 | `id`、`title`、`render` | `{ gameId }` | 详情页 `Open`/`Leave` | `ui/pages/game/game.go` 的 `tabs` |
| `sidebar.page` | 侧边栏导航项 + 整页，路由为 `/plugin/{pluginId}/{id}` | `id`、`title`、`icon`、`render` | — | 进入/离开该路由 | `ui/components/bar/sidebar.go` 的 `navItems`、`ui/routes.go` |
| `game.actions` | 游戏卡片右键菜单、详情页“更多”菜单 | `id`、`title`、`icon`、`when?(game)`、`run(game)` | — | 注册期间 | 不用描述树，只是菜单项 |

`ctx.config.define(schema)` 会自动生成一个 `settings.section`，插件不需要为简单设置写界面。schema 支持 `string`、`boolean`、`number`、`enum`、`secret`（密码输入框）。

插槽规格（名称、props、尺寸约束）属于 API 的一部分，变更规则见 §13。

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
  - `ctx` 和全部事件、插槽、`MetadataResult` 的 TS 类型。
  - JSX 运行时、hooks、组件工厂（`Column`、`Button`……只是返回描述节点的函数）。
  - CLI：`lunabox-plugin build` 用 esbuild 打包（`format: esm`、`platform: neutral`、`target: es2022`，把依赖全部打进去，`lunabox` 标记为 external），`lunabox-plugin pack` 打成 zip。
- `packages/plugin-sdk/examples/`：`hello`（`settings.section`）、`metadata-source`（元数据源）、`launch-guard`（`game/before-launch`）。
- 调试：`console.log` 写入应用日志；开发模式下界面错误直接显示在占位里。

## 13. 版本与兼容

- `apiVersion` 是插件 API 的主版本。同一主版本内只做**新增**（新服务、新事件、新组件、新属性），不改变已有成员的行为。
- 删除或修改已有成员时 `apiVersion + 1`。宿主同时支持当前版本和前一个版本，期间旧版本的 API 由适配层实现。
- `engines.lunabox` 用于要求宿主的最低版本（比如依赖某个新事件）。
- 宿主侧依赖的 moejs 锁定到具体版本。moejs 的 API 只在 `packages/backend/plugin` 的适配层中使用，换引擎时只改这一层。

## 14. 代码落点

| 路径 | 内容 |
| --- | --- |
| `packages/backend/plugin/` | `manifest.go`、`manager.go`（发现、加载、卸载、状态）、`runtime.go`（moejs 适配、插件线程）、`scope.go`、`host_*.go`（各个 `ctx` 服务）、`slots.go`（插槽注册表）、`uitree.go`（描述树类型、解码与校验，不依赖 MyGo） |
| `packages/backend/common/hooks/` | `Bus` 接口、`Nop`、事件名常量与 payload 类型 |
| `packages/backend/utils/metadata/registry.go` | 外部元数据源注册表 |
| `packages/backend/bootstrap/services.go` | 创建 `plugin.Manager`，向服务注入 `hooks.Bus` |
| `packages/backend/appconf/` | `Plugins`、`PluginDevPaths` 及规范化 |
| `api/` | `PluginService` 接口：列表、启用、重载、安装、卸载、插槽实例的挂载与卸载、事件投递、订阅新树 |
| `host/` | 开发目录监听；把插件线程产出的树转到界面线程 |
| `ui/pluginui/` | `Instance`、缓存、渲染器（树 → 组件）、输入框状态、错误占位 |
| `ui/pages/settings`、`ui/pages/game`、`ui/components/bar`、`ui/routes.go` | 插槽宿主点、插件管理分区 |
| `ui/metasource` | 加上插件来源 |
| `ui/locales/*.json` | 插件管理页与错误占位的文字（改完运行 `pnpm i18n`） |
| `packages/plugin-sdk/` | SDK、CLI、示例 |

## 15. 分期

| 阶段 | 内容 | 验收 |
| --- | --- | --- |
| **P1 运行时 + 元数据源** | 清单、Manager、插件线程、Scope、`logger/i18n/storage/config/html/shell/paths`、`ctx.metadata`、插件管理分区、开发热重载 | 示例元数据源插件能在导入、编辑页搜索、按 ID 拉取中使用；热重载后旧来源消失、新来源生效，无泄漏 |
| **P2 界面协议** | `pluginui` 渲染器、白名单组件、`settings.section`、`game.detail.tab`、`ctx.config.define` 自动设置页 | 中文输入法在插件输入框中正常；点击到重绘延迟不超过 2 帧；插件界面出错时页面其他部分正常 |
| **P3 钩子与更多插槽** | `hooks.Bus` 及 §7.1 的事件、`sidebar.page`、`game.actions` | 批量导入 2800 款游戏，无插件时耗时与当前一致 |
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
