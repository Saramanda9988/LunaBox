# 结构与运行环境

## 原则

- **同一份后端**：服务、数据库迁移、启动与退出流程都在 `packages/backend`（`go.mod` 里 `replace lunabox/backend => ../../packages/backend`）。服务的构造、后台任务、关闭到托盘、退出前同步、协议请求与关闭顺序在 `lunabox/backend/bootstrap`，与 Wails 版是同一份代码。这里只写 MyGo 相关的部分，不复制后端代码。页面通过 `api` 包的接口调用服务（`env.Env.Svc`），与原版前端调用 Wails 绑定一样。
- **同一份翻译**：界面文字来自原版的语言包，见 [i18n.md](i18n.md)。界面代码里不写死文字，`TestNoHardcodedText` 会检查。
- **一比一还原**：尺寸、颜色从运行中的原版导出计算样式，再逐像素对照，见 [parity.md](parity.md)。
- **测试不开窗口**：`ui.Tester` 构建视图、模拟输入、推进时钟（`tt.Advance`），测试在临时目录里新建数据库并跑完全部迁移（`hosttest.NewServices`）。

## 目录

目录对应原版 `apps/desktop/frontend/src`：

| 路径 | 内容 |
| --- | --- |
| `main.go` | 命令行参数与语言 |
| `app/` | MyGo 的应用外壳（对应 `apps/desktop/main.go`）：单实例、数据目录与日志、主窗口（隐藏标题栏与系统窗口按钮，由顶栏绘制；最大化按钮在 Windows 11 上显示贴靠布局）、托盘、启动错误窗，把窗口与框架事件交给 `bootstrap.App`；游戏库列表查询（`ui.Backend`，`-repeat` 压测时放大） |
| `host/` | 把后端接到 MyGo：`appruntime.Runtime`（原生文件对话框、事件、打开网址、开机自启）与 `bootstrap.Window` 的实现、把服务事件转到界面线程的 `Events`、给界面的 `api.Services`（`Backend.API`）、开发运行的数据目录；`host/hosttest` 给测试提供真实服务 |
| `api/` | 界面调用的后端接口（对应原版前端的 Wails 绑定），界面只依赖这些接口 |
| `ui/app.go`、`ui/routes.go` | 根布局（`routes/__root.tsx`）与路由表，用 MyGo 的 `ui.Router`；`ui/runtime_events.go` 是应用级事件（`useAppRuntimeEffects`、`useExitSyncToast`、`useDownloadNotifications`、`useCoverImageDownloadNotifications`）与启动时的 `GetPendingInstall`、`useUpdateCheck` |
| `ui/pages/home` | 首页（`routes/index.tsx`，默认路由 `/`） |
| `ui/pages/library` | 游戏库（`routes/library.tsx`） |
| `ui/pages/categories`、`category` | 收藏夹列表（`routes/categories.tsx`）与收藏详情（`routes/category.tsx`，路由 `/categories/{id}`） |
| `ui/pages/game` | 游戏详情页（`routes/game.tsx`，路由 `/game/{id}`）：页头、“游戏统计”标签页与页头弹窗 |
| `ui/pages/downloads` | 下载管理（`routes/downloads.tsx`） |
| `ui/pages/stats` | 统计（`routes/stats.tsx`） |
| `ui/pages/settings` | 设置页（`routes/settings.tsx`）：各分区的 CollapsibleSection、草稿与 250ms 后自动保存 |
| `ui/pages/startup` | 启动错误窗（`StartupWindow.tsx`） |
| `ui/components/panel/*` | `components/panel/*`，每个面板一个子包。详情页标签页：`edit`、`launch`（含启动游戏与运行状态）、`backup`、`progress`、`review`，共用输入 `panel.Game`。设置页分区：`basic`、`account`、`metadata`、`background`、`gamesettings`、`proxy`、`cloudbackup`、`autobackup`、`ai`、`dbbackup`、`fullbackup`、`update`、`appdata`、`portable`，共用输入 `panel.Settings`。首页的封面选择面板是 `homerail` |
| `ui/components` | `components/ui`：Modal / ConfirmModal / CategoryModal、Toaster、SlideButton、GameTags、EmojiPicker、NSFW 封面模糊、游戏状态配色、滚动区域、回到顶部等；`better` 子包是 Better 系列（Button、SplitButton、Select、Switch、Tooltip、DropdownMenu、Drawer、Input、ActionInput、ImageViewer、Textarea、EdgeIconButton、DataTable） |
| `ui/components/picker` | BetterDateRangePicker、BetterWheelPicker |
| `ui/components/importer` | 添加与导入游戏（`components/ui/import` 与 AddGameModal、BatchImportModal、DragDropImportModal、GameImportModal、SkippedImportCandidatesModal）：导入流程（`useImportFlow`）、预览表格、手动选择、元数据搜索结果（详情页编辑也用它）；根布局的拖放提示 |
| `ui/components/modal` | 多个页面共用的弹窗：AddToCategoryModal（详情页、游戏库批量操作）；根布局的 InstallConfirmModal、TimezoneSelectModal |
| `ui/components/bar`、`card`、`grid`、`chart` | `components/bar`（TopBar、SideBar、PlayingIsland、FilterBar）、`components/card`（GameCard、CategoryCard、DownloadCard）、`components/grid`（VirtualGameGrid 与分页加载）、`components/chart`（原版用到的全部图表，用 MyGo 的 `ui/chart`） |
| `ui/locales` | 多语言目录（由原版语言包生成）与类型化函数，见 [i18n.md](i18n.md) |
| `ui/theme`、`ui/assets` | `uno.config.ts` 的色板、字号、过渡曲线、像素吸附与玻璃模式（`theme/glass.go`）；图标、元数据来源图标、表情数据与图片 |
| `ui/env`、`ui/covers`、`ui/perf`、`ui/timefmt` | 页面共享的运行环境（后端、路由、主题、语言）、封面缓存与自定义背景图、性能 HUD 与压测、`utils/time.ts` 的时长与日期格式 |
| `ui/tagname` | `utils/tagTranslation.ts`：VNDB 标签按界面语言显示译名、按译名搜索标签 |
| `tools/mygo`、`tools/release` | 设好 `CGO_ENABLED=1` 再运行 `go tool mygo`；按 Wails 版的布局构建 Windows 便携版与安装版、macOS DMG、Linux AppImage、deb 与 rpm |
| `tools/i18n` | 把原版语言包转换成 `ui/locales`，复制 VNDB 标签译名；`extra/` 是原版没有的补充文字 |
| `tools/parity` | 与原版逐像素对照，见 [parity.md](parity.md) |
| `tools/icons`、`tools/perf`、`tools/emoji` | 导出 MDI 图标；测进程内存与 CPU；下载表情选择器的数据 |

