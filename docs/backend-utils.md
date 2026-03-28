# 后端工具函数速查

> 这是一份 `internal/utils/*` 的**渐进式披露索引**。先按场景选 package，再按需进入源码。

## 怎么读这页

1. 先看 `一级索引`，确认需求属于哪个场景
2. 只读对应 package 小节，判断是否已有可复用入口
3. 只有当现有入口不够时，才打开对应源码做扩展

---

## 一级索引

| 场景 | package | 优先函数 |
|------|---------|----------|
| 应用目录、文件复制、打开资源管理器、查找 exe | `internal/utils/apputils` | `GetDataDir`、`CopyFile`、`OpenFileOrFolder`、`FindExecutables` |
| 压缩与解压 | `internal/utils/archiveutils` | `ExtractArchive`、`ZipDirectory`、`ZipFileOrDirectory`、`UnzipFile` |
| 下载 URL / checksum / 文件名 / archive format / 传输辅助 | `internal/utils/downloadutils` | `ValidateDownloadURL`、`ValidateChecksumFields`、`SanitizeDownloadedFileName`、`BuildExpectedExtractDir`、`NewDownloader` |
| 封面图/背景图管理 | `internal/utils/imageutils` | `SaveCoverImage`、`DownloadAndSaveCoverImage`、`SaveBackgroundImage` |
| 游戏元数据抓取 | `internal/utils/metadata` | `NewBangumiInfoGetter`、`NewVNDBInfoGetterWithLanguage`、`NewSteamInfoGetterWithLanguage`、`NewYmgalInfoGetter` |
| 进程查询与退出监听 | `internal/utils/processutils` | `GetRunningProcesses`、`GetProcessPIDByName`、`WaitForProcessExitAsync` |
| 活跃时长与焦点检测 | `internal/utils/timerutils` | `NewActiveTimeTracker`、`focusing.NewFocusTracker` |
| 下载代理 | `internal/utils/proxyutils` | `ResolveDownloadProxy` |
| SQL / 搜索 / 备份辅助 | `internal/utils` | `BuildPlaceholders`、`UniqueNonEmptyStrings`、`GenerateUserID`、`SearchViaTavily` |

---

## `utils`

适用场景：轻量通用辅助，不值得单独起新 package 时先检查这里。

优先复用：

| 函数 | 作用 | 典型使用点 |
|------|------|------------|
| `UniqueNonEmptyStrings(values []string)` | 去空、去重、保留顺序 | `IN (...)` 参数前清洗 ID 列表 |
| `BuildPlaceholders(count int)` | 生成 `?,?,?` 占位符串 | DuckDB / SQL `IN` 查询 |
| `GenerateUserID(password string)` | 基于备份口令稳定派生用户 ID | 备份导出、远程同步身份 |
| `SearchViaTavily(query, apiKey)` | Tavily 搜索并返回整理后的文本摘要 | AI 搜索增强 |
| `SearchViaDuckDuckGo(query)` | 免费 DuckDuckGo Instant Answer | Tavily 不可用时兜底 |
| `SearchViaMoeGirl(query)` | 萌娘百科搜索并清理文本 | ACGN 词条补充 |

注意：

- `BuildPlaceholders` 只负责占位符，不负责参数数组拼接。
- Web 搜索函数返回的是“给 AI/上层消费的文本块”，不是结构化 DTO。

---

## `apputils`

适用场景：应用目录、文件复制、本地文件暴露、可执行文件发现、调用系统资源管理器。

优先复用：

| 函数 / 类型 | 作用 |
|-------------|------|
| `GetDataDir` / `GetCacheDir` / `GetConfigDir` | 获取应用数据、缓存、配置目录，并确保目录存在 |
| `GetSubDir` / `GetCacheSubDir` / `GetTemplatesDir` | 获取子目录并自动 `MkdirAll` |
| `IsPortableMode` / `GetBuildMode` | 区分便携版与安装版路径策略 |
| `CopyFile` / `CopyDir` | 文件或目录复制 |
| `OpenDirectory` / `OpenFileOrFolder` | 打开目录，或在资源管理器中选中文件 |
| `FindExecutables` | 在单层目录中找 `.exe` / `.bat` |
| `SelectBestExecutable` | 从候选 exe 中挑选更可能的主程序 |
| `NewLocalFileHandler` / `LocalFileHandler.ServeHTTP` | 暴露 `/local/...` 本地文件访问 |

