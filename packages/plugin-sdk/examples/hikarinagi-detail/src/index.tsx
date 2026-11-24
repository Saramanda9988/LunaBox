import type { Context } from 'lunabox';
import { createHikarinagiApi } from './api';
import { createDetailAside } from './components/DetailAside';
import { createHeroSection } from './components/HeroSection';
import { createOverviewTab } from './components/OverviewTab';
import { createScreenshotsTab } from './components/ScreenshotsTab';
import { createCacheDB } from './db';
import { createDetailStore } from './store';

export function apply(ctx: Context) {
  ctx.config.define({
    title: 'Hikarinagi 详情页重构',
    description: '使用 Hikarinagi Galgame 详情接口重构游戏详情页，并借助独立 SQLite 缓存接口数据。',
    icon: 'image-search-outline',
    fields: [
      {
        key: 'cacheTtlHours',
        type: 'number',
        label: '缓存有效期（小时）',
        description: '超过该时长后打开详情页会自动在后台静默更新缓存。',
        default: 24,
        min: 1,
        max: 720,
        step: 1,
      },
      {
        key: 'autoSearch',
        type: 'boolean',
        label: '非 Hikarinagi 来源自动按名称匹配',
        description: '当游戏来自 VNDB / Bangumi / Steam 等其他元数据来源时，自动搜索并关联 Hikarinagi 条目。',
        default: true,
      },
      {
        key: 'preferredLogoLang',
        type: 'enum',
        label: '优先徽标语言',
        default: 'auto',
        options: [
          { value: 'auto', label: '跟随应用语言' },
          { value: 'zh', label: '优先中文徽标' },
          { value: 'ja', label: '优先日文徽标' },
          { value: 'en', label: '优先英文徽标' },
        ],
      },
      {
        key: 'heroHeight',
        type: 'number',
        label: '头图区域高度（px）',
        description: '控制顶部全出血头图横幅的最小高度，推荐 520–620px。',
        default: 560,
        min: 360,
        max: 760,
        step: 10,
      },
      {
        key: 'blurHeroBanner',
        type: 'boolean',
        label: '模糊头图背景',
        description: '默认清晰展示原始高清头图；开启后对所有头图施加高斯模糊。',
        default: false,
      },
      {
        key: 'accessToken',
        type: 'secret',
        label: '自定义 Hikarinagi Token（可选）',
        description: '留空时自动使用 LunaBox 已登录账号或应用内置的 Hikarinagi 访问令牌。',
        default: '',
      },
    ],
  });

  const db = createCacheDB(ctx);
  const api = createHikarinagiApi(ctx);
  const store = createDetailStore(ctx, db, api);

  // 1. 顶部全出血 Hero 头图（保留原生返回、封面与启动栏，并隐藏「游玩进度」Tab）
  ctx.ui.slot('game.detail.hero', {
    id: 'hikarinagi-hero',
    hideTabs: ['progress'],
    render: createHeroSection(ctx, store),
  });

  // 2. 独立「概览」Tab（order < 0，排在最前并作为默认展示页，承载简介与标签）
  ctx.ui.slot('game.detail.tab', {
    id: 'overview',
    title: '概览',
    icon: 'file-document-outline',
    order: -10,
    render: createOverviewTab(store),
  });

  // 3. 「游戏截图」Tab（排在「游戏统计」之后）
  ctx.ui.slot('game.detail.tab', {
    id: 'screenshots',
    title: '游戏截图',
    icon: 'image-multiple-outline',
    order: 1,
    render: createScreenshotsTab(store),
  });

  // 4. 右侧无框「游戏详情信息区」
  ctx.ui.slot('game.detail.aside', {
    id: 'detail-info',
    render: createDetailAside(ctx, store),
  });
}