依赖方向：`pages` → `components/*` → `components` → `theme`、`assets`、`locales`。组件不读页面状态，通过参数与回调交互；组件自己的临时状态（弹层开关、悬停计时）用 `ui.Local` 挂在自己的元素上。

## 数据目录

`mygo build` 构建的正式版与 Wails 版使用同一个数据目录（由 `version.BuildMode` 决定：便携版是程序所在目录，安装版是 `%APPDATA%/LunaBox`），启动时同样运行云同步、定时数据库备份、MCP、CLI 的 IPC 服务与开机自启。

开发运行（`go run`、`go build` 或 `mygo dev`，即 `mygo.IsDev()`）有自己的数据目录 `build/data`（可执行文件在 `build/` 下时用它旁边的 `data/`）：

- 第一次运行时从安装版的 `%APPDATA%/LunaBox` **复制**（不是移动）设置、数据库、封面、背景、头像、模板与备份，之后所有读写都在这份拷贝上进行，安装版的数据不受影响。`-reseed` 重新复制一次覆盖这份数据，`-data-dir` 改用别的目录。
- 与安装版隔离（`bootstrap.Options.Isolated`）：单实例锁的名字是“LunaBox Dev”，不注册开机自启（设置里的开关只记日志），不启动 CLI 的 IPC 服务与 MCP 服务，不自动云同步。IPC 的端口文件与开机启动项都是全局的，会影响同时运行的安装版。
- 读取项目目录下的 `.env.build` 与 `.env`（格式见 `.env.build.example`），补上没有用 ldflags 注入的 OAuth Client ID 等构建期凭据，与 `wails3 dev` 相同。没有注入时 Bangumi 与鲲 Galgame 的授权直接失败（同未配置的原版）。
- 没有用 ldflags 指定时 `version.BuildMode` 是 portable，所以设置页会显示“本地集成”。点“注册协议”“添加到 PATH”会让 `lunabox://` 与 PATH 指向 MyGo 版的可执行文件（替换安装版的关联），它旁边没有 lunacli。
- 日志在数据目录的 `logs/`（`app-<日期>.log`）。

