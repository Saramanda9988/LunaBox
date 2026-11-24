// hikarinagi-theme/src/index.ts
import { defineThemePreset } from "lunabox";

// hikarinagi-theme/src/hina.ts
var brand = {
  50: "#eefdfb",
  100: "#d5faf5",
  200: "#a9f3eb",
  300: "#72e8dc",
  400: "#45d4c9",
  500: "#39c5bb",
  600: "#2ba79e",
  700: "#1f827b",
  800: "#1a6862",
  900: "#185450",
  950: "#0a302e"
};
var neutral = {
  0: "#ffffff",
  50: "#fafafa",
  100: "#f5f5f5",
  150: "#ededed",
  200: "#e5e5e5",
  300: "#d4d4d4",
  400: "#a3a3a3",
  500: "#737373",
  600: "#525252",
  700: "#404040",
  800: "#262626",
  850: "#1c1c1c",
  900: "#171717",
  950: "#0a0a0a",
  1e3: "#000000"
};
var light = {
  canvas: neutral[50],
  surface: neutral[0],
  subtle: neutral[100],
  inset: neutral[150],
  border: "#ebebeb",
  // #00000014 合成到白色
  borderStrong: "#d9d9d9",
  // #00000026 合成到白色
  fgMax: neutral[1e3],
  fg: neutral[900],
  fgMuted: neutral[600],
  fgSubtle: neutral[500],
  fgDisabled: neutral[400],
  hover: neutral[100],
  accent: brand[500],
  accentHover: "#36bab0",
  // accent-on 6% 叠加
  accentOn: neutral[950],
  accentText: brand[700],
  accentSoft: "#e7f8f7",
  // color-mix(brand-500 12%, surface)
  accentBorder: "#bcebe8",
  // #39c5bb57 合成到白色
  focusRing: brand[600],
  neutralSolid: neutral[900],
  neutralSolidOn: neutral[0]
};
var dark = {
  canvas: neutral[950],
  surface: neutral[1e3],
  subtle: "#141414",
  inset: neutral[800],
  border: "#1a1a1a",
  // #ffffff1a 合成到黑色
  borderStrong: "#2e2e2e",
  // #ffffff2e 合成到黑色
  fgMax: neutral[0],
  fg: neutral[150],
  fgMuted: neutral[400],
  fgSubtle: neutral[500],
  fgDisabled: neutral[600],
  hover: "#141414",
  accent: brand[400],
  accentHover: "#3fc1b7",
  // accent-on 9% 叠加
  accentOn: neutral[1e3],
  accentText: brand[400],
  accentSoft: "#113330",
  // color-mix(brand-400 24%, surface)
  accentBorder: "#15403c",
  // #45d4c94d 合成到黑色
  focusRing: brand[400],
  neutralSolid: neutral[150],
  neutralSolidOn: neutral[950]
};

// hikarinagi-theme/src/index.ts
function toPalette(r, isDark) {
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
    toolBtnHover: isDark ? "#303030" : neutral[200],
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
    actionBtnHover: isDark ? "#383838" : "#e8e8e8",
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
    menuHover: isDark ? "#151515" : "#f1f1f1",
    menuText: r.fg,
    menuSelected: "#00000000",
    menuCheck: r.accentText,
    sectionLabel: isDark ? neutral[300] : neutral[700],
    // 筛选标签：选中态用 accent
    chipBg: r.subtle,
    chipText: r.fgSubtle,
    chipHover: isDark ? r.inset : r.subtle,
    chipActiveBg: r.accentSoft,
    chipActiveText: r.accentText,
    chipRing: isDark ? "#1f827b" : r.accentBorder,
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
    tooltipBorder: isDark ? neutral[200] : "#ffffff1a"
  };
}
function apply(ctx) {
  ctx.config.define({
    title: "Hikarinagi \u4E3B\u9898",
    description: "\u6309 Hina UI \u8BBE\u8BA1\u8BED\u8A00\u91CD\u65B0\u914D\u8272\u5BBF\u4E3B\u754C\u9762\uFF1A\u7EAF\u51C0\u4E2D\u6027\u7070\u3001\u53D1\u4E1D\u7EBF\u5206\u9694\u4E0E Hikarinagi \u9752\u8272\u5F3A\u8C03\u8272\u3002",
    icon: "palette-outline",
    fields: [
      {
        key: "lockBackground",
        type: "boolean",
        label: "\u9501\u5B9A\u914D\u8272\uFF08\u505C\u7528\u81EA\u5B9A\u4E49\u80CC\u666F\uFF09",
        description: "\u5F00\u542F\u540E\u672C\u63D2\u4EF6\u542F\u7528\u671F\u95F4\u4E0D\u4F7F\u7528\u81EA\u5B9A\u4E49\u80CC\u666F\u56FE\u4E0E\u73BB\u7483\u6A21\u5F0F\uFF0C\u907F\u514D\u80CC\u666F\u7834\u574F Hina \u7684\u914D\u8272\u5C42\u7EA7\uFF1B\u5173\u95ED\u540E\u4E24\u8005\u53E0\u52A0\uFF0C\u6548\u679C\u4E0D\u4F5C\u4FDD\u8BC1\u3002",
        default: true
      }
    ]
  });
  let dispose = null;
  const applyTheme = (cfg) => {
    dispose?.();
    const lock = cfg.lockBackground !== false;
    dispose = ctx.theme.extend(
      defineThemePreset({
        id: "hikarinagi-hina",
        light: toPalette(light, false),
        dark: toPalette(dark, true),
        // enabled: false 让宿主跳过用户的自定义背景；不锁定时不声明 glass，保持用户设置
        // Hina 的 --hn-radius-md / --hn-radius-lg 都是 8px：图标按钮是圆角方形，菜单面板同为 8px
        shape: { control: 8, panel: 8 },
        ...lock ? { glass: { enabled: false } } : {}
      })
    );
  };
  applyTheme(ctx.config.get());
  ctx.config.onChange(applyTheme);
}
export {
  apply
};
