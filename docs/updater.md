# Windows 应用内更新

LunaBox 使用独立的 `LunaBoxUpdater.exe` 完成伪应用内更新，不依赖 Velopack 的目录结构或运行时 SDK。便携版和安装版都保持现有平铺目录，数据路径不迁移；下载、重建和 journal 放在系统临时目录，不在便携版根目录新增更新工作目录。

## 更新模型

- LunaBox 继续负责版本检测、代理配置、下载进度和退出前同步。
- `LunaBoxUpdater.exe prepare` 在 LunaBox 运行时重建并验证新文件，不修改应用目录。
- `LunaBoxUpdater.exe commit` 等待 LunaBox 退出，再事务性替换发生变化的文件并重启应用。
- 只为上一稳定版本的 `LunaBox.exe` 生成 Zstandard dictionary patch（`.zsdiff`）。源版本或源 SHA-256 不完全匹配时直接选择完整 `.zst`。
- CLI、updater、7z 和 DuckDB 不做 patch；它们未变化时不下载，变化时使用各自的完整 `.zst`。
- patch 重建或验证失败时，LunaBox 保持运行并下载 `LunaBox.exe` 的完整 `.zst` 后重新 prepare。

发布清单包含四个独立 channel：

```text
windows-amd64-portable
windows-amd64-installer
windows-arm64-portable
windows-arm64-installer
```

## 安全与失败处理

- 清单和每个下载产物必须使用 HTTPS，并记录大小和 SHA-256。
- prepare 标记绑定完整 task；prepare 后修改路径、哈希、重启参数等都会使 commit 拒绝执行。
- `LunaBox.exe`、`LunaBoxUpdater.exe` 和 `lunacli.exe` 在替换前通过 Windows Authenticode 校验。
- updater 只允许修改代码内列出的运行时文件，拒绝绝对路径、目录穿越和任意数据文件。
- 每次替换都有 journal 和备份。文件被 CLI/7z 等进程短暂锁定时会在 10 秒内重试，仍无法替换则按逆序回滚，但不会强杀用户进程；LunaBox 尚未退出时发生的验证/等待错误不会启动第二个实例，回滚本身失败时也不会冒险启动混合版本。
- 安装版 commit 通过 UAC 提权，仅更新文件和卸载项中的 `DisplayVersion`，不会重建 NSIS uninstaller。
- `duckdb.dll` 和 `7z.dll` 本身没有 LunaBox Authenticode 校验，其真实性依赖官方 GitHub release 清单和发布权限。默认更新源会强制使用官方 GitHub release 清单；自定义更新源的维护者需要承担同等的发布安全责任。

## 发布流程

`scripts/build.bat` 会把 updater 放入便携包和 NSIS 安装目录。SignPath 的 portable 和 installer-payload artifact configuration 必须同时签名：

```text
LunaBox.exe
lunacli.exe
LunaBoxUpdater.exe
```

release workflow 会验证三者的 Authenticode 签名，收集每个 channel 的实际运行时文件，生成完整 `.zst` 和 JSON 清单。若能取得上一稳定版的 `LunaBox.exe.zst`，还会调用官方 `zstd --patch-from` 生成 patch，并用 updater 自己的解码器重建、校验后再发布。

第一次包含 updater 的版本是 bootstrap 版本：旧版本目录里没有 `LunaBoxUpdater.exe`，必须由用户手动下载或安装一次。从下一个版本开始才能使用应用内更新；第一次生成更新资产时没有旧 `.zst` 基线也属于正常情况，只会发布完整更新。

默认版本源无需增加字段，LunaBox 会根据版本号推导官方清单 URL。自定义版本源需要在 `version.json` 中提供：

```json
{
  "update_manifest_url": "https://example.com/LunaBox-1.2.3-update-manifest.json"
}
```

当前 patch 只覆盖紧邻的上一稳定版本，不做多版本 patch 链。跳版本用户以及任何不匹配官方源文件哈希的用户都会使用完整 `.zst`，不会尝试模糊匹配。

## 测试发布

在 GitHub Actions 中手动运行 `.github/workflows/update-test.yml` 定义的 `Test Update Release`：

1. 使用工作流自带的 branch 下拉框选择要构建的分支。
2. `version` 填不带 `v` 的 `X.Y.Z-test.N`，并确保它在 SemVer 顺序上高于待更新客户端，例如从 `1.10.0` 测试时使用 `1.10.1-test.1`。
3. `previous_tag` 留空时只生成 full 资产；填写已有 release tag 时，会下载该版本的 `LunaBox.exe.zst` 并尝试生成 delta。

测试 tag 不会覆盖，成功后会创建 `vX.Y.Z-test.N` pre-release。要验证 delta，先发布并安装 A（例如 `1.10.1-test.1`），再发布 B（例如 `1.10.1-test.2`），且 B 的 `previous_tag` 填 A 的 tag。

每个测试 release 都包含 `LunaBox-<version>-version.json`。不要修改正式的 `sync/version.json`；关闭待测试客户端后，在其 `appconf.json` 中将 `update_check_url` 设置为 release notes 给出的测试 JSON 地址：

- 便携版：程序目录旁的 `appconf.json`
- 安装版：`%APPDATA%\LunaBox\appconf.json`

自定义更新源不会自动推导 manifest 地址，因此测试 JSON 已显式包含 `update_manifest_url`。测试结束后删除 `update_check_url` 或设为空字符串，即可恢复正式 Cloudflare/Netlify 更新源。
