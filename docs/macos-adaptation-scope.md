# macOS 适配需求收敛

> 本文记录 LunaBox macOS 适配中“托盘 / 开机自启 / lunabox:// 协议 / CLI 安装 / 系统代理”的需求边界与实现方向。
> 当前仓库主文档仍声明 LunaBox 仅支持 Windows；正式进入实现前，需要同步调整平台支持声明与构建发布流程。

## 背景

当前 LunaBox 已有部分 macOS build tag 代码，并且 `wails build` 可在 macOS 上产出 `.app`。但“能打包”不等于“可正式支持”。macOS 适配需要优先收敛系统集成需求，避免把 Windows 的 registry、`.exe`、NSIS、托盘行为直接平移。

相关外部资料：

- `energye/systray`：https://github.com/energye/systray
- Wails Custom Protocol Schemes：https://wails.io/docs/guides/custom-protocol-schemes/
- Apple `CFBundleURLTypes`：https://developer.apple.com/documentation/bundleresources/information-property-list/cfbundleurltypes
- Apple `SMAppService`：https://developer.apple.com/documentation/servicemanagement/smappservice
- Apple `SCDynamicStoreCopyProxies`：https://developer.apple.com/documentation/systemconfiguration/scdynamicstorecopyproxies%28_%3A%29
- Apple CFNetwork Global Proxy Settings Constants：https://developer.apple.com/documentation/cfnetwork/global-proxy-settings-constants
- Apple `networksetup` 说明：https://support.apple.com/guide/remote-desktop/about-networksetup-apdd0c5a2d5/mac
- Docker Desktop macOS install / CLI 行为参考：https://docs.docker.com/desktop/setup/install/mac-install/

## 需求结论

### 1. 托盘

`github.com/energye/systray` 本身支持 macOS，因此 macOS 托盘可以继续以该依赖为首选方案。但 LunaBox 是 Wails App，macOS 下 AppKit / Wails 主循环比较敏感，不能只把 `tray_systray.go` 的 build tag 改成包含 darwin 就算完成。

需求收敛：

- macOS 支持菜单栏托盘图标。
- 菜单项保持与 Windows 一致：显示主窗口、退出。
- `CloseToTray` 在 macOS 上只有托盘可用时才允许生效。
- 如果托盘初始化失败，关闭窗口应回退为正常退出或前端退出同步，而不是静默隐藏窗口。
- 实现优先尝试 `energye/systray` 的 macOS 能力；如与 Wails 主循环冲突，再切换到 Wails/macOS 原生菜单方案。

实现落点建议：

- `tray_darwin.go` 从 no-op 改为 macOS 专用实现。
- 保持 `lifecycleState.StartTray()` / `RequestTrayQuit()` 作为业务边界。
- 不在 service 或前端散落平台判断。

### 2. 开机自启

macOS 不使用 Windows registry。开机自启应走系统登录项能力，让用户可在系统设置中看到并管理。

需求收敛：

- macOS 13+ 优先使用 `SMAppService`。
- 用户在设置页打开“开机启动”后，系统将 LunaBox 加入 Login Items。
- 用户在系统设置中移除后，应用内状态应能反映为未启用或需要重新启用。
- macOS 旧版本首版不承诺完整兼容；如确需支持，再评估 `LaunchAgents` fallback。

实现落点建议：

- 新增 `internal/autostart/autostart_darwin.go`。
- 保持现有 `autostart.Sync(enabled bool)` 接口。
- 前端文案从“Windows 登录后自动启动”调整为跨平台表达（开机后自动启动）。

### 3. `lunabox://` 协议

当前 `lunabox://` 的作用主要有两个：

- 网站点击 `lunabox://install?...`，浏览器拉起 LunaBox，前端弹出下载确认。
- App 内导出 `lunabox://launch?game_id=...`，用于快捷启动游戏。

macOS 可以实现同等能力，但不是 registry，也不应依赖命令行参数作为主路径。

需求收敛：

- macOS 支持浏览器打开 `lunabox://install`，`lunabox://launch` 暂时不支持 macos 即可。
- macOS 不提供“手动注册 / 取消注册协议”按钮；协议由 App bundle 的 `Info.plist` 和系统 LaunchServices 管理。
- 便携版设置页协议模块按平台分流：
  - Windows：保留注册 / 取消注册按钮。
  - macOS：展示“协议随 App 安装由系统管理”的状态说明。
  - Linux：后续通过 `.desktop` / `xdg-mime` 单独适配。

