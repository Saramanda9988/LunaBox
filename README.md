<div align="center">

# LunaBox

**轻量、快速、功能强大的视觉小说管理与游玩统计工具**

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v2-DF0000?style=flat-square)](https://wails.io/)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react)](https://react.dev/)

</div>


## ✨ 特性

- **游戏分类管理** - 自定义分类，灵活管理游戏库
- **游玩时长追踪** - 启动游戏自动追踪游玩时长
- **极小的包体积** - 基于 Wails 构建，无需携带完整浏览器内核
- **多维度统计** - 支持按日/周/月/年等多维度统计游玩数据，一键导出统计卡片分享保存
- **AI 分析** - AI 分析游玩数据，生成个性化趣味报告
- **便捷的数据导入** - 支持从 PotatoVN, Playnite中导入数据，支持选择文件夹批量导入游戏
- **多渠道备份** - 支持本地备份, AWS S3、七牛云、阿里云 OSS 等兼容 S3 协议的存储服务与 OneDrive 云端备份
- **隐私与安全** - 所有敏感数据均保存在本地中

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

前往 [Releases](../../releases) 页面下载最新版本的安装包。

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
```


## 🚀 快速开始

1. **添加游戏** - 点击添加按钮，选择游戏可执行文件或从其他平台导入
2. **管理分类** - 创建自定义分类，将游戏归类整理
3. **开始游玩** - 点击游戏卡片上的启动按钮，自动追踪游玩时长
4. **查看统计** - 在统计页面查看你的游玩数据和图表
5. **AI 分析** - 使用 AI 功能生成个性化游玩报告
6. **导出分享** - 导出统计卡片，与朋友分享你的游戏历程


## ⚙️ 配置

### AI 配置

在设置页面配置 AI 服务：

| 配置项 | 说明 |
|--------|------|
| AI Provider | AI 服务提供商 (如 deepseek) |
| Base URL | API 基础地址 |
| API Key | API 密钥 |
| Model | 模型名称 |

### 云备份配置

#### S3 兼容存储

| 配置项 | 说明 |
|--------|------|
| Endpoint | S3 服务端点地址 |
| Region | 区域 |
| Bucket | 存储桶名称 |
| Access Key | 访问密钥 |
| Secret Key | 秘密密钥 |

#### OneDrive

在设置页面登录 Microsoft 账号并授权即可。


## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📁 项目结构

```
lunabox/
├── main.go              # 应用入口
├── wails.json           # Wails 配置
├── frontend/            # React 前端
│   ├── public/          # 静态资源
│   ├── src/
│   │   ├── components/  # 组件
│   │   ├── routes/      # 页面路由
│   │   ├── hooks/       # 自定义 Hooks
│   │   └── utils/       # 工具函数
│   └── wailsjs/         # Wails 生成的绑定
├── internal/            # Go 内部包
│   ├── appconf/         # 应用配置
│   ├── enums/           # 枚举定义
│   ├── models/          # 数据模型
│   ├── service/         # 业务服务层
│   ├── utils/           # 工具类
│   └── vo/              # 视图对象
└── build/               # 构建输出
```

## RoadMap

- [ ] 支持从ReinaManager中导入数据

- [ ] 支持自定义背景图片

- [ ] 更漂亮的默认首页，首页自定义

- [ ] 支持 i18n

- [ ] 自部署 docker 服务端

- [ ] im 平台机器人插件

- [ ] 更多的统计导出模板

- [ ] 更丰富的ai prompt预设

## 😀 从开源到开源

灵感来源:

- [PotatoVN](https://github.com/GoldenPotato137/PotatoVN) - Galgame 管理工具
- [ReinaManager](https://github.com/huoshen80/ReinaManager) - 一款轻量化的galgame和视觉小说管理工具
- [Playnite](https://github.com/JosefNemec/Playnite) - an open source video game library manager with one simple goal: To provide a unified interface for all of your games.

## 🙏 感谢

游戏数据搜索api提供:

- [Bangumi](https://github.com/bangumi) - Bangumi番组计划
- [VNDB](https://vndb.org/) - The Visual Novel Database
- [月幕gal](https://www.ymgal.games/) - 请感受这绝妙的文艺体裁

## 📄 开源协议

本项目采用 [AGPL v3](LICENSE) 协议开源。
