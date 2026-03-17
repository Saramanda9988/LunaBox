# 前端规范

## 路由（@tanstack/react-router）

- MUST 使用 `@tanstack/react-router` 管理路由。
- MUST 保持"页面 = route 文件"：`frontend/src/routes/*.tsx` 中每个页面导出 `Route`。
- MUST 新增页面路由时同时完成两步：
  1. 新增 `frontend/src/routes/<page>.tsx`
  2. 在 `frontend/src/App.tsx` 中把该 Route 加入 `routeTree`（与现有写法一致）

---

## 状态管理（Zustand）

Store 位于 `frontend/src/store.ts`，区分两层配置状态：

- `config`：当前**已生效**的运行态配置，供 App.tsx、根布局、各页面和 hooks 读取。
- `draftConfig`：设置页编辑中的**草稿配置**，仅供设置相关路由/面板使用。

**写入 API（MUST 使用，不要绕过）：**

| API | 用途 |
|-----|------|
| `patchLiveConfig(patch)` | 需要立即生效的配置（界面缩放、主题、语言、时区、侧边栏等），同步更新 config + draftConfig 并持久化 |
| `saveDraftConfig()` | 设置页草稿的统一提交 |

**使用模式：**

- 运行态页面/Hook → 读取 `config`
- 设置页表单 → 读取/修改 `draftConfig`
- 即时生效型控件 → 直接调用 `patchLiveConfig(...)`
- 普通输入框/开关 → 写入 `draftConfig`，由设置页 debounce 调用 `saveDraftConfig()`

MUST NOT 在设置页再额外维护一份与 `config`/`draftConfig` 平级的本地 `formData` 作为配置真源。局部 UI 临时状态（骨架屏、弹窗开关、loading）仍可使用组件 state。

跨页面共享、或与后端配置/数据缓存相关的状态放入 store；页面内临时 UI 状态使用组件本地 state。

---

## 样式（UnoCSS）

- MUST 使用 UnoCSS，入口已在 `frontend/src/main.tsx` 引入 `virtual:uno.css`。
- MUST 使用 `frontend/uno.config.ts` 中自定义的品牌色类，而非硬编码（如 `bg-blue-100`）。
- MUST 优先在 `frontend/uno.config.ts` 增加/调整 shortcuts/rules/variants。
- SHOULD 用 utility class 组合 UI，避免为单个组件创建大量独立 CSS。
- MUST 尽量减少在 `frontend/src/style.css` 写样式；仅允许"无法避免的全局样式"（统一滚动条、全局过渡禁用类等）。

反例（MUST NOT）：为一个按钮/卡片样式，在 `style.css` 新增几十行选择器。

---

## 组件与依赖约束

- MUST 以自定义组件为主。
- 允许使用的第三方 UI 构件：
  - `@headlessui/react`（可直接用或封装）
  - `@radix-ui/*` 的原子组件（**必须二次封装后再使用**）
- MUST NOT 引入其他 UI 库（MUI、Antd 等）。
- 参考封装模式：
  - `frontend/src/components/ui/BetterSelect.tsx`（HeadlessUI 封装）
  - `frontend/src/components/ui/BetterSwitch.tsx`（Radix 封装）
- 新增可复用组件放在 `frontend/src/components/ui/`。

---

## 暗黑模式与玻璃态（Glass）

- MUST 支持暗黑模式：使用 `dark:` 变体（项目通过在 `documentElement` 上切换 `light/dark` class 实现）。
- MUST 为新增组件补齐 dark 状态下的文本/背景/边框对比度，不允许"暗色下不可读"。
- MUST 适配"自定义背景 + 玻璃态"模式：
  - 除 modal 类组件外，新增组件 SHOULD 适当加入 `data-glass:` 相关样式或使用预制 `glass` 类
  - 在 `data-glass="true"` 时避免纯不透明大面积底色，优先半透明/边框/blur 维持层次
  - **每个页面的最外层盒子不要设置任何颜色与不透明度**，全部由 root 控制，保证背景图片的 blur/透明度一致性
- 根节点布局已在 `frontend/src/routes/__root.tsx` 上设置 `data-glass`，不要重复造全局开关。

---

## 工具函数

- MUST 新增工具函数前先检查 `frontend/src/utils/` 是否已有实现；优先复用或在原文件中扩展。
- SHOULD 保持 utils 纯函数化（输入/输出清晰、可复用），避免在 utils 内直接读写全局 store。

---

## 时间与日期

- MUST 涉及日期/时间相关的 UI 展示，必须使用 `frontend/src/utils/time.ts` 中的函数处理时间。

---

## 与后端交互（Wails）

- MUST 通过 `frontend/wailsjs/` 生成的绑定调用后端服务。
- SHOULD 将"后端调用 + 结果归一化/错误提示"封装到 hooks 或 store action 中，避免散落在各页面。