实现落点建议：

- 在 `wails.json` 的 `info.protocols` 声明 `lunabox`。
- Wails 构建时会把协议写入 macOS `Info.plist` 的 `CFBundleURLTypes`。
- 在 `mac.Options.OnUrlOpen` 中接收 URL。
- 将 `main.go` 当前基于 `os.Args` 的协议处理抽成共享函数，例如 `handleProtocolURL(rawURL)`。
- Windows argv、macOS `OnUrlOpen`、IPC 转发复用同一套解析与分发逻辑。

### 4. CLI 安装

当前 Windows 便携版逻辑是把 `lunacli.exe` 放在程序目录，并把目录加入用户 PATH。macOS / Linux 更适合采用 Docker Desktop 类似的 symlink/shim 思路。

结论：

- Docker Desktop 风格适合 CLI 安装。
- 不适合协议注册。
- 不适合开机自启。

需求收敛：

- macOS App bundle 内打包 `lunacli`，例如 `LunaBox.app/Contents/Resources/bin/lunacli`。
- 提供用户级 CLI 安装：
  - macOS / Linux：`~/.local/bin/lunacli`
  - 不需要管理员权限。
  - 如果 `~/.local/bin` 不在 PATH，提示用户添加。
- 可选提供系统级 CLI 安装：
  - macOS / Linux：`/usr/local/bin/lunacli`
  - 需要管理员权限或额外授权。
- Windows 保持现有 `lunacli.exe` 同目录 / PATH 逻辑。
- CLI 调用继续复用当前 IPC 模型：
  - GUI 已运行：转发到 GUI。
  - GUI 未运行：按现有 CLI CoreApp 行为处理，或对 GUI 依赖命令给出明确提示。

实现落点建议：

- 抽出平台函数：
  - Windows：`lunacli.exe`
  - macOS / Linux：`lunacli`
- `PortableSetupService` 改名或语义扩展为跨平台“系统集成设置”。
- `RegisterCLIPath` / `UnregisterCLIPath` 在 macOS / Linux 下改为创建 / 删除 symlink，而不是写 PATH。

### 5. `proxyutils` 系统代理

当前 `internal/utils/proxyutils` 已经把应用内网络请求统一收敛到 `system / manual / direct` 三种模式：

- `manual`：用户填写 `http://`、`https://`、`socks5://` 或裸地址，应用内请求显式走该代理。
- `direct`：应用内请求直连。
- `system`：优先读取系统代理；如果系统代理没有可用静态代理，再回退到环境变量代理。

问题在于目前只有 Windows 读取了系统代理。`net_proxy_windows.go` 会读注册表 `Internet Settings`；`net_proxy_other.go` 在 macOS / Linux 下是 no-op，因此 macOS 现在选择“自动跟随系统代理”时，实际上只能跟随 `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` 等环境变量。普通 `.app` 从 Finder / Dock 启动时通常没有 shell 环境变量，这会导致 macOS 用户以为跟随系统代理，但应用内元数据、图片、下载、AI、更新和云同步请求仍可能直连。

需求收敛：

- macOS 首版只需要“读取并应用系统代理”，不提供“修改系统代理”的能力。
- `system` 模式在 macOS 下应读取当前有效的 HTTP / HTTPS / SOCKS 静态代理。
- PAC / Auto Proxy Configuration 和 Auto Proxy Discovery 首版只检测并提示，不执行 PAC 脚本，也不尝试解析每个请求的动态代理结果。
- 如果系统代理只配置 PAC / 自动发现，`ResolveProxy` 应返回类似 `system (PAC detected, no static proxy)` 的描述，并继续回退环境变量；若仍无环境变量，提示用户切到 `manual`。
- `manual` / `direct` 的行为保持跨平台一致，不需要 macOS 特殊分支。
- 不把 LunaBox 做成系统代理管理器，不写入用户的网络服务配置，不要求管理员权限。

推荐实现方向：

