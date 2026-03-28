# LunaBox Agent 地图

> 这是一份**地图**，不是手册。先读这里，按需跳转专题文档。

## 项目概况

Wails v2 桌面应用（仅 Windows）。
前端：React + TypeScript + UnoCSS（presetWind3）+ Zustand + TanStack Router。
后端：Go + DuckDB + 自研 migrations。

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
| 路由注册 | `frontend/src/App.tsx` |
| 根布局 | `frontend/src/routes/__root.tsx` |
| 全局 Store | `frontend/src/store.ts` |
| UnoCSS 配置 | `frontend/uno.config.ts` |
| 后端启动/注入 | `main.go` |
| 初始建表 | `internal/migrations/init.go` |
| Migrations | `internal/migrations/migrations.go` |
| Services | `internal/service/*_service.go` |
| 后端工具函数 | `internal/utils/*`（按场景细分 package） |

---

## 渐进式披露阅读顺序

按任务复杂度逐层展开，不要一上来通读全部文档：

1. 先看本文件，只确定任务属于前端、后端还是流程问题
2. 涉及后端 service / DB / migration：先读 [backend.md](docs/backend.md)
3. 涉及文件、压缩包、图片、进程、代理、元数据抓取等辅助能力：再从 [backend-utils.md](docs/backend-utils.md) 进入对应 `internal/utils/*` 子包
4. 仍不确定文件落点：回到 [anchors.md](docs/anchors.md)

---

## 专题文档索引

按任务类型，读取对应文档：

| 任务类型 | 读取文档 |
|----------|----------|
| 新增/修改前端页面、组件、样式 | [frontend.md](docs/frontend.md) |
| 新增/修改后端 service、DB、migration | [backend.md](docs/backend.md) |
| 需要复用后端工具函数（`internal/utils/*`） | [backend-utils.md](docs/backend-utils.md) |
| 不确定文件落点 | [anchors.md](docs/anchors.md) |
| 提交前自检、变更流程 | [workflow.md](docs/workflow.md) |

---

## 交付要求

- 后端改动：`go build -tags dev` 不新增编译错误，`wails generate module` 可运行。
- 前端改动：`pnpm build` 可通过。
