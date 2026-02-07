# LunaBox AI Agent SPEC（操作手册）

本文件优先面向 **AI coding agent**：用于约束默认决策、代码落点、变更流程与禁止事项。

项目概况：Wails v2；前端 React + TypeScript + UnoCSS（presetWind3）+ Zustand + TanStack Router；后端 Go + DuckDB + 自研 migrations。

## 0. 关键词与优先级

- **MUST**：必须遵守（违反即视为实现不合格）。
- **SHOULD**：强烈建议（除非有明确原因，并给用户进行说明）。
- **MAY**：可选。

当本 SPEC 与用户需求冲突时：以用户需求为准；当用户需求不明确时：以本 SPEC 的默认规则为准。

## 1. Agent 默认行为（非常重要）

### 1.1 默认策略

- MUST 优先做 **最小可行改动**：优先改动已有文件、复用已有模式，不新增“架构性模块”。
- MUST 先对齐现有实现：改动风格、组织方式要跟随仓库当前写法（路由组织、service 注入、migration 形态等）。
- MUST “先搜再新建”：新增组件/工具函数/SQL/配置前，先在对应目录搜索是否已有实现，能扩展就不重复造。
- SHOULD 一次改动只解决一个问题域：避免顺手重构或格式化不相关代码。

### 1.2 何时必须问用户（否则默认按最简单解释）

仅当出现以下情况才需要追问：

- 需求会影响数据结构/迁移策略（是否需要数据回填、是否允许破坏性迁移）。
- UI/交互存在多种合理方案且会影响用户使用习惯（例如新增入口位置）。
- 需要引入新依赖或大改目录结构。

### 1.3 输出/交付要求

- MUST 让改动可验证：修改后应能 `go build -tags dev`（或至少不新增明显编译错误）并能成功运行 `wails generate module` ；前端改动应能 `pnpm build`。
- SHOULD 改动后快速自检：运行已有 task（如 workspace tasks 中的 build）。

## 2. 关键落点（文件锚点）

AI agent 在做决定时，优先参考这些“真实入口”。

### 2.1 前端锚点

- 路由注册：`frontend/src/App.tsx`（`routeTree` 组装）
- 根布局：`frontend/src/routes/__root.tsx`（`data-glass`、TopBar/SideBar、拖拽遮罩等）
- 路由文件目录：`frontend/src/routes/*.tsx`（每个页面导出 `Route`）
- 全局 store(Zustand)：`frontend/src/store.ts`
- UnoCSS：`frontend/uno.config.ts`，全局样式：`frontend/src/style.css`
- Wails 绑定：`frontend/wailsjs/`

### 2.2 后端锚点

- 启动与注入：`main.go`（创建 services、`Init(...)`、`SetXxxService(...)`、`Bind`）
- 初始建表：`main.go` 的 `initSchema(...)`
- migrations：`internal/migrations/migrations.go`
- services：`internal/service/*_service.go`
- utils：`internal/utils/*`

## 3. 前端规范（frontend/）

### 3.1 路由（@tanstack/react-router）

- MUST 使用 `@tanstack/react-router` 管理路由。
- MUST 保持“页面 = route 文件”的组织方式：`frontend/src/routes/*.tsx` 中每个页面导出 `Route`。
- MUST 新增页面路由时同时完成两步：
	1) 新增 `frontend/src/routes/<page>.tsx`
	2) 在 `frontend/src/App.tsx` 中把该 Route 加入 `routeTree`（与现有写法一致）

### 3.2 状态管理（zustand）

- MUST 使用 zustand 做全局状态管理（store 位于 `frontend/src/store.ts`）。
- SHOULD 优先把跨页面共享、或与后端配置/数据缓存相关的状态放入 store；页面内临时 UI 状态使用组件本地 state。
- SHOULD 与后端交互的“请求 + toast/错误提示 + 缓存更新”集中在 store action 或 hooks 内，避免在多个页面重复写。

### 3.3 样式（UnoCSS）

