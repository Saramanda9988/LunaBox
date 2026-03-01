<div align="center">

<img src="frontend/public/appicon.png" alt="LunaBox Logo" style="width:120px; height:120px; border-radius:16px;" />

# LunaBox

**軽量・高速・高機能なビジュアルノベル管理＆プレイ時間統計ツール**

[中文](README.zh-CN.md) | [English](README.md) | [日本語](README.ja.md)

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v2-DF0000?style=flat-square)](https://wails.io/)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react)](https://react.dev/)

</div>

## ✨ 主な機能

- **ゲームカテゴリ管理** - カスタムカテゴリでライブラリを柔軟に整理
- **プレイ時間トラッキング** - ゲーム起動時にプレイ時間を自動記録
- **軽量なバイナリサイズ** - Wails ベースで、ブラウザランタイム同梱が不要
- **多次元統計** - 日/週/月/年などでプレイデータを集計し、統計カードをワンクリックで出力
- **AI 分析** - プレイデータに基づくパーソナライズされたレポートを生成
- **便利なデータインポート** - PotatoVN、Playnite、Vnite からの取り込み、フォルダ一括/ドラッグ＆ドロップに対応
- **複数チャネルのバックアップ** - ローカル、AWS S3、七牛云、阿里云 OSS（S3 互換）、OneDrive に対応
- **プライバシーとセキュリティ** - 機密データはすべてローカルに保存

## スクリーンショット

<details>
<summary>カスタム背景スタイルをさらに表示</summary>

![ホーム](screenshot/home-img.png)

![ライブラリ](screenshot/lib-img.png)

![ゲーム詳細](screenshot/game-img.png)

</details>

<details>
<summary>統計エクスポート用ポスターテンプレートを表示</summary>

![ミニマル](screenshot/lunabox-stats-20260124-175553.png)

![フューチャーレトロ](screenshot/lunabox-stats-20260124-175602.png)

![手帳風](screenshot/lunabox-stats-20260124-175617.png)

</details>

アプリ内スクリーンショット（リポジトリ内の `screenshot/` ディレクトリ）：

![ホーム](screenshot/home.png)

![ライブラリ](screenshot/lib.png)

![ゲーム詳細](screenshot/game.png)

## 🛠️ 技術スタック

| レイヤー | 技術 |
|------|------|
| **フレームワーク** | [Wails v2](https://wails.io/) |
| **バックエンド** | [Go 1.24](https://go.dev/) |
| **フロントエンド** | [React 18](https://react.dev/) + [TypeScript](https://www.typescriptlang.org/) |
| **データベース** | [DuckDB](https://duckdb.org/) |
| **ビルドツール** | [Vite](https://vitejs.dev/) |
| **スタイル** | [UnoCSS](https://unocss.dev/) |
| **ルーティング** | [TanStack Router](https://tanstack.com/router) |
| **状態管理** | [Zustand](https://zustand-demo.pmnd.rs/) |
| **チャート** | [Chart.js](https://www.chartjs.org/) + [react-chartjs-2](https://react-chartjs-2.js.org/) |

## 📦 インストール

### Release からダウンロード

[Releases](https://github.com/Saramanda9988/LunaBox/releases) から最新版インストーラーをダウンロードしてください。

### ソースからビルド

#### 前提環境

- [Go 1.24+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [pnpm](https://pnpm.io/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- [msys2](https://www.msys2.org/)

```bash
# Wails CLI をインストール
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

#### ビルド手順

```bash
# リポジトリをクローン
git clone https://github.com/Saramanda9988/lunabox.git
cd lunabox

# フロントエンド依存関係をインストール
cd frontend && pnpm install && cd ..

# 開発モードで実行
wails dev

# 本番ビルド
wails build

# スクリプトでローカルビルド（Windows）
.\scripts\build.bat all 1.0.0-beta
```

## 🤝 コントリビューション

Issue と Pull Request を歓迎します。

## 🗺️ Roadmap

- [x] 自動更新チェックと通知
- [x] ログシステムの改善
- [ ] ReinaManager からのデータ取り込み対応
- [x] カスタム背景画像対応
- [x] より美しいデフォルトホームとホーム画面カスタマイズ
- [x] i18n 対応
- [ ] セルフホスト Docker サーバー
- [ ] IM プラットフォーム向け Bot プラグイン
- [ ] より良い AI システム：長期記憶 + ペルソナ + 外部検索
- [x] 統計エクスポートテンプレートの追加
- [x] Locale Emulator など起動オプション対応のゲーム起動

## 😀 オープンソースから学び、オープンソースへ

インスピレーション元：

- [PotatoVN](https://github.com/GoldenPotato137/PotatoVN) - Galgame 管理ツール
- [ReinaManager](https://github.com/huoshen80/ReinaManager) - 軽量な Galgame / ビジュアルノベル管理ツール
- [Playnite](https://github.com/JosefNemec/Playnite) - すべてのゲームを一元管理することを目的としたオープンソースのゲームライブラリマネージャー
- [Vnite](https://github.com/ximu3/vnite) - ゲームコレクション整理、プレイ追跡、デバイス間リアルタイム同期、詳細レポートを提供する統合プラットフォーム

## 🙏 謝辞

ゲームメタデータ API 提供：

- [Bangumi](https://github.com/bangumi) - Bangumi 番組計画
- [VNDB](https://vndb.org/) - The Visual Novel Database
- [月幕gal](https://www.ymgal.games/) - ビジュアルノベル向けメタデータプラットフォーム

解凍機能提供：

- [7-Zip](https://www.7-zip.org/) - 無料・オープンソースのアーカイブユーティリティ

## 📄 ライセンス

本プロジェクトは [AGPL v3](LICENSE) ライセンスで公開されています。
