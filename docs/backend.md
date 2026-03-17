# 后端规范

## 平台约束

- MUST：当前软件**仅支持 Windows**。
- MUST NOT：引入 macOS/Linux 专用逻辑或系统调用。若未来需要跨平台，必须通过 build tags 或清晰的抽象边界隔离（当前不在范围内）。

---

## 数据库（DuckDB）

- MUST 使用 DuckDB（驱动：`github.com/duckdb/duckdb-go/v2`）。
- SHOULD 积极使用 DuckDB 的优势能力（窗口函数、统计聚合等）简化统计逻辑。
- SHOULD 对时间字段优先使用 `TIMESTAMPTZ`，并考虑本地时区统计边界（项目启动时会 `SET TimeZone = ...`）。

---

## Schema 与 Migration

任何数据库结构变更（表/列/索引/数据修复）都必须通过 migration 管理。

**变更 Playbook（MUST 按顺序执行）：**

1. 在 `internal/migrations/migrations.go` 追加 migration（版本号递增，不可复用，如 141、142…）
2. 迁移逻辑必须幂等（`IF NOT EXISTS` 或显式检查）
3. 如是新表/新列：同步更新 `internal/migrations/init.go` 的 `InitSchema(...)`
4. 如涉及数据回填/修复：明确写在 migration 描述里，并尽量保证可重复执行

**Migration 要求：**

- 在事务中执行（当前 `Run(...)` 已以事务包裹）
- 保证新用户兼容（空库也能正常跑完，不报错）
- 错误必须 `return fmt.Errorf("...: %w", err)`，保留根因

---

## Service 设计与依赖注入

- MUST 以 service 作为业务边界，按 domain 划分，不需要 DAO 层。
- MUST SQL 操作封装在 service 内部的私有方法中，避免在多个文件随意拼 SQL。
- MUST 在 `main.go` 中创建 service 实例并调用 `Init(...)` 完成基础注入（ctx/db/config）。
- MUST service 间依赖通过 `SetXxxService(...)` 注入（参照 `StartService.SetSessionService`、`ImportService.SetSessionService`），不要直接 new 另一个 service。
- SHOULD 避免循环依赖；如果出现循环，优先重构职责或抽出更小的 service。

反例（MUST NOT）：
- 在一个 service 的方法里 `NewOtherService()` 直接创建并使用另一个 service。
- 把一段复杂 SQL 复制到多个 service 里"各写一份"。

---

## 工具函数（internal/utils）

- MUST 新增工具函数前先检查 `internal/utils/` 是否已有实现；优先复用/扩展。

---

## 系统底层操作（文件/进程/系统信息）

优先级：Wails runtime API > Go 标准库（`os`/`io`/`filepath`/`exec`）> Windows API

- MUST 优先使用 Wails runtime API（文件选择对话框、窗口控制、事件等）。
- MAY 确实需要时才使用 Windows API（参考 `internal/utils/process.go` 的风格与封装方式）。
- MUST NOT 用"命令输出文本/返回字符串内容"判断成功与否；以 Go 的 `error`、退出码、系统错误码为准，避免编码/语言环境导致的乱码与误判。

---

## 日志与错误处理

- MUST 错误向上返回时使用 `%w` 包装，保留原始错误。
- SHOULD 对用户可见的错误信息使用可理解的中文描述，对内部日志保留技术细节。
- SHOULD 在 service 内使用 `applog.LogInfof/LogErrorf`（有 ctx 的情况下）记录关键路径。