- MUST 使用 UnoCSS 管理样式，入口已在 `frontend/src/main.tsx` 引入 `virtual:uno.css`。
- MUST 使用`frontend/uno.config.ts`中自定义的tailwind css风格的品牌色类，而非硬编码bg-blue-100
- MUST 优先在 `frontend/uno.config.ts` 增加/调整 shortcuts/rules/variants。
- SHOULD 尽量用 utility class 组合 UI，避免为单个组件创建大量独立 CSS。
- MUST 尽量减少在 `frontend/src/style.css` 写样式；仅允许“无法避免的全局样式”，例如：统一滚动条、全局过渡禁用类等。

反例（MUST NOT）：为了一个按钮/卡片样式，在 `style.css` 新增几十行选择器。

### 3.4 组件与依赖约束

- MUST 以自定义组件为主。
- MAY 使用的第三方 UI 构件：
  - `@headlessui/react`（可直接用或封装）
  - `@radix-ui/*` 的原子组件（必须二次封装后再使用）
- MUST 能且只能使用`@headlessui/react`和`@radix-ui/*`的组件，禁止再引入其他库
- MUST 参考现有封装模式：
  - `frontend/src/components/ui/BetterSelect.tsx`（Headless UI 封装）
  - `frontend/src/components/ui/BetterSwitch.tsx`（Radix 封装）
- SHOULD 新增可复用组件放在 `frontend/src/components/ui/`；页面级组合组件按照组件类型放置再对应的

反例（MUST NOT）：直接引入完整 UI 框架（如 MUI/Antd）来实现一个简单控件。

### 3.5 暗黑模式与玻璃态（Glass）

- MUST 支持暗黑模式：使用 `dark:` 变体（项目通过在 `documentElement` 上切换 `light/dark` class 实现）。
- MUST 为新增组件补齐 dark 状态下的文本/背景/边框对比度，不允许“暗色下不可读”。
- MUST 适配“自定义背景 + 玻璃态”模式：
  - 除 modal 类组件外，新增组件 SHOULD 适当加入 `data-glass:` 相关样式或使用预制 `glass` 类
  - 在 `data-glass="true"` 时避免纯不透明大面积底色，优先半透明/边框/blur 维持层次
- NOTE：根节点布局已在 `frontend/src/routes/__root.tsx` 上设置 `data-glass`，不要重复造全局开关。

### 3.6 工具函数

- MUST 新增工具函数前先检查 `frontend/src/utils/` 是否已有实现；优先复用或在原文件中扩展。
- SHOULD 保持 utils 纯函数化（输入/输出清晰、可复用），避免在 utils 内直接读写全局 store。

### 3.7 时间与日期

- MUST 涉及日期/时间相关的ui展示，必须使用`frontend/src/utils/time.ts`中的函数对时间进行处理

### 3.8 与后端交互（Wails）

- MUST 通过 `frontend/wailsjs/` 生成的绑定调用后端服务。
- SHOULD 将“后端调用 + 结果归一化/错误提示”封装到 hooks 或 store action 中，避免散落在各页面。

## 4. 后端规范（internal/ + main.go）

### 4.1 平台约束

- MUST：当前软件仅支持 **Windows**。
- MUST：不要引入 macOS/Linux 专用逻辑或系统调用；若未来需要跨平台，必须通过 build tags 或清晰的抽象边界隔离（当前不在范围内）。

### 4.2 数据库（DuckDB）

- MUST 使用 DuckDB（驱动：`github.com/duckdb/duckdb-go/v2`）。
- SHOULD 积极使用 DuckDB 的优势能力（窗口函数、统计聚合等）来简化统计逻辑。
- SHOULD 对时间字段优先使用 `TIMESTAMPTZ`，并考虑本地时区统计边界（项目启动时会设置 `SET TimeZone = ...`）。

### 4.3 Schema 与 Migration

- MUST：任何数据库结构变更（表/列/索引/数据修复）都必须通过 migration 管理：`internal/migrations/migrations.go`。
- MUST：新增 schema 变更时同时更新两处：
	1) `main.go` 的 `initSchema(...)`（新安装时的初始结构）
	2) `internal/migrations` 增加新 migration（老用户升级路径）
