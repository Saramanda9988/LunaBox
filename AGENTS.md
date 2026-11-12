# LunaBox Agent 地图

> 这是一份**地图**，不是手册。先读这里，按需跳转专题文档。

## 项目概况

MyGo 原生 UI 桌面应用（Windows、macOS、Linux 三端），界面与原版 Wails + React 前端一比一还原。
界面：Go + [MyGo](https://github.com/egoist/mygo)（原生渲染，没有 WebView）。
后端：Go + DuckDB + 自研 migrations。

原版 Wails 应用已归档在标签 `wails-final`（`apps/desktop/` 下的 `frontend/`、`wailsruntime/`、`scripts/build.bat` 等）。文档与注释里的“原版”“Wails 版”都指它，需要对照时用 `git show wails-final:<路径>` 查看。

## 仓库结构（monorepo）

| 目录 | 内容 |
|------|------|
| `apps/desktop/` | 桌面应用（Go module `lunabox/desktop`）：`app/` 是应用外壳，`host/` 把后端接到 MyGo，`api/` 是界面调用的后端接口，`ui/` 是界面（`pages`、`components`、`theme`、`locales` 等）；`cmd/lunacli` 是命令行工具，`tools/release` 打发布包，`packaging/` 是安装包脚本，`lib/` 是随包的 7z 与 Windows arm64 的 DuckDB；后端通过 `replace` 引用 `packages/backend` |
| `apps/update-server/` | Cloudflare Worker 更新服务（独立 pnpm 项目） |
| `packages/backend/` | 后端（Go module `lunabox/backend`：service、migrations、utils 等；`bootstrap/` 是与界面框架无关的启动、生命周期与退出，`appruntime/` 是桌面端实现的运行时接口） |
| `packages/updater/` | 独立更新器与更新资产生成器（Go module `lunabox/updater`，桌面应用通过 `replace` 引用） |
| `sync/` | 版本源 `version.json`（发布流程与 Pages/Netlify 依赖此路径，不要移动） |
| `docs/` | 专题文档 |

- MUST 各子项目独立管理依赖：Go 不使用 `go.work`，JS 不共用根级 pnpm workspace（根 `package.json` 只放脚本，不加依赖）。
- 本文件与 `docs/` 中未标注前缀的路径（如 `ui/...`、`host/...`），均相对 `apps/desktop/`；桌面应用相关命令在 `apps/desktop/` 下执行。
- DuckDB 需要 cgo：运行 MyGo 的 CLI 用 `go run ./tools/mygo <命令>`（它设好 `CGO_ENABLED=1` 再运行 `go tool mygo`）。
- 根目录 `package.json` 是常用命令入口，可直接在仓库根执行：`pnpm dev`、`pnpm package`（为当前系统打发布包）、`pnpm i18n`（改了语言包后重新生成）、`pnpm check`（桌面编译 + 更新器测试 + 更新服务检查）等，完整列表见其 `scripts`。

## 关键词优先级

- **MUST**：必须遵守（违反即视为实现不合格）
- **SHOULD**：强烈建议（有明确原因可偏离，需说明）
- **MAY**：可选

当本文件与用户需求冲突时：**以用户需求为准**。

---

## 核心约束（每次任务都适用）

1. **最小可行改动**：优先改动已有文件，复用已有模式，不新增架构性模块。
2. **先搜再新建**：新增组件/函数/SQL 前，先搜索对应目录是否已有实现。
3. **跟随仓库风格**：路由组织、service 注入、migration 形态等，沿用当前写法。
4. **一次只解决一个问题域**：不顺手重构或格式化不相关代码。
5. **统一 User-Agent（MUST）**：后端 HTTP 请求不得硬编码应用 UA，默认值统一调用 `packages/backend/version.UserAgent()`；仅当上游明确要求时才允许传入完整自定义 UA，版本号不得单独维护。

---

## 何时必须问用户

仅以下情况才追问（否则按最简单解释执行）：

- 需求影响数据结构/迁移策略（是否需要数据回填、是否允许破坏性迁移）
- UI/交互存在多种合理方案且影响用户使用习惯（如新增入口位置）
- 需要引入新依赖或大改目录结构

---

## 关键文件落点（快速定位）

详见 → [docs/anchors.md](docs/anchors.md)

| 类型 | 文件 |
|------|------|
| 路由表 | `ui/routes.go` |
| 根布局 | `ui/app.go` |
| 页面共享环境（后端、路由、主题、语言） | `ui/env/` |
| 色板与字号 | `ui/theme/` |
| 界面文字 | `ui/locales/*.json` |
| 后端启动/注入 | `packages/backend/bootstrap/` |
| 初始建表 | `packages/backend/migrations/init.go` |
| Migrations | `packages/backend/migrations/migrations.go` |
| Services | `packages/backend/service/*_service.go` |
| 后端工具函数 | `packages/backend/utils/*`（按场景细分 package） |

---

## 渐进式披露阅读顺序

按任务复杂度逐层展开，不要一上来通读全部文档：

1. 先看本文件，只确定任务属于界面、后端还是流程问题
2. 涉及界面：先读 [frontend.md](docs/frontend.md) 的约束，运行与发布见 [apps/desktop/README.md](apps/desktop/README.md)
3. 涉及后端 service / DB / migration：先读 [backend.md](docs/backend.md)
4. 涉及文件、压缩包、图片、进程、代理、元数据抓取等辅助能力：再从 [backend-utils.md](docs/backend-utils.md) 进入对应 `packages/backend/utils/*` 子包
5. 仍不确定文件落点：回到 [anchors.md](docs/anchors.md)

---

## 专题文档索引

按任务类型，读取对应文档：

| 任务类型 | 读取文档 |
|----------|----------|
| 新增/修改页面、组件、样式、界面文字 | [frontend.md](docs/frontend.md) |
| 新增/修改后端 service、DB、migration | [backend.md](docs/backend.md) |
| 需要复用后端工具函数（`packages/backend/utils/*`） | [backend-utils.md](docs/backend-utils.md) |
| 应用内更新与发布 | [updater.md](docs/updater.md) |
| 不确定文件落点 | [anchors.md](docs/anchors.md) |
| 提交前自检、变更流程 | [workflow.md](docs/workflow.md) |

---

## 交付要求

- 界面改动：在 `apps/desktop/` 下 `go build ./...` 通过，改动所在包的 `go test` 通过（`ui/locales` 的 `TestNoHardcodedText` 检查硬编码文字）。
- 后端改动：`pnpm backend:test`（即在 `packages/backend/` 下 `go test -p 4 ./...`）通过，并确认 `apps/desktop/` 仍可编译。
- MUST 在 `packages/backend/`、`apps/desktop/` 下跑多包测试时加 `-p 4`（或用 `pnpm backend:test` / `pnpm desktop:test`）：这两处大部分测试包都会静态链接约 140MB 的 DuckDB 预编译库，每次链接（`link` + `ld`）峰值约 1.5GB，默认 `-p` 等于 CPU 核数，20 核机器上会同时链接 20 个，内存耗尽导致卡死；限到 4 个总耗时几乎不变。
- 更新器改动：在 `packages/updater/` 下 `go test ./...` 通过，并确认 `apps/desktop/` 仍可编译。
- 更新服务改动：在 `apps/update-server/` 下 `pnpm typecheck` 与 `pnpm test` 通过。

## IMPORTANT

- MUST 发布包的文件名与包内布局（`tools/release`）沿用 Wails 版：应用内更新、已安装的旧版与 SignPath 签名配置都依赖它，改动前先确认这三处。
- MUST `ui/locales/catalog_gen.go` 是 `mygo generate` 生成的，不要手改；改 `ui/locales/*.json` 后运行 `pnpm i18n`（或在 `apps/desktop` 下 `go run ./tools/mygo generate`）。
