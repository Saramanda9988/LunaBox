# 关键文件落点（Anchors）

## 前端

| 职责 | 路径 |
|------|------|
| 路由注册（routeTree 组装） | `frontend/src/App.tsx` |
| 根布局（TopBar/SideBar、data-glass、拖拽遮罩） | `frontend/src/routes/__root.tsx` |
| 页面路由文件 | `frontend/src/routes/*.tsx`（每个页面导出 `Route`） |
| 全局 Store（Zustand） | `frontend/src/store.ts` |
| UnoCSS 配置 | `frontend/uno.config.ts` |
| 全局样式（仅全局不可避免项） | `frontend/src/style.css` |
| Wails 绑定（自动生成，勿手改） | `frontend/wailsjs/` |
| 工具函数 | `frontend/src/utils/` |
| 时间处理函数 | `frontend/src/utils/time.ts` |
| 可复用 UI 组件 | `frontend/src/components/ui/` |
| HeadlessUI 封装参考 | `frontend/src/components/ui/BetterSelect.tsx` |
| Radix 封装参考 | `frontend/src/components/ui/BetterSwitch.tsx` |

## 后端

| 职责 | 路径 |
|------|------|
| 启动与依赖注入 | `main.go` |
| 初始建表（新安装时的完整 schema） | `internal/migrations/init.go` → `InitSchema(...)` |
| Migration 列表 | `internal/migrations/migrations.go` |
| Services | `internal/service/*_service.go` |
| 工具函数 | `internal/utils/`（分 `apputils` / `archiveutils` / `downloadutils` / `imageutils` / `metadata` / `processutils` / `proxyutils` / `timerutils` 等） |
| 工具函数索引 | `docs/backend-utils.md` |
| Windows API 封装参考 | `internal/utils/processutils/process_windows.go` |
