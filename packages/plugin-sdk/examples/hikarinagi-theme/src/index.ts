import type { Context, Disposer, ThemePaletteTokens } from 'lunabox';
import { defineThemePreset } from 'lunabox';
import { dark, light, neutral, type HinaRoles } from './hina';

/**
 * 把 Hina 的语义角色映射到 LunaBox 宿主色板。
 * 原则：宿主组件保持不变，只替换颜色——中性层级按色阶一一对应到 Hina 的纯灰，
 * “主操作 / 选中 / 当前状态”统一改用 Hina 的青色 accent（Hina：避免处处用 accent，其余保持中性）。
 */
function toPalette(r: HinaRoles, isDark: boolean): ThemePaletteTokens {
  return {
    // 文本
    text: r.fg,
    textStrong: r.fgMax,
    textMuted: r.fgSubtle,
    textFaint: r.fgDisabled,

    // 窗口骨架：canvas 做底，侧边栏、顶栏是 surface，以发丝线分隔
    appBg: r.canvas,
    topbarBg: r.surface,
    topbarBorder: r.border,
    controlIcon: r.fgMuted,
    controlHover: r.inset,

    // 侧边栏：当前页面是“当前状态”，用 accent-soft + accent-text
    sidebarBg: r.surface,
    sidebarBorder: r.border,
    navText: r.fgMuted,
    navHover: r.hover,
    navActiveBg: r.accentSoft,
    navActive: r.accentText,

    // 输入框：surface 底 + 强描边，聚焦用 focus ring 青色
    inputBg: r.surface,
    inputBorder: r.borderStrong,
    inputFocus: r.focusRing,
    inputIcon: r.fgSubtle,
    placeholder: r.fgSubtle,

    // 次要按钮：Hina 的中性 inset 填充
    toolBtnBg: r.inset,
    toolBtnBorder: isDark ? r.borderStrong : neutral[200],
    toolBtnHover: isDark ? '#303030' : neutral[200],
    toolBtnIcon: isDark ? neutral[300] : neutral[600],
    toolBtnText: isDark ? neutral[300] : neutral[700],
    toolBtnActiveBg: r.accentSoft,
    toolBtnActiveBorder: r.accentBorder,
    toolBtnActiveText: r.accentText,

    // 主按钮与开关：实心 accent，文字用 accent-on（深色）
    primaryBtn: r.accent,
    primaryBtnHover: r.accentHover,
    primaryBtnText: r.accentOn,

    // 强调色：选中、加载指示、链接；插件 UI（如 hikarinagi-detail）的 brand / accent 令牌也取它们
    accent: r.accent,
    accentText: r.accentText,
    accentSoft: r.accentSoft,
    accentSoftText: r.accentText,

    // 详情页标签栏（Hina Tabs）：中性文字，选中为 accent 文字 + accent 下划线
    tabText: r.fgMuted,
    tabHover: r.fg,
    tabActive: r.accentText,
    tabIndicator: r.accent,

    // 详情页工具栏图标按钮 = Hina IconButton（variant="soft" tone="neutral"）：中性填充 + 默认前景，
    // 悬停是前景色的状态层（浅色 6% / 深色 9%）预先合成；当前状态用 accent 外圈 + canvas 间隙（同 Hina 的焦点环）
    actionBtnBg: isDark ? r.inset : r.subtle,
    actionBtnHover: isDark ? '#383838' : '#e8e8e8',
    actionBtnIcon: r.fg,
    actionBtnIconHover: r.fgMax,
    actionBtnRing: r.accent,
    actionBtnRingOffset: r.canvas,

    // 卡片：surface + 发丝线，悬停加深描边
    cardBg: r.surface,
    cardBorder: r.border,
    cardBorderHover: r.borderStrong,
    coverBg: r.inset,

    scrollThumb: isDark ? neutral[700] : neutral[300],
    scrollThumbHover: isDark ? neutral[600] : neutral[400],

    // 面板、菜单
    panelBg: r.surface,
    panelBorder: r.border,
    divider: r.border,
    // 下拉菜单 = Hina DropdownMenu：surface 卡片，条目 text-fg，悬停是前景色状态层（合成到 surface），
    // 当前项不加底色、只在末尾显示对勾
    menuHover: isDark ? '#151515' : '#f1f1f1',
    menuText: r.fg,
    menuSelected: '#00000000',
    menuCheck: r.accentText,
    sectionLabel: isDark ? neutral[300] : neutral[700],

    // 筛选标签：选中态用 accent
    chipBg: r.subtle,
    chipText: r.fgSubtle,
    chipHover: isDark ? r.inset : r.subtle,
    chipActiveBg: r.accentSoft,
    chipActiveText: r.accentText,
    chipRing: isDark ? '#1f827b' : r.accentBorder,

    // 下拉、列表
    selectBg: isDark ? r.subtle : r.surface,
    selectBorder: r.borderStrong,
    selectText: r.fgMax,
    listFocus: isDark ? neutral[850] : r.subtle,
    listSelected: r.accentSoft,
    switchOff: isDark ? neutral[700] : neutral[300],

    // 提示气泡：Hina 的中性实心反色块
    tooltipBg: r.neutralSolid,
    tooltipText: r.neutralSolidOn,
    tooltipBorder: isDark ? neutral[200] : '#ffffff1a',
  };
}