注意：

- 路径函数已经处理安装版/便携版差异，新增数据目录逻辑前先复用。
- `LocalFileHandler` 已做路径清洗和目录穿越防护；本地文件访问不要重写一套。
- `FindExecutables` 只扫一级目录，且不包含 `.lnk`。

---

## `archiveutils`

适用场景：备份、导入、下载后的解压与打包。

优先复用：

| 函数 | 作用 |
|------|------|
| `ExtractArchive(source, target)` | 通用解压，优先尝试内置 7z，再回退到 `xtractr` |
| `ZipDirectory(source, target)` | 压缩整个目录 |
| `ZipFileOrDirectory(source, target)` | 自动判断压缩单文件或目录 |
| `UnzipFile(source, target)` | 解压 ZIP 文件 |
| `ExtractZip(zipReader, destDir)` | 从已打开的 `zip.ReadCloser` 解压 |
| `UnzipForRestore(src, dest)` | 恢复流程兼容入口，本质同 `UnzipFile` |

注意：

- `ExtractArchive` 的返回值 `(extracted bool, err error)` 有语义差异：
  `extracted=false` 通常表示“可回退失败”，上层可以转入手动解压模式。
- ZIP 解压已做 Zip Slip 防护；不要在业务层复制一套路径校验。
- Windows 下内置了 `7z.exe` / `7z.dll` 提高兼容性，优先复用现有入口。

---

## `downloadutils`

适用场景：下载请求的通用参数校验、archive format 归一化、下载文件名清洗、推导预期解压目录，以及可复用的安全下载传输能力。

优先复用：

| 函数 | 作用 |
|------|------|
| `NormalizeArchiveFormat(format)` | 统一标准化 archive format 字段 |
| `IsSupportedArchiveFormat(format)` | 判断是否是项目支持的压缩格式 |
| `TrimArchiveSuffixByFormat(name, format)` | 按 archive format 去掉尾缀 |
| `SanitizeFileName(name)` | 清洗通用文件名 |
| `SanitizeDownloadedFileName(name)` | 清洗下载目标文件名，额外拒绝 `.` / `..` |
| `BuildExpectedExtractDir(downloadedPath, fileName, archiveFormat, title)` | 生成下载后默认解压目录 |
| `ValidateDownloadURL(rawURL)` | 校验下载 URL 协议、host 和内网/回环等受限地址 |
| `ValidateChecksumFields(algo, checksum)` | 校验 checksum 算法和值格式 |
| `NewDownloader(config)` / `Downloader.Download(ctx, req)` | 统一下载入口，自动在单连接与分片下载间选择，并复用安全代理/拨号限制 |
| `InspectResumeOffset(destPath, expectedSize)` | 检查普通下载或分片下载的可续传字节数 |
| `MultipartTempDir(destPath)` | 获取分片下载的临时目录路径 |

注意：

- 这里适合放“下载协议/文件辅助”和“可复用的传输实现”，不适合放“下载任务状态机”。
- `downloadutils` 可以被 `protocol`、`service` 等多处复用；如果某段逻辑只服务单个下载流程，就继续留在当前 service。
- 代理拨号限制、续传探测、分片下载状态文件都可以放这里；任务暂停恢复、事件推送、解压失败后的业务回退不属于这个 package。

---

## `imageutils`

适用场景：封面图、背景图、本地图片 URL 与下载落盘。

优先复用：

| 函数 | 作用 |
|------|------|
| `SaveCoverImage(srcPath, gameID)` | 保存封面到托管目录并返回 `/local/covers/...` |
| `ResolveCoverPath(imagePath, tempDir)` | 导入压缩包时解析封面文件真实路径 |
| `DownloadAndSaveCoverImage(imageURL, gameID)` | 下载远程封面并保存到本地 |
| `RenameTempCover(tempCoverURL, gameID)` | 临时封面转正 |
| `SaveBackgroundImage(srcPath)` | 保存正式背景图 |
| `SaveTempBackgroundImage(srcPath)` | 保存临时背景图 |
| `CropAndSaveBackgroundImage(srcPath, x, y, width, height)` | 裁剪并保存背景图 |

注意：

- 这些函数会统一写入托管目录并返回 `/local/...` 路径；前后端路径约定要跟随现有模式。
- 背景图保存会清理旧的 `custom_bg_` / `temp_bg_` 文件，不要自行追加第二套清理逻辑。

