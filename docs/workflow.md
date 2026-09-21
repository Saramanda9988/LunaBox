# 变更流程与自检清单

## Agent 变更流程（推荐顺序）

当被要求"实现一个功能/修复一个问题"时，按此顺序执行：

1. **定位落点**：优先在现有 route/service/utils 中找最贴近的位置（参考 [anchors.md](anchors.md)）
2. **复用模式**：沿用同目录已有写法（migration 幂等、service 注入、UI glass/dark）
3. **实现最小改动**：避免新增大层级抽象
4. **本地验证**：尽量运行已有 build task
5. **自检**：对照下方清单

---

## 自检清单（交付前）

### 前端

- [ ] 新增/修改的组件在 `light` 与 `dark` 下都可读
- [ ] 背景开启时 `data-glass="true"` 下观感不崩
- [ ] 没有把局部样式硬塞进 `style.css`（除非是全局不可避免项）
- [ ] 新增组件遵守 HeadlessUI/Radix 封装约束，没有直接引入大型 UI 框架
- [ ] 页面最外层盒子没有设置颜色与不透明度
- [ ] 应用级副作用是否已优先抽到 `frontend/src/hooks/`，而不是把复杂 `useEffect` 堆在 `App.tsx`

### 后端

- [ ] 涉及 schema 变更时同时更新 `InitSchema` + 新 migration
- [ ] Migration 幂等且事务安全，空库也能跑通
- [ ] 没有引入非 Windows 平台的系统调用
- [ ] 底层操作优先 Wails/Go 标准库
- [ ] 若新增/修改 `AppConfig` 字段，已同步检查 `LoadConfig/SaveConfig` 与 `ConfigService.UpdateAppConfig(...)`
- [ ] 若修改退出逻辑，已核对 `OnBeforeClose` / 前端退出流 / `OnShutdown` 的职责边界，没有把交互流程塞进 `OnShutdown`

### 通用

- [ ] 新增工具函数前已搜索并复用现有 `frontend/src/utils` 或 `internal/utils`
- [ ] 没有顺手重构或格式化不相关代码

### Linux 渲染验证

开发、绑定生成和打包流程都会调用 `scripts/patch-wails-linux-tray.sh`。该脚本除了托盘修复，还会让 amd64 GTK4 构建保留 WebKit 的默认 GPU 渲染路径，并在创建 WebView 时，通过 WebKitGTK 2.42 起提供的公开 feature API 关闭 `PreferPageRenderingUpdatesNear60FPS`。直接使用 `go build` 前需要手动运行该脚本；设置 `LUNABOX_WEBKIT_PREFER_60FPS=1` 可以保留引擎默认偏好以便对比。

Wails beta.24 的 `application_linux.go` 在包初始化时检测 NVIDIA，并自动设置 `WEBKIT_DISABLE_DMABUF_RENDERER=1`。本机独立引擎测试确认该值会使 `webkit://gpu` 的硬件加速策略变成 `never`，即使 API 请求 `Always`。LunaBox 补丁在 amd64 上跳过这项自动禁用，用户显式设置的环境变量仍然有效；arm64 保留原有兼容措施。若 NVIDIA 环境出现白屏或驱动问题，可用 `WEBKIT_DISABLE_DMABUF_RENDERER=1` 恢复上游兼容行为。补丁需要重新构建并重启应用后生效，不能改变已经启动的 WebKit 子进程。

2026-09-13 本机 WebKitGTK 2.52.5 对比：禁用 DMA-BUF 时策略为 `never`；重新启用后为 `always`，渲染器为 `DMABuf (Hardware, Shared Memory)`，渲染进程使用 NVIDIA GBM 和 `GPU (2 threads)`。项目时间线录制中的 `paint` 中位耗时约 80ms、最高约 120ms，是恢复 GPU 绘制的主要验证目标；尚未获得重启后的同场景录制，不能据此宣称应用性能改善幅度。

