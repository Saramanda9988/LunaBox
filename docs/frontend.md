# 前端规范

## 路由（@tanstack/react-router）

- MUST 使用 `@tanstack/react-router` 管理路由。
- MUST 保持"页面 = route 文件"：`frontend/src/routes/*.tsx` 中每个页面导出 `Route`。
- MUST 新增页面路由时同时完成两步：
  1. 新增 `frontend/src/routes/<page>.tsx`
  2. 在 `frontend/src/App.tsx` 中把该 Route 加入 `routeTree`（与现有写法一致）
- SHOULD 将“应用级编排”与“页面路由”分开：
  `frontend/src/App.tsx` 保留 routeTree、全局 modal、全局 hook 装配；
  具体运行时副作用优先下沉到 `frontend/src/hooks/`。

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

**配置字段变更约束（MUST）：**

- 新增 `AppConfig` 字段时，前端设置页写入 `draftConfig` 还不够；
  必须同时检查后端 `ConfigService.UpdateAppConfig(...)` 是否把该字段同步回 in-memory config。
- 否则会出现“配置文件已写入，但当前运行态读到的仍是旧值”的问题，影响设置页回显和运行时逻辑判断。

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
- SHOULD 对“应用级副作用”优先使用 hook 封装，而不是把长 `useEffect` 直接堆在 `App.tsx`。
  例如：退出前云同步 / toast 状态机，放在 `frontend/src/hooks/useExitSyncToast.ts`。

---

## 时间与日期

- MUST 涉及日期/时间相关的 UI 展示，必须使用 `frontend/src/utils/time.ts` 中的函数处理时间。

---

## 与后端交互（Wails）

- MUST 通过 `frontend/wailsjs/` 生成的绑定调用后端服务。
- SHOULD 将"后端调用 + 结果归一化/错误提示"封装到 hooks 或 store action 中，避免散落在各页面。
- SHOULD 对跨页面、跨退出入口共享的运行时事件（如 `app:quit-sync-requested`）统一在应用级 hook 中监听，不要在多个页面重复订阅。

---

## 应用退出流（Frontend）

- `frontend/src/App.tsx` 只负责装配退出相关 hook，不直接承载复杂退出状态机。
- 退出前云同步、右上角 toast 提示、超时与自动退出逻辑统一放在 `frontend/src/hooks/useExitSyncToast.ts`。
- 当后端发出 `app:quit-sync-requested` 事件时，前端 SHOULD：
  1. 立刻提示“正在退出，请不要强行关闭应用”
  2. 执行数据库云同步
  3. 成功后提示成功并自动退出
  4. 失败或超时后提示失败并自动退出
- MUST 避免把这类交互再塞回 `OnShutdown` 对应的后端收尾逻辑，因为那时前端已经不适合再承担可交互流程。