---

## `metadata`

适用场景：从 Bangumi / VNDB / Steam / Ymgal 拉取游戏元数据与标签。

优先复用：

| 入口 | 作用 |
|------|------|
| `Getter` | 统一接口：`FetchMetadata` / `FetchMetadataByName` |
| `NewBangumiInfoGetter()` | Bangumi 抓取，需要 Bearer token |
| `NewVNDBInfoGetterWithLanguage(language)` | VNDB 抓取，支持语言偏好 |
| `NewSteamInfoGetterWithLanguage(language)` | Steam 抓取，支持语言与地区偏好 |
| `NewYmgalInfoGetter()` | Ymgal 抓取，内部管理 access token |
| `MetadataResult` / `TagItem` | 统一返回游戏信息与标签列表 |

注意：

- 统一通过 `Getter` 抽象调度，不要在 service 中散落各站点 HTTP 细节。
- 不同源的 token 约束不同：
  Bangumi 必须显式传 token；Ymgal 自己申请并缓存 token；Steam/VNDB 当前入口不需要额外 token。
- 各 getter 已处理名称搜索、语言偏好、评分归一化、标签裁剪等常见逻辑。

---

## `processutils`

适用场景：Windows 进程枚举、PID 查询、监听进程退出。

优先复用：

| 函数 / 类型 | 作用 |
|-------------|------|
| `CheckIfProcessRunning(processName)` | 判断进程名是否存在 |
| `GetRunningProcesses()` | 获取筛过系统噪音后的进程列表 |
| `GetProcessPIDByName(processName)` | 通过进程名找 PID |
| `IsProcessRunningByPID(pid, ctx)` | 用 PID 判断进程是否还活着 |
| `NewProcessMonitor(pid)` | 创建进程退出监听器 |
| `WaitForProcessExitAsync(pid)` | 异步等待进程退出 |
| `ProcessInfo` | 进程名 + PID DTO |

注意：

- 当前项目只支持 Windows，但包内仍有 `!windows` stub；正常业务代码不要绕开它直接写平台判断。
- 这里优先用 WinAPI / 系统错误码，而不是依赖命令输出文本。
- `GetRunningProcesses` 已过滤常见系统进程，适合前端选择器直接消费。

---

## `timerutils`

适用场景：游戏前台活跃时长统计。

优先复用：

| 函数 / 类型 | 作用 |
|-------------|------|
| `NewActiveTimeTracker(ctx, db)` | 创建活跃时长追踪器 |
| `ActiveTimeTracker.StartTracking(sessionID, gameID, processID)` | 开始追踪 |
| `ActiveTimeTracker.StopTracking(gameID)` | 停止追踪并返回累积秒数 |
| `ActiveTimeTracker.StopAllTracking()` | 程序退出时批量停止 |
| `focusing.NewFocusTracker(targetPID)` | 焦点变化追踪 |
| `focusing.IsProcessFocused(processID)` | 单次检查某 PID 是否前台 |

注意：

- `ActiveTimeTracker` 是内部服务能力，当前由 `StartService` 管理；新业务优先复用，不要再起第二套计时器。
- 焦点追踪失败时已有轮询降级逻辑。

---

## `proxyutils`

适用场景：下载代理、系统代理、手动代理的统一解析。

优先复用：

| 函数 / 类型 | 作用 |
|-------------|------|
| `ResolveDownloadProxy(mode, manualURL)` | 按 `system/manual/direct` 解析代理方案 |
| `ProxySelection.Proxy(req)` | 提供给 `http.Transport.Proxy` |
| `ProxySelection.AllowedDialTargets()` | 生成允许直连的代理目标集合 |
| `ProxySelection.Description()` | 生成给日志/UI 展示的描述 |

注意：

- Windows 下会读取注册表中的 Internet Settings；同时支持环境变量代理兜底。
- `ResolveDownloadProxy` 已统一处理空值、scheme 补全、PAC 提示等细节。

---

## 什么时候才新增工具函数

只有在下面三件事同时满足时，才考虑在 `internal/utils/*` 新增函数：

1. 现有 package 没有可复用入口
2. 逻辑确实跨多个 service 复用，而不是单个 service 的私有细节
3. 新函数能自然归入现有 package；否则优先放回当前 service 私有方法
