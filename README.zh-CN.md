<div align="center">

<img src="frontend/public/appicon.png" alt="LunaBox Logo" style="width:120px; height:120px; border-radius:16px;" />

# LunaBox

**轻量、快速、功能丰富的视觉小说管理与游玩统计工具**

[中文](README.zh-CN.md) | [English](README.md) | [日本語](README.ja.md)

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v2-DF0000?style=flat-square)](https://wails.io/)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react)](https://react.dev/)

</div>


## ✨ 特性

- **游戏分类管理** - 自定义分类，灵活管理游戏库
- **游玩时长追踪** - 启动游戏自动追踪游玩时长
- **较小的包体积** - 基于 Wails 构建，无需携带完整浏览器内核
- **多维度统计** - 支持按日/周/月/年等多维度统计游玩数据，一键导出统计卡片分享保存
- **AI 分析** - AI 分析游玩数据，生成个性化趣味报告
- **便捷的数据导入** - 支持从 PotatoVN, Playnite，Vnite中导入数据，支持选择文件夹批量导入/拖动导入游戏
- **多渠道备份** - 支持本地备份, AWS S3、七牛云、阿里云 OSS 等兼容 S3 协议的存储服务与 OneDrive 云端备份
- **隐私与安全** - 所有敏感数据均保存在本地中

## 截图

<details>
<summary>点击展开更多自定义背景样式</summary>

![主界面](screenshot/home-img.png)

![库视图](screenshot/lib-img.png)

![游戏详情](screenshot/game-img.png)

</details>

<details>
<summary>点击查看统计导出海报模板</summary>

![简约](screenshot/lunabox-stats-20260124-175553.png)

![未来复古](screenshot/lunabox-stats-20260124-175602.png)

![手账风](screenshot/lunabox-stats-20260124-175617.png)

</details>

应用中的部分截图（位于仓库的 `screenshot/` 目录）：

![主界面](screenshot/home.png)

![库视图](screenshot/lib.png)

![游戏详情](screenshot/game.png)

## 🛠️ 技术栈

| 层级 | 技术 |
|------|------|
| **框架** | [Wails v2](https://wails.io/) |
| **后端** | [Go 1.24](https://go.dev/) |
| **前端** | [React 18](https://react.dev/) + [TypeScript](https://www.typescriptlang.org/) |
| **数据库** | [DuckDB](https://duckdb.org/) |
| **构建工具** | [Vite](https://vitejs.dev/) |
| **样式** | [UnoCSS](https://unocss.dev/) |
| **路由** | [TanStack Router](https://tanstack.com/router) |
| **状态管理** | [Zustand](https://zustand-demo.pmnd.rs/) |
| **图表** | [Chart.js](https://www.chartjs.org/) + [react-chartjs-2](https://react-chartjs-2.js.org/) |


## 📦 安装

### 从 Release 下载

前往 [Releases](https://github.com/Saramanda9988/LunaBox/releases) 页面下载最新版本的安装包。

### 从源码构建

#### 前置要求

- [Go 1.24+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [pnpm](https://pnpm.io/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- [msys2](https://www.msys2.org/)

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

#### 构建步骤

```bash
# 克隆项目
git clone https://github.com/Saramanda9988/lunabox.git
cd lunabox

# 安装前端依赖
cd frontend && pnpm install && cd ..

# 开发模式运行
wails dev

# 构建生产版本
wails build

# 使用脚本进行本地构建版本(windows环境)
.\scripts\build.bat all 1.0.0-beta   
```

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 🗺️ RoadMap

- [x] 自动更新检查与提示

- [x] 完善日志系统

- [ ] 支持从ReinaManager中导入数据

- [x] 支持自定义背景图片

- [x] 更漂亮的默认首页，首页自定义

- [x] 支持 i18n

- [ ] 自部署 docker 服务端

- [ ] im 平台机器人插件

- [ ] 更好的ai系统，长期记忆 + 人格面具 + 外部搜素

- [x] 更多的统计导出模板

- [x] 支持locale emulator等启动参数的游戏启动

## 😀 从开源到开源

灵感来源:

- [PotatoVN](https://github.com/GoldenPotato137/PotatoVN) - Galgame 管理工具
- [ReinaManager](https://github.com/huoshen80/ReinaManager) - 一款轻量化的galgame和视觉小说管理工具
- [Playnite](https://github.com/JosefNemec/Playnite) - an open source video game library manager with one simple goal: To provide a unified interface for all of your games.
- [Vnite](https://github.com/ximu3/vnite) - A unified platform to organize your game collection, track gameplay, with real-time cloud sync across devices and detailed gameplay reports.

## 🙏 感谢

游戏数据搜索api提供:

- [Bangumi](https://github.com/bangumi) - Bangumi番组计划
- [VNDB](https://vndb.org/) - The Visual Novel Database
- [月幕gal](https://www.ymgal.games/) - 请感受这绝妙的文艺体裁

解压功能提供:

- [7-Zip](https://www.7-zip.org/) - A free and open-source file archiver, a utility used to place groups of files within compressed containers known as "archives".

## 📄 开源协议

本项目采用 [AGPL v3](LICENSE) 协议开源。
