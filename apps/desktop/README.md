# LunaBox 桌面应用

LunaBox 的桌面应用：用 [MyGo](https://github.com/egoist/mygo) v0.7.4 的原生 UI（Windows 上是 D3D11 渲染、DirectWrite 文字，没有 WebView；只有统计页的美化导出用系统 WebView 预览 HTML 模板），后端是 `packages/backend`。

它取代了原来的 Wails + React 应用（已归档在标签 `wails-final`），界面与原版一比一还原、文字沿用原版的语言包、行为由真实服务驱动并有测试覆盖。文档与注释里的“原版”“Wails 版”都指 `wails-final` 里的 `apps/desktop`，需要对照时用 `git show wails-final:apps/desktop/frontend/src/<路径>` 查看。

## 现状

已迁移的页面：首页、游戏库、游戏详情页（全部标签页与弹窗）、收藏夹列表与收藏详情、下载管理、统计、设置页（全部分区与弹窗），以及添加与导入游戏、顶栏、侧边栏、灵动岛、自定义背景与玻璃模式，支持与原版相同的四种语言。

主要还缺：筛选条件跨运行保存、首页的触控滑动、macOS 的签名与公证，以及 macOS、Linux 上的实测。

## 运行

需要 Go 与 cgo 工具链（DuckDB 需要 `CGO_ENABLED=1`）。日常开发在仓库根目录运行 `pnpm dev`，或者：

```sh
cd apps/desktop
go run ./tools/mygo dev                     # 构建并启动，代码、配置或图标改了自动重建、重启
go run ./tools/mygo dev -lang en-US         # 这次运行显示英语，不写进配置
```

开发运行（`go run`、`go build`、`mygo dev`）的数据在 `build/data`，第一次运行时从安装版的 `%APPDATA%/LunaBox` **复制**过来，不会改动安装版的数据：

- 与安装版隔离：单实例锁是“LunaBox Dev”，不注册开机自启，不启动 CLI 的 IPC 服务与 MCP 服务，不自动云同步。
- 读取项目目录下的 `.env.build` 与 `.env`（格式见 `.env.build.example`）补上构建期凭据；没有时 Bangumi 与鲲 Galgame 的授权直接失败。
- 编辑、备份恢复（会覆盖游戏真实的存档目录）、上传到云端（用设置里的云备份账号）都是真实操作。
- `mygo dev` 重启或退出时直接结束进程，不走退出流程，下次启动由 WAL 恢复；从界面或托盘退出则正常关闭。
- 日志在数据目录的 `logs/`。

`mygo dev` 不能给应用传参数，需要参数时先构建：

```sh
CGO_ENABLED=1 go build -o build/LunaBox.exe .
./build/LunaBox.exe -reseed                 # 重新从 %APPDATA%/LunaBox 复制数据
./build/LunaBox.exe -data-dir <目录>        # 使用别的数据目录
./build/LunaBox.exe -lang ja-JP             # zh-CN、zh-TW、en-US、ja-JP；en-XA、ar-XB 是伪语言
./build/LunaBox.exe -hud                    # 性能 HUD（F2 切换，F5 滚动压测）
```

截图与压测参数：

| 参数 | 作用 |
| --- | --- |
| `-route <路径>` | 启动时打开的页面，默认 `/`；如 `/library`、`/settings`、`/game/<游戏 ID>`。Git Bash 里要加 `MSYS_NO_PATHCONV=1` |
| `-state <状态>` | 进入某个界面状态：`drawer`、`addmenu`、`batch`、`addgame`、`batchimport`、`import:<来源>`、`collapsed`、`picker`、`setup`（初始设置向导）、`tab:<标签页>`、`settings:<分区>`、`settings:collapsed` |
| `-screenshot out.png` | 封面加载完后截图并退出 |
| `-width`、`-height` | 窗口尺寸（DIP），默认用上次的尺寸 |
| `-repeat N`、`-bench -bench-exit` | 把游戏库放大 N 倍、自动滚动压测（配合 `MYGO_FRAME_STATS=all`） |

## 测试

```sh
CGO_ENABLED=1 go vet ./... && CGO_ENABLED=1 go test ./...
```

测试不开窗口：`ui.Tester` 构建视图、模拟输入、推进时钟，每个测试在临时目录里新建数据库并跑完全部迁移。

## 发布

`pnpm package`，即：

```sh
go run ./tools/release                          # amd64，版本取最近的 v* 标签，默认的包
go run ./tools/release -version 1.2.0 -arch arm64 -mode installer
```

目标系统是当前系统（DuckDB 需要 cgo，不能交叉编译），`-mode` 选包的种类，输出在 `build/release`。包的文件名与布局沿用 Wails 版：应用内更新、已安装的旧版与 SignPath 的签名配置都依赖它。

| 系统 | `-mode` | 包 |
| --- | --- | --- |
| Windows | `portable`（默认） | `LunaBox-<版本>-windows-<架构>-portable.zip`，带 lunacli、LunaBoxUpdater 与 7z，数据在 `LunaBox.exe` 旁边 |
| Windows | `installer` | `LunaBox-<版本>-windows-<架构>-setup.exe`，用 `packaging/windows/nsis/project.nsi`（不装 WebView2），需要 NSIS |
| Windows | `installer-payload`、`installer-package` | 把 `installer` 拆成两步，中间给 exe 签名：前者只把安装包的内容放到 `LunaBox-<版本>-windows-<架构>-installer`，后者不编译，把这个目录打成安装包 |
| Linux | `appimage`（默认） | `LunaBox-<版本>-linux-<架构>.AppImage`，数据在 `~/.config/LunaBox`，需要 appimagetool |
| Linux | `installer` | `LunaBox-<版本>-linux-<架构>.deb` 与 `.rpm`，依赖 GTK 3，安装脚本在 `packaging/linux/nfpm/scripts`，需要 [nFPM](https://nfpm.goreleaser.com) |
| Linux | `all` | 以上两种 |
| macOS | `installer`（默认） | `LunaBox-<版本>-macos-<架构>.dmg`，`.app` 的 `Contents/Resources/bin` 带 lunacli 与 7zz，ad-hoc 签名 |

随包的 7z（与 Windows arm64 动态链接的 DuckDB）在 `lib/`。统计页的美化导出用系统的 WebView（WebView2 Runtime、WKWebView、WebKitGTK）预览与截取 HTML 模板，运行时检测，安装包不依赖它，没有时弹窗里提示。Linux 构建需要 gcc，运行需要系统的 GTK 3 与 libepoxy；macOS 构建需要 Xcode 命令行工具。安装版（Windows 安装包、deb、rpm、DMG）装到 Wails 版的位置、用同一个卸载项与包名，会直接替换已安装的 Wails 版。

发布前把 `mygo.json` 的 `version` 改成要发布的版本（exe 的文件版本与安装包的版本取它，`release.yml` 会检查它与 `sync/version.json` 一致）。GitHub Actions：`release.yml` 是正式发布（签名、应用内更新资产、GitHub Release），`autobuild.yml` 是滚动的开发版 `dev-latest`，`update-test.yml` 是应用内更新的测试版，见 [docs/updater.md](../../docs/updater.md)。

## 开发约定

界面的约束见仓库根的 [docs/frontend.md](../../docs/frontend.md)。
