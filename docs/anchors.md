# 关键文件落点（Anchors）

> 除特别标注外，本文路径均相对桌面应用目录 `apps/desktop/`。

## 界面

界面的约束见 [frontend.md](frontend.md)。

| 职责                                                   | 路径                                                     |
| ------------------------------------------------------ | -------------------------------------------------------- |
| 命令行参数与语言                                       | `main.go`                                                |
| 应用外壳（单实例、数据目录与日志、主窗口、托盘）       | `app/`                                                   |
| 把后端接到 MyGo（`appruntime.Runtime`、事件、开发数据） | `host/`                                                  |
| 界面调用的后端接口                                     | `api/`                                                   |
| 根布局                                                 | `ui/app.go`                                              |
| 路由表                                                 | `ui/routes.go`                                           |
| 应用级运行时事件、退出前同步提示                       | `ui/runtime_events.go`                                   |
| 页面                                                   | `ui/pages/<页面>/`                                        |
| 详情页标签页与设置页分区                               | `ui/components/panel/<面板>/`                             |
| 可复用组件（Modal、Toaster 等）与 Better 系列          | `ui/components/`、`ui/components/better/`                |
| 页面共享环境（后端、路由、主题、语言）                 | `ui/env/`                                                |
| 色板、字号、过渡曲线与玻璃模式                         | `ui/theme/`                                              |
| 界面文字（ARB，直接编辑）与生成的函数                  | `ui/locales/*.json`、`ui/locales/catalog_gen.go`（勿手改） |
| 时长与日期格式                                         | `ui/timefmt/`                                            |
| 命令行工具                                             | `cmd/lunacli/`                                           |
| 发布包构建                                             | `tools/release/`                                         |
| 安装包脚本（NSIS、deb/rpm 安装脚本）                   | `packaging/`                                             |

## 后端

| 职责                              | 路径                                                                                                                                                |
| --------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| 启动、退出生命周期与依赖注入      | `packages/backend/bootstrap/`（桌面端外壳在 `app/`）                                                                                                                                         |
| 初始建表（新安装时的完整 schema） | `packages/backend/migrations/init.go` → `InitSchema(...)`                                                                                                   |
| Migration 列表                    | `packages/backend/migrations/migrations.go`                                                                                                                 |
| Services                          | `packages/backend/service/*_service.go`                                                                                                                     |
| 游戏启动策略                      | `packages/backend/service/launcher/*.go`                                                                                                                    |
| 配置读写与运行时配置回写          | `packages/backend/service/config_service.go`                                                                                                                |
| 工具函数                          | `packages/backend/utils/`（分 `apputils` / `archiveutils` / `downloadutils` / `imageutils` / `metadata` / `processutils` / `proxyutils` / `timerutils` 等） |
| 工具函数索引                      | `docs/backend-utils.md`                                                                                                                             |
| Windows API 封装参考              | `packages/backend/utils/processutils/process_windows.go`                                                                                                    |
| 客户端更新协调                    | `packages/backend/updateclient/`                                                                                                                            |
| 独立更新器与更新资产生成器        | 仓库根 `packages/updater/`                                                                                                                                 |
| Cloudflare 更新服务               | 仓库根 `apps/update-server/`                                                                                                                                 |
