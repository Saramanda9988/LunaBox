# 界面规范

> 本文路径除特别标注外均相对 `apps/desktop/`。界面是 MyGo 原生 UI（Go），没有 WebView；“原版”指 `wails-final` 标签里的 Wails + React 前端。

## IMPORTANT

- MUST `ui/locales/catalog_gen.go` 是 `mygo generate` 生成的，不要手改；只改 `ui/locales/*.json`，改完重新生成。
- MUST 界面只通过 `api/` 的接口（`env.Env.Svc`）调用后端，不直接 import `packages/backend` 的 service 实现或数据库。
- MUST NOT 在界面线程上做可能阻塞的操作（文件、数据库、网络、进程），见[界面线程与性能](#界面线程与性能)。

---

## 目录与依赖方向

- MUST 保持依赖方向 `pages` → `components/*` → `components` → `theme`、`assets`、`locales`，不得反向引用。
- MUST 组件不读页面状态：只通过参数与回调和页面交互。
- MUST 组件自己的临时状态（弹层开关、悬停计时、滚动条）挂在自己的元素上（`ui.Local`），不放进页面或包级变量。
- MUST 跨帧保留的页面状态由页面的结构体持有；面板状态由持有它的页面持有（如设置页的 `panels`），不得放在包级变量。
- MUST 每个详情页标签页、每个设置分区一个 `ui/components/panel/<面板>` 子包，输入分别只用 `panel.Game`、`panel.Settings`。
- MUST 元数据来源的名称、链接、图标、说明只从 `ui/metasource` 取；来源列表与默认顺序只以后端 `enums.MetadataSourceTypes` 为准。

---

## 路由与页面

- MUST 路由只在 `ui/routes.go` 注册，页面放在 `ui/pages/<页面>/`。
- MUST 进入页面时的加载放在页面的 `Open`（只在位置变化时调用一次），不得在每帧构建时重复发起。
- MUST 离开页面时需要补存的内容（草稿、待保存的修改）在离开钩子里写入（设置页 `Flush`、详情页 `Leave`），不得依赖“之后还会再构建一帧”。
- MUST 弹窗、抽屉与浮层在页面内容之后构建，保证它们盖在页面之上。

---

## 配置状态

配置有两份：

- 生效的配置：`env.Env.Config()`，运行态页面、根布局与应用级逻辑读取它。
- 设置页草稿：`panel.Settings.Config`，只有设置页的面板读取它。

写入 API（MUST 使用，不要绕过）：

| API | 用途 |
| --- | --- |
| `panel.Settings.Change` | 设置页的普通修改：只改草稿，停止修改 250ms 后由设置页统一保存 |
| `panel.Settings.PatchLive` | 设置页里必须当场生效的修改（如界面缩放）：同时改草稿与生效的配置并立即保存 |
| `env.Env.PatchConfig` | 设置页以外的配置修改：立即保存 |

- MUST 设置页面板只读草稿，不得读 `Env.Config()`；运行态页面只读 `Env.Config()`，不得读草稿。
- MUST NOT 在设置页另外维护一份与草稿平级的配置副本作为真源；弹窗内部的编辑草稿 MAY 存在，但写回只能经过上表的 API。
- MUST NOT 在交互后还要播放动画的控件（开关、滑块等）的回调里调用 `PatchLive` / `PatchConfig`：保存是同步写文件，会卡住这一帧并吞掉控件的动画。设置页里用 `Change`；设置页以外的弹窗攒到停止修改后、关闭时或离开页面时再写入。
- SHOULD NOT 在其他高频交互（连续选择、拖动中）里同步保存配置；只在交互结束时保存一次。
- MUST 服务自己改了配置（授权、登出、安排恢复等）之后，重新读取配置覆盖草稿，不得让旧草稿把服务写入的值覆盖回去。
- MUST 新增 `AppConfig` 字段时同时检查后端 `SaveConfig` 的规范化与 `ConfigService.UpdateAppConfig(...)` 是否把该字段写回内存里的配置。

---

## 界面线程与性能

- MUST 构建视图与输入回调都在界面线程上执行；可能阻塞的后端调用一律放进 `env.Env.Async`，结果在它返回的函数里（回到界面线程）应用。
- MUST NOT 在 `Async` 的后台部分读写界面状态；后台只做后端调用，界面状态只在回到界面线程的函数里改。
- MUST NOT 在界面代码里自己开 goroutine 修改界面状态。
- MUST NOT 在每帧构建时调用后端或做与数据量成正比的重计算；需要的数据在进入页面或数据变化时取一次，缓存在页面状态里。
- MUST 动画、计时、防抖、悬停延迟等界面时间状态用帧时钟：构建时用 `c.Now()`，输入事件用事件自带的时间，不得用 `time.Now()`。只有写进数据或显示给用户的真实时间（今天的日期、记录的时间戳）MAY 用 `time.Now()`。
- MUST 需要在一段时间后再构建（防抖、倒计时）时向框架请求下一帧（`c.After`），不得用 goroutine + sleep。
- MUST NOT 为大面积或大量重复的元素使用背景模糊（`glass.Blur`）；只用在少数浮层与小元素上。
- SHOULD 改动滚动列表、游戏库网格、首页等高频页面后用 `-hud` 或 `-bench` 确认帧间隔没有变差。

---

## 样式与主题

- MUST 颜色、字号、过渡曲线只用 `ui/theme` 的色板与常量；不得硬编码新的颜色值或字号。
- MUST 文字用 `components.Text` 系列构建（行高与原版一致），不得直接用 `ui.Text` 设字号。
- MUST 图标用 `components.Icon` 与 MDI 的图标名，不得引入其他图标集。
- MUST 保持现有的尺寸、间距、颜色取值；改动界面时不凭截图估值。
- MUST 新增或修改的界面在浅色与深色主题下都可读，深浅取值成对给出（`t.Pick`）。
- MUST 适配自定义背景的玻璃模式：
  - 除弹窗与浮层外，新增的面板、卡片、输入框 SHOULD 使用 `theme` 的玻璃取值（`GlassPanel`、`GlassCard`、`GlassInput`、`IfGlass` 等），玻璃模式下不得用不透明的大面积底色。
  - 弹窗与浮层内 MUST 关闭玻璃模式（经过 `components.Overlay` / `components.Modal`），保持不透明。
  - 页面最外层容器 MUST NOT 设置底色或不透明度，由根布局统一控制。
- MUST 禁用态用 `components.Disable`，不得直接设置不透明度模拟禁用。

---

## 设置页面板

- MUST 设置页本身只负责分区的折叠与装配；单条配置项的样式在各 `ui/components/panel/<面板>` 内实现。
- MUST 单条配置项的标题、说明、小标题分别用 `panel.FieldLabel`、`panel.FieldHint`、`panel.GroupTitle`，不得自定义字号与颜色。
- MUST 开关型配置项用 `better.SettingSwitchRow`（左文案、右开关的两栏布局）。
- MUST NOT 在面板内部为普通设置分组再套一层卡片制造额外边距：分区容器已经提供外层面板与内边距。
- MAY 仅对独立语义块（危险提示、说明框、备份列表、恢复操作区等）使用额外卡片样式。
- SHOULD NOT 在小标题下方附带说明文字；说明挂在具体配置项上。

---

## 组件

- MUST 以 MyGo `ui` 包与项目自己的组件为主；可复用的控件放在 `ui/components`，Better 系列放在 `ui/components/better`。
- MUST NOT 引入第三方 UI 组件库；引入任何新依赖前先询问。
- MUST 新增组件前先在 `ui/components` 与 `ui/components/better` 中查找已有实现，优先复用或扩展。
- MUST 只有图标的可交互控件设置无障碍名称（`Label`）；需要悬停提示时用 `better.Tooltip`。
- MUST 元素的 key 用稳定的标识（路径、ID、英文常量），不得用翻译后的文字或会变化的显示内容。
- MUST 同一父元素下的兄弟元素 key 不重复；循环里的元素用数据的 ID 作 key，不用下标（列表顺序会变时）。

---

## 弹窗与浮层

- MUST 弹窗用 `components.Modal`（或基于它的 `ConfirmModal`、`ImportModalContainer` 等），浮层用 `components.Overlay`，不得自己画全窗口遮罩。
- MUST 弹窗遮罩从顶栏下方开始（`TopOffset`），保留顶栏与窗口按钮可用。
- MUST 可关闭的弹窗支持 Esc 与点击遮罩关闭（提供 `OnClose`）；进行中不可中断的弹窗（进度、授权等待）MAY 不提供。
- MUST 弹窗关闭时写入其中还没保存的修改；弹窗所在的页面离开时同样写入。

---

## 文字与多语言

- MUST 界面上的文字都来自 `ui/locales` 的生成函数；不得在界面代码里写死中文、日文（`TestNoHardcodedText` 会检查）。确实不翻译的写 `// i18n:keep：原因`。
- MUST 新消息四种语言（`zh-CN`、`zh-TW`、`en-US`、`ja-JP`）都写；加到源语言 `zh-CN.json` 时带 `@<消息名>` 的 `description`。
- MUST 优先复用原版在同一处用的 key；原版没有的文字用 `native_<名字>` 消息。
- MUST 固定的 key 用生成的函数；只有 key 由变量拼成时才用 `locales.Key`。
- MUST 每帧构建时取翻译；不得把翻译结果存进包级变量、常量、结构体字段或 `ui.Local`（切换语言后不会更新）。需要一张表时存函数或 key。
- MUST 消息参数里的数字由调用方格式化后传入。
- MUST NOT 翻译开发者看的文字（命令行参数说明、日志、内部错误前缀）、游戏数据与测试数据。

---

## 时间与日期

- MUST 时长、日期的显示只用 `ui/timefmt` 的函数。
- MUST 其他按语言格式化的日期、数字使用 `timefmt.FormatLocale()` 给出的区域，不得写死区域或格式串。

---

## 与后端交互

- MUST 后端的事件只在应用级（`ui/runtime_events.go`）或持有状态的页面订阅一次，不得在每帧或多个页面重复订阅。
- MUST 后台调用的结果回来时检查它是否已过期（页面已离开、已发起更新的请求），过期的结果直接丢弃。
- MUST 用户主动发起的操作失败时给出可见的提示（toast），不得只记日志；只有后台、用户无感知的写入（如保存窗口与侧边栏状态）MAY 只记日志。
- MUST 界面框架能力（文件对话框、打开网址、开机自启等）只经过 `appruntime.Runtime` 的实现（`host/`），界面不直接调用系统 API。

---

## 测试

- MUST 界面测试用 `ui.Tester`，不开窗口；推进时间用 `tt.Advance`，不得 `time.Sleep`。
- MUST 需要后端的测试使用真实服务（`host/hosttest`）与临时数据库，不得连接开发或安装版的数据目录。
- MUST 断言界面文字时用 `locales` 的函数或源语言 zh-CN 的文字；切换语言的测试结束时恢复。
- MUST 改动所在包的 `go test` 通过；多包测试加 `-p 4`。