开发进程 202205 的主页面 WebKit 子进程 202479 已确认使用 NVIDIA GPU，存在两个 `SkiaGPUWorker`，并持有 GPU 设备句柄。崩溃进程 200808、200298 的 core 堆栈均显示 `SkiaGLContext` 在线程局部变量销毁期间释放 `GrDirectContext` 和 OpenGL 资源，随后在 `libnvidia-eglcore.so.610.43.03` 内触发 SIGSEGV。补丁在 NVIDIA amd64、**运行时 WebKitGTK 2.52** 上默认设置 `WEBKIT_SKIA_GPU_PAINTING_THREADS=0`，避免这条 GPU 工作线程退出路径，保留显式指定的线程数。其他 WebKit 版本不自动设置该变量，避免升级后的绘制机制变化造成回归。

在 2.52 引擎中，零工作线程表示在主线程提交 GPU 绘制，不表示 CPU 软件绘制。本机独立测试仍报告加速策略 `always`、NVIDIA 渲染进程和 DMA-BUF，`Threaded rendering` 为 `Disabled`；六轮包含模糊背景和滚动动画的窗口创建、关闭测试，以及使用项目 Wails 依赖的启动窗口关闭、主窗口退出测试均通过，未发现新 core。此措施是驱动与 Skia 资源释放问题的兼容处理；绘制提交移到主线程后，复杂重绘仍可能阻塞交互，不能据此宣称卡顿已全部解决。应用必须重新构建并重启；已有的 202205 不会改变线程配置。

关闭该偏好不保证高刷新率：还需要 WebKit 的显示同步正常工作。验证时同时检查 `requestAnimationFrame` 帧率和 `webkit://gpu` 中的 `VBlank type`、`VBlank refresh rate`。DRM 同步失败时，WebKit 会回退到固定 60Hz 计时器，GPU 加速仍可能正常启用。

2026-09-13 本机独立 WebKitGTK 2.52.5 测试：GTK 识别 240Hz 屏幕，关闭偏好后 Wayland 约 61fps、X11 约 58fps；活动 NVIDIA DRM CRTC 的 `drmWaitVBlank` 和 `drmCrtcGetSequence` 均返回 `Operation not supported`。因此此补丁只解除帧率偏好，本机的同步回退问题尚未修复。后续需要驱动或 WebKit 显示同步修复，不能以编译通过作为高刷修复成功的依据。

参考：[Wails #6056（macOS 帧率偏好）](https://github.com/wailsapp/wails/issues/6056)、[WebKit DRM 同步实现](https://github.com/WebKit/WebKit/blob/main/Source/WebKit/UIProcess/glib/DisplayVBlankMonitorDRM.cpp)、[WebKit 回退计时器](https://github.com/WebKit/WebKit/blob/main/Source/WebKit/UIProcess/glib/DisplayVBlankMonitorTimer.cpp)。

GPU 参考：[Wails beta.24 NVIDIA 自动禁用逻辑](https://github.com/wailsapp/wails/blob/v3.0.0-beta.24/v3/pkg/application/application_linux.go)、[WebKitGTK 2.48 的 GPU 绘制线程](https://webkitgtk.org/2025/04/08/webkitgtk-2.48.html)。

线程兼容参考：[WebKit 2.52 的主线程 GPU 绘制实现](https://github.com/WebKit/WebKit/blob/webkitglib/2.52/Source/WebCore/platform/graphics/skia/SkiaPaintingEngine.cpp)。升级 WebKit 或 NVIDIA 驱动后，应重新验证 GPU 报告、启动窗口关闭与应用退出，再评估是否恢复 GPU 工作线程。

### macOS 透明窗口

Wails beta.19 起将透明 WKWebView 使用的私有 WebKit API 放入 `private_mac_apis` 构建标签。LunaBox 的主窗口与启动窗口使用透明背景和玻璃效果，因此 macOS 开发构建与 DMG 构建均启用该标签，以保持原有外观。Mac App Store 审核场景应改用 Wails 的公共 API 模式，并重新检查窗口背景效果。
