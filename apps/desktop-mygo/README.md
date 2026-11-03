# desktop-mygo

LunaBox 的原生桌面前端：用 [MyGo](https://github.com/egoist/mygo) v0.6.3 的原生 UI（Windows 上是 D3D11 渲染、DirectWrite 文字，没有 WebView；只有统计页的美化导出用系统 WebView 预览 HTML 模板）重写 `apps/desktop/frontend`，后端与 Wails 版共用 `packages/backend`。

它和 Wails 版并行开发，目标是之后**替代 Wails + WebView 前端**，所以按正式产品的标准来做：界面与原版一比一、文字走同一套多语言、行为由真实服务驱动并有测试覆盖。

## 现状

**还不参与发布。** 已迁移的页面：首页、游戏库、游戏详情页（全部标签页与弹窗）、收藏夹列表与收藏详情、下载管理、统计、设置页（全部分区与弹窗），以及添加与导入游戏、顶栏、侧边栏、灵动岛、自定义背景与玻璃模式，支持与原版相同的四种语言。

主要还缺：筛选条件跨运行保存、首页的触控滑动、代码签名与替换 Wails 版的正式发布流程，以及 macOS、Linux 上的实测与对照。完整清单（包括还没对照过的界面和已知的视觉差异）见 [docs/gaps.md](docs/gaps.md)。

在 150% 缩放、深色主题、简体中文下与原版逐像素对照过的区域，元素坐标与原版 DOM 一致到 0.1 DIP，剩下的像素差异来自抗锯齿、封面缩放算法和 1 像素的窗口宽度差，明细见 [docs/parity.md](docs/parity.md#读结果)。

## 运行

需要 Go 与 cgo 工具链（DuckDB 需要 `CGO_ENABLED=1`）。日常开发在仓库根目录运行 `pnpm mygo:dev`，或者：

```sh
cd apps/desktop-mygo
go run ./tools/mygo dev                     # 构建并启动，代码、配置或图标改了自动重建、重启
go run ./tools/mygo dev -lang en-US         # 这次运行显示英语，不写进配置
```

开发运行的数据在 `build/data`，第一次运行时从安装版的 `%APPDATA%/LunaBox` **复制**过来，不会改动安装版的数据。但编辑、备份恢复、上传到云端等操作都是真实的，详见 [docs/architecture.md](docs/architecture.md#数据目录)。

`mygo dev` 不能给应用传参数，需要参数时先构建：

```sh
CGO_ENABLED=1 go build -o build/desktop-mygo.exe .   # 即 pnpm mygo:build
./build/desktop-mygo.exe -reseed            # 重新从 %APPDATA%/LunaBox 复制数据
./build/desktop-mygo.exe -data-dir <目录>   # 使用别的数据目录
./build/desktop-mygo.exe -lang ja-JP        # zh-CN、zh-TW、en-US、ja-JP；en-XA、ar-XB 是伪语言
./build/desktop-mygo.exe -hud               # 性能 HUD（F2 切换，F5 滚动压测）
```

截图与压测参数：

| 参数 | 作用 |
| --- | --- |
| `-route <路径>` | 启动时打开的页面，默认 `/`；如 `/library`、`/settings`、`/game/<游戏 ID>`。Git Bash 里要加 `MSYS_NO_PATHCONV=1` |
| `-state <状态>` | 进入某个界面状态：`drawer`、`addmenu`、`batch`、`addgame`、`batchimport`、`import:<来源>`、`collapsed`、`picker`、`tab:<标签页>`、`settings:<分区>`、`settings:collapsed` |
| `-screenshot out.png` | 封面加载完后截图并退出 |
| `-width`、`-height` | 窗口尺寸（DIP），默认用上次的尺寸 |
| `-repeat N`、`-bench -bench-exit` | 把游戏库放大 N 倍、自动滚动压测（配合 `MYGO_FRAME_STATS=all`） |

## 测试

```sh
CGO_ENABLED=1 go vet ./... && CGO_ENABLED=1 go test ./...
```

测试不开窗口：`ui.Tester` 构建视图、模拟输入、推进时钟，每个测试在临时目录里新建数据库并跑完全部迁移。

## 发布

`pnpm mygo:portable`，即：

```sh
go run ./tools/release                          # amd64，版本取最近的 v* 标签，默认的包
go run ./tools/release -version 1.2.0 -arch arm64 -mode installer
```

目标系统是当前系统（DuckDB 需要 cgo，不能交叉编译），`-mode` 选包的种类，输出在 `build/release`，布局与 Wails 版相同：

| 系统 | `-mode` | 包 | 对应 Wails 版 |
| --- | --- | --- | --- |
| Windows | `portable`（默认） | `LunaBox-<版本>-windows-<架构>-portable.zip`，带 lunacli、LunaBoxUpdater 与 7z，数据在 `LunaBox.exe` 旁边 | `scripts\build.bat portable` |
| Windows | `installer` | `LunaBox-<版本>-windows-<架构>-setup.exe`，用 Wails 版的 `build/windows/nsis/project.nsi`（不装 WebView2），需要 NSIS | `scripts\build.bat installer` |
| Windows | `installer-payload`、`installer-package` | 把 `installer` 拆成两步，中间给 exe 签名：前者只把安装包的内容放到 `LunaBox-<版本>-windows-<架构>-installer`，后者不编译，把这个目录打成安装包 | `scripts\build.bat installer-payload`、`installer-package` |
| Linux | `appimage`（默认） | `LunaBox-<版本>-linux-<架构>.AppImage`，数据与 Wails 版的 AppImage 一样在 `~/.config/LunaBox`，需要 appimagetool | `scripts/build.sh appimage` |
| Linux | `installer` | `LunaBox-<版本>-linux-<架构>.deb` 与 `.rpm`，依赖 GTK 3 而不是 WebKitGTK，需要 [nFPM](https://nfpm.goreleaser.com) | `scripts/build.sh installer` |
| Linux | `all` | 以上两种 | `scripts/build.sh all` |
| macOS | `installer`（默认） | `LunaBox-<版本>-macos-<架构>.dmg`，`.app` 的 `Contents/Resources/bin` 带 lunacli 与 7zz，ad-hoc 签名 | `scripts/build.sh installer` |

统计页的美化导出用系统的 WebView（WebView2 Runtime、WKWebView、WebKitGTK）预览与截取 HTML 模板，运行时检测，安装包不依赖它，没有时弹窗里提示。Linux 构建需要 gcc，运行需要系统的 GTK 3 与 libepoxy；macOS 构建需要 Xcode 命令行工具。安装版（Windows 安装包、deb、rpm、DMG）装到 Wails 版的位置，会替换已安装的 Wails 版。Windows 的应用内更新与 Wails 版共用官方更新清单（见 [docs/gaps.md](docs/gaps.md#发布与平台)）；Windows 的代码签名在 `autobuild.yml` 里做。

GitHub Actions 的开发版 **Auto Build Pre-release**（`.github/workflows/autobuild.yml`）已改用 `tools/release` 构建三端，Windows 包经 SignPath 签名，发布到 `dev-latest`。**MyGo Build Pre-release**（`.github/workflows/mygo-build.yml`，手动触发，不签名）的矩阵：Windows amd64、arm64 的便携版与安装版，macOS arm64、amd64 的 DMG，Linux 的 AppImage、deb、rpm（Wails 版暂停了 Linux arm64，MyGo 版保留），发布到滚动的预发布 `mygo-dev-latest`（与 Wails 版的 `dev-latest` 分开）。

## 开发约定

- 迁移一个原版组件时，在 Go 代码的注释里写它对应的原版文件与类名（`对应 GameCard.tsx：rounded-xl border …`），尺寸颜色按 [docs/parity.md](docs/parity.md) 取值。
- 文字用 `ui/locales` 的函数，key 与原版组件在同一处用的 key 相同；原版没有的加到 `tools/i18n/extra` 的 `native` 下，四种语言都写。原版语言包改了以后运行 `go run ./tools/i18n`。
- 时间相关的状态用帧时钟：构建时用 `c.Now()`，输入事件用 `ev.Time`，异步回调里记一个标记、到下一次构建再取 `c.Now()`。测试里用 `tt.Advance(d)` 推进时钟，不要 `time.Sleep`。
- 暂时做不了的功能弹 `locales.NativeFeatureNotMigrated`，并记到 [docs/gaps.md](docs/gaps.md)；补上一项就从那里删掉。

## 文档

| 文档 | 内容 |
| --- | --- |
| [docs/architecture.md](docs/architecture.md) | 目录结构、与后端的关系、数据目录、构建细节、性能 |
| [docs/gaps.md](docs/gaps.md) | 与原版的差距：未实现的功能、没对照过的界面、已知的视觉差异 |
| [docs/parity.md](docs/parity.md) | 与原版逐像素对照的工具、流程与还原要点 |
| [docs/i18n.md](docs/i18n.md) | 多语言：从原版语言包生成目录、约定与日期格式 |
