# 与原版的差距

desktop-mygo 替代 Wails 版之前还要补的东西。按要不要做分成四类：

1. [功能未实现](#功能未实现)：原版有、这里没有或只弹“尚未迁移”的功能，**必须做**。
2. [还没对照过的界面](#还没对照过的界面)：已经实现，但没有和原版截图逐像素比较过，**要补对照场景**。
3. [已知的视觉差异](#已知的视觉差异)：对照过、知道哪里不一样，**可以修**。
4. [有意不同或受 MyGo 限制](#有意不同或受-mygo-限制)：暂时不打算改，记在这里免得被当成 bug。

最后是[发布与平台](#发布与平台)的工作和[原版自身的问题](#原版自身的问题)。完成一项后从这里删掉；新发现的差异按上面的分类加进来。

清单在 2026-10-21 对照原版 `apps/desktop/frontend/src` 的组件与 hooks 核对过：弹“尚未迁移”的地方都调用 `locales.NativeFeatureNotMigrated`，可以搜这个函数找到。

## 功能未实现

### 游戏库

- 筛选条件不像原版那样存进 localStorage，只在一次运行内跨页面保留。

### 统计页

- HTML 模板在系统 WebView 里预览（`ui/pages/stats/export.go`），运行时用 `mygo.WebViewAvailable()` 检测：Linux 安装包仍不依赖 WebKitGTK（`mygo.json` 的 `"linux": {"webview": false}`），没装时弹窗里提示，不能预览和导出。封面拼贴是原生绘制的，不受影响。

### 首页

- 没有触控、触控笔上下滑动展开或收起封面选择面板（`useVerticalSwipe`）。

## 还没对照过的界面

对照场景在 `tools/parity/scenarios.mjs`，流程见 [parity.md](parity.md)。现在只有深色主题、简体中文、150% 缩放下的游戏库、收藏夹、收藏详情、设置页各分区与下载管理有场景。还没有场景的：

- **整页**：首页、游戏详情页（图表与日期、滚轮选择器单独对照过）、统计页（`DESKTOP_MYGO_PNG=<目录> go test ./ui/pages/stats -run TestScreenshots` 可以画出整页目测）、灵动岛、自定义背景与玻璃模式。
- **状态与弹窗**：
  - 游戏库：批量“设为状态”菜单、添加到收藏夹、删除确认与导入到 Steam 的弹窗、空状态；筛选抽屉里的预设表单与预设列表。
  - 添加与导入游戏（`ui/components/importer`）：添加游戏的各个步骤、批量导入（选择目录、扫描方式菜单、预览表格、匹配进度、手动选择、跳过列表）、
    拖放导入与拖动经过窗口时的提示、从其他启动器导入的预览。`-route /library -state addgame|batchimport|import:<来源>` 可以直接打开它们。
  - 侧边栏：云同步的悬停面板（同步中、出错、未配置）。
  - 收藏夹：批量删除、重命名、添加游戏弹窗、空收藏夹。
  - 设置页：各个弹窗。
  - 根布局：安装确认（`InstallConfirmModal`）、时区选择（`TimezoneSelectModal`）与启动时的更新弹窗。
  - 下载管理：下载中、空列表、出错、自动解压失败的卡片（对照数据里只有已完成的任务）。
- **主题、语言与缩放**：浅色主题；繁体中文、英语、日语（英语、日语的文字更长，已知筛选抽屉的“游戏筛选预设”一行会顶到边缘，其他页面没有逐页检查）；150% 以外的缩放。

## 已知的视觉差异

- 首页：标题与说明文字的 `drop-shadow` 没有还原；选中封面的旋转描边没有 `blur-[1px]`。
- 游戏库：卡片标题的 `drop-shadow-sm` 没有还原；GIF 动图封面只显示第一帧。
- 收藏夹：卡片悬停时展开的封面不会浮到相邻卡片之上（原版 `hover:z-30`），最多盖住 2 像素的间距。
- 灵动岛：多个游戏的切换顺序按启动时间（原版按 `gameRuntimes` 的插入顺序）；封面悬停的 `brightness-75` 用 25% 的黑色遮罩近似；标题跑马灯的宽度按文字测量，不跟随 ResizeObserver。
- 统计页：加载时的骨架屏与 AI 总结的三条占位条按同一节奏闪烁（原版各自延迟 75ms、150ms）；排行榜名次没有 `tabular-nums`；并排时“游玩时长趋势”取“游玩时段分布”上一帧的高度，数据变化后的第一帧可能差一点。
- 图表：没有 Chart.js 的首次出现与数据变化动画；旋转的标签比 Chrome 略粗糙。
- 设置页：分区展开用 MyGo Collapsible 自带的缓动（原版 ease-out）；滑块禁用时拇指的颜色没有对照；数据表格的表头在滚动区外，滚动条只在表体旁边。
- 导入预览表格：原版 table-fixed 的固定宽度列（`w-16`、`w-32`）之外，百分比列按比例分剩下的宽度，与 Chromium 分配多余宽度的方式不完全相同。
- 下载管理：“已取消选择可执行文件”与“已导入，但元数据获取失败”是普通提示，没有原版的 ⚠️ 图标（`components.Toaster` 还不支持自定义图标）。
- 快捷启动缺少 Wine 运行器时，“打开启动配置”按钮放在 AppToaster 的卡片里（原版那一条是 `toast.custom` 自己画的卡片）。

## 有意不同或受 MyGo 限制

### MyGo 做不到

- Toast 最多同时显示 3 条（原版 4 条）。
- 禁用元素的不透明度最高 0.5：MyGo 会把禁用元素再减半，`components.Disable` 补偿，但 `opacity-60`、`disabled:opacity-65`、`disabled:opacity-70` 做不到。
- 没有 `cursor-zoom-in` 光标。
- 文本框没有 `resize-y` 拖动柄，评价的文本框固定 240px 高、可滚动。
- 写剪贴板不报告失败，所以下载管理没有“复制失败”的提示。
- 原生复选框（导入预览、跳过无路径的游戏）按 Chromium 浅色样式自己画；禁用时用 MyGo 的半透明近似 Chromium 的灰色。
- 拖放导入用 MyGo 的原生拖放，只接受文件列表；原版 dragleave 后 80ms 才隐藏提示的延迟没有。
- 批量模式的拖动选择（`ui/components/grid/drag_select.go`）只有鼠标右键：MyGo 的指针事件不区分鼠标、触控与触控笔，原版触控、触控笔移动 10px 后开始拖动、之后 500ms 内屏蔽点击的那条路径没有。

### 实现方式不同

- 封面主色（`useImageAccentRgb`）按 fast-average-color 的 sqrt 算法在解码后的封面上计算，缩小时的重采样与浏览器不同，颜色可能略有出入。
- 封面大图最大缓存 1024×576 像素（200% 缩放下的最大显示尺寸），原图更大时按缓存的尺寸限制“不放大”。
- 名称排序用 `golang.org/x/text/collate`（中文按拼音），多音字的读音可能与 Chromium 的 ICU 不同：如“测似乎”原版排在“测试”之后，这里排在之前。
- 日期按应用的语言与系统的区域格式显示；原版按 WebView 的默认语言（`toLocaleDateString(undefined)`），两者在语言与系统不一致时可能不同。
- 表情选择器没有照搬 emoji-picker-element 的布局，改成九列网格：分组导航与表情列对齐、面板按内容收紧、“移除”是带图标的按钮（没有图标时禁用）；数据内嵌（`tools/emoji` 下载，原版运行时从 CDN 下载）。
- 自定义背景在后台按窗口大小裁剪、缩小并模糊一次（`covers.Background`），不是每帧由浏览器模糊；窗口大小变化不到一成时沿用旧图（按 cover 裁剪显示），变化更大时在后台重新计算，算好前显示旧图。
- 玻璃模式下 GameCard 正文只保留半透明底色，没有原版的 `backdrop-blur-24`（深色 `backdrop-blur-12`）：一屏几十张卡片各自在 GPU 上模糊代价太大。真正的背景模糊（`glass.Blur`）只用在提示条、收藏详情的“…”按钮等少数元素上。
- 首页悬停在展开的面板上时不暂停轮播：面板展开时轮播本来就停了。
- 云同步悬停面板里状态胶囊的 `ring-1` 画成 1px 边框（占内容的空间）。
- 筛选预设在游戏库每次重新查询时读取，读取失败只记日志，不像原版那样提示 `filterPresets.loadFailed`（那时没有可以显示提示的帧）。
- 启动配置没有原版 `GetLocalProtonTools` 失败时的提示：Go 的服务方法不返回错误，原版只有 Wails 调用本身失败时才走到那里。
- 裁剪弹窗的手柄与裁剪框一样只在键盘聚焦时显示聚焦色（原版用 `:focus`，用鼠标按过的手柄也会显示）。
- 拖动选择按上一帧的布局判断指针下的卡片（原版 `elementFromPoint`），自动滚动后选中指针下的卡片可能晚一帧；每张可选的卡片外面多包一层不加样式的 Box，用来取它的位置。
- 封面拼贴（`ui/pages/stats/mosaic.go`、`poster_layout.go`、`poster_paint.go`）：排布与原版 `posterLayout.ts` 逐项一致（`testdata/poster_layout.json` 由原版代码生成）；封面解码最宽 1000 像素（原版长边 1400），很大的横图导出时略软；背景模糊用元素的 `Blur` 加不透明度，超出海报边缘的处理与 canvas `filter` 略有不同；字体每个系统取一个（雅黑、PingFang SC、Noto Sans CJK SC），不是字体栈；“显示数量”用 BetterSelect、缩放滑块自己画（原版是原生 select 与 range）；编辑器换封面没有 0.4→1 的淡入；滚轮缩放沿用 `exp(-deltaY*0.001)`，但 MyGo 每格的 DY 与浏览器不一定相同；抬起封面的阴影偏移随封面旋转（canvas 的 shadowOffset 不随变换）；右下角品牌图标由 96px 的图放大到 114px 绘制。

### 比原版更合理的行为

- 设置页离开时补存还没保存的草稿（原版丢掉）；保存失败时提示并把草稿恢复成生效的配置（原版只打日志，草稿留着，之后每次保存都会失败）。配置在主线程同步读取，所以没有骨架屏。
- 设置页面板的状态在页面的整个生命周期里保留（原版每次进入设置页重新挂载）：更新检查结果、元数据刷新进度会保留；数据库备份、本地集成、账号卡片在分区重新展开时重新加载。
- 安装确认弹窗里 `StartDownload` 失败时提示原因（原版不处理，只关闭弹窗）；启动时检查更新失败的提示用“检查更新失败”（原版是写死的英文）。
- 原版硬编码的中文（如 BetterSelect 的“请选择”）在这里也翻译了；收藏卡片右上角“从收藏夹中移除”按钮原版没有无障碍名称，这里用 `native.removeFromCategory`。

### 照原版保留

- 裁剪弹窗换了比例后不拖动直接确认，用的还是上一次的裁剪区域。
- 原版里本来就不显示的图标（`content-save-clock-outline`、`application-search-outline`、`database-star-outline` 在 MDI 里不存在）同样留空。
- 编辑页日期选择器的星期与翻页按钮是写死的英文。

## 发布与平台

- **与 Wails 版并存**：单实例锁用的是同一个标识 `io.github.saramanda9988.lunabox`（`mygo.json` 的 `identifier`，即 Wails 版的 `applicationUniqueID`；开发版是 `LunaBox Dev`），但机制不同（Wails 是 Windows 的命名互斥量、Linux 的 D-Bus 名称、macOS 临时目录里的锁文件，MyGo 是 Unix socket），两个版本仍可能同时打开同一份数据（DuckDB 会拒绝第二个进程，后启动的那个显示启动错误窗）。
- **跨平台**：在 macOS（Metal）与 Linux 上跑通并对照。启动配置的 macOS 兼容层、Linux 的 Wine / Proton 已迁移，但只在 Windows 上用替换的服务测过；macOS 的 `lunabox://`（`mygo.json` 的 `urlSchemes` 写进 Info.plist，`App.OnOpenURL` 交给 `HandleLaunchedWithURL`）还没在 macOS 上实测。
- **打包发布**：`tools/release` 已按 Wails 版的布局打出全部种类的包（Windows 便携版与安装版、macOS DMG、Linux AppImage、deb、rpm），Windows 的两种包带 LunaBoxUpdater.exe；还没有代码签名（SignPath、macOS 的签名与公证），没有替换 Wails 版的正式发布流程。
- **应用内更新与 Wails 版共用官方渠道**：“立即更新”与 Wails 版一样按官方更新清单的 `windows-<架构>-<portable|installer>` 渠道下载并交给 LunaBoxUpdater。正式发布切换到 MyGo 版之前，清单里是 Wails 版的文件，MyGo 预发布版更新后会变成 Wails 版（数据通用）。完整的下载、替换与重启只能等有清单时验证。

## 原版自身的问题

没有修改，两个前端都受影响：

- `BackupService.RestoreBackup` 在存档目录里只有一个文件时会把它当作单文件存档，恢复到目录自身的路径上。