AVIF 封面缩小后的图缓存在系统缓存目录（Windows 是 `%LOCALAPPDATA%/LunaBox/covers`，Linux 是 `~/.cache/LunaBox/covers`），不在数据目录里，不进备份，正式版、便携版与开发运行共用。AVIF 只能用 WASM 解码（一张 1400×1000 的一百多毫秒，每次启动第一张还要先编译解码器约 1 秒），有了缓存，之后只读小图（几毫秒）。上限 200MB，超出时启动时从最久没用的删起；整个目录可以随时删掉，删了只是重新解码（`ui/covers/disk.go`）。

编辑、状态、标签、收藏、评价、进度都会写进这份数据库。备份页的“恢复”会覆盖游戏真实的存档目录，“上传到云端”使用设置里的云备份账号——与原版行为相同，操作前请确认。设置页安排的数据库、全量数据恢复在退出（`SafeQuit`）后的下一次启动时执行。

## 构建细节

- DuckDB 需要 cgo，构建时要 `CGO_ENABLED=1`。MyGo 的 CLI 在 `CGO_ENABLED` 未设置时关闭 cgo，所以通过 `tools/mygo` 运行。
- `mygo dev` 监听应用依赖的全部本地包（包括 `packages/backend`）。它在 Windows 上重启或退出（Ctrl+C、`q`）时直接结束进程，不走退出流程（不检查点数据库、不保存窗口尺寸），下次启动由 WAL 恢复；从界面或托盘退出则正常关闭。
- `go build` 构建的 exe 没有图标资源，启动后由 `Dock.SetIcon` 设置窗口与任务栏图标。
- `tools/release`：`mygo build` 的 `-ldflags` 是固定的，所以构建期间临时写一个 `release_gen.go`，在 `init` 里给 `version` 包赋值（版本号与 `.env.build` 里的构建期凭据），再和 lunacli、7z 一起打包。

## 性能

自动滚动压测（`-hud` 后按 F5，或 `-bench -bench-exit`），240Hz 显示器：

| 场景 | 结果 |
| --- | --- |
| 56 款 | 帧间隔 p50 4.17 ms / p95 7.7 ms，2 帧超过 25 ms |
| 2800 款（`-repeat 50`），21,179 帧 | 帧间隔 p50 4.12 ms / p95 9.1 ms，17 帧超过 25 ms |
| 首帧 | 约 580 ms（启动时先复制数据库、初始化服务） |
| 内存 | working set 约 200 MB（含 DuckDB、封面位图的 CPU 侧副本） |

`MYGO_FRAME_STATS=all` 输出各阶段耗时，`tools/perf/measure.ps1` 统计进程树的内存与空闲 CPU。