interface HinaConfig {
  lockBackground?: boolean;
  showWordmark?: boolean;
  showIcon?: boolean;
}

export function apply(ctx: Context) {
  ctx.config.define({
    title: 'Hikarinagi 主题',
    description: '按 Hina UI 设计语言重新配色宿主界面：纯净中性灰、发丝线分隔与 Hikarinagi 青色强调色。',
    icon: 'palette-outline',
    fields: [
      {
        key: 'lockBackground',
        type: 'boolean',
        label: '锁定配色（停用自定义背景）',
        description: '开启后本插件启用期间不使用自定义背景图与玻璃模式，避免背景破坏 Hina 的配色层级；关闭后两者叠加，效果不作保证。',
        default: true,
      },
      {
        key: 'showWordmark',
        type: 'boolean',
        label: '显示 Hikarinagi 字标',
        description: '侧边栏与顶栏的 LunaBox 字标替换为 Hikarinagi 字标；关闭后不显示字标。',
        default: true,
      },
      {
        key: 'showIcon',
        type: 'boolean',
        label: '显示应用图标',
        description: '侧边栏字标前的月亮图标；Hikarinagi 字标自带识别度，默认隐藏。',
        default: false,
      },
    ],
  });

  let dispose: Disposer | null = null;
  const applyTheme = (cfg: HinaConfig) => {
    dispose?.();
    const lock = cfg.lockBackground !== false;
    dispose = ctx.theme.extend(
      defineThemePreset({
        id: 'hikarinagi-hina',
        light: toPalette(light, false),
        dark: toPalette(dark, true),
        // enabled: false 让宿主跳过用户的自定义背景；不锁定时不声明 glass，保持用户设置
        // Hina 的 --hn-radius-md / --hn-radius-lg 都是 8px：图标按钮是圆角方形，菜单面板同为 8px
        shape: { control: 8, panel: 8 },
        // 品牌标志：字标换成 Hikarinagi 字标（路径相对插件目录），图标与字标可在设置里分别隐藏
        brand: {
          wordmark: 'assets/hikarinagi-wordmark.svg',
          hideWordmark: cfg.showWordmark === false,
          hideIcon: cfg.showIcon !== true,
        },
        ...(lock ? { glass: { enabled: false } } : {}),
      }),
    );
  };

  // theme.extend 的注册随插件作用域回收，卸载时无需手动清理；配置变化时先撤掉旧的再注册
  applyTheme(ctx.config.get<HinaConfig>());
  ctx.config.onChange<HinaConfig>(applyTheme);
}