- MUST：migration 需要满足：
	- 版本号递增、不可复用（例如 141、142…）
	- 在事务中执行（当前 `Run(...)` 已以事务包裹）
	- 可重复执行（推荐使用 `IF NOT EXISTS` / 幂等判断）
    - 保证新用户兼容（即使是空库也要能正常跑完migration，不能出现空库无法跑迁移脚本，会报错的情况）
	- 错误必须 `return fmt.Errorf("...: %w", err)`，保留根因

迁移变更 playbook（MUST 按顺序执行）：

1) 在 `internal/migrations/migrations.go` 追加 migration（版本号递增）
2) 迁移逻辑必须幂等（`IF NOT EXISTS` 或显式检查）
3) 如是新表/新列：同步更新 `main.go` 的 `initSchema(...)`
4) 如果迁移涉及数据回填/修复：明确写在 migration 描述里，并尽量保证可重复执行

### 4.4 Service 设计与依赖注入（Spring-like）

- MUST：以 service 作为业务边界，按照负责的domain划分service，不需要 DAO 层。
- MUST：SQL 操作尽量封装在 service 内部的私有方法/小方法中，避免在多个文件随意拼 SQL。
- MUST：在 `main.go` 中创建 service 实例并调用 `Init(...)` 完成基础注入（ctx/db/config）。
- MUST：service 间依赖通过 `SetXxxService(...)` 注入（参照 `StartService.SetSessionService`、`ImportService.SetSessionService`），不要直接 new 另一个 service。
- SHOULD：尽量避免循环依赖；如果出现循环，优先重构职责或抽出更小的 service，而不是互相持有引用。

反例（MUST NOT）：

- 在一个 service 的方法里 `NewOtherService()` 直接创建并使用另一个 service。
- 把一段复杂 SQL 复制到多个 service 里“各写一份”。

### 4.5 工具函数（internal/utils）

- MUST：新增工具函数前先检查 `internal/utils/` 是否已有实现；优先复用/扩展。

### 4.6 系统底层操作（文件/进程/系统信息）

- MUST：优先使用 Wails runtime API（例如文件选择对话框、窗口控制、事件等）。
- SHOULD：Wails 不覆盖的能力，再考虑 Go 标准库（`os` / `io` / `filepath` / `exec` 等）。
- MAY：确实需要时才使用 Windows API（可参考 `internal/utils/process.go` 的风格与封装方式）。
- MUST：永远不要用“命令输出文本/返回字符串内容”判断成功与否；以 Go 的 `error`、退出码、系统错误码为准，避免编码/语言环境导致的乱码与误判。

### 4.7 日志与错误处理

- MUST：错误向上返回时使用 `%w` 包装，保留原始错误。
- SHOULD：对用户可见的错误信息使用可理解的中文描述，对内部日志保留技术细节。
- SHOULD：在 service 内使用 `runtime.LogInfof/LogErrorf`（有 ctx 的情况下）记录关键路径。

## 5. Agent 变更流程（推荐顺序）

当你被要求“实现一个功能/修复一个问题”时，按此顺序执行：

1) 定位落点：优先在现有 route/service/utils 中找最贴近的位置
2) 复用模式：沿用同目录已有写法（例如 migration 幂等、service 注入、UI glass/dark）
3) 实现最小改动：避免新增大层级抽象
4) 本地验证：尽量运行已有 build task
5) 自检清单：对照第 6 节

## 6. 自检清单（交付前）

- 前端：新增/修改的组件在 `light` 与 `dark` 下都可读；背景开启时 `data-glass="true"` 下观感不崩。
- 前端：没有把局部样式硬塞进 `style.css`（除非是全局不可避免项）。
- 前端：新增组件遵守 HeadlessUI/Radix 封装约束，没有直接引入大型 UI 框架。
- 后端：涉及 schema 变更时同时更新 `initSchema` + 新 migration，并确保幂等与事务安全。
- 后端：没有引入非 Windows 平台的系统调用；底层操作优先 Wails/Go 标准库。
- 通用：新增工具函数前已搜索并复用现有 `frontend/src/utils` 或 `internal/utils`。