- 新增 `internal/utils/proxyutils/net_proxy_darwin.go`，替代当前 `!windows` stub 在 darwin 下的 no-op。
- 首选通过 `SystemConfiguration` / `CFNetwork` 读取当前有效代理：
  - `SCDynamicStoreCopyProxies(nil)` 返回当前 internet proxy settings。
  - 读取 `HTTPEnable` / `HTTPProxy` / `HTTPPort`，`HTTPSEnable` / `HTTPSProxy` / `HTTPSPort`，`SOCKSEnable` / `SOCKSProxy` / `SOCKSPort`。
  - 读取 `ProxyAutoConfigEnable` / `ProxyAutoConfigURLString`、`ProxyAutoDiscoveryEnable`，用于日志和 UI 提示。
  - 读取 `ExceptionsList` / bypass domains，尽量保持与系统“绕过代理”规则一致。
- 如果系统 API 接入成本过高，可先用命令行兜底：
  - `scutil --proxy` 可作为诊断和解析来源，输出已经是当前有效代理字典。
  - `networksetup -listallnetworkservices`、`-getwebproxy`、`-getsecurewebproxy`、`-getsocksfirewallproxy` 可作为按 network service 读取的备用方案。
- 不建议首版使用 `networksetup -setwebproxy` / `-setsecurewebproxy` / `-setsocksfirewallproxy`。这些命令是修改系统网络配置的入口，会触发权限、网络服务选择、用户预期和回滚问题。

实现细节建议：

- 复用现有 `ProxySelection`：
  - HTTP proxy -> `HTTPProxy`
  - HTTPS proxy -> `HTTPSProxy`
  - SOCKS proxy -> `AllProxy`
  - `Source` 设为 `system`
- `loadSystemProxySelection()` 保持只返回“可直接给 `http.Transport.Proxy` 使用”的静态代理。
- 当前 `ProxySelection` 还没有 bypass list 表达能力；实现 macOS 读取时应同步评估是否给 `ProxySelection.Proxy(req)` 增加跨平台 bypass 判断，避免系统设置了 `localhost`、`*.local`、内网 CIDR 时仍被强制走代理。
- 系统代理检测到 PAC / 自动发现但没有静态代理时，返回 note，不返回不可用的 `ProxySelection`。
- `Description()` 不要泄漏认证信息；继续使用 `url.URL.Redacted()`。
- 单元测试可覆盖解析层，不强依赖真实 macOS 系统配置；系统 API / 命令输出解析用 fixture 文本测试。

前端文案需要同步：

- 当前设置页文案“自动跟随系统代理”可以保留。
- macOS 补齐读取后，说明里应追加边界：“PAC / 自动发现暂不解析；如未生效请使用手动代理”。
- 现有 Windows 下载代理提示里提到 PAC / TUN 场景，后续应改成跨平台提示：
  - Windows / macOS：能读取静态系统代理。
  - PAC / TUN-only / VPN 内置路由：无法保证自动识别，建议手动填写本地代理地址。

## 建议阶段

### 阶段 1：macOS 首版系统集成

- macOS 托盘不再 no-op。
- `CloseToTray` 只在托盘可用时生效。
- `lunabox://` 通过 `wails.json info.protocols` + `mac.OnUrlOpen` 接入。
- 设置页按平台拆分协议和 CLI UI。
- CLI 名称从硬编码 `lunacli.exe` 改为平台函数。
- `proxyutils` 在 darwin 下读取静态系统代理，不再只依赖环境变量兜底。

### 阶段 2：macOS 完整体验

- 使用 `SMAppService` 实现开机自启。
- CLI 打包到 `.app` 内，并支持用户级 symlink。
- 补齐 macOS icon、签名、公证、DMG / zip 发布链路。
- 更新版本检查下载链接结构，区分 Windows / macOS / 架构。
- 设置页展示系统代理检测状态，明确 PAC / 自动发现 / TUN-only 的限制。

### 阶段 3：Linux 后续适配

- `lunabox://` 通过 `.desktop` / `xdg-mime` 注册。
- CLI 复用 `~/.local/bin/lunacli` symlink 策略。
- 托盘按桌面环境验证。
- `proxyutils` 在 Linux 下优先沿用环境变量代理；是否读取桌面环境代理配置另行评估。
