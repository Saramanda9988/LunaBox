# 后端规范

## 建议阅读顺序

按需展开，不要把本页当成必须逐字读完的手册：

1. 只改 service / SQL / migration：先读 `平台约束`、`Schema 与 Migration`、`Service 设计与依赖注入`
2. 需要文件、压缩包、下载参数校验、图片、进程、代理、元数据抓取等辅助能力：再看本页的 `internal/utils 速查`
3. 确认要复用某个工具包后，再打开 [backend-utils.md](backend-utils.md) 或对应源码目录

---

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

## internal/utils 速查

- MUST 新增工具函数前先检查 `internal/utils/` 是否已有实现；优先复用/扩展。
- SHOULD 先按“场景”找 package，不要直接在 service 里重复封装文件/压缩/图片/进程/代理逻辑。
- SHOULD 把“下载协议/文件辅助”和“下载任务状态机”分开：
  通用的 URL、checksum、文件名、archive format 处理优先放 `internal/utils/downloadutils`，
  任务状态、暂停恢复、解压后导入继续留在 service。

常见场景与优先入口：

| 场景 | 优先 package | 常用入口 |
|------|--------------|----------|
| 应用数据目录、缓存目录、模板目录 | `internal/utils/apputils` | `GetDataDir`、`GetCacheDir`、`GetConfigDir`、`GetTemplatesDir` |
| 文件复制、打开目录、查找可执行文件 | `internal/utils/apputils` | `CopyFile`、`CopyDir`、`OpenDirectory`、`OpenFileOrFolder`、`FindExecutables` |
| ZIP / 7z / RAR 等归档处理 | `internal/utils/archiveutils` | `ExtractArchive`、`ZipDirectory`、`ZipFileOrDirectory`、`UnzipFile` |
| 下载 URL / checksum / 文件名 / archive format 辅助 | `internal/utils/downloadutils` | `ValidateDownloadURL`、`ValidateChecksumFields`、`SanitizeDownloadedFileName`、`BuildExpectedExtractDir` |
| 封面/背景图落盘与本地路径管理 | `internal/utils/imageutils` | `SaveCoverImage`、`DownloadAndSaveCoverImage`、`SaveBackgroundImage`、`CropAndSaveBackgroundImage` |
| 元数据抓取（Bangumi/VNDB/Steam/Ymgal） | `internal/utils/metadata` | `NewBangumiInfoGetter`、`NewVNDBInfoGetterWithLanguage`、`NewSteamInfoGetterWithLanguage`、`NewYmgalInfoGetter` |
| 进程枚举、PID 查询、退出监听 | `internal/utils/processutils` | `GetRunningProcesses`、`GetProcessPIDByName`、`WaitForProcessExitAsync` |
| 活跃时长与焦点追踪 | `internal/utils/timerutils` | `NewActiveTimeTracker`、`focusing.NewFocusTracker` |
| 下载代理解析 | `internal/utils/proxyutils` | `ResolveDownloadProxy` |
| SQL 小工具 | `internal/utils` | `UniqueNonEmptyStrings`、`BuildPlaceholders` |
| 备份口令派生用户 ID | `internal/utils` | `GenerateUserID` |
| Web 搜索补充信息 | `internal/utils` | `SearchViaTavily`、`SearchViaDuckDuckGo`、`SearchViaMoeGirl` |

细节说明与注意事项见 [backend-utils.md](backend-utils.md)。

---

## 系统底层操作（文件/进程/系统信息）

优先级：Wails runtime API > Go 标准库（`os`/`io`/`filepath`/`exec`）> Windows API

- MUST 优先使用 Wails runtime API（文件选择对话框、窗口控制、事件等）。
- MAY 确实需要时才使用 Windows API（参考 `internal/utils/processutils/process_windows.go` 与 `internal/utils/timerutils/focusing/windows_focus.go` 的封装方式）。
- MUST NOT 用"命令输出文本/返回字符串内容"判断成功与否；以 Go 的 `error`、退出码、系统错误码为准，避免编码/语言环境导致的乱码与误判。

---

## 日志与错误处理

- MUST 错误向上返回时使用 `%w` 包装，保留原始错误。
- SHOULD 对用户可见的错误信息使用可理解的中文描述，对内部日志保留技术细节。
- SHOULD 在 service 内使用 `applog.LogInfof/LogErrorf`（有 ctx 的情况下）记录关键路径。
